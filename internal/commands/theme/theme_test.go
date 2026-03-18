package themecmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestThemeCommandRun(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Theme:     "default",
		ThemesDir: filepath.Join(root, "themes"),
	}
	cfg.ApplyDefaults()

	if _, err := theme.Scaffold(cfg.ThemesDir, "default"); err != nil {
		t.Fatalf("scaffold default theme: %v", err)
	}

	writeConfigFile(t, root, "theme: default\n")
	chdirTempRoot(t, root)

	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "theme", "list"}); err != nil {
		t.Fatalf("theme list: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "theme", "current"}); err != nil {
		t.Fatalf("theme current: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "theme", "validate", "default"}); err != nil {
		t.Fatalf("theme validate: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "theme", "scaffold", "new-theme"}); err != nil {
		t.Fatalf("theme scaffold: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "theme", "switch", "default"}); err != nil {
		t.Fatalf("theme switch: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "theme"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "theme", "missing"}); err == nil {
		t.Fatal("expected unknown subcommand error")
	}
}

func writeConfigFile(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, "content", "config", "site.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func chdirTempRoot(t *testing.T, root string) {
	t.Helper()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
}
