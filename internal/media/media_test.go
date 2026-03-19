package media

import (
	"strings"
	"testing"
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
	if _, err := ResolveReference("media:videos/demo.mp4"); err == nil {
		t.Fatal("expected unsupported collection rejection")
	}
}

func TestRewriteHTMLTransformsMediaEmbedsAndLinks(t *testing.T) {
	html := `<p><img src="media:images/hero.png" alt="Hero"></p><p><img src="media:uploads/demo.mp4" alt="Demo clip"></p><p><img src="media:uploads/theme.mp3" alt="Theme song"></p><p><a href="media:uploads/spec.pdf">Spec sheet</a></p>`
	rewritten := RewriteHTML(html)

	for _, want := range []string{`<img src="/images/hero.png" alt="Hero">`, `<video controls preload="metadata" src="/uploads/demo.mp4" title="Demo clip" aria-label="Demo clip"></video>`, `<audio controls preload="metadata" src="/uploads/theme.mp3" title="Theme song" aria-label="Theme song"></audio>`, `<a href="/uploads/spec.pdf">Spec sheet</a>`} {
		if !strings.Contains(rewritten, want) {
			t.Fatalf("expected rewritten html to contain %q, got %q", want, rewritten)
		}
	}
}

func TestDefaultCollectionAndFilenameSanitization(t *testing.T) {
	if got := DefaultCollection("poster.webp", ""); got != "images" {
		t.Fatalf("expected images collection, got %q", got)
	}
	if got := DefaultCollection("clip.mp4", "video/mp4"); got != "uploads" {
		t.Fatalf("expected uploads collection, got %q", got)
	}
	if got := DefaultCollection("raw.bin", "image/png"); got != "images" {
		t.Fatalf("expected content type to force images collection, got %q", got)
	}

	if got := SanitizeFilename("My Demo File!!.MP4"); got != "my-demo-file.mp4" {
		t.Fatalf("unexpected sanitized filename: %q", got)
	}
}
