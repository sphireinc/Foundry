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

	if _, err := validatePluginForSync(pluginsDir, ""); err == nil {
		t.Fatal("expected empty name error")
	}
	if _, err := validatePluginForSync(pluginsDir, "alpha/bad"); err == nil {
		t.Fatal("expected invalid path error")
	}
	if _, err := validatePluginForSync(pluginsDir, ".."); err == nil {
		t.Fatal("expected traversal plugin name error")
	}
	if _, err := validatePluginForSync(pluginsDir, "missing"); err == nil {
		t.Fatal("expected missing plugin error")
	}
	if _, err := validatePluginForSync(pluginsDir, "alpha"); err == nil {
		t.Fatal("expected missing go file error")
	}

	opts := normalizeSyncOptions(SyncOptions{})
	if opts.ConfigPath == "" || opts.PluginsDir == "" || opts.OutputPath == "" || opts.ModulePath == "" {
		t.Fatal("expected sync defaults to be populated")
	}
}

func TestSyncAllowsRPCPluginWithoutRootPackageImport(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "site.yaml")
	if err := os.WriteFile(configPath, []byte("plugins:\n  enabled:\n    - rpc-demo\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	dir := filepath.Join(pluginsDir, "rpc-demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "server"), 0o755); err != nil {
		t.Fatalf("mkdir rpc plugin: %v", err)
	}
	writePluginMetaFile(t, pluginsDir, "rpc-demo", "name: rpc-demo\nrepo: github.com/acme/rpc-demo\nfoundry_api: v1\nmin_foundry_version: 0.1.0\nruntime:\n  mode: rpc\n  protocol_version: v1alpha1\n  command: [go, run, ./cmd/server]\n  sandbox:\n    profile: default\n    allow_network: false\n    allow_filesystem_write: false\n    allow_process_exec: false\n")
	if err := os.WriteFile(filepath.Join(dir, "cmd", "server", "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatalf("write rpc main: %v", err)
	}
	outPath := filepath.Join(root, "internal", "generated", "plugins_gen.go")
	opts := SyncOptions{ConfigPath: configPath, PluginsDir: pluginsDir, OutputPath: outPath, ModulePath: "example.com/foundry"}
	if err := SyncFromConfig(opts); err != nil {
		t.Fatalf("sync from config: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated imports: %v", err)
	}
	if strings.Contains(string(body), "rpc-demo") {
		t.Fatalf("expected rpc plugin to be excluded from generated imports: %s", string(body))
	}
}
