package toc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

func TestTOCPlugin(t *testing.T) {
	p := &Plugin{}
	doc := &content.Document{Type: "post", RawBody: "# Hello\n## `Code`\n## [Link](https://example.com)\n# Hello"}
	if p.Name() != "toc" {
		t.Fatal("unexpected plugin name")
	}
	if err := p.OnDocumentParsed(doc); err != nil {
		t.Fatalf("on document parsed: %v", err)
	}
	items := extractTOC(doc.RawBody)
	if len(items) != 4 || items[1].Text != "Code" || items[3].ID != "hello-1" {
		t.Fatalf("unexpected toc items: %#v", items)
	}

	ctx := &renderer.ViewData{Page: doc}
	if err := p.OnContext(ctx); err != nil {
		t.Fatalf("on context: %v", err)
	}
	if ctx.Data["has_toc"] != true {
		t.Fatalf("expected has_toc in context data, got %#v", ctx.Data)
	}
	if got, ok := ctx.Data["toc"].([]Item); !ok || len(got) != 4 {
		t.Fatalf("expected toc items in context data, got %#v", ctx.Data["toc"])
	}
	assets := renderer.NewAssetSet()
	if err := p.OnAssets(ctx, assets); err != nil {
		t.Fatalf("on assets: %v", err)
	}
	slots := renderer.NewSlots()
	if err := p.OnHTMLSlots(ctx, slots); err != nil {
		t.Fatalf("on html slots: %v", err)
	}
	if !strings.Contains(string(slots.Render("post.sidebar.bottom")), "On this page") {
		t.Fatal("expected toc html")
	}

	var out bytes.Buffer
	cmds := p.Commands()
	if len(cmds) != 1 {
		t.Fatal("expected toc command")
	}
	if err := cmds[0].Run(plugins.CommandContext{Stdout: &out}); err != nil || !strings.Contains(out.String(), "installed") {
		t.Fatalf("unexpected toc command output: %q %v", out.String(), err)
	}
}

func TestTOCHelpers(t *testing.T) {
	if normalizeHeadingText("`Code` [Link](x) *Bold*") != "Code Link Bold" {
		t.Fatal("unexpected normalized heading text")
	}
	if slugify("Hello, World!") != "hello-world" {
		t.Fatal("unexpected slugify")
	}
}
