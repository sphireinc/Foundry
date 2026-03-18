package clean

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestCommandMetadataAndRun(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "clean" || !cmd.RequiresConfig() {
		t.Fatalf("unexpected command metadata")
	}

	root := t.TempDir()
	cfg := &config.Config{PublicDir: filepath.Join(root, "public")}
	cfg.ApplyDefaults()
	for _, p := range []string{cfg.PublicDir, filepath.Join(root, "bin"), filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
	}

	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := cmd.Run(cfg, nil); err != nil {
		t.Fatalf("run clean: %v", err)
	}
	if _, err := os.Stat(cfg.PublicDir); !os.IsNotExist(err) {
		t.Fatalf("expected public dir removed, got %v", err)
	}
}

func TestCommandRejectsUnsafePath(t *testing.T) {
	cmd := command{}
	cfg := &config.Config{PublicDir: "."}
	if err := cmd.Run(cfg, nil); err == nil {
		t.Fatal("expected unsafe path error")
	}
}
