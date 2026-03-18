package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/deps"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/theme"
)

type watchRecorder []string

func (w *watchRecorder) Add(path string) error {
	*w = append(*w, path)
	return nil
}

func TestServerHelpersAndHandlers(t *testing.T) {
	cfg := testServerConfig(t)
	writeServerTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	doc := &content.Document{
		ID:         "post-1",
		Type:       "post",
		Lang:       "en",
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md")),
		HTMLBody:   template.HTML("<p>Hello</p>"),
		Summary:    "Summary",
		Taxonomies: map[string][]string{"tags": {"go"}},
	}
	graph.Add(doc)

	s := &Server{
		cfg:          cfg,
		router:       router.NewResolver(cfg),
		renderer:     renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil),
		graph:        graph,
		depGraph:     deps.BuildSiteDependencyGraph(graph, cfg.Theme),
		reloadSignal: make(chan struct{}, 1),
	}

	if !hasRenderableChanges(deps.ChangeSet{Sources: []string{"x"}}) || hasRenderableChanges(deps.ChangeSet{Assets: []string{"x"}}) {
		t.Fatal("unexpected renderable change detection")
	}
	s.signalReload()
	s.signalReload()
	select {
	case <-s.reloadSignal:
	default:
		t.Fatal("expected reload signal")
	}

	if got := s.listenURL(); got != "http://localhost:8080" {
		t.Fatalf("unexpected listen url: %q", got)
	}
	cfg.Server.Addr = "127.0.0.1:9000"
	if got := s.listenURL(); got != "http://127.0.0.1:9000" {
		t.Fatalf("unexpected listen url: %q", got)
	}
	cfg.Server.Addr = "https://example.com"
	if got := s.listenURL(); got != "https://example.com" {
		t.Fatalf("unexpected listen url: %q", got)
	}

	rr := httptest.NewRecorder()
	s.handleRSS(rr, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "<rss") {
		t.Fatalf("unexpected rss response: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.handleSitemap(rr, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "<urlset") {
		t.Fatalf("unexpected sitemap response: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.handleDepsDebug(rr, httptest.NewRequest(http.MethodGet, "/__debug/deps", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected deps debug code: %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode deps debug: %v", err)
	}

	cfg.Server.LiveReload = false
	rr = httptest.NewRecorder()
	s.handlePage(rr, httptest.NewRequest(http.MethodGet, "/posts/hello", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "post Hello") {
		t.Fatalf("unexpected page response: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.handlePage(rr, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServerChangeClassificationAndWatchHelpers(t *testing.T) {
	cfg := testServerConfig(t)
	s := &Server{cfg: cfg}

	changes := s.classifyChanges([]string{
		filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"),
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "post.html"),
		filepath.Join(cfg.DataDir, "nav.yaml"),
		filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, "app.css"),
	})
	if len(changes.Sources) != 1 || len(changes.Templates) != 1 || len(changes.DataKeys) != 1 || len(changes.Assets) != 1 {
		t.Fatalf("unexpected changes: %#v", changes)
	}

	root := t.TempDir()
	dir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !shouldAddWatch(dir) || shouldAddWatch(filepath.Join(root, "missing")) {
		t.Fatal("unexpected watch decisions")
	}

	var rec watchRecorder
	if err := addWatchRecursively(&rec, root); err != nil {
		t.Fatalf("add watch recursively: %v", err)
	}
	if len(rec) < 2 {
		t.Fatalf("expected directories to be added, got %v", rec)
	}
}

func testServerConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		Title:       "Foundry",
		BaseURL:     "https://example.com",
		DefaultLang: "en",
		Theme:       "default",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		DataDir:     filepath.Join(root, "data"),
		PluginsDir:  filepath.Join(root, "plugins"),
		Server:      config.ServerConfig{Addr: ":8080"},
		Content: config.ContentConfig{
			PagesDir:          "pages",
			PostsDir:          "posts",
			AssetsDir:         "assets",
			ImagesDir:         "images",
			UploadsDir:        "uploads",
			DefaultLayoutPost: "post",
			DefaultLayoutPage: "page",
		},
		Taxonomies: config.TaxonomyConfig{
			DefaultSet: []string{"tags"},
		},
		Feed: config.FeedConfig{
			RSSPath:        "/rss.xml",
			SitemapPath:    "/sitemap.xml",
			RSSTitle:       "Feed Title",
			RSSDescription: "Feed Description",
			RSSLimit:       10,
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

func writeServerTheme(t *testing.T, cfg *config.Config) {
	t.Helper()
	files := map[string]string{
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "base.html"):               `{{ define "base" }}{{ template "content" . }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "post.html"):               `{{ define "content" }}post {{ .Page.Title }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "page.html"):               `{{ define "content" }}page {{ .Page.Title }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "index.html"):              `{{ define "content" }}index{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "list.html"):               `{{ define "content" }}list{{ end }}`,
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
