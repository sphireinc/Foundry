package version

import (
	"os/exec"
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

func TestCheckoutMetadataRequiresMatchingRevision(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...) // #nosec G204 -- fixed test-defined Git arguments in an isolated temporary repository.
		cmd.Dir = root
		if body, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, body)
		}
	}
	run("init", "-q")
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "site")
	run("-c", "tag.gpgsign=false", "tag", "v9.9.9")
	revision := gitOutput(root, "rev-parse", "HEAD")
	for _, candidate := range []string{"", "unrelated", revision} {
		meta := Metadata{VCSRevision: candidate, ModuleVersion: "original", Commit: "build-commit"}
		applyCheckoutMetadata(&meta, root)
		if candidate == revision {
			if meta.ModuleVersion != "v9.9.9" || meta.NearestTag != "v9.9.9" {
				t.Fatalf("matching checkout not recognized: %+v", meta)
			}
		} else if meta.ModuleVersion != "original" || meta.NearestTag != "" || meta.Commit != "build-commit" {
			t.Fatalf("foreign checkout adopted: %+v", meta)
		}
	}
}

func TestContainerBuildModificationState(t *testing.T) {
	oldTag, oldContainer, oldModified := BuildTag, ContainerBuild, BuildModified
	t.Cleanup(func() { BuildTag, ContainerBuild, BuildModified = oldTag, oldContainer, oldModified })
	BuildTag, ContainerBuild = "v1.4.6", "true"
	for _, tc := range []struct {
		modified, kind string
		comparable     bool
	}{
		{"false", "tagged_release", true},
		{"true", "modified_build", false},
		{"unknown", "source_snapshot", false},
		{"", "source_snapshot", false},
	} {
		BuildModified = tc.modified
		meta := Metadata{Version: "v1.4.6", Commit: "abc123", InstallMode: "docker"}
		classifyBuild(&meta)
		if meta.BuildKind != tc.kind || meta.ReleaseComparable != tc.comparable {
			t.Fatalf("state %q: %+v", tc.modified, meta)
		}
	}
}
