package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	debug        bool
	activeReqs   atomic.Int64
	connMu       sync.Mutex
	connStates   map[net.Conn]http.ConnState
	mu           sync.RWMutex
	graph        *content.SiteGraph
	depGraph     *deps.Graph
	reloadSignal chan struct{}
	reloadVer    atomic.Uint64
}

type Option func(*Server)

var requestSequence atomic.Uint64

const slowServeRequestInterval = 2 * time.Second
const debugHeartbeatInterval = 2 * time.Second

type runtimeSnapshot struct {
	HeapAllocBytes   uint64
	HeapInuseBytes   uint64
	StackInuseBytes  uint64
	SysBytes         uint64
	NumGC            uint32
	Goroutines       int
	ActiveRequests   int64
	ProcessUserCPU   time.Duration
	ProcessSystemCPU time.Duration
}

func WithDebugMode(enabled bool) Option {
	return func(s *Server) {
		s.debug = enabled
	}
}

func New(
	cfg *config.Config,
	loader Loader,
	router *router.Resolver,
	renderer *renderer.Renderer,
	hooks Hooks,
	preview bool,
	opts ...Option,
) *Server {
	if hooks == nil {
		hooks = noopHooks{}
	}

	s := &Server{
		cfg:          cfg,
		loader:       loader,
		router:       router,
		renderer:     renderer,
		hooks:        hooks,
		preview:      preview,
		connStates:   make(map[net.Conn]http.ConnState),
		reloadSignal: make(chan struct{}, 1),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	return s
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
	if s.debug {
		go s.debugHeartbeat(ctx)
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
		mux.HandleFunc("/__reload/poll", s.handleReloadPoll)
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

	if s.debug {
		return s.wrapDebugHTTP(mux)
	}

	return mux
}

func (s *Server) newHTTPServer(handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:              s.cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if s.debug {
		srv.ConnState = s.onConnState
	}
	return srv
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
	start := time.Now()
	before := s.captureRuntimeSnapshot()
	if s.debug {
		logx.Info("serve rebuild started", append([]any{"preview", s.preview}, before.logFields("runtime_")...)...)
	}

	graph, err := s.loadPreparedGraph(ctx)
	if err != nil {
		return err
	}
	if err := s.syncAssets(); err != nil {
		return err
	}

	s.updateGraphState(graph)
	logx.Info("site rebuilt", "documents", len(graph.Documents), "routes", len(graph.ByURL))
	if s.debug {
		after := s.captureRuntimeSnapshot()
		args := []any{"elapsed", time.Since(start).String()}
		args = append(args, after.logFields("runtime_")...)
		args = append(args, after.deltaFields(before, time.Since(start), "delta_")...)
		logx.Info("serve rebuild finished", args...)
	}
	s.signalReload()
	return nil
}

func (s *Server) incrementalRebuild(ctx context.Context, changes deps.ChangeSet) error {
	start := time.Now()
	before := s.captureRuntimeSnapshot()
	if s.debug {
		args := []any{
			"full", changes.Full,
			"sources", len(changes.Sources),
			"templates", len(changes.Templates),
			"data_keys", len(changes.DataKeys),
			"assets", len(changes.Assets),
		}
		args = append(args, before.logFields("runtime_")...)
		logx.Info("serve incremental rebuild started", args...)
	}

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
	if s.debug {
		after := s.captureRuntimeSnapshot()
		args := []any{
			"outputs", len(plan.OutputURLs),
			"elapsed", time.Since(start).String(),
		}
		args = append(args, after.logFields("runtime_")...)
		args = append(args, after.deltaFields(before, time.Since(start), "delta_")...)
		logx.Info("serve incremental rebuild finished", args...)
	}
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
	s.reloadVer.Add(1)
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

	lastSeen := s.reloadVer.Load()
	notify := r.Context().Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-notify:
			return
		case <-s.reloadSignal:
			if s.writeReloadEvent(w, flusher, &lastSeen) {
				return
			}
		case <-ticker.C:
			if s.writeReloadEvent(w, flusher, &lastSeen) {
				return
			}
		}
	}
}

func (s *Server) handleReloadPoll(w http.ResponseWriter, r *http.Request) {
	current := s.reloadVer.Load()
	since := parseReloadVersion(r.URL.Query().Get("since"))

	payload := map[string]any{
		"version": current,
		"reload":  since > 0 && current > since,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) writeReloadEvent(w http.ResponseWriter, flusher http.Flusher, lastSeen *uint64) bool {
	current := s.reloadVer.Load()
	if current <= *lastSeen {
		return false
	}
	*lastSeen = current
	if _, err := fmt.Fprintf(w, "data: %s\n\n", `{"reload":true}`); err != nil {
		return true
	}
	flusher.Flush()
	return false
}

func parseReloadVersion(raw string) uint64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()

	path := r.URL.Path
	if !strings.HasSuffix(path, "/") && !strings.Contains(filepath.Base(path), ".") {
		path += "/"
	}

	_, finishDebug := s.beginDebugRequest(r, path)
	out, err := s.renderer.RenderURL(graph, path, s.cfg.Server.LiveReload)
	finishDebug(err, len(out))
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

func (s *Server) wrapDebugHTTP(next http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := requestSequence.Add(1)
		start := time.Now()
		before := s.captureRuntimeSnapshot()

		args := []any{
			"http_request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		}
		args = append(args, before.logFields("runtime_")...)
		args = append(args, s.connectionStateFields("connections_")...)
		logx.Info("http request started", args...)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		after := s.captureRuntimeSnapshot()
		finishArgs := []any{
			"http_request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"elapsed", time.Since(start).String(),
		}
		finishArgs = append(finishArgs, after.logFields("runtime_")...)
		finishArgs = append(finishArgs, after.deltaFields(before, time.Since(start), "delta_")...)
		finishArgs = append(finishArgs, s.connectionStateFields("connections_")...)
		logx.Info("http request finished", finishArgs...)
	}))
	return mux
}

func (s *Server) beginDebugRequest(r *http.Request, path string) (uint64, func(error, int)) {
	if !s.debug {
		return 0, func(error, int) {}
	}

	reqID := requestSequence.Add(1)
	start := time.Now()
	s.activeReqs.Add(1)
	before := s.captureRuntimeSnapshot()
	done := make(chan struct{})

	args := []any{
		"request_id", reqID,
		"method", r.Method,
		"path", path,
		"remote_addr", r.RemoteAddr,
	}
	args = append(args, before.logFields("runtime_")...)
	args = append(args, s.connectionStateFields("connections_")...)
	logx.Info("serve request started", args...)

	go func() {
		ticker := time.NewTicker(slowServeRequestInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				snapshot := s.captureRuntimeSnapshot()
				args := []any{
					"request_id", reqID,
					"path", path,
					"elapsed", time.Since(start).String(),
				}
				args = append(args, snapshot.logFields("runtime_")...)
				args = append(args, snapshot.deltaFields(before, time.Since(start), "delta_")...)
				args = append(args, s.connectionStateFields("connections_")...)
				logx.Warn("serve request still rendering", args...)
			}
		}
	}()

	return reqID, func(err error, bytes int) {
		close(done)
		s.activeReqs.Add(-1)
		after := s.captureRuntimeSnapshot()
		elapsed := time.Since(start)

		args := []any{
			"request_id", reqID,
			"path", path,
			"elapsed", elapsed.String(),
			"bytes", bytes,
		}
		args = append(args, after.logFields("runtime_")...)
		args = append(args, after.deltaFields(before, elapsed, "delta_")...)
		args = append(args, s.connectionStateFields("connections_")...)
		if err != nil {
			args = append(args, "error", err)
		}

		logx.Info("serve request finished", args...)
	}
}

func (s *Server) debugHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(debugHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := s.captureRuntimeSnapshot()
			args := snapshot.logFields("runtime_")
			args = append(args, s.connectionStateFields("connections_")...)
			logx.Info("serve debug heartbeat", args...)
		}
	}
}

func (s *Server) onConnState(conn net.Conn, state http.ConnState) {
	s.connMu.Lock()
	prev, hadPrev := s.connStates[conn]
	if state == http.StateClosed || state == http.StateHijacked {
		delete(s.connStates, conn)
	} else {
		s.connStates[conn] = state
	}
	snapshot := s.connectionStateSnapshotLocked()
	s.connMu.Unlock()

	args := []any{
		"remote_addr", conn.RemoteAddr().String(),
		"state", state.String(),
	}
	if hadPrev {
		args = append(args, "previous_state", prev.String())
	}
	args = append(args, snapshot.logFields("connections_")...)
	logx.Info("http connection state", args...)
}

type connectionSnapshot struct {
	New      int
	Active   int
	Idle     int
	Hijacked int
	Closed   int
	Total    int
}

func (s *Server) connectionStateFields(prefix string) []any {
	s.connMu.Lock()
	snapshot := s.connectionStateSnapshotLocked()
	s.connMu.Unlock()
	return snapshot.logFields(prefix)
}

func (s *Server) connectionStateSnapshotLocked() connectionSnapshot {
	var snapshot connectionSnapshot
	for _, state := range s.connStates {
		switch state {
		case http.StateNew:
			snapshot.New++
		case http.StateActive:
			snapshot.Active++
		case http.StateIdle:
			snapshot.Idle++
		case http.StateHijacked:
			snapshot.Hijacked++
		case http.StateClosed:
			snapshot.Closed++
		}
	}
	snapshot.Total = len(s.connStates)
	return snapshot
}

func (s connectionSnapshot) logFields(prefix string) []any {
	return []any{
		prefix + "new", s.New,
		prefix + "active", s.Active,
		prefix + "idle", s.Idle,
		prefix + "hijacked", s.Hijacked,
		prefix + "closed", s.Closed,
		prefix + "total", s.Total,
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := r.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (s *Server) captureRuntimeSnapshot() runtimeSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	userCPU, systemCPU := processCPUTime()

	return runtimeSnapshot{
		HeapAllocBytes:   mem.HeapAlloc,
		HeapInuseBytes:   mem.HeapInuse,
		StackInuseBytes:  mem.StackInuse,
		SysBytes:         mem.Sys,
		NumGC:            mem.NumGC,
		Goroutines:       runtime.NumGoroutine(),
		ActiveRequests:   s.activeReqs.Load(),
		ProcessUserCPU:   userCPU,
		ProcessSystemCPU: systemCPU,
	}
}

func (s runtimeSnapshot) logFields(prefix string) []any {
	return []any{
		prefix + "heap_alloc_bytes", s.HeapAllocBytes,
		prefix + "heap_inuse_bytes", s.HeapInuseBytes,
		prefix + "stack_inuse_bytes", s.StackInuseBytes,
		prefix + "sys_bytes", s.SysBytes,
		prefix + "num_gc", s.NumGC,
		prefix + "goroutines", s.Goroutines,
		prefix + "active_requests", s.ActiveRequests,
		prefix + "process_user_cpu_ms", s.ProcessUserCPU.Milliseconds(),
		prefix + "process_system_cpu_ms", s.ProcessSystemCPU.Milliseconds(),
	}
}

func (s runtimeSnapshot) deltaFields(before runtimeSnapshot, elapsed time.Duration, prefix string) []any {
	userDelta := s.ProcessUserCPU - before.ProcessUserCPU
	systemDelta := s.ProcessSystemCPU - before.ProcessSystemCPU
	totalCPUPercent := 0.0
	if elapsed > 0 {
		totalCPUPercent = (float64(userDelta+systemDelta) / float64(elapsed)) * 100
	}

	return []any{
		prefix + "heap_alloc_bytes", int64(s.HeapAllocBytes) - int64(before.HeapAllocBytes),
		prefix + "heap_inuse_bytes", int64(s.HeapInuseBytes) - int64(before.HeapInuseBytes),
		prefix + "stack_inuse_bytes", int64(s.StackInuseBytes) - int64(before.StackInuseBytes),
		prefix + "sys_bytes", int64(s.SysBytes) - int64(before.SysBytes),
		prefix + "num_gc", int64(s.NumGC) - int64(before.NumGC),
		prefix + "goroutines", s.Goroutines - before.Goroutines,
		prefix + "active_requests", s.ActiveRequests - before.ActiveRequests,
		prefix + "process_user_cpu_ms", userDelta.Milliseconds(),
		prefix + "process_system_cpu_ms", systemDelta.Milliseconds(),
		prefix + "process_cpu_percent", fmt.Sprintf("%.2f", totalCPUPercent),
	}
}
