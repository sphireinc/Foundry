package feedcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestFeedCommandRun(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, "hello.md"), "---\ntitle: Hello\nslug: hello\nlayout: post\ndate: 2026-01-01\nsummary: Hi\n---\n\n# Hello")

	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "feed", "validate"}); err != nil {
		t.Fatalf("feed validate: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "feed", "build"}); err != nil {
		t.Fatalf("feed build: %v", err)
	}
	for _, path := range []string{
		filepath.Join(cfg.PublicDir, "rss.xml"),
		filepath.Join(cfg.PublicDir, "sitemap.xml"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected feed output %s: %v", path, err)
		}
	}
	if err := cmd.Run(cfg, []string{"foundry", "feed"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "feed", "bad"}); err == nil {
		t.Fatal("expected unknown subcommand error")
	}
}

func TestFeedCommandMetadata(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "feed" {
		t.Fatalf("unexpected command name: %q", cmd.Name())
	}
	if cmd.Summary() == "" || cmd.Group() == "" || !cmd.RequiresConfig() || len(cmd.Details()) != 2 {
		t.Fatal("expected populated command metadata")
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
		Feed: config.FeedConfig{
			RSSPath:        "/rss.xml",
			SitemapPath:    "/sitemap.xml",
			RSSTitle:       "Feed",
			RSSDescription: "Feed desc",
		},
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
