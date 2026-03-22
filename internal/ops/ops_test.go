package ops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/theme"
)

type stubLoader struct {
	graph *content.SiteGraph
	err   error
}

func (s stubLoader) Load(context.Context) (*content.SiteGraph, error) {
	return s.graph, s.err
}

type stubHooks struct {
	called bool
}

func (s *stubHooks) OnRoutesAssigned(*content.SiteGraph) error {
	s.called = true
	return nil
}

func TestWritePreviewManifest(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		BaseURL:     "https://preview.example.com",
		PublicDir:   filepath.Join(root, "public"),
		DefaultLang: "en",
	}
	cfg.ApplyDefaults()

	graph := &content.SiteGraph{
		Documents: []*content.Document{
			{Title: "Draft", Status: "draft", Type: "post", Lang: "en", SourcePath: "content/posts/draft.md", URL: "/posts/draft/"},
			{Title: "Published", Status: "published", Draft: false, Type: "post", Lang: "en", SourcePath: "content/posts/live.md", URL: "/posts/live/"},
		},
	}

	if err := WritePreviewManifest(cfg, graph, "preview", true); err != nil {
		t.Fatalf("write preview manifest: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(cfg.PublicDir, "preview-links.json"))
	if err != nil {
		t.Fatalf("read preview manifest: %v", err)
	}

	var manifest PreviewManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatalf("unmarshal preview manifest: %v", err)
	}
	if manifest.Target != "preview" || len(manifest.Links) != 1 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.Links[0].PreviewURL != "https://preview.example.com/posts/draft/" {
		t.Fatalf("unexpected preview url: %+v", manifest.Links[0])
	}
}

func TestAnalyzeSiteFindsOperationalIssues(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Theme:       "default",
		DefaultLang: "en",
		ContentDir:  filepath.Join(root, "content"),
		ThemesDir:   filepath.Join(root, "themes"),
		PublicDir:   filepath.Join(root, "public"),
		DataDir:     filepath.Join(root, "data"),
		PluginsDir:  filepath.Join(root, "plugins"),
		Content: config.ContentConfig{
			PagesDir:          "pages",
			PostsDir:          "posts",
			ImagesDir:         "images",
			AssetsDir:         "assets",
			UploadsDir:        "uploads",
			DefaultLayoutPage: "page",
			DefaultLayoutPost: "post",
		},
	}
	cfg.ApplyDefaults()

	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir), 0o755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "unused.png"), []byte("img"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	graph := &content.SiteGraph{
		Config: cfg,
		Documents: []*content.Document{
			{Title: "About", Type: "page", Lang: "en", Slug: "about", URL: "/about/", Layout: "missing-layout", SourcePath: "content/pages/about.md", RawBody: "[Missing](/missing/)\n![Media](media:images/missing.png)", Taxonomies: map[string][]string{"unknown": {"x"}}},
			{Title: "About 2", Type: "page", Lang: "en", Slug: "about", URL: "/about/", Layout: "page", SourcePath: "content/pages/about-2.md", RawBody: ""},
		},
		ByURL: map[string]*content.Document{},
	}
	for _, doc := range graph.Documents {
		graph.ByURL[doc.URL] = doc
	}

	report := AnalyzeSite(cfg, graph)
	if len(report.DuplicateURLs) == 0 || len(report.DuplicateSlugs) == 0 || len(report.BrokenInternalLinks) == 0 || len(report.BrokenMediaRefs) == 0 || len(report.MissingTemplates) == 0 || len(report.OrphanedMedia) == 0 || len(report.TaxonomyInconsistency) == 0 {
		t.Fatalf("expected all diagnostics to fire, got %+v", report)
	}
}

func TestLoadPreparedGraphBuildsDependencyGraph(t *testing.T) {
	cfg := &config.Config{DefaultLang: "en"}
	cfg.ApplyDefaults()
	graph := &content.SiteGraph{
		Config: cfg,
		Documents: []*content.Document{
			{ID: "doc-1", Type: "page", Lang: "en", Slug: "about", URL: "/about/", Layout: "page", SourcePath: "content/pages/about.md"},
		},
		ByURL: map[string]*content.Document{},
	}
	graph.ByURL["/about/"] = graph.Documents[0]

	hooks := &stubHooks{}
	prepared, err := LoadPreparedGraph(context.Background(), stubLoader{graph: graph}, router.NewResolver(cfg), hooks, "default")
	if err != nil {
		t.Fatalf("load prepared graph: %v", err)
	}
	if !hooks.called || prepared.Graph == nil || prepared.DepGraph == nil {
		t.Fatalf("expected prepared graph and dependency graph, got %+v", prepared)
	}
	foundOutput := false
	for _, node := range prepared.DepGraph.Nodes() {
		if node.Type == "output" && node.ID == "output:/about/" {
			foundOutput = true
			break
		}
	}
	if !foundOutput {
		t.Fatalf("expected dep graph to contain /about/ output node")
	}
}
