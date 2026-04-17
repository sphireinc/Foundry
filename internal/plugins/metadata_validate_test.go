package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetadataAndValidationHelpers(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "good", "name: good\nrepo: https://github.com/acme/good.git\nrequires:\n  - github.com/acme/dep\n  - github.com/acme/dep\nfoundry_api: v1\nmin_foundry_version: 0.1.0\npermissions:\n  admin:\n    extensions:\n      pages: true\nadmin:\n  pages:\n    - key: search-console\n      title: Search Console\n      route: /plugins/search\n      nav_group: manage\n")
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

	writePluginMetaFile(t, root, "bad-risk", "name: bad-risk\nfoundry_api: v1\nmin_foundry_version: 0.1.0\npermissions:\n  process:\n    exec:\n      allowed: true\n      commands: [git]\n")
	if _, err := LoadMetadata(root, "bad-risk"); err == nil {
		t.Fatal("expected dangerous permissions without approval to fail metadata load")
	}
}

func TestPermissionNormalizationAndSummary(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "secure", "name: secure\nfoundry_api: v1\nmin_foundry_version: 0.1.0\npermissions:\n  content:\n    documents:\n      read: true\n  render:\n    context:\n      read: true\n      write: true\n  capabilities:\n    requires_admin_approval: false\n  network:\n    outbound:\n      http: true\n      methods: [post, get, get]\n      domains: [api.example.com, api.example.com]\n")
	meta, err := LoadMetadata(root, "secure")
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if got := meta.Permissions.RiskTier(); got != "medium" {
		t.Fatalf("expected medium risk tier, got %q", got)
	}
	if got := meta.Permissions.Network.Outbound.Methods; len(got) != 2 || got[0] != "GET" || got[1] != "POST" {
		t.Fatalf("expected normalized methods, got %#v", got)
	}
	if got := meta.Permissions.Summary(); len(got) == 0 {
		t.Fatal("expected permission summary")
	}
}

func TestAnalyzeInstalledDetectsUndeclaredCapabilitiesAndRuntime(t *testing.T) {
	root := t.TempDir()
	writePluginMetaFile(t, root, "risky", "name: risky\nfoundry_api: v1\nmin_foundry_version: 0.1.0\nruntime:\n  mode: rpc\n  command: [./bin/risky]\n")
	dir := filepath.Join(root, "risky")
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte(`package risky
import (
  "os"
  "os/exec"
)
func (p *Plugin) Name() string { return "risky" }
func (p *Plugin) OnDocumentParsed(doc any) error {
  _, _ = os.ReadFile("content/page.md")
  _, _ = exec.Command("sh", "-lc", "echo hi").Output()
  return nil
}`), 0o644); err != nil {
		t.Fatalf("write plugin code: %v", err)
	}
	meta, err := LoadMetadata(root, "risky")
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	report := AnalyzeInstalled(meta)
	if !report.RequiresApproval || report.Runtime.Mode != "rpc" {
		t.Fatalf("expected approval-required rpc report, got %#v", report)
	}

	writePluginMetaFile(t, root, "risky-approved", "name: risky-approved\nfoundry_api: v1\nmin_foundry_version: 0.1.0\npermissions:\n  capabilities:\n    requires_admin_approval: true\nruntime:\n  mode: rpc\n  command: [./bin/risky]\n")
	dir = filepath.Join(root, "risky-approved")
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte(`package riskyapproved
import (
  "os"
  "os/exec"
)
func (p *Plugin) Name() string { return "risky-approved" }
func (p *Plugin) OnDocumentParsed(doc any) error {
  _, _ = os.ReadFile("content/page.md")
  _, _ = exec.Command("sh", "-lc", "echo hi").Output()
  return nil
}`), 0o644); err != nil {
		t.Fatalf("write plugin code: %v", err)
	}
	meta, err = LoadMetadata(root, "risky-approved")
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	report = AnalyzeInstalled(meta)
	if len(report.Findings) == 0 || len(report.Mismatches) == 0 {
		t.Fatalf("expected findings and mismatches, got %#v", report)
	}
	if !report.RequiresApproval || report.Runtime.Mode != "rpc" {
		t.Fatalf("expected approval-required rpc report, got %#v", report)
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

func TestLooksLikeSecretPathDoesNotFlagTokenBudgetFields(t *testing.T) {
	for _, value := range []string{"max_tokens", "max_output_tokens", "maxOutputTokens"} {
		if looksLikeSecretPath(value) {
			t.Fatalf("expected %q not to be classified as a secret path", value)
		}
	}
	for _, value := range []string{".env", "config/secrets.yaml", "admin/session/store.yaml", "api-token"} {
		if !looksLikeSecretPath(value) {
			t.Fatalf("expected %q to be classified as a secret path", value)
		}
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
