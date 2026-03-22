package contentcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestContentCommandRun(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), "---\ntitle: About\nslug: about\nlayout: page\n---\n\n# About")

	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "content", "new", "page", "contact us"}); err != nil {
		t.Fatalf("content new page: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "new", "post", "hello-post"}); err != nil {
		t.Fatalf("content new post: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "lint"}); err != nil {
		t.Fatalf("content lint: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "list"}); err != nil {
		t.Fatalf("content list: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "graph"}); err != nil {
		t.Fatalf("content graph: %v", err)
	}
	importRoot := filepath.Join(root, "import")
	writeMarkdown(t, filepath.Join(importRoot, "pages", "imported.md"), "---\ntitle: Imported\nslug: imported\nlayout: page\n---\n\nImported")
	if err := cmd.Run(cfg, []string{"foundry", "content", "import", "markdown", importRoot}); err != nil {
		t.Fatalf("content import markdown: %v", err)
	}
	wxrPath := filepath.Join(root, "wordpress.xml")
	if err := os.WriteFile(wxrPath, []byte(`<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>Hello WXR</title><wp:post_name>hello-wxr</wp:post_name><wp:post_type>post</wp:post_type><wp:status>publish</wp:status><wp:post_date>2026-03-21 10:00:00</wp:post_date><content:encoded><![CDATA[# Hello from WordPress]]></content:encoded></item></channel></rss>`), 0o644); err != nil {
		t.Fatalf("write wordpress xml: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "import", "wordpress", wxrPath}); err != nil {
		t.Fatalf("content import wordpress: %v", err)
	}
	bundlePath := filepath.Join(root, "bundle.zip")
	if err := cmd.Run(cfg, []string{"foundry", "content", "export", bundlePath}); err != nil {
		t.Fatalf("content export: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "migrate", "layout", "page", "landing"}); err != nil {
		t.Fatalf("content migrate layout: %v", err)
	}
	schemaDoc := filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "schema-page.md")
	writeMarkdown(t, schemaDoc, "---\ntitle: Schema\nslug: schema-page\nlayout: landing\nfields:\n  old_field: value\nschema: marketing\n---\n\nSchema")
	if err := cmd.Run(cfg, []string{"foundry", "content", "migrate", "field-rename", "marketing", "old_field", "new_field"}); err != nil {
		t.Fatalf("content migrate field-rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "contact-us.md")); err != nil {
		t.Fatalf("expected page scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, "hello-post.md")); err != nil {
		t.Fatalf("expected post scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "imported.md")); err != nil {
		t.Fatalf("expected imported markdown file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, "hello-wxr.md")); err != nil {
		t.Fatalf("expected imported wordpress file: %v", err)
	}
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("expected content bundle: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"))
	if err != nil {
		t.Fatalf("read migrated about page: %v", err)
	}
	if string(b) == "" || !contains(string(b), "layout: landing") {
		t.Fatalf("expected layout migration to update about page: %s", string(b))
	}
	schemaBody, err := os.ReadFile(schemaDoc)
	if err != nil {
		t.Fatalf("read migrated schema page: %v", err)
	}
	if !contains(string(schemaBody), "new_field: value") || contains(string(schemaBody), "old_field: value") {
		t.Fatalf("expected field rename migration to update schema page: %s", string(schemaBody))
	}
	if err := cmd.Run(cfg, []string{"foundry", "content"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "bad"}); err == nil {
		t.Fatal("expected unknown subcommand error")
	}
}

func TestContentMigrateDryRunDoesNotWriteFiles(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	path := filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md")
	writeMarkdown(t, path, "---\ntitle: About\nslug: about\nlayout: page\n---\n\n# About")

	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "content", "migrate", "layout", "page", "landing", "--dry-run"}); err != nil {
		t.Fatalf("content migrate dry-run: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file after dry-run: %v", err)
	}
	if !contains(string(body), "layout: page") || contains(string(body), "layout: landing") {
		t.Fatalf("expected dry-run to leave file unchanged, got %s", string(body))
	}
}

func TestWriteNewContentFileRejectsDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.md")
	if err := writeNewContentFile(path, "body"); err != nil {
		t.Fatalf("initial write: %v", err)
	}
	if err := writeNewContentFile(path, "body"); err == nil {
		t.Fatal("expected duplicate file error")
	}
}

func testProjectConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Title:       "Foundry",
		BaseURL:     "https://example.com",
		Theme:       "default",
		DefaultLang: "en",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		PluginsDir:  filepath.Join(root, "plugins"),
		DataDir:     filepath.Join(root, "data"),
		Content: config.ContentConfig{
			PagesDir:          "pages",
			PostsDir:          "posts",
			AssetsDir:         "assets",
			ImagesDir:         "images",
			UploadsDir:        "uploads",
			DefaultLayoutPage: "page",
			DefaultLayoutPost: "post",
		},
	}
	cfg.ApplyDefaults()
	for _, dir := range []string{
		filepath.Join(cfg.ContentDir, cfg.Content.PagesDir),
		filepath.Join(cfg.ContentDir, cfg.Content.PostsDir),
		cfg.DataDir,
		cfg.PluginsDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return cfg
}

func writeMarkdown(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func contains(body, fragment string) bool {
	return strings.Contains(body, fragment)
}
