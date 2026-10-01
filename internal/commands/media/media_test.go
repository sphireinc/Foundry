package mediacmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/admin/service"
	"github.com/sphireinc/foundry/internal/config"
)

func TestTrashPreviewApplyAndRestore(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{ContentDir: filepath.Join(root, "content"), PublicDir: filepath.Join(root, "public"), DataDir: filepath.Join(root, "data"), ThemesDir: filepath.Join(root, "themes"), PluginsDir: filepath.Join(root, "plugins"), Theme: "default"}
	cfg.ApplyDefaults()
	images := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir)
	if err := os.MkdirAll(images, 0o750); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(images, "unused.png")
	if err := os.WriteFile(filename, []byte("original bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename+".meta.yaml", []byte("alt: A tree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := command{}
	reference := "media:images/unused.png"
	if err := cmd.Run(cfg, []string{"foundry", "media", "trash", reference}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatal("preview changed file")
	}
	if err := cmd.Run(cfg, []string{"foundry", "media", "trash", reference, "--apply"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatal("apply did not trash original")
	}
	trash, err := service.New(cfg).ListMediaTrash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(trash) != 1 {
		t.Fatalf("expected one recoverable media file: %#v", trash)
	}
	if err := cmd.Run(cfg, []string{"foundry", "media", "restore", trash[0].Path}); err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- test-owned original path verifies byte-for-byte restoration.
	body, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "original bytes" {
		t.Fatal("restore lost original bytes")
	}
	// #nosec G304 -- test-owned sidecar path verifies restoration.
	body, err = os.ReadFile(filename + ".meta.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "alt: A tree\n" {
		t.Fatal("restore lost metadata")
	}
	pages := filepath.Join(cfg.ContentDir, cfg.Content.PagesDir)
	if err := os.MkdirAll(pages, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pages, "index.md"), []byte("![tree](media:images/unused.png)"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "media", "trash", reference, "--apply"}); err == nil {
		t.Fatal("referenced file was trashed")
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatal("rejected trash changed file")
	}
}

func TestTrashRefusesImplicitCSSBundleInput(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{ContentDir: filepath.Join(root, "content"), PublicDir: filepath.Join(root, "public"), DataDir: filepath.Join(root, "data"), ThemesDir: filepath.Join(root, "themes"), PluginsDir: filepath.Join(root, "plugins"), Theme: "default"}
	cfg.ApplyDefaults()
	directory := filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, "css")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "site.css")
	if err := os.WriteFile(filename, []byte("body { color: red }"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"foundry", "media", "trash", "media:assets/css/site.css"}, {"foundry", "media", "trash", "media:assets/css/site.css", "--apply"}} {
		if err := (command{}).Run(cfg, args); err == nil {
			t.Fatal("allowed bundled CSS to be trashed")
		}
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("bundled CSS was changed: %v", err)
	}
}
