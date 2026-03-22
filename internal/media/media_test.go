package media

import (
	"strings"
	"testing"
	"time"
)

func TestResolveReferenceAndKindDetection(t *testing.T) {
	ref, err := ResolveReference("media:images/hero/banner.JPG")
	if err != nil {
		t.Fatalf("resolve reference: %v", err)
	}
	if ref.Collection != "images" || ref.Path != "hero/banner.JPG" || ref.PublicURL != "/images/hero/banner.JPG" || ref.Kind != KindImage {
		t.Fatalf("unexpected resolved reference: %#v", ref)
	}

	if _, err := ResolveReference("media:../escape.png"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := ResolveReference("media:images/hero.version.20260319T143209Z.png"); err == nil {
		t.Fatal("expected derived media reference rejection")
	}
	videoRef, err := ResolveReference("media:videos/demo.mp4")
	if err != nil {
		t.Fatalf("resolve video reference: %v", err)
	}
	if videoRef.PublicURL != "/videos/demo.mp4" || videoRef.Kind != KindVideo {
		t.Fatalf("unexpected resolved video reference: %#v", videoRef)
	}
}

func TestRewriteHTMLTransformsMediaEmbedsAndLinks(t *testing.T) {
	html := `<p><img src="media:images/hero.png" alt="Hero"></p><p><img src="media:videos/demo.mp4" alt="Demo clip"></p><p><img src="media:audio/theme.mp3" alt="Theme song"></p><p><a href="media:documents/spec.pdf">Spec sheet</a></p>`
	rewritten := RewriteHTML(html)

	for _, want := range []string{`<img src="/images/hero.png" alt="Hero">`, `<video controls preload="metadata" src="/videos/demo.mp4" title="Demo clip" aria-label="Demo clip"></video>`, `<audio controls preload="metadata" src="/audio/theme.mp3" title="Theme song" aria-label="Theme song"></audio>`, `<a href="/documents/spec.pdf">Spec sheet</a>`} {
		if !strings.Contains(rewritten, want) {
			t.Fatalf("expected rewritten html to contain %q, got %q", want, rewritten)
		}
	}
}

func TestDefaultCollectionAndFilenameSanitization(t *testing.T) {
	if got := DefaultCollection("poster.webp", ""); got != "images" {
		t.Fatalf("expected images collection, got %q", got)
	}
	if got := DefaultCollection("clip.mp4", "video/mp4"); got != "videos" {
		t.Fatalf("expected videos collection, got %q", got)
	}
	if got := DefaultCollection("theme.mp3", "audio/mpeg"); got != "audio" {
		t.Fatalf("expected audio collection, got %q", got)
	}
	if got := DefaultCollection("raw.bin", "image/png"); got != "images" {
		t.Fatalf("expected content type to force images collection, got %q", got)
	}

	if got := SanitizeFilename("My Demo File!!.MP4"); got != "my-demo-file.mp4" {
		t.Fatalf("unexpected sanitized filename: %q", got)
	}
}

func TestPrepareUploadRejectsDangerousContentAndMismatches(t *testing.T) {
	body := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}

	info, err := PrepareUpload("", "Hero Image.PNG", body, testTime())
	if err != nil {
		t.Fatalf("prepare upload: %v", err)
	}
	if info.Collection != "images" || info.SafeFilename != "hero-image.png" || info.Kind != KindImage {
		t.Fatalf("unexpected upload normalization: %#v", info)
	}
	if !strings.HasPrefix(info.StoredFilename, "hero-image-") || !strings.HasSuffix(info.StoredFilename, ".png") {
		t.Fatalf("unexpected stored filename: %#v", info)
	}

	if _, err := PrepareUpload("images", "hero.png", []byte("<html>nope</html>"), testTime()); err == nil {
		t.Fatal("expected html upload rejection")
	}
	if _, err := PrepareUpload("videos", "hero.svg", []byte("<svg></svg>"), testTime()); err == nil {
		t.Fatal("expected svg upload rejection")
	}
	if _, err := PrepareUpload("audio", "clip.mp4", []byte("video"), testTime()); err == nil {
		t.Fatal("expected mismatched audio upload rejection")
	}
	if _, err := PrepareUpload("assets", "theme.js", []byte("alert(1)"), testTime()); err == nil {
		t.Fatal("expected assets collection rejection")
	}
}

func TestShouldForceDownload(t *testing.T) {
	if !ShouldForceDownload("danger.html") {
		t.Fatal("expected html to force download")
	}
	if !ShouldForceDownload("vector.svg") {
		t.Fatal("expected svg to force download")
	}
	if ShouldForceDownload("clip.mp4") {
		t.Fatal("expected safe media to remain inline")
	}
}

func testTime() time.Time {
	return time.Date(2026, time.March, 21, 12, 34, 56, 0, time.UTC)
}
