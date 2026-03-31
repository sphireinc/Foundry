package standalone

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLaunchCommandRewritesServeStandaloneToServe(t *testing.T) {
	projectDir := t.TempDir()
	cmdDir := filepath.Join(projectDir, "cmd", "foundry")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("mkdir cmd dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	oldBuilder := buildStandaloneBinary
	t.Cleanup(func() { buildStandaloneBinary = oldBuilder })
	buildStandaloneBinary = func(projectDir, target string) error {
		return os.WriteFile(target, []byte("binary"), 0o755)
	}

	managedName := ManagedBin
	if runtime.GOOS == "windows" {
		managedName += ".exe"
	}
	paths, err := EnsureRunDir(projectDir)
	if err != nil {
		t.Fatalf("ensure run dir: %v", err)
	}
	managedPath := filepath.Join(paths.RunDir, managedName)

	got, err := LaunchCommand(projectDir, []string{"foundry", "serve-standalone", "--debug"})
	if err != nil {
		t.Fatalf("launch command: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("expected rewritten command, got %#v", got)
	}
	if got[0] == "go" || strings.Contains(strings.Join(got, " "), "go run") {
		t.Fatalf("expected managed binary command instead of go run, got %#v", got)
	}
	if filepath.Clean(got[0]) != filepath.Clean(managedPath) {
		t.Fatalf("expected managed binary path %q, got %q", managedPath, got[0])
	}
	if got[1] != "serve" {
		t.Fatalf("expected serve command, got %#v", got)
	}
}
