package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestBuildNewContentWithDefaultsAndArchetype(t *testing.T) {
	cfg := &config.Config{
		DefaultLang: "en",
		ContentDir:  t.TempDir(),
		Content: config.ContentConfig{
			DefaultLayoutPage: "page",
			DefaultLayoutPost: "post",
		},
	}
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, "archetypes"), 0o755); err != nil {
		t.Fatalf("mkdir archetypes: %v", err)
	}
	body := "title: {{title}}\nslug: {{slug}}\nlayout: {{layout}}\nlang: {{lang}}\ntype: {{type}}\n"
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "archetypes", "page.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write archetype: %v", err)
	}

	rendered, err := BuildNewContent(cfg, "page", "hello-world")
	if err != nil {
		t.Fatalf("build new content: %v", err)
	}
	if !strings.Contains(rendered, "Hello World") || !strings.Contains(rendered, "lang: en") {
		t.Fatalf("unexpected rendered archetype: %q", rendered)
	}

	post, err := BuildNewContentWithOptions(cfg, NewContentOptions{Kind: "post", Slug: "my-post", Lang: "es"})
	if err != nil {
		t.Fatalf("build post content: %v", err)
	}
	if !strings.Contains(post, "draft: true") || !strings.Contains(post, "lang: es") {
		t.Fatalf("unexpected default post archetype: %q", post)
	}
}

func TestBuildNewContentErrorsAndHelpers(t *testing.T) {
	cfg := &config.Config{
		DefaultLang: "en",
		ContentDir:  t.TempDir(),
		Content: config.ContentConfig{
			DefaultLayoutPage: "page",
			DefaultLayoutPost: "post",
		},
	}

	if _, err := BuildNewContentWithOptions(cfg, NewContentOptions{}); err == nil {
		t.Fatal("expected missing kind/slug error")
	}
	if _, err := BuildNewContentWithOptions(cfg, NewContentOptions{Kind: "x", Slug: "slug"}); err == nil {
		t.Fatal("expected unknown kind error")
	}
	if humanizeSlug("hello-world") != "Hello World" {
		t.Fatal("expected humanized slug")
	}
	if normalizeContentLang(cfg, "bad/lang") != "en" {
		t.Fatal("expected invalid lang fallback")
	}
}
