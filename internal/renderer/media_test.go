package renderer

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/assets"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestRenderedMediaReceivesVariantsAndAltValidation(t *testing.T) {
	cfg := testRendererConfig(t)
	writeRendererTheme(t, cfg)
	cfg.Media.ResponsiveImages = true
	cfg.Media.RequireAlt = true
	cfg.Media.Widths = []int{20}
	cfg.Build.CopyImages = true
	root := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir)
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewNRGBA(image.Rect(0, 0, 40, 20))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "test.png"), original.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "test.png.meta.yaml"), []byte("alt: A green tree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	layout := filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "page.html")
	if err := os.WriteFile(layout, []byte(`{{ define "content" }}<img src="/images/test.png">{{ end }}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := assets.Sync(cfg, nil); err != nil {
		t.Fatal(err)
	}
	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{ID: "test", Type: "page", Lang: cfg.DefaultLang, URL: "/test/", Layout: "page", Title: "Test"})
	renderer := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil)
	output, err := renderer.RenderURL(graph, "/test/", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`alt="A green tree"`, `srcset="/_foundry/media/`, `width="40"`, `height="20"`} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("missing %s in %s", want, output)
		}
	}
	if err := os.Remove(filepath.Join(root, "test.png.meta.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.RenderURL(graph, "/test/", false); err == nil {
		t.Fatal("strict accessibility failed to reject missing alt")
	}
	cfg.Media.RequireAlt = false
	if _, err := renderer.RenderURL(graph, "/test/", false); err != nil {
		t.Fatal("default rendering unexpectedly enforced alt")
	}
}
