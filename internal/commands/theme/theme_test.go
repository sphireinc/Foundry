package themecmd

import (
	"os"
	"path/filepath"
	"strings"
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
	writeConfigFile(t, root, "theme: default\nfields:\n  enabled: true\n  allow_anything: false\n  schemas:\n    post:\n      fields:\n        - name: hero_title\n          type: text\n")
	if err := cmd.Run(cfg, []string{"foundry", "theme", "migrate", "field-contracts"}); err != nil {
		t.Fatalf("theme migrate field-contracts: %v", err)
	}
	manifestBody, err := os.ReadFile(filepath.Join(cfg.ThemesDir, "default", "theme.yaml"))
	if err != nil {
		t.Fatalf("read migrated theme manifest: %v", err)
	}
	if !strings.Contains(string(manifestBody), "field_contracts:") || !strings.Contains(string(manifestBody), "hero_title") {
		t.Fatalf("expected migrated field contracts in theme manifest, got %q", string(manifestBody))
	}
	configBody, err := os.ReadFile(filepath.Join(root, "content", "config", "site.yaml"))
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if strings.Contains(string(configBody), "\nfields:") {
		t.Fatalf("expected legacy fields block removed from config, got %q", string(configBody))
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
