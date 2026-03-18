package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

type assetHooks struct{ called bool }

func (h *assetHooks) OnAssetsBuilding(*config.Config) error { h.called = true; return nil }

func TestSyncCopiesAssetsAndBuildsBundle(t *testing.T) {
	cfg := testAssetsConfig(t)
	writeFile(t, filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, "css", "content.css"), "body { color: red; }")
	writeFile(t, filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "logo.txt"), "img")
	writeFile(t, filepath.Join(cfg.ContentDir, cfg.Content.UploadsDir, "file.txt"), "upload")
	writeFile(t, filepath.Join(cfg.ThemesDir, cfg.Theme, "assets", "css", "base.css"), "html { color: black; }")
	writeFile(t, filepath.Join(cfg.PluginsDir, "toc", "assets", "toc.css"), ".toc {}")

	hooks := &assetHooks{}
	if err := Sync(cfg, hooks); err != nil {
		t.Fatalf("sync assets: %v", err)
	}
	if !hooks.called {
		t.Fatal("expected assets hook to be called")
	}

	bundle, err := os.ReadFile(filepath.Join(cfg.PublicDir, "assets", "css", "foundry.bundle.css"))
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	body := string(bundle)
	if !strings.Contains(body, "content.css") || !strings.Contains(body, "base.css") {
		t.Fatalf("expected bundled css comments, got %q", body)
	}
	for _, rel := range []string{"images/logo.txt", "uploads/file.txt", "plugins/toc/toc.css"} {
		if _, err := os.Stat(filepath.Join(cfg.PublicDir, rel)); err != nil {
			t.Fatalf("expected copied asset %s: %v", rel, err)
		}
	}
}

func TestAssetHelpers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.css"), "a{}")
	writeFile(t, filepath.Join(root, "sub", "b.CSS"), "b{}")

	files, err := listFiles(root, ".css")
	if err != nil || len(files) != 2 {
		t.Fatalf("expected css files, got %v %v", files, err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := copyDirIfExists(root, dst); err != nil {
		t.Fatalf("copy dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "b.CSS")); err != nil {
		t.Fatalf("expected copied file: %v", err)
	}

	if err := copyFile(filepath.Join(root, "a.css"), filepath.Join(dst, "single.css"), 0o644); err != nil {
		t.Fatalf("copy file: %v", err)
	}
}

func TestSyncRejectsUnsafeThemeAndPluginNames(t *testing.T) {
	cfg := testAssetsConfig(t)
	cfg.Theme = ".."
	if err := Sync(cfg, nil); err == nil {
		t.Fatal("expected unsafe theme name rejection")
	}

	cfg = testAssetsConfig(t)
	cfg.Plugins.Enabled = []string{".."}
	if err := Sync(cfg, nil); err == nil {
		t.Fatal("expected unsafe plugin name rejection")
	}
}

func TestAssetHelpersRejectSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.css")
	writeFile(t, outside, "body{}")
	link := filepath.Join(root, "linked.css")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if _, err := listFiles(root, ".css"); err == nil {
		t.Fatal("expected listFiles to reject symlinked asset")
	}
	if err := copyDirIfExists(root, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected copyDirIfExists to reject symlinked asset")
	}
	if err := copyFile(link, filepath.Join(t.TempDir(), "single.css"), os.ModeSymlink); err == nil {
		t.Fatal("expected copyFile to reject symlink mode")
	}
}

func testAssetsConfig(t *testing.T) *config.Config {
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
		Build: config.BuildConfig{
			CopyAssets:  true,
			CopyImages:  true,
			CopyUploads: true,
		},
		Plugins: config.PluginConfig{Enabled: []string{"toc"}},
	}
	cfg.ApplyDefaults()
	return cfg
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
