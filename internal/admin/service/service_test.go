package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

func TestLoadCachesGraphsWithinTTL(t *testing.T) {
	cfg := testServiceConfig(t)
	var loads int

	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		loads++
		return content.NewSiteGraph(cfg), nil
	}))

	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if loads != 1 {
		t.Fatalf("expected one graph load, got %d", loads)
	}
}

func TestSaveDocumentInvalidatesGraphCache(t *testing.T) {
	cfg := testServiceConfig(t)
	var loads int

	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		loads++
		return content.NewSiteGraph(cfg), nil
	}))

	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("prime cache failed: %v", err)
	}
	if loads != 1 {
		t.Fatalf("expected one graph load, got %d", loads)
	}

	_, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{
		SourcePath: filepath.Join("pages", "cache-test.md"),
		Raw:        "---\ntitle: Cache Test\nslug: cache-test\n---\n\nBody",
	})
	if err != nil {
		t.Fatalf("save document failed: %v", err)
	}

	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("load after save failed: %v", err)
	}
	if loads != 2 {
		t.Fatalf("expected cache invalidation to force second load, got %d loads", loads)
	}
}

func testServiceConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	cfg := &config.Config{
		ContentDir: filepath.Join(root, "content"),
		PublicDir:  filepath.Join(root, "public"),
		ThemesDir:  filepath.Join(root, "themes"),
		PluginsDir: filepath.Join(root, "plugins"),
		DataDir:    filepath.Join(root, "data"),
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
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir), 0o755); err != nil {
		t.Fatalf("mkdir pages dir: %v", err)
	}
	return cfg
}
