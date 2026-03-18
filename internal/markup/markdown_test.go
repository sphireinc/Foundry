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
