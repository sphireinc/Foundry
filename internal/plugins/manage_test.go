package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListInstalledAndConfigToggles(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "beta", "name: beta\nrepo: github.com/acme/beta\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "beta")
	writePluginMetaFile(t, root, "alpha", "name: alpha\nrepo: github.com/acme/alpha\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "alpha")
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	installed, err := ListInstalled(root)
	if err != nil {
		t.Fatalf("list installed: %v", err)
	}
	if len(installed) != 2 || installed[0].Name != "alpha" || installed[1].Name != "beta" {
		t.Fatalf("unexpected installed plugins: %#v", installed)
	}

	cfgPath := filepath.Join(t.TempDir(), "site.yaml")
	if err := os.WriteFile(cfgPath, []byte("plugins:\n  enabled:\n    - alpha\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := EnableInConfig(cfgPath, "beta"); err != nil {
		t.Fatalf("enable in config: %v", err)
	}
	if err := DisableInConfig(cfgPath, "alpha"); err != nil {
		t.Fatalf("disable in config: %v", err)
	}

	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(body) == "" || !containsAll(string(body), "beta") {
		t.Fatalf("unexpected config body: %s", string(body))
	}
}

func TestUpdateInstalledRejectsInvalidCases(t *testing.T) {
	root := t.TempDir()
	if _, err := UpdateInstalled(root, ""); err == nil {
		t.Fatal("expected empty name error")
	}
	if _, err := UpdateInstalled(root, "missing"); err == nil {
		t.Fatal("expected missing plugin error")
	}

	writePluginMetaFile(t, root, "alpha", "name: alpha\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "alpha")
	if _, err := UpdateInstalled(root, "alpha"); err == nil {
		t.Fatal("expected fallback update error without repo metadata")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
