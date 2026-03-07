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
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
)

type Loader interface {
	Load(context.Context) (*content.SiteGraph, error)
}

type Server struct {
	cfg          *config.Config
	loader       Loader
	router       *router.Resolver
	renderer     *renderer.Renderer
	preview      bool
	mu           sync.RWMutex
	graph        *content.SiteGraph
	depGraph     *deps.Graph
	reloadSignal chan struct{}
}

func New(cfg *config.Config, loader Loader, router *router.Resolver, renderer *renderer.Renderer, preview bool) *Server {
	return &Server{
		cfg:          cfg,
		loader:       loader,
		router:       router,
		renderer:     renderer,
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

	mux.HandleFunc("/", s.handlePage)

	serverURL := s.listenURL()
	fmt.Printf("Foundry running at %s\n", serverURL)
	fmt.Printf("theme=%s preview=%t liveReload=%t\n", s.cfg.Theme, s.preview, s.cfg.Server.LiveReload)

	if s.cfg.Server.AutoOpenBrowser {
		go func() {
			time.Sleep(250 * time.Millisecond)
			_ = openBrowser(serverURL)
		}()
	}

	return http.ListenAndServe(s.cfg.Server.Addr, mux)
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
		return fmt.Errorf("load site graph: %w", err)
	}
	if err := s.router.AssignURLs(graph); err != nil {
		return fmt.Errorf("assign urls: %w", err)
	}
	if err := assets.Sync(s.cfg); err != nil {
		return fmt.Errorf("sync assets: %w", err)
	}

	depGraph := deps.BuildSiteDependencyGraph(graph, s.cfg.Theme)

	s.mu.Lock()
	s.graph = graph
	s.depGraph = depGraph
	s.mu.Unlock()

	s.signalReload()
	return nil
}

func (s *Server) incrementalRebuild(ctx context.Context, changes deps.ChangeSet) error {
	s.mu.RLock()
	oldDepGraph := s.depGraph
	s.mu.RUnlock()

	if len(changes.Assets) > 0 {
		if err := assets.Sync(s.cfg); err != nil {
			return fmt.Errorf("sync assets: %w", err)
		}
	}

	if oldDepGraph == nil || changes.Full {
		return s.rebuild(ctx)
	}

	if !hasRenderableChanges(changes) {
		s.signalReload()
		return nil
	}

	plan := deps.ResolveRebuildPlan(oldDepGraph, changes)
	if plan.FullRebuild {
		return s.rebuild(ctx)
	}

	graph, err := s.loader.Load(ctx)
	if err != nil {
		return fmt.Errorf("load site graph: %w", err)
	}
	if err := s.router.AssignURLs(graph); err != nil {
		return fmt.Errorf("assign urls: %w", err)
	}

	if len(plan.OutputURLs) > 0 {
		if err := s.renderer.BuildURLs(ctx, graph, plan.OutputURLs); err != nil {
			return fmt.Errorf("build urls: %w", err)
		}
		if err := s.renderer.BuildTaxonomyArchives(ctx, graph); err != nil {
			return fmt.Errorf("build taxonomy archives: %w", err)
		}
	}

	depGraph := deps.BuildSiteDependencyGraph(graph, s.cfg.Theme)

	s.mu.Lock()
	s.graph = graph
	s.depGraph = depGraph
	s.mu.Unlock()

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
		return
	}
	defer func(w *fsnotify.Watcher) {
		err := w.Close()
		if err != nil {
			_ = fmt.Errorf("close watcher: %w", err)
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
			_ = s.incrementalRebuild(ctx, changeSet)
			changedPaths = nil
			debounce = nil

		case <-w.Errors:
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

type watcherAdder interface {
	Add(string) error
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

	contentDir := filepath.ToSlash(s.cfg.ContentDir)
	themesDir := filepath.ToSlash(s.cfg.ThemesDir)
	dataDir := filepath.ToSlash(s.cfg.DataDir)
	configPath := filepath.ToSlash(filepath.Join(s.cfg.ContentDir, "config", "site.yaml"))
	pagesRoot := filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.PagesDir))
	postsRoot := filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.PostsDir))
	contentAssetsRoot := filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.AssetsDir))
	contentImagesRoot := filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.ImagesDir))
	contentUploadsRoot := filepath.ToSlash(filepath.Join(s.cfg.ContentDir, s.cfg.Content.UploadsDir))
	themeAssetsRoot := filepath.ToSlash(filepath.Join(s.cfg.ThemesDir, s.cfg.Theme, "assets"))
	themeLayoutsRoot := filepath.ToSlash(filepath.Join(s.cfg.ThemesDir, s.cfg.Theme, "layouts"))

	for _, path := range paths {
		clean := filepath.ToSlash(path)

		switch {
		case clean == configPath:
			changes.Full = true

		case strings.HasPrefix(clean, pagesRoot+"/") || strings.HasPrefix(clean, postsRoot+"/"):
			changes.Sources = append(changes.Sources, clean)

		case strings.HasPrefix(clean, themeLayoutsRoot+"/"):
			changes.Templates = append(changes.Templates, clean)

		case strings.HasPrefix(clean, contentAssetsRoot+"/"),
			strings.HasPrefix(clean, contentImagesRoot+"/"),
			strings.HasPrefix(clean, contentUploadsRoot+"/"),
			strings.HasPrefix(clean, themeAssetsRoot+"/"):
			changes.Assets = append(changes.Assets, clean)

		case strings.HasPrefix(clean, dataDir+"/"):
			rel, err := filepath.Rel(s.cfg.DataDir, path)
			if err == nil {
				key := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
				changes.DataKeys = append(changes.DataKeys, key)
			} else {
				changes.Full = true
			}

		case strings.HasPrefix(clean, filepath.ToSlash(s.cfg.PluginsDir)+"/"):
			changes.Full = true

		case strings.HasPrefix(clean, themesDir+"/"), strings.HasPrefix(clean, contentDir+"/"):
			changes.Full = true

		default:
			changes.Full = true
		}
	}

	return changes
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
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		http.Error(w, string(b), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out)
}
