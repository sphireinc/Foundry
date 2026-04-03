package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetadataAndValidationHelpers(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "good", "name: good\nrepo: https://github.com/acme/good.git\nrequires:\n  - github.com/acme/dep\n  - github.com/acme/dep\nfoundry_api: v1\nmin_foundry_version: 0.1.0\nadmin:\n  pages:\n    - key: search-console\n      title: Search Console\n      route: /plugins/search\n      nav_group: manage\n")
	writePluginCodeFile(t, root, "good")
	writePluginMetaFile(t, root, "dep", "name: dep\nrepo: github.com/acme/dep\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "dep")

	meta, err := LoadMetadata(root, "good")
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta.Repo != "github.com/acme/good" || len(meta.Requires) != 1 {
		t.Fatalf("unexpected metadata normalization: %#v", meta)
	}
	if got := meta.AdminExtensions.Pages[0].NavGroup; got != "manage" {
		t.Fatalf("expected normalized nav group, got %q", got)
	}
	all, err := LoadAllMetadata(root, []string{"good", "dep"})
	if err != nil || len(all) != 2 {
		t.Fatalf("unexpected load all metadata: %#v %v", all, err)
	}

	if normalizeRepoRef("https://github.com/acme/repo.git") != "github.com/acme/repo" {
		t.Fatal("unexpected repo normalization")
	}
	if err := validateMetadataCompatibility(meta); err != nil {
		t.Fatalf("expected metadata compatibility: %v", err)
	}
	if err := validateDependencies(all); err != nil {
		t.Fatalf("expected dependency validation to pass: %v", err)
	}
	if issue := (ValidationIssue{Name: "x", Status: "bad"}).String(); issue == "" {
		t.Fatal("expected validation issue string")
	}
	if status := enabledPluginStatus(root, "good"); status != "enabled" {
		t.Fatalf("unexpected enabled plugin status: %q", status)
	}
}

func TestValidationFailures(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "bad", "name: bad\nfoundry_api: v2\nmin_foundry_version: 0.1.0\n")
	if status := enabledPluginStatus(root, "bad"); status != "api unsupported" {
		t.Fatalf("unexpected bad plugin status: %q", status)
	}

	report := ValidateEnabledPlugins(root, []string{"bad"})
	if len(report.Issues) == 0 {
		t.Fatal("expected validation issues")
	}

	writePluginMetaFile(t, root, "bad-nav", "name: bad-nav\nfoundry_api: v1\nmin_foundry_version: 0.1.0\nadmin:\n  pages:\n    - key: console\n      title: Console\n      route: /plugins/console\n      nav_group: misc\n")
	if _, err := LoadMetadata(root, "bad-nav"); err == nil {
		t.Fatal("expected invalid nav_group to fail metadata load")
	}
}

func TestEnabledPluginStatusBranches(t *testing.T) {
	root := t.TempDir()
	if status := enabledPluginStatus(root, ""); status != "invalid name" {
		t.Fatalf("unexpected empty-name status: %q", status)
	}
	if status := enabledPluginStatus(root, "missing"); status != "not installed" {
		t.Fatalf("unexpected missing-metadata status: %q", status)
	}

	writePluginMetaFile(t, root, "api-missing", "name: api-missing\nmin_foundry_version: 0.1.0\n")
	if status := enabledPluginStatus(root, "api-missing"); status != "api missing" {
		t.Fatalf("unexpected api-missing status: %q", status)
	}

	writePluginMetaFile(t, root, "version-missing", "name: version-missing\nfoundry_api: v1\n")
	if status := enabledPluginStatus(root, "version-missing"); status != "version missing" {
		t.Fatalf("unexpected version-missing status: %q", status)
	}

	writePluginMetaFile(t, root, "code-missing", "name: code-missing\nrepo: github.com/acme/code-missing\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	if status := enabledPluginStatus(root, "code-missing"); status != "code missing" {
		t.Fatalf("unexpected code-missing status: %q", status)
	}

	writePluginMetaFile(t, root, "enabled", "name: enabled\nrepo: github.com/acme/enabled\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
	writePluginCodeFile(t, root, "enabled")
	if status := enabledPluginStatus(root, "enabled"); status != "enabled" {
		t.Fatalf("unexpected enabled status: %q", status)
	}
}

func writePluginMetaFile(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write plugin metadata: %v", err)
	}
}

func writePluginCodeFile(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name, "plugin.go"), []byte("package "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write plugin code: %v", err)
	}
}
