package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncHelpers(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "site.yaml")
	if err := os.WriteFile(configPath, []byte("plugins:\n  enabled:\n    - beta\n    - alpha\n    - alpha\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	writePluginMetaFile(t, pluginsDir, "alpha", "name: alpha\nrepo: github.com/acme/alpha\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, pluginsDir, "alpha")
	writePluginMetaFile(t, pluginsDir, "beta", "name: beta\nrepo: github.com/acme/beta\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, pluginsDir, "beta")

	outPath := filepath.Join(root, "internal", "generated", "plugins_gen.go")
	opts := SyncOptions{
		ConfigPath: configPath,
		PluginsDir: pluginsDir,
		OutputPath: outPath,
		ModulePath: "example.com/foundry",
	}
	if err := SyncFromConfig(opts); err != nil {
		t.Fatalf("sync from config: %v", err)
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated imports: %v", err)
	}
	if !strings.Contains(string(body), "example.com/foundry/plugins/alpha") || !strings.Contains(string(body), "example.com/foundry/plugins/beta") {
		t.Fatalf("unexpected imports file: %s", string(body))
	}

	if cfg, err := loadSyncConfig(configPath); err != nil || len(cfg.Plugins.Enabled) != 3 {
		t.Fatalf("unexpected loaded sync config: %#v %v", cfg, err)
	}
	if len(uniqueSorted([]string{" beta ", "alpha", "alpha", ""})) != 2 {
		t.Fatal("expected uniqueSorted to trim and dedupe")
	}
	if !isValidRepoRef("github.com/acme/repo") || isValidRepoRef("repo") {
		t.Fatal("unexpected repo ref validation")
	}
}

func TestSyncValidationFailures(t *testing.T) {
	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(filepath.Join(pluginsDir, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	writePluginMetaFile(t, pluginsDir, "alpha", "name: alpha\nrepo: github.com/acme/alpha\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")

	if err := validatePluginForSync(pluginsDir, ""); err == nil {
		t.Fatal("expected empty name error")
	}
	if err := validatePluginForSync(pluginsDir, "alpha/bad"); err == nil {
		t.Fatal("expected invalid path error")
	}
	if err := validatePluginForSync(pluginsDir, "missing"); err == nil {
		t.Fatal("expected missing plugin error")
	}
	if err := validatePluginForSync(pluginsDir, "alpha"); err == nil {
		t.Fatal("expected missing go file error")
	}

	opts := normalizeSyncOptions(SyncOptions{})
	if opts.ConfigPath == "" || opts.PluginsDir == "" || opts.OutputPath == "" || opts.ModulePath == "" {
		t.Fatal("expected sync defaults to be populated")
	}
}
