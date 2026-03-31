package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetVersionPrefersEnv(t *testing.T) {
	old := os.Getenv("VERSION")
	t.Cleanup(func() { _ = os.Setenv("VERSION", old) })
	if err := os.Setenv("VERSION", "1.2.3-test"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if got := getVersion(); got != "1.2.3-test" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestGitAndChecksumHelpers(t *testing.T) {
	if got := getGitCommit(); got == "" {
		t.Fatal("expected git commit or fallback value")
	}

	file := filepath.Join(t.TempDir(), "bin")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	sum, err := sha256File(file)
	if err != nil {
		t.Fatalf("sha256 file: %v", err)
	}
	if len(sum) != 64 {
		t.Fatalf("unexpected checksum length: %q", sum)
	}

	out, err := outputQuiet("pwd")
	if err != nil {
		t.Fatalf("outputQuiet pwd: %v", err)
	}
	if !strings.Contains(out, "/") {
		t.Fatalf("unexpected pwd output: %q", out)
	}
}

func TestReleaseArchiveHelpers(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "foundry")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	archivePath := filepath.Join(root, releaseArchiveName("linux", "amd64"))
	if err := createTarGz(archivePath, binaryPath, "foundry"); err != nil {
		t.Fatalf("create archive: %v", err)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if got := releaseArchiveName("darwin", "arm64"); got != "foundry-darwin-arm64.tar.gz" {
		t.Fatalf("unexpected release archive name: %q", got)
	}
}

func TestBuildTargetHelpers(t *testing.T) {
	oldGOOS := os.Getenv("TARGET_GOOS")
	oldGOARCH := os.Getenv("TARGET_GOARCH")
	t.Cleanup(func() {
		_ = os.Setenv("TARGET_GOOS", oldGOOS)
		_ = os.Setenv("TARGET_GOARCH", oldGOARCH)
	})
	if err := os.Setenv("TARGET_GOOS", "linux"); err != nil {
		t.Fatalf("set TARGET_GOOS: %v", err)
	}
	if err := os.Setenv("TARGET_GOARCH", "arm64"); err != nil {
		t.Fatalf("set TARGET_GOARCH: %v", err)
	}
	if got := buildTargetGOOS(); got != "linux" {
		t.Fatalf("unexpected target goos: %q", got)
	}
	if got := buildTargetGOARCH(); got != "arm64" {
		t.Fatalf("unexpected target goarch: %q", got)
	}
}
