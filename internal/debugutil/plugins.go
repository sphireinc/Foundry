package debugutil

import "github.com/sphireinc/foundry/internal/plugins"

func DetectHooks(p plugins.Plugin) []string {
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

func Implements[T any](v any) bool {
	return implements[T](v)
}

func implements[T any](v any) bool {
	_, ok := v.(T)
	return ok
}
