package markup

import (
	"strings"
	"testing"
)

func TestMarkdownToHTMLStripsRawHTMLByDefault(t *testing.T) {
	html, err := MarkdownToHTML("<script>alert(1)</script>\n\n# Hi", false)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if strings.Contains(string(html), "<script>") {
		t.Fatalf("expected raw script tag to be stripped, got %q", string(html))
	}
}

func TestMarkdownToHTMLAllowsRawHTMLWhenConfigured(t *testing.T) {
	html, err := MarkdownToHTML("<span>ok</span>\n\n# Hi", true)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if !strings.Contains(string(html), "<span>ok</span>") {
		t.Fatalf("expected raw html to be preserved, got %q", string(html))
	}
}

func TestMarkdownToHTMLRewritesMediaReferences(t *testing.T) {
	html, err := MarkdownToHTML("![Screenshot](media:images/shot.png)\n\n![Walkthrough](media:uploads/demo.mp4)\n\n[Download](media:uploads/spec.pdf)", false)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	rendered := string(html)
	for _, want := range []string{`<img src="/images/shot.png" alt="Screenshot">`, `<video controls preload="metadata" src="/uploads/demo.mp4" title="Walkthrough" aria-label="Walkthrough"></video>`, `<a href="/uploads/spec.pdf">Download</a>`} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected rendered html to contain %q, got %q", want, rendered)
		}
	}
}
