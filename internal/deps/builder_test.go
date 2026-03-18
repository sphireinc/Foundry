package deps

import (
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

func TestBuildSiteDependencyGraphIncludesTaxonomyOutputs(t *testing.T) {
	cfg := testDepsConfig(t)
	graph := content.NewSiteGraph(cfg)
	graph.Data["navigation"] = map[string]any{"main": []any{}}

	graph.Add(&content.Document{
		ID:         "doc-en",
		Type:       "post",
		Lang:       "en",
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md")),
		Taxonomies: map[string][]string{"tags": {"go"}},
	})
	graph.Add(&content.Document{
		ID:         "doc-es",
		Type:       "post",
		Lang:       "es",
		Title:      "Hola",
		Slug:       "hola",
		URL:        "/es/posts/hola/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "es", "hola.md")),
		Taxonomies: map[string][]string{"tags": {"go"}},
	})

	depGraph := BuildSiteDependencyGraph(graph, cfg.Theme)

	for _, url := range []string{"/tags/go/", "/es/tags/go/"} {
		node, ok := depGraph.Node(outputNodeID(url))
		if !ok {
			t.Fatalf("expected taxonomy output node for %s", url)
		}
		if node.Type != NodeOutput {
			t.Fatalf("expected output node for %s, got %s", url, node.Type)
		}
	}

	plan := ResolveRebuildPlan(depGraph, ChangeSet{
		Sources: []string{filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md"))},
	})

	assertURLs(t, plan.OutputURLs, "/posts/hello/", "/tags/go/")
}

func TestResolveRebuildPlanIncludesTaxonomyOutputsForTemplateAndDataChanges(t *testing.T) {
	cfg := testDepsConfig(t)
	graph := content.NewSiteGraph(cfg)
	graph.Data["navigation"] = map[string]any{"main": []any{}}
	graph.Add(&content.Document{
		ID:         "doc-en",
		Type:       "post",
		Lang:       "en",
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md")),
		Taxonomies: map[string][]string{"tags": {"go"}},
	})

	depGraph := BuildSiteDependencyGraph(graph, cfg.Theme)
	termLayout := filepath.ToSlash(filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "taxonomy-term.html"))

	templatePlan := ResolveRebuildPlan(depGraph, ChangeSet{
		Templates: []string{termLayout},
	})
	assertURLs(t, templatePlan.OutputURLs, "/tags/go/")

	dataPlan := ResolveRebuildPlan(depGraph, ChangeSet{
		DataKeys: []string{"navigation"},
	})
	assertURLs(t, dataPlan.OutputURLs, "/posts/hello/", "/tags/go/")
}

func testDepsConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	cfg := &config.Config{
		DefaultLang: "en",
		Theme:       "default",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		DataDir:     filepath.Join(root, "data"),
		Taxonomies: config.TaxonomyConfig{
			DefaultSet: []string{"tags"},
			Definitions: map[string]config.TaxonomyDefinition{
				"tags": {TermLayout: "taxonomy-term"},
			},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

func assertURLs(t *testing.T, got []string, want ...string) {
	t.Helper()

	set := make(map[string]struct{}, len(got))
	for _, url := range got {
		set[url] = struct{}{}
	}

	for _, url := range want {
		if _, ok := set[url]; !ok {
			t.Fatalf("expected rebuild plan to include %s, got %v", url, got)
		}
	}
}
