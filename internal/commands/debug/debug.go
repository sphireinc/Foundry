package debug

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/site"
	"gopkg.in/yaml.v3"
)

type command struct{}

func (command) Name() string {
	return "debug"
}

func (command) Summary() string {
	return "Show internal diagnostic information"
}

func (command) Group() string {
	return "debug commands"
}

func (command) Details() []string {
	return []string{
		"foundry debug routes",
		"foundry debug plugins",
		"foundry debug config",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry debug [routes|plugins|config]")
	}

	switch args[2] {
	case "routes":
		return runRoutes(cfg)
	case "plugins":
		return runPlugins(cfg)
	case "config":
		return runConfig(cfg)
	default:
		return fmt.Errorf("unknown debug subcommand: %s", args[2])
	}
}

func runRoutes(cfg *config.Config) error {
	graph, err := loadGraph(cfg)
	if err != nil {
		return err
	}

	type row struct {
		URL    string
		Type   string
		Lang   string
		Slug   string
		Layout string
		Title  string
		Source string
	}

	rows := make([]row, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		rows = append(rows, row{
			URL:    doc.URL,
			Type:   doc.Type,
			Lang:   doc.Lang,
			Slug:   doc.Slug,
			Layout: doc.Layout,
			Title:  doc.Title,
			Source: doc.SourcePath,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].URL != rows[j].URL {
			return rows[i].URL < rows[j].URL
		}
		return rows[i].Source < rows[j].Source
	})

	urlWidth := len("URL")
	typeWidth := len("TYPE")
	langWidth := len("LANG")
	slugWidth := len("SLUG")
	layoutWidth := len("LAYOUT")

	for _, row := range rows {
		if len(row.URL) > urlWidth {
			urlWidth = len(row.URL)
		}
		if len(row.Type) > typeWidth {
			typeWidth = len(row.Type)
		}
		if len(row.Lang) > langWidth {
			langWidth = len(row.Lang)
		}
		if len(row.Slug) > slugWidth {
			slugWidth = len(row.Slug)
		}
		if len(row.Layout) > layoutWidth {
			layoutWidth = len(row.Layout)
		}
	}

	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
		urlWidth, "URL",
		typeWidth, "TYPE",
		langWidth, "LANG",
		slugWidth, "SLUG",
		layoutWidth, "LAYOUT",
		"SOURCE",
	)

	for _, row := range rows {
		fmt.Printf("%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
			urlWidth, row.URL,
			typeWidth, row.Type,
			langWidth, row.Lang,
			slugWidth, row.Slug,
			layoutWidth, row.Layout,
			row.Source,
		)
	}

	fmt.Println("")
	fmt.Printf("documents: %d\n", len(graph.Documents))
	fmt.Printf("languages: %d\n", len(graph.ByLang))
	fmt.Printf("types: %d\n", len(graph.ByType))
	fmt.Printf("urls: %d\n", len(graph.ByURL))

	return nil
}

func runPlugins(cfg *config.Config) error {
	pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		return err
	}

	metas := pm.MetadataList()
	if len(metas) == 0 {
		fmt.Println("no enabled plugins")
		return nil
	}

	pluginInstances := pm.Plugins()
	pluginTypes := make(map[string]string, len(pluginInstances))
	pluginHooks := make(map[string][]string, len(pluginInstances))

	for _, p := range pluginInstances {
		name := p.Name()
		pluginTypes[name] = reflect.TypeOf(p).String()
		pluginHooks[name] = detectHooks(p)
	}

	for _, meta := range metas {
		fmt.Printf("Name:        %s\n", meta.Name)
		fmt.Printf("Title:       %s\n", meta.Title)
		fmt.Printf("Version:     %s\n", meta.Version)
		fmt.Printf("Directory:   %s\n", meta.Directory)
		fmt.Printf("Repo:        %s\n", meta.Repo)
		if t := pluginTypes[meta.Name]; t != "" {
			fmt.Printf("Type:        %s\n", t)
		}
		if hooks := pluginHooks[meta.Name]; len(hooks) > 0 {
			fmt.Printf("Hooks:       %s\n", strings.Join(hooks, ", "))
		} else {
			fmt.Printf("Hooks:       none detected\n")
		}
		if len(meta.Requires) > 0 {
			fmt.Println("Requires:")
			for _, dep := range meta.Requires {
				fmt.Printf("  - %s\n", dep)
			}
		}
		fmt.Println("")
	}

	fmt.Printf("enabled plugins: %d\n", len(metas))
	return nil
}

func runConfig(cfg *config.Config) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Print(string(b))
	return nil
}

func detectHooks(p plugins.Plugin) []string {
	type hookCheck struct {
		name string
		ok   bool
	}

	hooks := []hookCheck{
		{"ConfigLoadedHook", implements[plugins.ConfigLoadedHook](p)},
		{"ContentDiscoveredHook", implements[plugins.ContentDiscoveredHook](p)},
		{"FrontmatterParsedHook", implements[plugins.FrontmatterParsedHook](p)},
		{"MarkdownRenderedHook", implements[plugins.MarkdownRenderedHook](p)},
		{"DocumentParsedHook", implements[plugins.DocumentParsedHook](p)},
		{"DataLoadedHook", implements[plugins.DataLoadedHook](p)},
		{"GraphBuildingHook", implements[plugins.GraphBuildingHook](p)},
		{"GraphBuiltHook", implements[plugins.GraphBuiltHook](p)},
		{"TaxonomyBuiltHook", implements[plugins.TaxonomyBuiltHook](p)},
		{"RoutesAssignedHook", implements[plugins.RoutesAssignedHook](p)},
		{"ContextHook", implements[plugins.ContextHook](p)},
		{"AssetsHook", implements[plugins.AssetsHook](p)},
		{"HTMLSlotsHook", implements[plugins.HTMLSlotsHook](p)},
		{"BeforeRenderHook", implements[plugins.BeforeRenderHook](p)},
		{"AfterRenderHook", implements[plugins.AfterRenderHook](p)},
		{"AssetsBuildingHook", implements[plugins.AssetsBuildingHook](p)},
		{"BuildStartedHook", implements[plugins.BuildStartedHook](p)},
		{"BuildCompletedHook", implements[plugins.BuildCompletedHook](p)},
		{"ServerStartedHook", implements[plugins.ServerStartedHook](p)},
		{"RoutesRegisterHook", implements[plugins.RoutesRegisterHook](p)},
		{"CLIHook", implements[plugins.CLIHook](p)},
	}

	out := make([]string, 0)
	for _, h := range hooks {
		if h.ok {
			out = append(out, h.name)
		}
	}
	return out
}

func implements[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func loadGraph(cfg *config.Config) (*content.SiteGraph, error) {
	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, true)
	if err != nil {
		return nil, err
	}
	return graph, nil
}

func init() {
	registry.Register(command{})
}
