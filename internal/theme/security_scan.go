package theme

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/safepath"
)

type themeSecurityReference struct {
	Kind string
	URL  string
	Path string
	Line int
}

var (
	htmlThemeTagPattern   = regexp.MustCompile(`(?is)<(script|link|img|video|audio|source)\b[^>]*>`)
	htmlStyleBlockPattern = regexp.MustCompile(`(?is)<style\b[^>]*>(.*?)</style\s*>`)
	cssImportPattern      = regexp.MustCompile(`(?is)@import\s+(?:url\(\s*)?["']?((?:https?|wss?)://[^\s"')]+)`)
	cssURLPattern         = regexp.MustCompile(`(?is)url\(\s*["']?((?:https?|wss?)://[^\s"')]+)`)
	jsFetchPattern        = regexp.MustCompile("(?is)\\bfetch\\s*\\(\\s*[\\\"'`]((?:https?|wss?)://[^\\\"'`]+)[\\\"'`]")
	jsWebSocketPattern    = regexp.MustCompile("(?is)\\b(?:new\\s+)?(?:WebSocket|EventSource)\\s*\\(\\s*[\\\"'`]((?:https?|wss?)://[^\\\"'`]+)[\\\"'`]")
	jsXHROpenPattern      = regexp.MustCompile("(?is)\\.(?:open)\\s*\\(\\s*[\\\"'`][A-Z]+[\\\"'`]\\s*,\\s*[\\\"'`]((?:https?|wss?)://[^\\\"'`]+)[\\\"'`]")
	jsAxiosPattern        = regexp.MustCompile("(?is)\\baxios\\.(?:get|post|put|patch|delete|request)\\s*\\(\\s*[\\\"'`]((?:https?|wss?)://[^\\\"'`]+)[\\\"'`]")
)

func scanThemeSecurityReferences(root string) ([]themeSecurityReference, error) {
	references := []themeSecurityReference{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := safepath.EnsureNoSymlinkEscape(root, path); err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".css" && ext != ".js" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		references = append(references, scanThemeSecurityFile(filepath.ToSlash(path), ext, body)...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(references, func(i, j int) bool {
		if references[i].Path != references[j].Path {
			return references[i].Path < references[j].Path
		}
		if references[i].Line != references[j].Line {
			return references[i].Line < references[j].Line
		}
		if references[i].Kind != references[j].Kind {
			return references[i].Kind < references[j].Kind
		}
		return references[i].URL < references[j].URL
	})
	return references, nil
}

func scanThemeSecurityFile(path, ext string, body []byte) []themeSecurityReference {
	var references []themeSecurityReference
	switch ext {
	case ".html":
		masked := maskHTMLComments(body)
		maskedTags := maskHTMLRawText(masked)
		for _, match := range htmlThemeTagPattern.FindAllSubmatchIndex(maskedTags, -1) {
			if len(match) < 4 || match[2] < 0 || match[3] < 0 {
				continue
			}
			tag := string(body[match[0]:match[1]])
			tagName := strings.ToLower(string(maskedTags[match[2]:match[3]]))
			references = append(references, htmlTagReferences(path, body, match[0], tagName, tag)...)
		}
		for _, match := range htmlStyleBlockPattern.FindAllSubmatchIndex(masked, -1) {
			if len(match) < 4 || match[2] < 0 || match[3] < 0 {
				continue
			}
			references = append(references, cssReferences(path, body, match[2], body[match[2]:match[3]])...)
		}
	case ".css":
		references = append(references, cssReferences(path, body, 0, body)...)
	case ".js":
		references = append(references, jsReferences(path, body)...)
	}
	return deduplicateThemeSecurityReferences(references)
}

func htmlTagReferences(path string, body []byte, offset int, tagName, tag string) []themeSecurityReference {
	kind := ""
	var values []string
	switch strings.ToLower(tagName) {
	case "script":
		kind = "script"
		values = append(values, htmlAttribute(tag, "src"))
	case "link":
		if !strings.Contains(strings.ToLower(htmlAttribute(tag, "rel")), "stylesheet") {
			return nil
		}
		kind = "style"
		values = append(values, htmlAttribute(tag, "href"))
	case "img":
		kind = "image"
		values = append(values, htmlAttribute(tag, "src"), htmlAttribute(tag, "srcset"))
	case "video", "audio", "source":
		kind = "media"
		values = append(values, htmlAttribute(tag, "src"), htmlAttribute(tag, "srcset"))
	default:
		return nil
	}

	var references []themeSecurityReference
	for _, value := range values {
		for _, raw := range splitHTMLURLs(value) {
			if isRemoteAssetURL(raw) {
				references = append(references, themeSecurityReference{Kind: kind, URL: raw, Path: path, Line: lineNumber(body, offset)})
			}
		}
	}
	return references
}

func htmlAttribute(tag, name string) string {
	name = regexp.QuoteMeta(name)
	for _, pattern := range []string{
		`(?is)\b` + name + `\s*=\s*"([^"]+)"`,
		`(?is)\b` + name + `\s*=\s*'([^']+)'`,
	} {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(tag); len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func splitHTMLURLs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if fields := strings.Fields(part); len(fields) > 0 {
			out = append(out, strings.Trim(fields[0], `"'`))
		}
	}
	return out
}

func cssReferences(path string, body []byte, offset int, source []byte) []themeSecurityReference {
	masked := maskCSSComments(source)
	references := []themeSecurityReference{}
	importRanges := [][2]int{}
	for _, match := range cssImportPattern.FindAllSubmatchIndex(masked, -1) {
		if len(match) < 4 || match[2] < 0 || match[3] < 0 {
			continue
		}
		raw := string(source[match[2]:match[3]])
		if isRemoteAssetURL(raw) {
			importRanges = append(importRanges, [2]int{match[0], match[1]})
			references = append(references, themeSecurityReference{Kind: "style", URL: raw, Path: path, Line: lineNumber(body, offset+match[0])})
		}
	}
	for _, match := range cssURLPattern.FindAllSubmatchIndex(masked, -1) {
		if len(match) < 4 || match[2] < 0 || match[3] < 0 {
			continue
		}
		raw := string(source[match[2]:match[3]])
		if !isRemoteAssetURL(raw) {
			continue
		}
		inImport := false
		for _, importRange := range importRanges {
			if match[0] >= importRange[0] && match[0] < importRange[1] {
				inImport = true
				break
			}
		}
		if inImport {
			continue
		}
		kind := cssURLKind(raw)
		references = append(references, themeSecurityReference{Kind: kind, URL: raw, Path: path, Line: lineNumber(body, offset+match[0])})
	}
	return references
}

func jsReferences(path string, body []byte) []themeSecurityReference {
	masked := maskJSComments(body)
	patterns := []*regexp.Regexp{jsFetchPattern, jsWebSocketPattern, jsXHROpenPattern, jsAxiosPattern}
	references := []themeSecurityReference{}
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllSubmatchIndex(masked, -1) {
			if len(match) < 4 || match[2] < 0 || match[3] < 0 {
				continue
			}
			if !jsCodeAtOffset(body, match[0]) {
				continue
			}
			raw := string(body[match[2]:match[3]])
			if !isRemoteThemeURL(raw) {
				continue
			}
			if _, ok := seen[raw]; ok {
				continue
			}
			seen[raw] = struct{}{}
			references = append(references, themeSecurityReference{Kind: "request", URL: raw, Path: path, Line: lineNumber(body, match[0])})
		}
	}
	return references
}

func jsCodeAtOffset(body []byte, offset int) bool {
	if offset < 0 || offset > len(body) {
		return false
	}
	state := byte(0)
	for i := 0; i < offset; i++ {
		if state == 1 || state == 2 || state == 3 {
			if body[i] == '\\' {
				i++
				continue
			}
			if (state == 1 && body[i] == '\'') || (state == 2 && body[i] == '"') || (state == 3 && body[i] == '`') {
				state = 0
			}
			continue
		}
		if body[i] == '\'' {
			state = 1
			continue
		}
		if body[i] == '"' {
			state = 2
			continue
		}
		if body[i] == '`' {
			state = 3
			continue
		}
		if body[i] == '/' && i+1 < offset && body[i+1] == '/' {
			state = 4
			i++
			continue
		}
		if body[i] == '/' && i+1 < offset && body[i+1] == '*' {
			state = 5
			i++
			continue
		}
		if state == 4 && body[i] == '\n' {
			state = 0
		}
		if state == 5 && body[i] == '*' && i+1 < offset && body[i+1] == '/' {
			state = 0
			i++
		}
	}
	return state == 0
}

func cssURLKind(raw string) string {
	ext := strings.ToLower(filepath.Ext(strings.Split(raw, "?")[0]))
	switch ext {
	case ".woff", ".woff2", ".ttf", ".otf", ".eot":
		return "font"
	case ".mp3", ".wav", ".m4a", ".ogg", ".mp4", ".webm", ".mov", ".avi":
		return "media"
	default:
		return "image"
	}
}

func isRemoteThemeURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "ws" || u.Scheme == "wss")
}

func isRemoteAssetURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func lineNumber(body []byte, offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(body) {
		offset = len(body)
	}
	return bytes.Count(body[:offset], []byte("\n")) + 1
}

func deduplicateThemeSecurityReferences(items []themeSecurityReference) []themeSecurityReference {
	seen := make(map[string]struct{}, len(items))
	out := make([]themeSecurityReference, 0, len(items))
	for _, item := range items {
		key := fmt.Sprintf("%s|%s|%s|%d", item.Path, item.Kind, item.URL, item.Line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func maskHTMLComments(body []byte) []byte {
	out := append([]byte(nil), body...)
	for search := 0; search < len(out); {
		relStart := bytes.Index(out[search:], []byte("<!--"))
		if relStart < 0 {
			break
		}
		start := search + relStart
		relEnd := bytes.Index(out[start+4:], []byte("-->"))
		end := len(out)
		if relEnd >= 0 {
			end = start + 4 + relEnd + 3
		}
		maskBytesPreserveNewlines(out[start:end])
		if relEnd < 0 {
			break
		}
		search = end
	}
	return out
}

var htmlRawTextOpenPattern = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>`)

func maskHTMLRawText(body []byte) []byte {
	out := append([]byte(nil), body...)
	for search := 0; search < len(out); {
		match := htmlRawTextOpenPattern.FindSubmatchIndex(out[search:])
		if len(match) < 4 {
			break
		}
		openStart := search + match[0]
		contentStart := search + match[1]
		tagName := string(out[search+match[2] : search+match[3]])
		closePattern := regexp.MustCompile(`(?is)</` + regexp.QuoteMeta(tagName) + `\s*>`)
		closeMatch := closePattern.FindIndex(out[contentStart:])
		if closeMatch == nil {
			maskBytesPreserveNewlines(out[contentStart:])
			break
		}
		contentEnd := contentStart + closeMatch[0]
		maskBytesPreserveNewlines(out[contentStart:contentEnd])
		search = contentStart + closeMatch[1]
		if search <= openStart {
			search = openStart + 1
		}
	}
	return out
}

func maskCSSComments(body []byte) []byte {
	return maskDelimitedComments(body, []byte("/*"), []byte("*/"))
}

func maskDelimitedComments(body, startToken, endToken []byte) []byte {
	out := append([]byte(nil), body...)
	for start := bytes.Index(out, startToken); start >= 0; {
		relEnd := bytes.Index(out[start+len(startToken):], endToken)
		end := len(out)
		if relEnd >= 0 {
			end = start + len(startToken) + relEnd + len(endToken)
		}
		maskBytesPreserveNewlines(out[start:end])
		if relEnd < 0 {
			break
		}
		nextRel := bytes.Index(out[end:], startToken)
		if nextRel < 0 {
			break
		}
		start = end + nextRel
	}
	return out
}

func maskJSComments(body []byte) []byte {
	out := append([]byte(nil), body...)
	state := byte(0)
	for i := 0; i < len(out); i++ {
		if state == 1 || state == 2 || state == 3 {
			if out[i] == '\\' {
				i++
				continue
			}
			if (state == 1 && out[i] == '\'') || (state == 2 && out[i] == '"') || (state == 3 && out[i] == '`') {
				state = 0
			}
			continue
		}
		if out[i] == '\'' {
			state = 1
			continue
		}
		if out[i] == '"' {
			state = 2
			continue
		}
		if out[i] == '`' {
			state = 3
			continue
		}
		if out[i] == '/' && i+1 < len(out) && out[i+1] == '/' {
			start := i
			i += 2
			for i < len(out) && out[i] != '\n' {
				i++
			}
			maskBytesPreserveNewlines(out[start:i])
			i--
			continue
		}
		if out[i] == '/' && i+1 < len(out) && out[i+1] == '*' {
			start := i
			i += 2
			for i+1 < len(out) && !(out[i] == '*' && out[i+1] == '/') {
				i++
			}
			if i+1 < len(out) {
				i += 2
			}
			maskBytesPreserveNewlines(out[start:i])
			i--
		}
	}
	return out
}

func maskBytesPreserveNewlines(body []byte) {
	for i, value := range body {
		if value != '\n' && value != '\r' {
			body[i] = ' '
		}
	}
}

func securityReferenceField(kind string) string {
	switch kind {
	case "script":
		return "security.external_assets.scripts"
	case "style":
		return "security.external_assets.styles"
	case "font":
		return "security.external_assets.fonts"
	case "image":
		return "security.external_assets.images"
	case "media":
		return "security.external_assets.media"
	case "request":
		return "security.frontend_requests.origins"
	default:
		return "security"
	}
}

func securityReferenceCategory(kind string) string {
	switch kind {
	case "script":
		return "scripts"
	case "style":
		return "styles"
	case "font":
		return "fonts"
	case "image":
		return "images"
	case "media":
		return "media"
	case "request":
		return "frontend_requests"
	default:
		return kind
	}
}

func securityReferenceAllowed(ref themeSecurityReference, sec ThemeSecurity) bool {
	switch ref.Kind {
	case "script":
		return sec.ExternalAssets.Allowed && URLAllowedByPatterns(ref.URL, sec.ExternalAssets.Scripts)
	case "style":
		return sec.ExternalAssets.Allowed && URLAllowedByPatterns(ref.URL, sec.ExternalAssets.Styles)
	case "font":
		return sec.ExternalAssets.Allowed && URLAllowedByPatterns(ref.URL, sec.ExternalAssets.Fonts)
	case "image":
		return sec.ExternalAssets.Allowed && URLAllowedByPatterns(ref.URL, sec.ExternalAssets.Images)
	case "media":
		return sec.ExternalAssets.Allowed && URLAllowedByPatterns(ref.URL, sec.ExternalAssets.Media)
	case "request":
		return sec.FrontendRequests.Allowed && URLAllowedByPatterns(ref.URL, sec.FrontendRequests.Origins)
	default:
		return false
	}
}

func securityReferenceHint(ref themeSecurityReference) string {
	origin := securityOrigin(ref.URL)
	if origin == "" {
		origin = ref.URL
	}
	if ref.Kind == "request" {
		return fmt.Sprintf("Add %s to %s in theme.yaml and set security.frontend_requests.allowed: true.", origin, securityReferenceField(ref.Kind))
	}
	return fmt.Sprintf("Add %s to %s in theme.yaml and set security.external_assets.allowed: true.", origin, securityReferenceField(ref.Kind))
}

func securityOrigin(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/")
}
