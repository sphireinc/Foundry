package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/sphireinc/foundry/internal/assets"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/deps"
	"github.com/sphireinc/foundry/internal/diag"
	"github.com/sphireinc/foundry/internal/logx"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
)

type Loader interface {
	Load(context.Context) (*content.SiteGraph, error)
}

type Hooks interface {
	RegisterRoutes(mux *http.ServeMux)
	OnServerStarted(addr string) error
	OnRoutesAssigned(graph *content.SiteGraph) error
	OnAssetsBuilding(*config.Config) error
}

type noopHooks struct{}

func (noopHooks) RegisterRoutes(_ *http.ServeMux)             {}
func (noopHooks) OnServerStarted(_ string) error              { return nil }
func (noopHooks) OnRoutesAssigned(_ *content.SiteGraph) error { return nil }
func (noopHooks) OnAssetsBuilding(_ *config.Config) error     { return nil }

type Server struct {
	cfg          *config.Config
	loader       Loader
	router       *router.Resolver
	renderer     *renderer.Renderer
	hooks        Hooks
	preview      bool
	mu           sync.RWMutex
	graph        *content.SiteGraph
	depGraph     *deps.Graph
	reloadSignal chan struct{}
}

func New(
	cfg *config.Config,
	loader Loader,
	router *router.Resolver,
	renderer *renderer.Renderer,
	hooks Hooks,
	preview bool,
) *Server {
	if hooks == nil {
		hooks = noopHooks{}
	}

	return &Server{
		cfg:          cfg,
		loader:       loader,
		router:       router,
		renderer:     renderer,
		hooks:        hooks,
		preview:      preview,
		reloadSignal: make(chan struct{}, 1),
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := s.rebuild(ctx); err != nil {
		return err
	}

	go s.watch(ctx)

	serverURL := s.listenURL()
	logx.Info(
		"Foundry is running",
		"url", serverURL,
		"theme", s.cfg.Theme,
		"preview", s.preview,
		"live_reload", s.cfg.Server.LiveReload,
	)

	if err := s.hooks.OnServerStarted(serverURL); err != nil {
		return diag.Wrap(diag.KindPlugin, "run server started hooks", err)
	}

	if s.cfg.Server.AutoOpenBrowser {
		go s.openBrowserAfterStartup(serverURL)
	}

	srv := s.newHTTPServer(s.newMux())
	go s.shutdownOnContextDone(ctx, srv)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return diag.Wrap(diag.KindServe, "listen and serve", err)
	}

	return nil
}

func (s *Server) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	if s.cfg.Server.LiveReload {
		mux.HandleFunc("/__reload", s.handleReload)
	}
	if s.cfg.Server.DebugRoutes {
		mux.HandleFunc("/__debug/deps", s.handleDepsDebug)
	}

	mux.HandleFunc(s.cfg.Feed.RSSPath, s.handleRSS)
	mux.HandleFunc(s.cfg.Feed.SitemapPath, s.handleSitemap)

	for _, prefix := range []string{"/assets/", "/images/", "/uploads/", "/theme/", "/plugins/"} {
		mux.Handle(prefix, http.StripPrefix("/", http.FileServer(http.Dir(s.cfg.PublicDir))))
	}

	s.hooks.RegisterRoutes(mux)
	mux.HandleFunc("/", s.handlePage)

	return mux
}

func (s *Server) newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              s.cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func (s *Server) shutdownOnContextDone(ctx context.Context, srv *http.Server) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func (s *Server) openBrowserAfterStartup(serverURL string) {
	time.Sleep(250 * time.Millisecond)
	if err := openBrowser(serverURL); err != nil {
		logx.Warn("open browser failed", "url", serverURL, "error", err)
	}
}

func (s *Server) listenURL() string {
	addr := s.cfg.Server.Addr
	switch {
	case strings.HasPrefix(addr, ":"):
		return "http://localhost" + addr
	case strings.HasPrefix(addr, "127.0.0.1:"), strings.HasPrefix(addr, "localhost:"):
		return "http://" + addr
	case strings.HasPrefix(addr, "http://"), strings.HasPrefix(addr, "https://"):
		return addr
	default:
		return "http://" + addr
	}
}

func (s *Server) rebuild(ctx context.Context) error {
	graph, err := s.loadPreparedGraph(ctx)
	if err != nil {
		return err
	}
	if err := s.syncAssets(); err != nil {
		return err
	}

	s.updateGraphState(graph)
	logx.Info("site rebuilt", "documents", len(graph.Documents), "routes", len(graph.ByURL))
	s.signalReload()
	return nil
}

func (s *Server) incrementalRebuild(ctx context.Context, changes deps.ChangeSet) error {
	oldDepGraph := s.currentDepGraph()

	if len(changes.Assets) > 0 {
		if err := s.syncAssets(); err != nil {
			return err
		}
	}

	if oldDepGraph == nil || changes.Full {
		logx.Debug("performing full rebuild")
		return s.rebuild(ctx)
	}

	if !hasRenderableChanges(changes) {
		logx.Debug("asset-only change detected", "asset_count", len(changes.Assets))
		s.signalReload()
		return nil
	}

	plan := deps.ResolveRebuildPlan(oldDepGraph, changes)
	if plan.FullRebuild {
		logx.Debug("dependency plan requested full rebuild")
		return s.rebuild(ctx)
	}

	graph, err := s.loadPreparedGraph(ctx)
	if err != nil {
		return err
	}
	if len(plan.OutputURLs) > 0 {
		if err := s.renderer.BuildURLs(ctx, graph, plan.OutputURLs); err != nil {
			return diag.Wrap(diag.KindRender, "build urls", err)
		}
	}

	s.updateGraphState(graph)
	logx.Debug("incremental rebuild complete", "output_count", len(plan.OutputURLs))
	s.signalReload()
	return nil
}

func (s *Server) loadPreparedGraph(ctx context.Context) (*content.SiteGraph, error) {
	graph, err := s.loader.Load(ctx)
	if err != nil {
		return nil, diag.Wrap(diag.KindBuild, "load site graph", err)
	}
	if err := s.router.AssignURLs(graph); err != nil {
		return nil, diag.Wrap(diag.KindBuild, "assign urls", err)
	}
	if err := s.hooks.OnRoutesAssigned(graph); err != nil {
		return nil, diag.Wrap(diag.KindPlugin, "run route hooks", err)
	}
	return graph, nil
}

func (s *Server) syncAssets() error {
	if err := assets.Sync(s.cfg, s.hooks); err != nil {
		return diag.Wrap(diag.KindIO, "sync assets", err)
	}
	return nil
}

func (s *Server) updateGraphState(graph *content.SiteGraph) {
	s.mu.Lock()
	s.graph = graph
	s.depGraph = deps.BuildSiteDependencyGraph(graph, s.cfg.Theme)
	s.mu.Unlock()
}

func (s *Server) currentDepGraph() *deps.Graph {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.depGraph
}

func hasRenderableChanges(changes deps.ChangeSet) bool {
	return changes.Full ||
		len(changes.Sources) > 0 ||
		len(changes.Templates) > 0 ||
		len(changes.DataKeys) > 0
}

func (s *Server) signalReload() {
	select {
	case s.reloadSignal <- struct{}{}:
	default:
	}
}

func (s *Server) watch(ctx context.Context) {
	w, err := content.NewWatcher()
	if err != nil {
		logx.Warn("watcher setup failed", "error", err)
		return
	}
	defer func(w *fsnotify.Watcher) {
		if err := w.Close(); err != nil {
			logx.Warn("close watcher failed", "error", err)
		}
	}(w)

	s.walkWatchRoots(w)

	var changedPaths []string
	var debounce <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-w.Events:
			if ev.Op != 0 {
				if shouldAddWatch(ev.Name) {
					_ = addWatchRecursively(w, ev.Name)
				}
				changedPaths = append(changedPaths, ev.Name)
				debounce = time.After(250 * time.Millisecond)
			}
		case <-debounce:
			if len(changedPaths) == 0 {
				continue
			}
			changeSet := s.classifyChanges(changedPaths)
			if err := s.incrementalRebuild(ctx, changeSet); err != nil {
				logx.Error("incremental rebuild failed", "kind", diag.KindOf(err), "error", err)
			}
			changedPaths = nil
			debounce = nil
		case err := <-w.Errors:
			if err != nil {
				logx.Warn("watcher error", "error", err)
			}
		}
	}
}

func (s *Server) watchRoots() []string {
	return []string{
		s.cfg.ContentDir,
		s.cfg.ThemesDir,
		s.cfg.DataDir,
		s.cfg.PluginsDir,
		filepath.Join(s.cfg.ContentDir, "config"),
	}
}

type watcherAdder interface {
	Add(string) error
}

func (s *Server) walkWatchRoots(w watcherAdder) {
	for _, root := range s.watchRoots() {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = w.Add(path)
			}
			return nil
		})
	}
}

func shouldAddWatch(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func addWatchRecursively(w watcherAdder, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			_ = w.Add(path)
		}
		return nil
	})
}

func (s *Server) classifyChanges(paths []string) deps.ChangeSet {
	changes := deps.ChangeSet{
		Sources:   make([]string, 0),
		Templates: make([]string, 0),
		DataKeys:  make([]string, 0),
		Assets:    make([]string, 0),
	}

	roots := s.changeRoots()

	for _, path := range paths {
		clean := filepath.ToSlash(path)

		switch {
		case clean == roots["config"]:
			changes.Full = true
		case strings.HasPrefix(clean, roots["pages"]+"/"), strings.HasPrefix(clean, roots["posts"]+"/"):
			changes.Sources = append(changes.Sources, clean)
		case strings.HasPrefix(clean, roots["theme_layouts"]+"/"):
			changes.Templates = append(changes.Templates, clean)
		case strings.HasPrefix(clean, roots["content_assets"]+"/"),
			strings.HasPrefix(clean, roots["content_images"]+"/"),
			strings.HasPrefix(clean, roots["content_uploads"]+"/"),
			strings.HasPrefix(clean, roots["theme_assets"]+"/"):
			changes.Assets = append(changes.Assets, clean)
		case strings.HasPrefix(clean, roots["data"]+"/"):
			key, err := s.classifyDataKey(path)
			if err != nil {
				changes.Full = true
				continue
			}
			changes.DataKeys = append(changes.DataKeys, key)
		case strings.HasPrefix(clean, roots["plugins"]+"/"):
			changes.Full = true
		case strings.HasPrefix(clean, roots["themes"]+"/"), strings.HasPrefix(clean, roots["content"]+"/"):
			changes.Full = true
		default:
			changes.Full = true
		}
	}

	return changes
}

func (s *Server) changeRoots() map[string]string {
	return map[string]string{
		"config":          filepath.ToSlash(s.contentConfigPath()),
		"pages":           filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.PagesDir)),
		"posts":           filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.PostsDir)),
		"content_assets":  filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.AssetsDir)),
		"content_images":  filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.ImagesDir)),
		"content_uploads": filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.UploadsDir)),
		"theme_assets":    filepath.ToSlash(filepath.Join(s.cfg.ThemesDir, s.cfg.Theme, "assets")),
		"theme_layouts":   filepath.ToSlash(filepath.Join(s.cfg.ThemesDir, s.cfg.Theme, "layouts")),
		"data":            filepath.ToSlash(s.cfg.DataDir),
		"plugins":         filepath.ToSlash(s.cfg.PluginsDir),
		"themes":          filepath.ToSlash(s.cfg.ThemesDir),
		"content":         filepath.ToSlash(s.cfg.ContentDir),
	}
}

func (s *Server) contentConfigPath() string {
	return filepath.Join(s.cfg.ContentDir, "config", "site.yaml")
}

func (s *Server) classifyDataKey(path string) (string, error) {
	rel, err := filepath.Rel(s.cfg.DataDir, path)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel)), nil
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-s.reloadSignal:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"reload":true}`)
			flusher.Flush()
		}
	}
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()

	path := r.URL.Path
	if !strings.HasSuffix(path, "/") && !strings.Contains(filepath.Base(path), ".") {
		path += "/"
	}

	out, err := s.renderer.RenderURL(graph, path, s.cfg.Server.LiveReload)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		logx.Error("render page failed", "path", path, "error", err)
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		http.Error(w, string(b), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out)
}
