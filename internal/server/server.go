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

	mux := http.NewServeMux()

	if s.cfg.Server.LiveReload {
		mux.HandleFunc("/__reload", s.handleReload)
	}

	if s.cfg.Server.DebugRoutes {
		mux.HandleFunc("/__debug/deps", s.handleDepsDebug)
	}

	mux.HandleFunc(s.cfg.Feed.RSSPath, s.handleRSS)
	mux.HandleFunc(s.cfg.Feed.SitemapPath, s.handleSitemap)

	mux.Handle("/assets/", http.StripPrefix("/", http.FileServer(http.Dir(s.cfg.PublicDir))))
	mux.Handle("/images/", http.StripPrefix("/", http.FileServer(http.Dir(s.cfg.PublicDir))))
	mux.Handle("/uploads/", http.StripPrefix("/", http.FileServer(http.Dir(s.cfg.PublicDir))))
	mux.Handle("/theme/", http.StripPrefix("/", http.FileServer(http.Dir(s.cfg.PublicDir))))
	mux.Handle("/plugins/", http.StripPrefix("/", http.FileServer(http.Dir(s.cfg.PublicDir))))

	s.hooks.RegisterRoutes(mux)

	mux.HandleFunc("/", s.handlePage)

	serverURL := s.listenURL()
	logx.Info("Foundry is starting",
		"url", serverURL,
		"theme", s.cfg.Theme,
		"preview", s.preview,
		"live_reload", s.cfg.Server.LiveReload,
	)

	if err := s.hooks.OnServerStarted(serverURL); err != nil {
		return diag.Wrap(diag.KindPlugin, "run server started hooks", err)
	}

	if s.cfg.Server.AutoOpenBrowser {
		go func() {
			time.Sleep(250 * time.Millisecond)
			if err := openBrowser(serverURL); err != nil {
				logx.Warn("open browser failed", "url", serverURL, "error", err)
			}
		}()
	}

	err := http.ListenAndServe(s.cfg.Server.Addr, mux)
	if err != nil {
		return diag.Wrap(diag.KindServe, "listen and serve", err)
	}
	return nil
}

func (s *Server) listenURL() string {
	addr := s.cfg.Server.Addr
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	if strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "localhost:") {
		return "http://" + addr
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}

func (s *Server) rebuild(ctx context.Context) error {
	graph, err := s.loader.Load(ctx)
	if err != nil {
		return diag.Wrap(diag.KindBuild, "load site graph", err)
	}
	if err := s.router.AssignURLs(graph); err != nil {
		return diag.Wrap(diag.KindBuild, "assign urls", err)
	}
	if err := s.hooks.OnRoutesAssigned(graph); err != nil {
		return diag.Wrap(diag.KindPlugin, "run route hooks", err)
	}
	if err := assets.Sync(s.cfg, s.hooks); err != nil {
		return diag.Wrap(diag.KindIO, "sync assets", err)
	}

	depGraph := deps.BuildSiteDependencyGraph(graph, s.cfg.Theme)

	s.mu.Lock()
	s.graph = graph
	s.depGraph = depGraph
	s.mu.Unlock()

	logx.Info("site graph rebuilt", "documents", len(graph.Documents), "routes", len(graph.ByURL))

	s.signalReload()
	return nil
}

func (s *Server) incrementalRebuild(ctx context.Context, changes deps.ChangeSet) error {
	s.mu.RLock()
	oldDepGraph := s.depGraph
	s.mu.RUnlock()

	if len(changes.Assets) > 0 {
		if err := assets.Sync(s.cfg, s.hooks); err != nil {
			return diag.Wrap(diag.KindIO, "sync assets", err)
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

	graph, err := s.loader.Load(ctx)
	if err != nil {
		return diag.Wrap(diag.KindBuild, "load site graph", err)
	}
	if err := s.router.AssignURLs(graph); err != nil {
		return diag.Wrap(diag.KindBuild, "assign urls", err)
	}
	if err := s.hooks.OnRoutesAssigned(graph); err != nil {
		return diag.Wrap(diag.KindPlugin, "run route hooks", err)
	}

	if len(plan.OutputURLs) > 0 {
		if err := s.renderer.BuildURLs(ctx, graph, plan.OutputURLs); err != nil {
			return diag.Wrap(diag.KindRender, "build urls", err)
		}
	}

	if err := s.renderer.BuildTaxonomyArchives(ctx, graph); err != nil {
		return diag.Wrap(diag.KindRender, "build taxonomy archives", err)
	}

	depGraph := deps.BuildSiteDependencyGraph(graph, s.cfg.Theme)

	s.mu.Lock()
	s.graph = graph
	s.depGraph = depGraph
	s.mu.Unlock()

	logx.Debug("incremental rebuild complete", "output_count", len(plan.OutputURLs))

	s.signalReload()
	return nil
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

	paths := []string{
		s.cfg.ContentDir,
		s.cfg.ThemesDir,
		s.cfg.DataDir,
		s.cfg.PluginsDir,
		filepath.Join(s.cfg.ContentDir, "config"),
	}

	for _, p := range paths {
		_ = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = w.Add(path)
			}
			return nil
		})
	}

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

func shouldAddWatch(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func addWatchRecursively(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			_ = w.Add(path)
		}
		return nil
	})
}

func (s *Server) classifyChanges(paths []string) deps.ChangeSet {
	changes := deps.ChangeSet{}

	for _, p := range paths {
		p = filepath.ToSlash(p)

		switch {
		case strings.HasPrefix(p, filepath.ToSlash(s.cfg.ContentDir)+"/"):
			if strings.HasSuffix(p, ".md") {
				changes.Sources = append(changes.Sources, p)
			} else if strings.HasPrefix(p, filepath.ToSlash(s.cfg.ContentDir)+"/assets/") ||
				strings.HasPrefix(p, filepath.ToSlash(s.cfg.ContentDir)+"/images/") ||
				strings.HasPrefix(p, filepath.ToSlash(s.cfg.ContentDir)+"/uploads/") {
				changes.Assets = append(changes.Assets, p)
			} else if strings.HasPrefix(p, filepath.ToSlash(s.cfg.ContentDir)+"/config/") {
				changes.Full = true
			}

		case strings.HasPrefix(p, filepath.ToSlash(s.cfg.ThemesDir)+"/"):
			if strings.Contains(p, "/layouts/") && strings.HasSuffix(p, ".html") {
				changes.Templates = append(changes.Templates, p)
			} else if strings.Contains(p, "/assets/") {
				changes.Assets = append(changes.Assets, p)
			} else {
				changes.Full = true
			}

		case strings.HasPrefix(p, filepath.ToSlash(s.cfg.DataDir)+"/"):
			rel, err := filepath.Rel(s.cfg.DataDir, p)
			if err == nil {
				key := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
				key = strings.TrimPrefix(key, "./")
				changes.DataKeys = append(changes.DataKeys, key)
			} else {
				changes.Full = true
			}

		case strings.HasPrefix(p, filepath.ToSlash(s.cfg.PluginsDir)+"/"):
			if strings.Contains(p, "/assets/") {
				changes.Assets = append(changes.Assets, p)
			} else {
				changes.Full = true
			}

		default:
			changes.Full = true
		}
	}

	changes.Sources = uniq(changes.Sources)
	changes.Templates = uniq(changes.Templates)
	changes.DataKeys = uniq(changes.DataKeys)
	changes.Assets = uniq(changes.Assets)

	return changes
}

func uniq(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()

	if graph == nil {
		http.Error(w, "site graph unavailable", http.StatusServiceUnavailable)
		return
	}

	path := r.URL.Path
	doc, ok := graph.ByURL[path]
	if !ok {
		data, handled := s.renderer.RenderTaxonomyArchiveData(graph, path, s.cfg.Server.LiveReload)
		if handled {
			html, err := s.renderer.RenderData(r.Context(), path, data)
			if err != nil {
				http.Error(w, fmt.Sprintf("render taxonomy archive: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(html)
			return
		}

		http.NotFound(w, r)
		return
	}

	html, err := s.renderer.RenderPage(r.Context(), graph, doc.URL)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}

		http.Error(w, fmt.Sprintf("render page: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(html)
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
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

func (s *Server) handleDepsDebug(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	graph := s.depGraph
	s.mu.RUnlock()

	if graph == nil {
		http.Error(w, "dependency graph unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(graph.Export())
}
