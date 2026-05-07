package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPlaywrightOutputsRemovesReportDirs(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"test-results", "playwright-report"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name, "artifact.txt"), []byte("artifact"), 0o644); err != nil {
			t.Fatalf("write %s artifact: %v", name, err)
		}
	}

	if err := cleanupPlaywrightOutputs(root); err != nil {
		t.Fatalf("cleanup playwright outputs: %v", err)
	}
	for _, name := range []string{"test-results", "playwright-report"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", name, err)
		}
	}
}

func TestCleanupPlaywrightOutputsHandlesEmptyRoot(t *testing.T) {
	if err := cleanupPlaywrightOutputs("   "); err != nil {
		t.Fatalf("cleanup should ignore empty root: %v", err)
	}
}
