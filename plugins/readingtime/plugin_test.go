package readingtime

import (
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
)

func TestReadingTimePlugin(t *testing.T) {
	p := &Plugin{}
	if p.Name() != "reading-time" || countWords("one two") != 2 || estimateMinutes(1) != 1 || estimateMinutes(201) != 2 {
		t.Fatal("unexpected reading time helpers")
	}

	doc := &content.Document{Type: "post", RawBody: "one two three"}
	if err := p.OnDocumentParsed(doc); err != nil {
		t.Fatalf("on document parsed: %v", err)
	}
	if len(doc.Fields) != 0 {
		t.Fatalf("expected plugin not to persist derived fields, got %#v", doc.Fields)
	}
	ctx := &renderer.ViewData{Page: doc}
	if err := p.OnContext(ctx); err != nil {
		t.Fatalf("on context: %v", err)
	}
	if got := ctx.Data["reading_time"]; got != 1 {
		t.Fatalf("expected reading_time in runtime data, got %#v", got)
	}
	if got := ctx.Data["word_count"]; got != 3 {
		t.Fatalf("expected word_count in runtime data, got %#v", got)
	}
	slots := renderer.NewSlots()
	if err := p.OnHTMLSlots(ctx, slots); err != nil {
		t.Fatalf("on html slots: %v", err)
	}
	if !strings.Contains(string(slots.Render("post.sidebar.top")), "Reading time:") {
		t.Fatalf("expected reading time html")
	}
}
