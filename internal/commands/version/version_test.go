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

func TestBuildClassification(t *testing.T) {
	oldTag, oldImage := BuildTag, ContainerImage
	t.Cleanup(func() { BuildTag, ContainerImage = oldTag, oldImage })
	for _, tc := range []struct {
		name, tag, module, mode string
		dirty                   bool
		kind                    string
		comparable              bool
	}{
		{"fallback", "", "", "binary", false, "unknown", false},
		{"snapshot", "", "", "source", false, "source_snapshot", false},
		{"release", "v1.4.6", "", "binary", false, "tagged_release", true},
		{"module release", "", "v1.4.6", "binary", false, "tagged_release", true},
		{"pseudo version", "", "v1.4.7-0.20260101000000-abcdef123456", "binary", false, "source_snapshot", false},
		{"modified", "v1.4.6", "", "standalone", true, "modified_build", false},
		{"container tag", "v1.4.6", "", "docker", false, "tagged_release", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			BuildTag, ContainerImage = tc.tag, "runtime-v1.4.6"
			meta := Metadata{Version: "v1.4.6", Commit: "unknown", ModuleVersion: tc.module, InstallMode: tc.mode, Dirty: tc.dirty}
			if tc.name == "pseudo version" {
				meta.Commit = "abcdef123456"
			}
			classifyBuild(&meta)
			if meta.BuildKind != tc.kind || meta.ReleaseComparable != tc.comparable {
				t.Fatalf("got %+v", meta)
			}
		})
	}
}

func TestContainerIgnoresSiteCheckout(t *testing.T) {
	t.Setenv("FOUNDRY_CONTAINER", "true")
	meta := Current("../../..")
	if meta.InstallMode != "docker" || meta.NearestTag != "" {
		t.Fatalf("site checkout leaked into container identity: %+v", meta)
	}
}

func TestMissingCheckoutIsNotDirty(t *testing.T) {
	if gitDirty(t.TempDir()) {
		t.Fatal("missing repository reported as local modifications")
	}
}
