package plugins

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
)

// Plugin is the minimum interface every Foundry plugin must implement.
//
// Plugins become active when their name appears in site configuration and the
// package has been registered with Register. Additional behavior is opt-in via
// the hook interfaces in this file.
type Plugin interface {
	Name() string
}

// Factory constructs a fresh plugin instance for the current process.
//
// Foundry instantiates plugins during manager creation, so pluginsshould
// avoid global mutable state and return a new value on each call.
type Factory func() Plugin

// ConfigLoadedHook runs after site configuration has been loaded and defaults
// have been applied, but before content is discovered or the graph is built.
//
// Use it to validate plugin-specific configuration or to derive runtime state
// from cfg. Mutations to cfg should be conservative because later build stages
// depend on the normalized config.
type ConfigLoadedHook interface {
	OnConfigLoaded(*config.Config) error
}

// ContentDiscoveredHook runs once for each content file path discovered during
// loader traversal, before frontmatter or Markdown parsing.
//
// The path is the filesystem path found by the loader. This hook is best used
// for inventorying files or early validation, not for mutating documents.
type ContentDiscoveredHook interface {
	OnContentDiscovered(path string) error
}

// FrontmatterParsedHook runs after a document's frontmatter has been parsed and
// normalized, but before Markdown rendering.
//
// Use it to inspect or adjust structured metadata before the rendered body and
// derived fields are finalized.
type FrontmatterParsedHook interface {
	OnFrontmatterParsed(*content.Document) error
}

// MarkdownRenderedHook runs after Markdown has been rendered into HTML for a
// document, but before the loader finishes constructing the final Document.
//
// This is the right place to inspect rendered HTML-dependent metadata, collect
// references, or rewrite rendered content.
type MarkdownRenderedHook interface {
	OnMarkdownRendered(*content.Document) error
}

// DocumentParsedHook runs after Foundry has fully parsed a document, including
// frontmatter normalization and Markdown rendering.
//
// At this point the document is close to its final graph-ready form.
type DocumentParsedHook interface {
	OnDocumentParsed(*content.Document) error
}

// DataLoadedHook runs after structured data files have been loaded into the
// shared site data map but before graph construction begins.
//
// Implementations may add derived values to the map for later template access.
type DataLoadedHook interface {
	OnDataLoaded(map[string]any) error
}

// GraphBuildingHook runs immediately before the loader finalizes the site graph.
//
// It is the last hook that sees the graph while relationships and aggregates
// are still being assembled.
type GraphBuildingHook interface {
	OnGraphBuilding(*content.SiteGraph) error
}

// GraphBuiltHook runs after the site graph has been fully assembled, but before
// taxonomy-specific post-processing and route assignment complete.
type GraphBuiltHook interface {
	OnGraphBuilt(*content.SiteGraph) error
}

// TaxonomyBuiltHook runs after taxonomy pages and term indexes have been added
// to the site graph.
type TaxonomyBuiltHook interface {
	OnTaxonomyBuilt(*content.SiteGraph) error
}

// RoutesAssignedHook runs after the router has assigned final output URLs to
// graph documents.
//
// Use it for work that depends on canonical routes, permalinks, or final slug
// resolution.
type RoutesAssignedHook interface {
	OnRoutesAssigned(*content.SiteGraph) error
}

// ContextHook runs during rendering after ViewData has been assembled for a
// page, post, list, or index view and before assets/slots are finalized.
//
// Use it to enrich template context with derived values. Changes made here are
// visible to templates and later rendering hooks.
type ContextHook interface {
	OnContext(*renderer.ViewData) error
}

// AssetsHook runs during rendering after ViewData has been built and before
// AssetSet is rendered into HTML slots.
//
// Add CSS or JS assets here when they should participate in the normal theme
// slot flow.
type AssetsHook interface {
	OnAssets(*renderer.ViewData, *renderer.AssetSet) error
}

// HTMLSlotsHook runs during rendering after assets have been converted into
// HTML slots but before template execution.
//
// This hook is the best place to inject raw HTML fragments into named theme
// slots declared by the active theme manifest.
type HTMLSlotsHook interface {
	OnHTMLSlots(*renderer.ViewData, *renderer.Slots) error
}

// BeforeRenderHook runs immediately before template execution for a single
// output URL.
//
// It sees the final ViewData after context, assets, and slots have been
// prepared.
type BeforeRenderHook interface {
	OnBeforeRender(*renderer.ViewData) error
}

// AfterRenderHook runs after template execution for a single output URL.
//
// Hooks receive the rendered HTML bytes and may return a modified copy. Hooks
// are chained in plugin order, so each hook receives the output of the previous
// one.
type AfterRenderHook interface {
	OnAfterRender(url string, html []byte) ([]byte, error)
}

// AssetsBuildingHook runs when Foundry is copying or building theme/media asset
// output for a build or serve cycle.
type AssetsBuildingHook interface {
	OnAssetsBuilding(*config.Config) error
}

// BuildStartedHook runs once at the beginning of a build or serve graph refresh
// before content loading begins.
type BuildStartedHook interface {
	OnBuildStarted() error
}

// BuildCompletedHook runs after a graph has been built successfully.
//
// The provided graph is the final graph used for rendering or serving.
type BuildCompletedHook interface {
	OnBuildCompleted(*content.SiteGraph) error
}

// ServerStartedHook runs after the preview server has successfully bound its
// listening address.
type ServerStartedHook interface {
	OnServerStarted(addr string) error
}

// RoutesRegisterHook lets a plugin register additional HTTP handlers on the
// preview server mux.
//
// Handlers registered here share the Foundry preview server process, so they
// should avoid conflicting with Foundry-owned paths and should apply their own
// authorization if they expose privileged behavior.
type RoutesRegisterHook interface {
	RegisterRoutes(mux *http.ServeMux)
}

// Manager owns the enabled plugin instances for the current process and fans
// out lifecycle hooks to each plugin that implements them.
type Manager struct {
	plugins  []Plugin
	metadata map[string]Metadata
}

// NewManager loads metadata for enabled plugins, validates dependency
// declarations, and instantiates registered plugin factories for the enabled
// list in configuration order.
func NewManager(pluginsDir string, enabled []string) (*Manager, error) {
	m := &Manager{
		plugins:  make([]Plugin, 0),
		metadata: make(map[string]Metadata),
	}

	metadata, err := LoadAllMetadata(pluginsDir, enabled)
	if err != nil {
		return nil, err
	}
	m.metadata = metadata

	if err := validateDependencies(metadata); err != nil {
		return nil, err
	}

	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if factory, ok := registry[name]; ok {
			m.plugins = append(m.plugins, factory())
		}
	}

	return m, nil
}

// Plugins returns a shallow copy of the enabled plugin slice.
func (m *Manager) Plugins() []Plugin {
	out := make([]Plugin, len(m.plugins))
	copy(out, m.plugins)
	return out
}

// Metadata returns a copy of metadata keyed by plugin name for enabled plugins.
func (m *Manager) Metadata() map[string]Metadata {
	out := make(map[string]Metadata, len(m.metadata))
	for k, v := range m.metadata {
		out[k] = v
	}
	return out
}

// MetadataFor returns metadata for a single enabled plugin.
func (m *Manager) MetadataFor(name string) (Metadata, bool) {
	meta, ok := m.metadata[name]
	return meta, ok
}

// MetadataList returns enabled plugin metadata sorted by plugin name.
func (m *Manager) MetadataList() []Metadata {
	items := make([]Metadata, 0, len(m.metadata))
	for _, meta := range m.metadata {
		items = append(items, meta)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items
}

func validateDependencies(metadata map[string]Metadata) error {
	installedByRepo := make(map[string]string, len(metadata))

	for pluginName, meta := range metadata {
		repo := normalizeRepoRef(meta.Repo)
		if repo == "" {
			continue
		}

		if existing, ok := installedByRepo[repo]; ok && existing != pluginName {
			return fmt.Errorf("duplicate plugin repo %q declared by %q and %q", repo, existing, pluginName)
		}

		installedByRepo[repo] = pluginName
	}

	for pluginName, meta := range metadata {
		for _, dep := range meta.Requires {
			if _, ok := installedByRepo[dep]; !ok {
				return fmt.Errorf("plugin %q requires %q, but no enabled plugin declares repo %q", pluginName, dep, dep)
			}
		}
		for _, dep := range meta.Dependencies {
			if dep.Name == "" || dep.Optional {
				continue
			}
			if _, ok := installedByRepo[dep.Name]; !ok {
				return fmt.Errorf("plugin %q depends on %q, but no enabled plugin declares repo %q", pluginName, dep.Name, dep.Name)
			}
		}
	}

	return nil
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

func (m *Manager) OnAssets(ctx *renderer.ViewData, assetSet *renderer.AssetSet) error {
	for _, p := range m.plugins {
		if hook, ok := p.(AssetsHook); ok {
			if err := hook.OnAssets(ctx, assetSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnHTMLSlots(ctx *renderer.ViewData, slots *renderer.Slots) error {
	for _, p := range m.plugins {
		if hook, ok := p.(HTMLSlotsHook); ok {
			if err := hook.OnHTMLSlots(ctx, slots); err != nil {
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

// RegisterRoutes asks each plugin that implements RoutesRegisterHook to attach
// its HTTP handlers to mux.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	for _, p := range m.plugins {
		if hook, ok := p.(RoutesRegisterHook); ok {
			hook.RegisterRoutes(mux)
		}
	}
}
