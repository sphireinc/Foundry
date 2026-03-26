package media

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/lifecycle"
)

type Kind string

const (
	KindImage Kind = "image"
	KindVideo Kind = "video"
	KindAudio Kind = "audio"
	KindFile  Kind = "file"
)

const ReferenceScheme = "media:"

var SupportedCollections = []string{"images", "videos", "audio", "documents", "uploads", "assets"}

var (
	allowedImageExts = map[string]struct{}{
		".png":  {},
		".jpg":  {},
		".jpeg": {},
		".gif":  {},
		".webp": {},
		".avif": {},
	}
	allowedUploadExts = map[string]struct{}{
		".png":  {},
		".jpg":  {},
		".jpeg": {},
		".gif":  {},
		".webp": {},
		".avif": {},
		".mp4":  {},
		".webm": {},
		".mov":  {},
		".m4v":  {},
		".ogv":  {},
		".mp3":  {},
		".wav":  {},
		".ogg":  {},
		".m4a":  {},
		".flac": {},
		".pdf":  {},
		".txt":  {},
		".csv":  {},
		".zip":  {},
	}
	forceDownloadExts = map[string]struct{}{
		".css":  {},
		".htm":  {},
		".html": {},
		".js":   {},
		".mjs":  {},
		".svg":  {},
		".xml":  {},
	}
)

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

type UploadInfo struct {
	Collection       string
	OriginalFilename string
	SafeFilename     string
	StoredFilename   string
	MIMEType         string
	Extension        string
	Kind             Kind
	ContentHash      string
	Size             int64
	Width            int
	Height           int
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

	collection := canonicalCollection(strings.TrimSpace(parts[0]))
	if !isSupportedCollection(collection) {
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

func isSupportedCollection(collection string) bool {
	collection = canonicalCollection(collection)
	for _, candidate := range SupportedCollections {
		if collection == candidate {
			return true
		}
	}
	return false
}

func MustReference(collection, relPath string) string {
	ref, err := NewReference(collection, relPath)
	if err != nil {
		panic(err)
	}
	return ref
}

func NewReference(collection, relPath string) (string, error) {
	collection = canonicalCollection(strings.TrimSpace(collection))
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
	case KindVideo:
		return "videos"
	case KindAudio:
		return "audio"
	default:
		return "documents"
	}
}

func PrepareUpload(requestedCollection, filename string, body []byte, now time.Time) (UploadInfo, error) {
	safeName := SanitizeFilename(filename)
	ext := strings.ToLower(filepath.Ext(safeName))
	if ext == "" {
		return UploadInfo{}, fmt.Errorf("uploaded media must have a file extension")
	}

	detected := detectedContentType(body)
	if isDangerousServedType(ext, detected) {
		return UploadInfo{}, fmt.Errorf("uploaded media type is not allowed")
	}

	collection := canonicalCollection(strings.TrimSpace(requestedCollection))
	if collection == "" {
		collection = DefaultCollection(safeName, detected)
	}
	if !isSupportedCollection(collection) {
		return UploadInfo{}, fmt.Errorf("unsupported media collection: %s", collection)
	}
	if collection == "assets" {
		return UploadInfo{}, fmt.Errorf("admin media uploads cannot write to the assets collection")
	}

	kind := DetectKind(safeName)
	switch collection {
	case "images":
		if kind != KindImage {
			return UploadInfo{}, fmt.Errorf("images collection only accepts image files")
		}
		if _, ok := allowedImageExts[ext]; !ok {
			return UploadInfo{}, fmt.Errorf("unsupported image file type: %s", ext)
		}
		if !matchesDetectedType(ext, detected) {
			return UploadInfo{}, fmt.Errorf("uploaded file content does not match its extension")
		}
	case "videos":
		if kind != KindVideo {
			return UploadInfo{}, fmt.Errorf("videos collection only accepts video files")
		}
		if _, ok := allowedUploadExts[ext]; !ok {
			return UploadInfo{}, fmt.Errorf("unsupported video file type: %s", ext)
		}
		if !matchesDetectedType(ext, detected) {
			return UploadInfo{}, fmt.Errorf("uploaded file content does not match its extension")
		}
	case "audio":
		if kind != KindAudio {
			return UploadInfo{}, fmt.Errorf("audio collection only accepts audio files")
		}
		if _, ok := allowedUploadExts[ext]; !ok {
			return UploadInfo{}, fmt.Errorf("unsupported audio file type: %s", ext)
		}
		if !matchesDetectedType(ext, detected) {
			return UploadInfo{}, fmt.Errorf("uploaded file content does not match its extension")
		}
	case "documents":
		if kind != KindFile {
			return UploadInfo{}, fmt.Errorf("documents collection only accepts document files")
		}
		if _, ok := allowedUploadExts[ext]; !ok {
			return UploadInfo{}, fmt.Errorf("unsupported document file type: %s", ext)
		}
		if !matchesDetectedType(ext, detected) {
			return UploadInfo{}, fmt.Errorf("uploaded file content does not match its extension")
		}
	case "uploads":
		if _, ok := allowedUploadExts[ext]; !ok {
			return UploadInfo{}, fmt.Errorf("unsupported upload file type: %s", ext)
		}
		if !matchesDetectedType(ext, detected) {
			return UploadInfo{}, fmt.Errorf("uploaded file content does not match its extension")
		}
	default:
		return UploadInfo{}, fmt.Errorf("unsupported media collection: %s", collection)
	}

	storedName, err := BuildStoredFilename(safeName, now)
	if err != nil {
		return UploadInfo{}, err
	}
	dim := imageDimensions(body, kind)
	return UploadInfo{
		Collection:       collection,
		OriginalFilename: filepath.Base(strings.TrimSpace(filename)),
		SafeFilename:     safeName,
		StoredFilename:   storedName,
		MIMEType:         detected,
		Extension:        ext,
		Kind:             kind,
		ContentHash:      ContentHash(body),
		Size:             int64(len(body)),
		Width:            dim.Width,
		Height:           dim.Height,
	}, nil
}

type dimensions struct {
	Width  int
	Height int
}

func imageDimensions(body []byte, kind Kind) dimensions {
	if kind != KindImage || len(body) == 0 {
		return dimensions{}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return dimensions{}
	}
	return dimensions{Width: cfg.Width, Height: cfg.Height}
}

func canonicalCollection(collection string) string {
	switch strings.TrimSpace(collection) {
	case "video":
		return "videos"
	case "videos":
		return "videos"
	case "audio":
		return "audio"
	case "uploads":
		return "uploads"
	case "assets":
		return "assets"
	default:
		return strings.TrimSpace(collection)
	}
}

func BuildStoredFilename(safeName string, now time.Time) (string, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(safeName)))
	base := strings.TrimSuffix(strings.TrimSpace(safeName), ext)
	if base == "" || ext == "" {
		return "", fmt.Errorf("stored filename requires a base name and extension")
	}
	suffix, err := randomSuffix(4)
	if err != nil {
		return "", err
	}
	return base + "-" + now.UTC().Format("20060102T150405Z") + "-" + suffix + ext, nil
}

func ContentHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func ShouldForceDownload(name string) bool {
	_, ok := forceDownloadExts[strings.ToLower(filepath.Ext(strings.TrimSpace(name)))]
	return ok
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
	if lifecycle.IsDerivedPath(cleaned) {
		return "", fmt.Errorf("media path must reference a current media file")
	}
	return cleaned, nil
}

func detectedContentType(body []byte) string {
	if len(body) == 0 {
		return "application/octet-stream"
	}
	sample := body
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(sample), ";")[0]))
}

func randomSuffix(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isDangerousServedType(ext, detected string) bool {
	if _, ok := forceDownloadExts[ext]; ok {
		return true
	}
	switch detected {
	case "application/javascript", "application/xhtml+xml", "image/svg+xml", "text/css", "text/html", "text/javascript", "text/xml", "application/xml":
		return true
	default:
		return false
	}
}

func matchesDetectedType(ext, detected string) bool {
	switch ext {
	case ".png":
		return detected == "image/png"
	case ".jpg", ".jpeg":
		return detected == "image/jpeg"
	case ".gif":
		return detected == "image/gif"
	case ".webp":
		return detected == "image/webp"
	case ".avif":
		return detected == "image/avif" || detected == "application/octet-stream"
	case ".mp4", ".m4v":
		return detected == "video/mp4"
	case ".webm":
		return detected == "video/webm"
	case ".mov":
		return detected == "video/quicktime" || detected == "application/octet-stream"
	case ".ogv":
		return detected == "video/ogg" || detected == "application/ogg"
	case ".mp3":
		return detected == "audio/mpeg" || detected == "application/octet-stream"
	case ".wav":
		return detected == "audio/wave" || detected == "audio/x-wav" || detected == "application/octet-stream"
	case ".ogg":
		return detected == "audio/ogg" || detected == "application/ogg"
	case ".m4a":
		return detected == "audio/mp4" || detected == "application/octet-stream"
	case ".flac":
		return detected == "audio/flac" || detected == "application/octet-stream"
	case ".pdf":
		return detected == "application/pdf"
	case ".txt":
		return detected == "text/plain" || detected == "application/octet-stream"
	case ".csv":
		return detected == "text/csv" || detected == "text/plain" || detected == "application/octet-stream"
	case ".zip":
		return detected == "application/zip" || detected == "application/x-zip-compressed" || detected == "application/octet-stream"
	default:
		return false
	}
}
