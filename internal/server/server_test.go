package server

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/deps"
	"github.com/sphireinc/foundry/internal/diag"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/theme"
)

type watchRecorder []string

type stubLoader struct {
	graph *content.SiteGraph
	err   error
}

func (s stubLoader) Load(context.Context) (*content.SiteGraph, error) { return s.graph, s.err }

func (w *watchRecorder) Add(path string) error {
	*w = append(*w, path)
	return nil
}

type hookRecorder struct {
	started []string
	graphs  int
	err     error
}

func (h *hookRecorder) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/hook", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hook")) })
}
func (h *hookRecorder) OnServerStarted(addr string) error {
	h.started = append(h.started, addr)
	return h.err
}
func (h *hookRecorder) OnRoutesAssigned(*content.SiteGraph) error {
	h.graphs++
	return h.err
}
func (h *hookRecorder) OnAssetsBuilding(*config.Config) error { return h.err }

type responseWriterNoFlush struct {
	header http.Header
	body   strings.Builder
	status int
}

func (w *responseWriterNoFlush) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}
func (w *responseWriterNoFlush) Write(b []byte) (int, error) { return w.body.Write(b) }
func (w *responseWriterNoFlush) WriteHeader(status int)      { w.status = status }

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

func TestServerRebuildIncrementalAndNew(t *testing.T) {
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
		Taxonomies: map[string][]string{"tags": {"go"}},
	}
	graph.Add(doc)

	hooks := &hookRecorder{}
	s := New(cfg, stubLoader{graph: graph}, router.NewResolver(cfg), renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil), hooks, false)
	if s.hooks == nil || s.reloadSignal == nil {
		t.Fatal("expected server defaults to be initialized")
	}

	if err := s.rebuild(context.Background()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if s.graph == nil || s.depGraph == nil || hooks.graphs != 1 {
		t.Fatalf("expected rebuilt graph and depgraph: %#v %#v %#v", s.graph, s.depGraph, hooks)
	}

	if err := s.incrementalRebuild(context.Background(), deps.ChangeSet{Assets: []string{"asset.css"}}); err != nil {
		t.Fatalf("asset-only incremental rebuild: %v", err)
	}
	if err := s.incrementalRebuild(context.Background(), deps.ChangeSet{Full: true}); err != nil {
		t.Fatalf("full incremental rebuild: %v", err)
	}
	if err := s.incrementalRebuild(context.Background(), deps.ChangeSet{Sources: []string{doc.SourcePath}}); err != nil {
		t.Fatalf("source incremental rebuild: %v", err)
	}
}

func TestServerErrorAndUnavailablePaths(t *testing.T) {
	cfg := testServerConfig(t)
	writeServerTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	doc := &content.Document{
		ID:         "page-1",
		Type:       "page",
		Lang:       "en",
		Title:      "About",
		Slug:       "about",
		URL:        "/about/",
		Layout:     "page",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "about.md")),
	}
	graph.Add(doc)

	s := &Server{
		cfg:          cfg,
		loader:       stubLoader{graph: graph},
		router:       router.NewResolver(cfg),
		renderer:     renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil),
		hooks:        &hookRecorder{},
		reloadSignal: make(chan struct{}, 1),
	}

	if err := (&Server{cfg: cfg, loader: stubLoader{err: os.ErrInvalid}, router: router.NewResolver(cfg), renderer: renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil), hooks: &hookRecorder{}, reloadSignal: make(chan struct{}, 1)}).rebuild(context.Background()); diag.KindOf(err) != diag.KindBuild {
		t.Fatalf("expected rebuild build error, got %v", err)
	}
	if err := (&Server{cfg: cfg, loader: stubLoader{graph: graph}, router: router.NewResolver(cfg), renderer: renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil), hooks: &hookRecorder{err: os.ErrInvalid}, reloadSignal: make(chan struct{}, 1)}).rebuild(context.Background()); diag.KindOf(err) != diag.KindPlugin {
		t.Fatalf("expected rebuild plugin error, got %v", err)
	}

	rr := httptest.NewRecorder()
	(&Server{cfg: cfg}).handleRSS(rr, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected rss unavailable, got %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	(&Server{cfg: cfg}).handleSitemap(rr, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected sitemap unavailable, got %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	(&Server{}).handleDepsDebug(rr, httptest.NewRequest(http.MethodGet, "/__debug/deps", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected deps debug unavailable, got %d", rr.Code)
	}

	w := &responseWriterNoFlush{}
	s.handleReload(w, httptest.NewRequest(http.MethodGet, "/__reload", nil))
	if w.status != http.StatusInternalServerError {
		t.Fatalf("expected reload stream unsupported, got %d", w.status)
	}
}

func TestListenAndServeLifecycleAndHookError(t *testing.T) {
	cfg := testServerConfig(t)
	cfg.Server.Addr = "127.0.0.1:0"
	cfg.Server.LiveReload = true
	cfg.Server.DebugRoutes = true
	writeServerTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:         "page-1",
		Type:       "page",
		Lang:       "en",
		Title:      "About",
		Slug:       "about",
		URL:        "/about/",
		Layout:     "page",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "about.md")),
	})

	s := New(cfg, stubLoader{graph: graph}, router.NewResolver(cfg), renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil), &hookRecorder{}, false)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	if err := s.ListenAndServe(ctx); err != nil {
		t.Fatalf("listen and serve lifecycle: %v", err)
	}

	errSrv := New(cfg, stubLoader{graph: graph}, router.NewResolver(cfg), renderer.New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil), &hookRecorder{err: os.ErrInvalid}, false)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if err := errSrv.ListenAndServe(ctx2); diag.KindOf(err) != diag.KindPlugin {
		t.Fatalf("expected hook error from listen and serve, got %v", err)
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
