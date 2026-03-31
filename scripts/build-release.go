package main

import (
	"archive/tar"
	"compress/gzip"
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
	targetGOOS := buildTargetGOOS()
	targetGOARCH := buildTargetGOARCH()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fail("create output dir", err)
	}

	outputName := appName
	if targetGOOS == "windows" {
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
	fmt.Printf("  target:  %s/%s\n", targetGOOS, targetGOARCH)
	fmt.Printf("  output:  %s\n", outputPath)

	// Keep generated plugin imports up to date before building.
	run("go", "run", "./cmd/plugin-sync")

	// Build the main Foundry CLI binary.
	runBuild(targetGOOS, targetGOARCH, "go", "build", "-ldflags", ldflags, "-o", outputPath, entrypoint)

	sum, err := sha256File(outputPath)
	if err != nil {
		fail("compute checksum", err)
	}

	checksumPath := outputPath + ".sha256"
	checksumBody := fmt.Sprintf("%s  %s\n", sum, filepath.Base(outputPath))
	if err := os.WriteFile(checksumPath, []byte(checksumBody), 0o644); err != nil {
		fail("write checksum file", err)
	}

	archiveName := releaseArchiveName(targetGOOS, targetGOARCH)
	archivePath := filepath.Join(outputDir, archiveName)
	if err := createTarGz(archivePath, outputPath, filepath.Base(outputPath)); err != nil {
		fail("create release archive", err)
	}
	archiveSum, err := sha256File(archivePath)
	if err != nil {
		fail("compute archive checksum", err)
	}
	archiveChecksumPath := archivePath + ".sha256"
	archiveChecksumBody := fmt.Sprintf("%s  %s\n", archiveSum, filepath.Base(archivePath))
	if err := os.WriteFile(archiveChecksumPath, []byte(archiveChecksumBody), 0o644); err != nil {
		fail("write archive checksum file", err)
	}

	fmt.Println("Build complete.")
	fmt.Printf("Checksum: %s\n", sum)
	fmt.Printf("Checksum file: %s\n", checksumPath)
	fmt.Printf("Archive: %s\n", archivePath)
	fmt.Printf("Archive checksum: %s\n", archiveChecksumPath)
	fmt.Println("")
	fmt.Printf("Run with: %s version\n", outputPath)
}

func buildTargetGOOS() string {
	if value := strings.TrimSpace(os.Getenv("TARGET_GOOS")); value != "" {
		return value
	}
	return runtime.GOOS
}

func buildTargetGOARCH() string {
	if value := strings.TrimSpace(os.Getenv("TARGET_GOARCH")); value != "" {
		return value
	}
	return runtime.GOARCH
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

func releaseArchiveName(goos, goarch string) string {
	return fmt.Sprintf("foundry-%s-%s.tar.gz", goos, goarch)
}

func createTarGz(targetPath, sourcePath, archiveName string) error {
	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	gzw := gzip.NewWriter(target)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = archiveName
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(tw, file)
	return err
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

func runBuild(goos, goarch, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+goos,
		"GOARCH="+goarch,
	)
	if err := cmd.Run(); err != nil {
		fail(fmt.Sprintf("%s %s", name, strings.Join(args, " ")), err)
	}
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "build-release: %s: %v\n", step, err)
	os.Exit(1)
}
