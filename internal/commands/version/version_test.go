package version

import (
	"strings"
	"testing"
)

func TestCommandMetadataAndString(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "version" || cmd.Group() == "" || cmd.RequiresConfig() {
		t.Fatalf("unexpected command metadata")
	}
	out := String()
	if !strings.Contains(out, "Foundry") ||
		!strings.Contains(out, "Commit:") ||
		!strings.Contains(out, "Built:") ||
		!strings.Contains(out, "Install mode:") {
		t.Fatalf("unexpected version string: %q", out)
	}
	if err := cmd.Run(nil, nil); err != nil {
		t.Fatalf("run version: %v", err)
	}
}

func TestSourceDisplayVersion(t *testing.T) {
	meta := Metadata{
		Version:     "v1.3.5",
		NearestTag:  "v1.3.5",
		Commit:      "82d13ba",
		CommitCount: 10,
		Dirty:       true,
	}
	if got := sourceDisplayVersion(meta); got != "v1.3.5+10.g82d13ba-dirty" {
		t.Fatalf("unexpected source display version: %q", got)
	}
}

func TestEmbeddedVersionFallback(t *testing.T) {
	if got := embeddedVersion(); strings.TrimSpace(got) == "" {
		t.Fatal("expected embedded version fallback to be non-empty")
	}
}

func TestCurrentReportsManagedRuntimeFromEnvironment(t *testing.T) {
	t.Setenv("FOUNDRY_MANAGED_RUNTIME", "true")
	meta := Current(t.TempDir())
	if !meta.ManagedRuntime {
		t.Fatal("expected managed runtime metadata when env flag is set")
	}
	if !strings.Contains(meta.String(), "Managed runtime: enabled") {
		t.Fatalf("expected version string to show managed runtime, got %q", meta.String())
	}
}
