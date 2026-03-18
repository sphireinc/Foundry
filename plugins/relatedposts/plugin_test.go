package relatedposts

import (
	"strings"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
)

func TestRelatedPostsPlugin(t *testing.T) {
	now := time.Now()
	p := &Plugin{related: make(map[string][]Item)}
	graph := content.NewSiteGraph(nil)
	current := &content.Document{
		ID: "a", Type: "post", Lang: "en", Title: "A", URL: "/a/", Summary: "A",
		Taxonomies: map[string][]string{"tags": {"go"}, "categories": {"cms"}}, Date: &now,
	}
	other := &content.Document{
		ID: "b", Type: "post", Lang: "en", Title: "B", URL: "/b/", Summary: "B",
		Taxonomies: map[string][]string{"tags": {"go"}, "categories": {"cms"}}, Date: &now,
	}
	graph.Documents = []*content.Document{current, other}
	if err := p.OnRoutesAssigned(graph); err != nil {
		t.Fatalf("on routes assigned: %v", err)
	}

	ctx := &renderer.ViewData{Page: current}
	if err := p.OnContext(ctx); err != nil {
		t.Fatalf("on context: %v", err)
	}
	slots := renderer.NewSlots()
	if err := p.OnHTMLSlots(ctx, slots); err != nil {
		t.Fatalf("on html slots: %v", err)
	}
	if !strings.Contains(string(slots.Render("post.after_content")), "Related posts") {
		t.Fatal("expected related posts html")
	}
}

func TestRelatedPostsHelpers(t *testing.T) {
	a := &content.Document{Taxonomies: map[string][]string{"tags": {"go", "cms"}, "categories": {"news"}}}
	b := &content.Document{Taxonomies: map[string][]string{"tags": {"go", "go"}, "categories": {"news"}}}
	if scoreDocuments(a, b) <= 0 {
		t.Fatal("expected positive related score")
	}
	if countSharedTerms([]string{"a", "b"}, []string{"b", "b", "c"}) != 1 {
		t.Fatal("expected unique shared term count")
	}
	items := cloneItems([]Item{{Title: "x"}})
	if len(items) != 1 || cloneItems(nil) != nil {
		t.Fatal("unexpected cloneItems behavior")
	}
}
