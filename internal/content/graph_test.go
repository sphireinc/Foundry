package content

import (
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestSiteGraphAddAndDefinitions(t *testing.T) {
	cfg := &config.Config{
		DefaultLang: "en",
		Taxonomies: config.TaxonomyConfig{
			DefaultSet: []string{"tags"},
			Definitions: map[string]config.TaxonomyDefinition{
				"categories": {Title: "Categories"},
			},
		},
	}
	graph := NewSiteGraph(cfg)
	doc := &Document{
		ID:         "doc-1",
		Type:       "post",
		Lang:       "en",
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Taxonomies: map[string][]string{"tags": {"go"}},
	}

	graph.Add(doc)

	if graph.ByURL["/posts/hello/"] != doc {
		t.Fatal("expected document by URL")
	}
	if len(graph.ByType["post"]) != 1 || len(graph.ByLang["en"]) != 1 {
		t.Fatal("expected document indexes to be populated")
	}
	if graph.Taxonomies.Definition("categories").Title != "Categories" {
		t.Fatal("expected configured taxonomy definition")
	}
	if graph.Taxonomies.Definition("tags").Name != "tags" {
		t.Fatal("expected default taxonomy definition")
	}
}
