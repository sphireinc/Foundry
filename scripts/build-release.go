package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	appName    = "foundry"
	entrypoint = "./cmd/foundry"
	outputDir  = "bin"
)

func main() {
	version := getVersion()
	commit := getGitCommit()
	date := time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fail("create output dir", err)
	}

	outputName := appName
	if runtime.GOOS == "windows" {
		outputName += ".exe"
	}
	outputPath := filepath.Join(outputDir, outputName)

	ldflags := strings.Join([]string{
		"-X github.com/sphireinc/foundry/internal/commands/version.Version=" + version,
		"-X github.com/sphireinc/foundry/internal/commands/version.Commit=" + commit,
		"-X github.com/sphireinc/foundry/internal/commands/version.Date=" + date,
	}, " ")

	fmt.Printf("Building %s\n", appName)
	fmt.Printf("  version: %s\n", version)
	fmt.Printf("  commit:  %s\n", commit)
	fmt.Printf("  built:   %s\n", date)
	fmt.Printf("  output:  %s\n", outputPath)

	// Keep generated plugin imports up to date before building.
	run("go", "run", "./cmd/plugin-sync")

	// Build the main Foundry CLI binary.
	run("go", "build", "-ldflags", ldflags, "-o", outputPath, entrypoint)

	sum, err := sha256File(outputPath)
	if err != nil {
		fail("compute checksum", err)
	}

	checksumPath := outputPath + ".sha256"
	checksumBody := fmt.Sprintf("%s  %s\n", sum, filepath.Base(outputPath))
	if err := os.WriteFile(checksumPath, []byte(checksumBody), 0o644); err != nil {
		fail("write checksum file", err)
	}

	fmt.Println("Build complete.")
	fmt.Printf("Checksum: %s\n", sum)
	fmt.Printf("Checksum file: %s\n", checksumPath)
	fmt.Println("")
	fmt.Printf("Run with: %s version\n", outputPath)
}

func getVersion() string {
	// Priority:
	// 1. VERSION env var
	// 2. git tag if available
	// 3. dev
	if v := strings.TrimSpace(os.Getenv("VERSION")); v != "" {
		return v
	}

	if tag, err := outputQuiet("git", "describe", "--tags", "--abbrev=0"); err == nil && tag != "" {
		return tag
	}

	return "dev"
}

func getGitCommit() string {
	if commit, err := outputQuiet("git", "rev-parse", "--short", "HEAD"); err == nil && commit != "" {
		return commit
	}
	return "none"
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

//func output(name string, args ...string) (string, error) {
//	cmd := exec.Command(name, args...)
//	cmd.Stderr = os.Stderr
//	b, err := cmd.Output()
//	if err != nil {
//		return "", err
//	}
//	return strings.TrimSpace(string(b)), nil
//}

func outputQuiet(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fail(fmt.Sprintf("%s %s", name, strings.Join(args, " ")), err)
	}
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "build-release: %s: %v\n", step, err)
	os.Exit(1)
}
