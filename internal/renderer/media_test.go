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
	"github.com/sphireinc/foundry/internal/media"
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

func TestImageIndexCacheReusesAndRefreshesManifest(t *testing.T) {
	cfg := testRendererConfig(t)
	cfg.Media.ResponsiveImages = true
	renderer := New(cfg, nil, nil)
	if index, err := renderer.loadImageIndex(); err != nil || index != nil {
		t.Fatalf("missing manifest: %v %v", index, err)
	}
	directory := filepath.Join(cfg.PublicDir, media.VariantDirectory)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "index.json")
	if err := os.WriteFile(filename, []byte(`{"/images/a.png":{"width":10}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := renderer.loadImageIndex()
	if err != nil {
		t.Fatal(err)
	}
	// A marker in the decoded map proves the next load reused that map.
	first["cache-marker"] = media.ImageEntry{Width: 42}
	second, err := renderer.loadImageIndex()
	if err != nil || second["cache-marker"].Width != 42 {
		t.Fatalf("index not reused: %v", err)
	}
	oldInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(directory, "replacement.json")
	if err := os.WriteFile(replacement, []byte(`{"/images/a.png":{"width":20}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, oldInfo.ModTime(), oldInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filename); err != nil {
		t.Fatal(err)
	}
	refreshed, err := renderer.loadImageIndex()
	if err != nil || refreshed["/images/a.png"].Width != 20 || refreshed["cache-marker"].Width != 0 {
		t.Fatalf("replacement not reloaded: %v %v", refreshed, err)
	}
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if index, err := renderer.loadImageIndex(); err != nil || index != nil {
		t.Fatalf("removed manifest cached: %v %v", index, err)
	}
}
