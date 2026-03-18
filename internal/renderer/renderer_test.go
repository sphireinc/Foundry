package renderer

import (
	"context"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestBuildURLsRendersTaxonomyArchiveWithConfiguredLayout(t *testing.T) {
	cfg := testRendererConfig(t)
	writeRendererTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:         "post-1",
		Type:       "post",
		Lang:       cfg.DefaultLang,
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md")),
		HTMLBody:   template.HTML("<p>Hello</p>"),
		Taxonomies: map[string][]string{"tags": {"go"}},
	})

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil)
	if err := r.BuildURLs(context.Background(), graph, []string{"/tags/go/"}); err != nil {
		t.Fatalf("build urls failed: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(cfg.PublicDir, "tags", "go", "index.html"))
	if err != nil {
		t.Fatalf("expected taxonomy archive output: %v", err)
	}
	if !strings.Contains(string(body), "taxonomy layout for tags/go") {
		t.Fatalf("expected taxonomy term layout to render, got %q", string(body))
	}
}

func TestBuildURLsSkipsUnknownURLs(t *testing.T) {
	cfg := testRendererConfig(t)
	writeRendererTheme(t, cfg)

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil)
	graph := content.NewSiteGraph(cfg)

	if err := r.BuildURLs(context.Background(), graph, []string{"/missing/"}); err != nil {
		t.Fatalf("expected missing URL to be skipped, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.PublicDir, "missing", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected no output for missing URL, got err=%v", err)
	}
}

func testRendererConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	cfg := &config.Config{
		Title:       "Foundry",
		DefaultLang: "en",
		Theme:       "default",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		DataDir:     filepath.Join(root, "data"),
		Feed: config.FeedConfig{
			RSSPath:     "/rss.xml",
			SitemapPath: "/sitemap.xml",
		},
		Taxonomies: config.TaxonomyConfig{
			DefaultSet: []string{"tags"},
			Definitions: map[string]config.TaxonomyDefinition{
				"tags": {TermLayout: "taxonomy-term", Title: "Tags"},
			},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

func writeRendererTheme(t *testing.T, cfg *config.Config) {
	t.Helper()

	files := map[string]string{
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "base.html"):               `{{ define "base" }}{{ template "content" . }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "post.html"):               `{{ define "content" }}post {{ .Page.Title }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "index.html"):              `{{ define "content" }}index{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "taxonomy-term.html"):      `{{ define "content" }}taxonomy layout for {{ .TaxonomyName }}/{{ .TaxonomyTerm }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "partials", "head.html"):   `{{ define "head" }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "partials", "header.html"): `{{ define "header" }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "partials", "footer.html"): `{{ define "footer" }}{{ end }}`,
	}

	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
