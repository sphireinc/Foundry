package taxonomy

import "testing"

func TestIndexAddDocumentAndOrdering(t *testing.T) {
	idx := New(map[string]Definition{
		"tags": {Title: "Tags", Labels: map[string]string{"es": "Etiquetas"}},
	})

	idx.AddDocument("doc-1", "/posts/hello/", "en", "post", "Hello", "hello", map[string][]string{
		"tags": {"go", "  "},
	})
	idx.AddDocument("doc-2", "/es/posts/hola/", "es", "post", "Hola", "hola", map[string][]string{
		"tags": {"go"},
		"cats": {"news"},
	})

	if len(idx.Values["tags"]["go"]) != 2 {
		t.Fatalf("expected two tag entries, got %d", len(idx.Values["tags"]["go"]))
	}
	if got := idx.OrderedNames(); len(got) != 2 || got[0] != "cats" || got[1] != "tags" {
		t.Fatalf("unexpected ordered names: %v", got)
	}
	if got := idx.OrderedTerms("tags"); len(got) != 1 || got[0] != "go" {
		t.Fatalf("unexpected ordered terms: %v", got)
	}
}

func TestDefinitionHelpers(t *testing.T) {
	def := Definition{
		Name:          "tags",
		Title:         "Tags",
		Labels:        map[string]string{"es": "Etiquetas"},
		ArchiveLayout: "archive",
	}

	if def.DisplayTitle("es") != "Etiquetas" {
		t.Fatalf("expected localized title, got %q", def.DisplayTitle("es"))
	}
	if def.DisplayTitle("en") != "Tags" {
		t.Fatalf("expected fallback title, got %q", def.DisplayTitle("en"))
	}
	if def.EffectiveTermLayout() != "archive" {
		t.Fatalf("expected archive layout, got %q", def.EffectiveTermLayout())
	}

	norm := normalizeDefinition(" tags ", Definition{})
	if norm.Name != "tags" || norm.Title != "tags" || norm.Labels == nil {
		t.Fatalf("unexpected normalized definition: %#v", norm)
	}
}
