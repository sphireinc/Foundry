package lifecycle

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBuildAndParsePaths(t *testing.T) {
	now := time.Date(2026, 3, 19, 14, 32, 10, 0, time.UTC)

	current := filepath.Join("content", "pages", "about.md")
	version := BuildVersionPath(current, now)
	trash := BuildTrashPath(current, now)

	if filepath.ToSlash(version) != "content/pages/about.version.20260319T143210Z.md" {
		t.Fatalf("unexpected version path: %s", version)
	}
	if filepath.ToSlash(trash) != "content/pages/about.trash.20260319T143210Z.md" {
		t.Fatalf("unexpected trash path: %s", trash)
	}

	original, state, ok := ParsePath(version)
	if !ok || state != StateVersion || filepath.ToSlash(original) != filepath.ToSlash(current) {
		t.Fatalf("unexpected parse version result: %q %q %v", original, state, ok)
	}
	original, state, ok = ParsePath(trash)
	if !ok || state != StateTrash || filepath.ToSlash(original) != filepath.ToSlash(current) {
		t.Fatalf("unexpected parse trash result: %q %q %v", original, state, ok)
	}
}

func TestSidecarPathsPreservePrimaryExtension(t *testing.T) {
	now := time.Date(2026, 3, 19, 14, 32, 10, 0, time.UTC)
	current := filepath.Join("content", "images", "hero.png.meta.yaml")

	version := BuildVersionPath(current, now)
	if filepath.ToSlash(version) != "content/images/hero.version.20260319T143210Z.png.meta.yaml" {
		t.Fatalf("unexpected sidecar version path: %s", version)
	}
	if !IsDerivedPath(version) {
		t.Fatal("expected derived sidecar path")
	}
	if err := ValidateCurrentPath(version); err == nil {
		t.Fatal("expected derived sidecar to be rejected as current path")
	}
}
