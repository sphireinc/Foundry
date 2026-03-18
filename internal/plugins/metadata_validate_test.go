package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetadataAndValidationHelpers(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "good", "name: good\nrepo: https://github.com/acme/good.git\nrequires:\n  - github.com/acme/dep\n  - github.com/acme/dep\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n")
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
