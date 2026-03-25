package service

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/site"
)

// FileSystem abstracts filesystem access for admin service operations so tests
// can substitute a controlled implementation.
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(name string) error
}

type osFS struct{}

func (osFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (osFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (osFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (osFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }
func (osFS) Remove(name string) error                     { return os.Remove(name) }

// GraphLoader loads a site graph for admin read operations.
//
// The boolean controls whether draft content should be included.
type GraphLoader func(context.Context, *config.Config, bool) (*content.SiteGraph, error)

// StatusProvider contributes one cohesive section of data to the admin status
// dashboard.
type StatusProvider interface {
	Name() string
	Provide(context.Context, *Service, *types.SystemStatus) error
}

// Service is the main business-logic layer behind the admin API.
//
// It owns filesystem access, graph loading, plugin metadata access, and a
// short-lived graph cache used by multiple admin endpoints.
type Service struct {
	cfg             *config.Config
	fs              FileSystem
	loadGraph       GraphLoader
	pluginMetadata  func() map[string]plugins.Metadata
	mu              sync.RWMutex
	lockMu          sync.Mutex
	statusProviders map[string]StatusProvider
	graphCache      map[bool]cachedGraph
}

type cachedGraph struct {
	graph    *content.SiteGraph
	loadedAt time.Time
}

const graphCacheTTL = time.Second

// Option customizes Service construction for tests and embedding.
type Option func(*Service)

// WithFS overrides the filesystem implementation used by the service.
func WithFS(fs FileSystem) Option {
	return func(s *Service) {
		if fs != nil {
			s.fs = fs
		}
	}
}

// WithGraphLoader overrides the graph loader used by the service.
func WithGraphLoader(loader GraphLoader) Option {
	return func(s *Service) {
		if loader != nil {
			s.loadGraph = loader
		}
	}
}

// WithPluginMetadata overrides plugin metadata lookup used by admin extension
// and plugin-management views.
func WithPluginMetadata(loader func() map[string]plugins.Metadata) Option {
	return func(s *Service) {
		if loader != nil {
			s.pluginMetadata = loader
		}
	}
}

// New constructs the admin service with default providers and a default graph
// loader based on the site package.
func New(cfg *config.Config, opts ...Option) *Service {
	s := &Service{
		cfg:             cfg,
		fs:              osFS{},
		statusProviders: make(map[string]StatusProvider),
		graphCache:      make(map[bool]cachedGraph),
		loadGraph: func(ctx context.Context, cfg *config.Config, includeDrafts bool) (*content.SiteGraph, error) {
			graph, _, err := site.LoadConfiguredGraph(ctx, cfg, includeDrafts)
			return graph, err
		},
		pluginMetadata: func() map[string]plugins.Metadata {
			return map[string]plugins.Metadata{}
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.RegisterStatusProvider(configStatusProvider{})
	s.RegisterStatusProvider(contentStatusProvider{})
	s.RegisterStatusProvider(themeStatusProvider{})
	s.RegisterStatusProvider(pluginStatusProvider{})
	s.RegisterStatusProvider(taxonomyStatusProvider{})

	return s
}

// Config returns the service's active site configuration.
func (s *Service) Config() *config.Config {
	return s.cfg
}

// RegisterStatusProvider adds or replaces a named dashboard status provider.
func (s *Service) RegisterStatusProvider(provider StatusProvider) {
	if provider == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusProviders[provider.Name()] = provider
}

// load returns a cached site graph when possible and reloads it when the cache
// has expired.
func (s *Service) load(ctx context.Context, includeDrafts bool) (*content.SiteGraph, error) {
	s.mu.RLock()
	cached, ok := s.graphCache[includeDrafts]
	s.mu.RUnlock()
	if ok && cached.graph != nil && time.Since(cached.loadedAt) < graphCacheTTL {
		return cached.graph, nil
	}

	graph, err := s.loadGraph(ctx, s.cfg, includeDrafts)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.graphCache[includeDrafts] = cachedGraph{
		graph:    graph,
		loadedAt: time.Now(),
	}
	s.mu.Unlock()

	return graph, nil
}

// providers returns the registered status providers in unspecified order.
func (s *Service) providers() []StatusProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]StatusProvider, 0, len(s.statusProviders))
	for _, p := range s.statusProviders {
		out = append(out, p)
	}
	return out
}

// invalidateGraphCache clears all cached graph variants after content-affecting
// admin operations.
func (s *Service) invalidateGraphCache() {
	s.mu.Lock()
	s.graphCache = make(map[bool]cachedGraph)
	s.mu.Unlock()
}
