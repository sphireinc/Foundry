package content

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/config"
)

type recordingHooks struct {
	discovered []string
	front      []string
	markdown   []string
	document   []string
	dataLoaded bool
	graphBuild bool
	graphBuilt bool
	taxBuilt   bool
	failOn     string
}

func (h *recordingHooks) OnContentDiscovered(path string) error {
	h.discovered = append(h.discovered, filepath.Base(path))
	if h.failOn == "discover" {
		return os.ErrInvalid
	}
	return nil
}
func (h *recordingHooks) OnFrontmatterParsed(doc *Document) error {
	h.front = append(h.front, doc.ID)
	if h.failOn == "front" {
		return os.ErrInvalid
	}
	return nil
}
func (h *recordingHooks) OnMarkdownRendered(doc *Document) error {
	h.markdown = append(h.markdown, doc.ID)
	if h.failOn == "markdown" {
		return os.ErrInvalid
	}
	return nil
}
func (h *recordingHooks) OnDocumentParsed(doc *Document) error {
	h.document = append(h.document, doc.ID)
	if h.failOn == "document" {
		return os.ErrInvalid
	}
	return nil
}
func (h *recordingHooks) OnDataLoaded(map[string]any) error {
	h.dataLoaded = true
	if h.failOn == "data" {
		return os.ErrInvalid
	}
	return nil
}
func (h *recordingHooks) OnGraphBuilding(*SiteGraph) error {
	h.graphBuild = true
	if h.failOn == "graph-building" {
		return os.ErrInvalid
	}
	return nil
}
func (h *recordingHooks) OnGraphBuilt(*SiteGraph) error {
	h.graphBuilt = true
	if h.failOn == "graph-built" {
		return os.ErrInvalid
	}
	return nil
}
func (h *recordingHooks) OnTaxonomyBuilt(*SiteGraph) error {
	h.taxBuilt = true
	if h.failOn == "taxonomy" {
		return os.ErrInvalid
	}
	return nil
}

func TestLoaderLoadAndHelpers(t *testing.T) {
	cfg := testLoaderConfig(t)
	hooks := &recordingHooks{}
	loader := NewLoader(cfg, hooks, false)

	graph, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("load content: %v", err)
	}
	if len(graph.Documents) != 2 {
		t.Fatalf("expected 2 non-draft documents, got %d", len(graph.Documents))
	}
	if !hooks.dataLoaded || !hooks.graphBuild || !hooks.taxBuilt || !hooks.graphBuilt {
		t.Fatalf("expected graph/data hooks to run: %#v", hooks)
	}
	if len(hooks.discovered) != 3 || len(hooks.front) != 3 || len(hooks.markdown) != 3 || len(hooks.document) != 2 {
		t.Fatalf("unexpected hook counts: %#v", hooks)
	}

	pageDoc, err := loader.loadDocument(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), "about.md", "en", true, "page")
	if err != nil {
		t.Fatalf("load page document: %v", err)
	}
	if pageDoc.Title != "About" || pageDoc.Layout != "page" || pageDoc.Summary == "" || pageDoc.HTMLBody == "" {
		t.Fatalf("unexpected page document: %#v", pageDoc)
	}

	lang, rel, isDefault := loader.resolveLanguage(filepath.Join("fr", "bonjour.md"))
	if lang != "fr" || rel != "bonjour.md" || isDefault {
		t.Fatalf("unexpected resolved language: %q %q %v", lang, rel, isDefault)
	}

	if got := buildSummary("", strings.Repeat("word ", 100)); !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncated summary, got %q", got)
	}
	if got := buildSummary(" explicit ", "ignored"); got != "explicit" {
		t.Fatalf("expected explicit summary, got %q", got)
	}
}

func TestLoaderHookFailures(t *testing.T) {
	cfg := testLoaderConfig(t)
	failures := []string{"data", "graph-building", "discover", "front", "markdown", "document", "taxonomy", "graph-built"}
	for _, failOn := range failures {
		t.Run(failOn, func(t *testing.T) {
			loader := NewLoader(cfg, &recordingHooks{failOn: failOn}, true)
			if _, err := loader.Load(context.Background()); err == nil {
				t.Fatalf("expected loader failure for %s", failOn)
			}
		})
	}
}

func TestLoaderDefaultsAndErrors(t *testing.T) {
	cfg := testLoaderConfig(t)
	loader := NewLoader(cfg, nil, true)
	if _, ok := loader.hooks.(noopHooks); !ok {
		t.Fatal("expected nil hooks to default to noopHooks")
	}

	postPath := filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, "hello.md")
	doc, err := loader.loadDocument(postPath, "hello.md", "en", true, "post")
	if err != nil {
		t.Fatalf("load post document: %v", err)
	}
	if doc.Layout != "post" || doc.Title != "hello" || doc.Slug != "hello" {
		t.Fatalf("expected defaults from filename/layout, got %#v", doc)
	}

	badPath := filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "bad.md")
	if err := os.WriteFile(badPath, []byte("---\ntitle: [\n---"), 0o644); err != nil {
		t.Fatalf("write bad document: %v", err)
	}
	if _, err := loader.loadDocument(badPath, "bad.md", "en", true, "page"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := loader.loadDocument(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "missing.md"), "missing.md", "en", true, "page"); err == nil {
		t.Fatal("expected read error")
	}
}

func testLoaderConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		Title:       "Foundry",
		BaseURL:     "https://example.com",
		Theme:       "default",
		DefaultLang: "en",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		PluginsDir:  filepath.Join(root, "plugins"),
		DataDir:     filepath.Join(root, "data"),
		Content: config.ContentConfig{
			PagesDir:          "pages",
			PostsDir:          "posts",
			DefaultLayoutPage: "page",
			DefaultLayoutPost: "post",
		},
	}
	cfg.ApplyDefaults()
	for _, dir := range []string{
		filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "fr"),
		filepath.Join(cfg.ContentDir, cfg.Content.PostsDir),
		cfg.DataDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "site.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatalf("write data file: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	page := "---\ntitle: About\nslug: about\nlayout: page\nsummary: Summary\nfields:\n  hero: yes\ntags:\n  - intro\ncategories:\n  - docs\ndate: " + now + "\n---\n\n# About\n\nBody"
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), []byte(page), 0o644); err != nil {
		t.Fatalf("write page: %v", err)
	}
	fr := "---\ntitle: Bonjour\nslug: bonjour\nlayout: page\nlang: fr\n---\n\n# Bonjour"
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "fr", "bonjour.md"), []byte(fr), 0o644); err != nil {
		t.Fatalf("write fr page: %v", err)
	}
	post := "---\ndraft: true\n---\n\n# Draft Post"
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, "hello.md"), []byte(post), 0o644); err != nil {
		t.Fatalf("write post: %v", err)
	}
	return cfg
}
