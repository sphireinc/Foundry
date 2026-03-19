package media

import (
	"fmt"
	"html/template"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type Kind string

const (
	KindImage Kind = "image"
	KindVideo Kind = "video"
	KindAudio Kind = "audio"
	KindFile  Kind = "file"
)

const ReferenceScheme = "media:"

var (
	imgTagRE     = regexp.MustCompile(`(?i)<img\b[^>]*\bsrc="([^"]+)"[^>]*>`)
	linkTagRE    = regexp.MustCompile(`(?is)<a\b([^>]*?)\bhref="([^"]+)"([^>]*)>(.*?)</a>`)
	attrRE       = regexp.MustCompile(`([a-zA-Z_:][a-zA-Z0-9:._-]*)\s*=\s*"([^"]*)"`)
	spacesRE     = regexp.MustCompile(`\s+`)
	unsafeNameRE = regexp.MustCompile(`[^a-z0-9._-]+`)
)

type Reference struct {
	Collection string
	Path       string
	PublicURL  string
	Kind       Kind
}

func ResolveReference(ref string) (Reference, error) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, ReferenceScheme) {
		return Reference{}, fmt.Errorf("unsupported media reference: %s", ref)
	}

	raw := strings.TrimPrefix(ref, ReferenceScheme)
	raw = strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/"))
	raw = strings.TrimPrefix(raw, "/")
	if raw == "" {
		return Reference{}, fmt.Errorf("media reference path cannot be empty")
	}

	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return Reference{}, fmt.Errorf("media reference must include collection and path")
	}

	collection := strings.TrimSpace(parts[0])
	if collection != "images" && collection != "uploads" && collection != "assets" {
		return Reference{}, fmt.Errorf("unsupported media collection: %s", collection)
	}

	cleanPath, err := cleanRelativePath(parts[1])
	if err != nil {
		return Reference{}, err
	}

	return Reference{
		Collection: collection,
		Path:       cleanPath,
		PublicURL:  "/" + collection + "/" + cleanPath,
		Kind:       DetectKind(cleanPath),
	}, nil
}

func MustReference(collection, relPath string) string {
	ref, err := NewReference(collection, relPath)
	if err != nil {
		panic(err)
	}
	return ref
}

func NewReference(collection, relPath string) (string, error) {
	collection = strings.TrimSpace(collection)
	cleanPath, err := cleanRelativePath(relPath)
	if err != nil {
		return "", err
	}
	ref, err := ResolveReference(ReferenceScheme + collection + "/" + cleanPath)
	if err != nil {
		return "", err
	}
	return ReferenceScheme + ref.Collection + "/" + ref.Path, nil
}

func DetectKind(name string) Kind {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".avif":
		return KindImage
	case ".mp4", ".webm", ".mov", ".m4v", ".ogv":
		return KindVideo
	case ".mp3", ".wav", ".ogg", ".m4a", ".flac":
		return KindAudio
	default:
		return KindFile
	}
}

func DefaultCollection(filename, contentType string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/") {
		return "images"
	}
	switch DetectKind(filename) {
	case KindImage:
		return "images"
	default:
		return "uploads"
	}
}

func SanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	rawExt := filepath.Ext(name)
	ext := strings.ToLower(rawExt)
	base := strings.TrimSuffix(name, rawExt)
	base = strings.ToLower(strings.TrimSpace(base))
	base = spacesRE.ReplaceAllString(base, "-")
	base = unsafeNameRE.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-.")
	if base == "" {
		base = "file"
	}
	return base + ext
}

func RewriteHTML(html string) string {
	html = imgTagRE.ReplaceAllStringFunc(html, rewriteImageTag)
	html = linkTagRE.ReplaceAllStringFunc(html, rewriteLinkTag)
	return html
}

func rewriteImageTag(tag string) string {
	match := imgTagRE.FindStringSubmatch(tag)
	if len(match) != 2 {
		return tag
	}

	ref, err := ResolveReference(match[1])
	if err != nil {
		return tag
	}
	if ref.Kind == KindImage {
		return strings.Replace(tag, `src="`+match[1]+`"`, `src="`+template.HTMLEscapeString(ref.PublicURL)+`"`, 1)
	}

	attrs := parseAttributes(tag)
	classAttr := renderOptionalAttribute("class", attrs["class"])
	titleAttr := renderOptionalAttribute("title", firstNonEmpty(attrs["title"], attrs["alt"]))
	labelAttr := renderOptionalAttribute("aria-label", firstNonEmpty(attrs["alt"], attrs["title"]))
	switch ref.Kind {
	case KindVideo:
		return `<video controls preload="metadata" src="` + template.HTMLEscapeString(ref.PublicURL) + `"` + classAttr + titleAttr + labelAttr + `></video>`
	case KindAudio:
		return `<audio controls preload="metadata" src="` + template.HTMLEscapeString(ref.PublicURL) + `"` + classAttr + titleAttr + labelAttr + `></audio>`
	default:
		return `<a href="` + template.HTMLEscapeString(ref.PublicURL) + `"` + classAttr + titleAttr + `>` + template.HTMLEscapeString(firstNonEmpty(attrs["alt"], path.Base(ref.Path))) + `</a>`
	}
}

func rewriteLinkTag(tag string) string {
	match := linkTagRE.FindStringSubmatch(tag)
	if len(match) != 5 {
		return tag
	}
	ref, err := ResolveReference(match[2])
	if err != nil {
		return tag
	}
	return strings.Replace(tag, `href="`+match[2]+`"`, `href="`+template.HTMLEscapeString(ref.PublicURL)+`"`, 1)
}

func parseAttributes(tag string) map[string]string {
	out := make(map[string]string)
	for _, match := range attrRE.FindAllStringSubmatch(tag, -1) {
		if len(match) != 3 {
			continue
		}
		out[strings.ToLower(match[1])] = match[2]
	}
	return out
}

func renderOptionalAttribute(name, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return ` ` + name + `="` + template.HTMLEscapeString(value) + `"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func cleanRelativePath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return "", fmt.Errorf("media path cannot be empty")
	}

	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == "/" || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid media path: path must stay inside the media root")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("media path cannot be empty")
	}
	return cleaned, nil
}
