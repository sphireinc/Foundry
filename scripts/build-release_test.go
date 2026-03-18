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
