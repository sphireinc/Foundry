package assetscmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestCommandMetadataAndRun(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "assets" || !cmd.RequiresConfig() || len(cmd.Details()) != 3 {
		t.Fatalf("unexpected command metadata")
	}

	cfg := testAssetsCmdConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, "app.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	if err := cmd.Run(cfg, []string{"foundry", "assets", "build"}); err != nil {
		t.Fatalf("assets build: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "assets", "list"}); err != nil {
		t.Fatalf("assets list: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "assets", "clean"}); err != nil {
		t.Fatalf("assets clean: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "assets"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "assets", "missing"}); err == nil {
		t.Fatal("expected unknown subcommand error")
	}
}

func testAssetsCmdConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		Theme:      "default",
		ContentDir: filepath.Join(root, "content"),
		PublicDir:  filepath.Join(root, "public"),
		ThemesDir:  filepath.Join(root, "themes"),
		PluginsDir: filepath.Join(root, "plugins"),
		Content: config.ContentConfig{
			AssetsDir:  "assets",
			ImagesDir:  "images",
			UploadsDir: "uploads",
		},
		Build: config.BuildConfig{CopyAssets: true},
	}
	cfg.ApplyDefaults()
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir), 0o755); err != nil {
		t.Fatalf("mkdir assets dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.ThemesDir, cfg.Theme, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir theme assets dir: %v", err)
	}
	return cfg
}
