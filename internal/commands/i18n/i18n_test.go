package i18ncmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestI18nCommandRun(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), "---\ntitle: About\nslug: about\nlayout: page\n---\n\n# About")
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "fr", "contact.md"), "---\ntitle: Contact\nslug: contact\nlayout: page\nlang: fr\n---\n\n# Contact")

	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "i18n", "list"}); err != nil {
		t.Fatalf("i18n list: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "i18n", "missing"}); err == nil {
		t.Fatal("expected missing translation error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "i18n", "scaffold", "fr", "page", "about"}); err != nil {
		t.Fatalf("i18n scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "fr", "about.md")); err != nil {
		t.Fatalf("expected translated file: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "i18n"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "i18n", "bad"}); err == nil {
		t.Fatal("expected unknown subcommand error")
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
