package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestValidateRun(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), "---\ntitle: About\nslug: about\nlayout: page\n---\n\n# About")

	cmd := command{}
	if err := cmd.Run(cfg, nil); err != nil {
		t.Fatalf("validate run: %v", err)
	}
}

func TestValidateRunFailsOnDuplicateRoute(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "first.md"), "---\ntitle: First\nslug: dupe\nlayout: page\n---\n\n# First")
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "second.md"), "---\ntitle: Second\nslug: dupe\nlayout: page\n---\n\n# Second")

	cmd := command{}
	if err := cmd.Run(cfg, nil); err == nil {
		t.Fatal("expected duplicate route error")
	}
}

func TestValidateRunFailsOnBrokenMediaAndInternalLinks(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), "---\ntitle: About\nslug: about\nlayout: page\n---\n\n[Broken page](/missing/)\n\n![Missing media](media:images/missing.png)")

	cmd := command{}
	if err := cmd.Run(cfg, nil); err == nil {
		t.Fatal("expected broken media/internal link validation failure")
	}
}

func TestValidateCommandMetadata(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "validate" {
		t.Fatalf("unexpected command name: %q", cmd.Name())
	}
	if cmd.Summary() == "" || cmd.Group() == "" || !cmd.RequiresConfig() {
		t.Fatal("expected populated command metadata")
	}
	if cmd.Details() != nil {
		t.Fatal("expected nil details")
	}
}

func TestValidateRunFailsWhenThemeMissing(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	cmd := command{}
	if err := cmd.Run(cfg, nil); err == nil {
		t.Fatal("expected missing theme failure")
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
