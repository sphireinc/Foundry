package router

import (
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

func TestResolverAssignURLsAndHelpers(t *testing.T) {
	cfg := &config.Config{DefaultLang: "en"}
	r := NewResolver(cfg)
	graph := content.NewSiteGraph(cfg)
	graph.Documents = []*content.Document{
		{Type: "page", Slug: "index", Lang: "en", SourcePath: "content/pages/index.md"},
		{Type: "page", Slug: "about", Lang: "es", SourcePath: "content/pages/es/about.md"},
		{Type: "post", Slug: "hello", Lang: "en", SourcePath: "content/posts/hello.md"},
	}

	if err := r.AssignURLs(graph); err != nil {
		t.Fatalf("assign urls: %v", err)
	}
	if graph.Documents[0].URL != "/" {
		t.Fatalf("expected root page URL, got %q", graph.Documents[0].URL)
	}
	if graph.Documents[1].URL != "/es/about/" {
		t.Fatalf("expected translated page URL, got %q", graph.Documents[1].URL)
	}
	if graph.Documents[2].URL != "/posts/hello/" {
		t.Fatalf("expected post URL, got %q", graph.Documents[2].URL)
	}
	if sourceToSlug(filepath.Join("content", "posts", "hello.md")) != "hello" {
		t.Fatal("expected slug from source path")
	}
}

func TestResolverErrors(t *testing.T) {
	cfg := &config.Config{DefaultLang: "en"}
	r := NewResolver(cfg)

	if _, err := r.URLForDocument(nil); err == nil {
		t.Fatal("expected nil document error")
	}
	if _, err := r.URLForDocument(&content.Document{Type: "unknown"}); err == nil {
		t.Fatal("expected unsupported type error")
	}
	if err := r.AssignURLs(nil); err == nil {
		t.Fatal("expected nil graph error")
	}

	graph := content.NewSiteGraph(cfg)
	graph.Documents = []*content.Document{
		{Type: "page", Slug: "about", Lang: "en", SourcePath: "a.md"},
		{Type: "page", Slug: "about", Lang: "en", SourcePath: "b.md"},
	}
	if err := r.AssignURLs(graph); err == nil {
		t.Fatal("expected route collision")
	}
}
