package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMainSyncsPlugins(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "content", "config")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatalf("mkdir config path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "site.yaml"), []byte("plugins:\n  enabled:\n    - alpha\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	pluginDir := filepath.Join(root, "plugins", "alpha")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte("name: alpha\nrepo: github.com/acme/alpha\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.go"), []byte("package alpha\n"), 0o644); err != nil {
		t.Fatalf("write plugin code: %v", err)
	}

	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	main()

	if _, err := os.Stat(filepath.Join(root, "internal", "generated", "plugins_gen.go")); err != nil {
		t.Fatalf("expected generated plugin imports: %v", err)
	}
}
