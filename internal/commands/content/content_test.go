package contentcmd

import (
	"os"
	"path/filepath"
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
	if _, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "contact-us.md")); err != nil {
		t.Fatalf("expected page scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, "hello-post.md")); err != nil {
		t.Fatalf("expected post scaffold: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "content"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "content", "bad"}); err == nil {
		t.Fatal("expected unknown subcommand error")
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
