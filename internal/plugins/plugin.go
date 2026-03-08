package plugins

import (
	"net/http"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
)

type Plugin interface {
	Name() string
}

type Factory func() Plugin

type ConfigLoadedHook interface {
	OnConfigLoaded(*config.Config) error
}

type ContentDiscoveredHook interface {
	OnContentDiscovered(path string) error
}

type FrontmatterParsedHook interface {
	OnFrontmatterParsed(*content.Document) error
}

type MarkdownRenderedHook interface {
	OnMarkdownRendered(*content.Document) error
}

type DocumentParsedHook interface {
	OnDocumentParsed(*content.Document) error
}

type DataLoadedHook interface {
	OnDataLoaded(map[string]any) error
}

type GraphBuildingHook interface {
	OnGraphBuilding(*content.SiteGraph) error
}

type GraphBuiltHook interface {
	OnGraphBuilt(*content.SiteGraph) error
}

type TaxonomyBuiltHook interface {
	OnTaxonomyBuilt(*content.SiteGraph) error
}

type RoutesAssignedHook interface {
	OnRoutesAssigned(*content.SiteGraph) error
}

type ContextHook interface {
	OnContext(*renderer.ViewData) error
}

type BeforeRenderHook interface {
	OnBeforeRender(*renderer.ViewData) error
}

type AfterRenderHook interface {
	OnAfterRender(url string, html []byte) ([]byte, error)
}

type AssetsBuildingHook interface {
	OnAssetsBuilding(*config.Config) error
}

type BuildStartedHook interface {
	OnBuildStarted() error
}

type BuildCompletedHook interface {
	OnBuildCompleted(*content.SiteGraph) error
}

type ServerStartedHook interface {
	OnServerStarted(addr string) error
}

type RoutesRegisterHook interface {
	RegisterRoutes(mux *http.ServeMux)
}

type Manager struct {
	plugins []Plugin
}

func NewManager(enabled []string) *Manager {
	m := &Manager{
		plugins: make([]Plugin, 0),
	}

	for _, name := range enabled {
		if factory, ok := registry[name]; ok {
			m.plugins = append(m.plugins, factory())
		}
	}

	return m
}

func (m *Manager) Plugins() []Plugin {
	out := make([]Plugin, len(m.plugins))
	copy(out, m.plugins)
	return out
}

func (m *Manager) OnConfigLoaded(cfg *config.Config) error {
	for _, p := range m.plugins {
		if hook, ok := p.(ConfigLoadedHook); ok {
			if err := hook.OnConfigLoaded(cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnContentDiscovered(path string) error {
	for _, p := range m.plugins {
		if hook, ok := p.(ContentDiscoveredHook); ok {
			if err := hook.OnContentDiscovered(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnFrontmatterParsed(doc *content.Document) error {
	for _, p := range m.plugins {
		if hook, ok := p.(FrontmatterParsedHook); ok {
			if err := hook.OnFrontmatterParsed(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnMarkdownRendered(doc *content.Document) error {
	for _, p := range m.plugins {
		if hook, ok := p.(MarkdownRenderedHook); ok {
			if err := hook.OnMarkdownRendered(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnDocumentParsed(doc *content.Document) error {
	for _, p := range m.plugins {
		if hook, ok := p.(DocumentParsedHook); ok {
			if err := hook.OnDocumentParsed(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnDataLoaded(values map[string]any) error {
	for _, p := range m.plugins {
		if hook, ok := p.(DataLoadedHook); ok {
			if err := hook.OnDataLoaded(values); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnGraphBuilding(graph *content.SiteGraph) error {
	for _, p := range m.plugins {
		if hook, ok := p.(GraphBuildingHook); ok {
			if err := hook.OnGraphBuilding(graph); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnGraphBuilt(graph *content.SiteGraph) error {
	for _, p := range m.plugins {
		if hook, ok := p.(GraphBuiltHook); ok {
			if err := hook.OnGraphBuilt(graph); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnTaxonomyBuilt(graph *content.SiteGraph) error {
	for _, p := range m.plugins {
		if hook, ok := p.(TaxonomyBuiltHook); ok {
			if err := hook.OnTaxonomyBuilt(graph); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnRoutesAssigned(graph *content.SiteGraph) error {
	for _, p := range m.plugins {
		if hook, ok := p.(RoutesAssignedHook); ok {
			if err := hook.OnRoutesAssigned(graph); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnContext(ctx *renderer.ViewData) error {
	for _, p := range m.plugins {
		if hook, ok := p.(ContextHook); ok {
			if err := hook.OnContext(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnBeforeRender(ctx *renderer.ViewData) error {
	for _, p := range m.plugins {
		if hook, ok := p.(BeforeRenderHook); ok {
			if err := hook.OnBeforeRender(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnAfterRender(url string, html []byte) ([]byte, error) {
	out := html
	for _, p := range m.plugins {
		if hook, ok := p.(AfterRenderHook); ok {
			next, err := hook.OnAfterRender(url, out)
			if err != nil {
				return nil, err
			}
			out = next
		}
	}
	return out, nil
}

func (m *Manager) OnAssetsBuilding(cfg *config.Config) error {
	for _, p := range m.plugins {
		if hook, ok := p.(AssetsBuildingHook); ok {
			if err := hook.OnAssetsBuilding(cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnBuildStarted() error {
	for _, p := range m.plugins {
		if hook, ok := p.(BuildStartedHook); ok {
			if err := hook.OnBuildStarted(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnBuildCompleted(graph *content.SiteGraph) error {
	for _, p := range m.plugins {
		if hook, ok := p.(BuildCompletedHook); ok {
			if err := hook.OnBuildCompleted(graph); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnServerStarted(addr string) error {
	for _, p := range m.plugins {
		if hook, ok := p.(ServerStartedHook); ok {
			if err := hook.OnServerStarted(addr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	for _, p := range m.plugins {
		if hook, ok := p.(RoutesRegisterHook); ok {
			hook.RegisterRoutes(mux)
		}
	}
}
