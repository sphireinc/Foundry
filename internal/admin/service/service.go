package service

import (
	"context"
	"os"
	"sync"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/site"
)

type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
}

type osFS struct{}

func (osFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (osFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (osFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }

type GraphLoader func(context.Context, *config.Config, bool) (*content.SiteGraph, error)

type StatusProvider interface {
	Name() string
	Provide(context.Context, *Service, *types.SystemStatus) error
}

type Service struct {
	cfg             *config.Config
	fs              FileSystem
	loadGraph       GraphLoader
	mu              sync.RWMutex
	statusProviders map[string]StatusProvider
}

type Option func(*Service)

func WithFS(fs FileSystem) Option {
	return func(s *Service) {
		if fs != nil {
			s.fs = fs
		}
	}
}

func WithGraphLoader(loader GraphLoader) Option {
	return func(s *Service) {
		if loader != nil {
			s.loadGraph = loader
		}
	}
}

func New(cfg *config.Config, opts ...Option) *Service {
	s := &Service{
		cfg:             cfg,
		fs:              osFS{},
		statusProviders: make(map[string]StatusProvider),
		loadGraph: func(ctx context.Context, cfg *config.Config, includeDrafts bool) (*content.SiteGraph, error) {
			graph, _, err := site.LoadConfiguredGraph(ctx, cfg, includeDrafts)
			return graph, err
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

func (s *Service) Config() *config.Config {
	return s.cfg
}

func (s *Service) RegisterStatusProvider(provider StatusProvider) {
	if provider == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusProviders[provider.Name()] = provider
}

func (s *Service) load(ctx context.Context, includeDrafts bool) (*content.SiteGraph, error) {
	return s.loadGraph(ctx, s.cfg, includeDrafts)
}

func (s *Service) providers() []StatusProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]StatusProvider, 0, len(s.statusProviders))
	for _, p := range s.statusProviders {
		out = append(out, p)
	}
	return out
}
