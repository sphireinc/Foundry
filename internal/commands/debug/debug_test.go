package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestDebugCommandRun(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	writeMarkdown(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), "---\ntitle: About\nslug: about\nlayout: page\n---\n\n# About")

	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "debug", "routes"}); err != nil {
		t.Fatalf("debug routes: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "debug", "plugins"}); err != nil {
		t.Fatalf("debug plugins: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "debug", "config"}); err != nil {
		t.Fatalf("debug config: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "debug"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "debug", "bad"}); err == nil {
		t.Fatal("expected unknown subcommand error")
	}
}

func TestDebugCommandMetadataAndDetectHooks(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "debug" {
		t.Fatalf("unexpected command name: %q", cmd.Name())
	}
	if cmd.Summary() == "" || cmd.Group() == "" || !cmd.RequiresConfig() || len(cmd.Details()) != 3 {
		t.Fatal("expected populated command metadata")
	}

	hooks := detectHooks(&testDebugPlugin{})
	if len(hooks) == 0 {
		t.Fatal("expected detected hooks")
	}
	if implements[interface{ Missing() }](testDebugPlugin{}) {
		t.Fatal("expected unrelated interface check to be false")
	}
}

type testDebugPlugin struct{}

func (testDebugPlugin) Name() string { return "debug" }
func (testDebugPlugin) OnBuildStarted() error {
	return nil
}

func TestRunPluginsWithMissingEnabledPlugin(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		PluginsDir:  root,
		ContentDir:  t.TempDir(),
		DefaultLang: "en",
	}
	cfg.ApplyDefaults()
	cfg.Plugins.Enabled = []string{"broken"}
	if err := os.MkdirAll(filepath.Join(root, "broken"), 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken", "plugin.yaml"), []byte("name: broken\nfoundry_api: v2\nmin_foundry_version: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	err := runPlugins(cfg)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected plugin load error, got %v", err)
	}
}

func TestRunPluginsWithRegisteredPlugin(t *testing.T) {
	const name = "debug-test-plugin"
	plugins.Register(name, func() plugins.Plugin { return testDebugPluginWithName(name) })

	root := t.TempDir()
	cfg := &config.Config{
		PluginsDir:  root,
		ContentDir:  t.TempDir(),
		DefaultLang: "en",
	}
	cfg.ApplyDefaults()
	cfg.Plugins.Enabled = []string{name}
	if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, name, "plugin.yaml"), []byte("name: "+name+"\nrepo: github.com/acme/"+name+"\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, name, "plugin.go"), []byte("package "+strings.ReplaceAll(name, "-", "")+"\n"), 0o644); err != nil {
		t.Fatalf("write plugin file: %v", err)
	}

	if err := runPlugins(cfg); err != nil {
		t.Fatalf("expected registered plugin output, got %v", err)
	}
}

type testDebugPluginWithName string

func (p testDebugPluginWithName) Name() string          { return string(p) }
func (p testDebugPluginWithName) OnBuildStarted() error { return nil }

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
