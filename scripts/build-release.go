package main

import (
	"fmt"
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

	outputPath := filepath.Join(outputDir, appName)
	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}

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

	fmt.Println("Build complete.")
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

	if tag, err := output("git", "describe", "--tags", "--abbrev=0"); err == nil && tag != "" {
		return tag
	}

	return "dev"
}

func getGitCommit() string {
	if commit, err := output("git", "rev-parse", "--short", "HEAD"); err == nil && commit != "" {
		return commit
	}
	return "none"
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
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
