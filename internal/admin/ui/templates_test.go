package ui

import (
	"strings"
	"testing"
)

func TestIndexHTMLEscapesTitle(t *testing.T) {
	html := IndexHTML(`<script>alert(1)</script>`)
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected title to be escaped, got %q", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt; Admin") {
		t.Fatalf("expected escaped title in output, got %q", html)
	}
}
