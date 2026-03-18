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

func TestManageHelpersValidation(t *testing.T) {
	if _, err := ListInstalled(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("expected missing plugins dir to be empty list, got %v", err)
	}
	if err := ValidateInstalledPlugin(t.TempDir(), "missing"); err == nil {
		t.Fatal("expected validate installed failure")
	}
	if err := EnableInConfig(filepath.Join(t.TempDir(), "site.yaml"), ""); err == nil {
		t.Fatal("expected empty plugin enable failure")
	}
	if err := DisableInConfig(filepath.Join(t.TempDir(), "site.yaml"), ""); err == nil {
		t.Fatal("expected empty plugin disable failure")
	}
}

func TestInstallHelpersAndUninstall(t *testing.T) {
	root := t.TempDir()
	if normalizeInstallURL("acme/demo") != "https://github.com/acme/demo.git" {
		t.Fatal("expected GitHub shorthand normalization")
	}
	if normalizeInstallURL("https://github.com/acme/demo") != "https://github.com/acme/demo.git" {
		t.Fatal("expected GitHub https normalization")
	}
	if normalizeInstallURL("git@github.com:acme/demo.git") != "git@github.com:acme/demo.git" {
		t.Fatal("expected git URL preserved")
	}
	if normalizeInstallURL("") != "" {
		t.Fatal("expected empty install URL to stay empty")
	}

	if name, err := inferPluginName("https://github.com/acme/demo.git"); err != nil || name != "demo" {
		t.Fatalf("unexpected inferred name: %q %v", name, err)
	}
	if name, err := inferPluginName("git@github.com:acme/demo.git"); err != nil || name != "demo" {
		t.Fatalf("unexpected inferred git name: %q %v", name, err)
	}
	if _, err := inferPluginName("https://github.com"); err == nil {
		t.Fatal("expected infer name failure")
	}

	if zipURL, err := repoZipURL("https://github.com/acme/demo.git"); err != nil || zipURL != "https://github.com/acme/demo/archive/refs/heads/main.zip" {
		t.Fatalf("unexpected zip URL: %q %v", zipURL, err)
	}
	if _, err := repoZipURL("https://example.com/acme/demo.git"); err == nil {
		t.Fatal("expected non-GitHub zip URL rejection")
	}

	if _, err := Install(InstallOptions{}); err == nil {
		t.Fatal("expected install usage failure")
	}

	pluginDir := filepath.Join(root, "alpha")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := Uninstall(root, "alpha"); err != nil {
		t.Fatalf("uninstall plugin: %v", err)
	}
	if err := Uninstall(root, "alpha"); err == nil {
		t.Fatal("expected uninstall missing plugin failure")
	}
	if err := Uninstall(root, ""); err == nil {
		t.Fatal("expected uninstall empty name failure")
	}
	if err := Uninstall("", "alpha"); err == nil {
		t.Fatal("expected uninstall empty dir failure")
	}
	if err := Uninstall(root, "bad/name"); err == nil {
		t.Fatal("expected uninstall invalid name failure")
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
