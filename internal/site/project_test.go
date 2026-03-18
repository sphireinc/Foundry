package site

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/diag"
)

func TestNewPluginManagerAndLoadConfiguredGraph(t *testing.T) {
	cfg := testSiteConfig(t)

	pm, err := NewPluginManager(cfg)
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}
	if pm == nil {
		t.Fatal("expected plugin manager")
	}

	graph, pm2, err := LoadConfiguredGraph(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("load configured graph: %v", err)
	}
	if pm2 == nil || len(graph.Documents) != 1 {
		t.Fatalf("unexpected load configured graph result: %#v %#v", pm2, graph)
	}

	graph2, err := LoadGraphWithManager(context.Background(), cfg, pm, true)
	if err != nil {
		t.Fatalf("load graph with manager: %v", err)
	}
	if len(graph2.Documents) != 1 || graph2.Documents[0].URL != "/about/" {
		t.Fatalf("unexpected graph load result: %#v", graph2.Documents)
	}
}

func TestProjectHelpersErrorPaths(t *testing.T) {
	cfg := testSiteConfig(t)

	if _, err := LoadGraphWithManager(context.Background(), nil, nil, true); diag.KindOf(err) != diag.KindInternal {
		t.Fatalf("expected nil config internal error, got %v", err)
	}
	if _, err := LoadGraphWithManager(context.Background(), cfg, nil, true); diag.KindOf(err) != diag.KindInternal {
		t.Fatalf("expected nil manager internal error, got %v", err)
	}

	badCfg := testSiteConfig(t)
	badCfg.Plugins.Enabled = []string{"broken"}
	if err := os.MkdirAll(filepath.Join(badCfg.PluginsDir, "broken"), 0o755); err != nil {
		t.Fatalf("mkdir broken plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badCfg.PluginsDir, "broken", "plugin.yaml"), []byte("name: broken\nfoundry_api: v2\nmin_foundry_version: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write broken metadata: %v", err)
	}
	if _, err := NewPluginManager(badCfg); diag.KindOf(err) != diag.KindPlugin {
		t.Fatalf("expected plugin manager load error, got %v", err)
	}
	if _, _, err := LoadConfiguredGraph(context.Background(), badCfg, true); diag.KindOf(err) != diag.KindPlugin {
		t.Fatalf("expected configured graph plugin error, got %v", err)
	}
}

func testSiteConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
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
		cfg.PluginsDir,
		cfg.DataDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	body := "---\ntitle: About\nslug: about\nlayout: page\n---\n\n# About"
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write content: %v", err)
	}
	return cfg
}
