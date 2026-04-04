package releasecmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
)

var versionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

type command struct{}

func init() {
	registry.Register(command{})
}

func (command) Name() string         { return "release" }
func (command) Summary() string      { return "Cut a Foundry release tag" }
func (command) Group() string        { return "runtime" }
func (command) RequiresConfig() bool { return false }
func (command) Details() []string {
	return []string{
		"foundry release cut v1.3.3",
		"foundry release cut v1.3.3 --push",
	}
}

func (command) Run(_ *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry release cut <vX.Y.Z> [--push]")
	}
	switch strings.TrimSpace(args[2]) {
	case "cut":
		return runCut(args)
	default:
		return fmt.Errorf("unknown release subcommand: %s", args[2])
	}
}

func runCut(args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry release cut <vX.Y.Z> [--push]")
	}
	version := strings.TrimSpace(args[3])
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("release version must match vX.Y.Z, got %q", version)
	}
	push := false
	for _, arg := range args[4:] {
		switch strings.TrimSpace(arg) {
		case "--push":
			push = true
		case "":
		default:
			return fmt.Errorf("unknown release flag: %s", arg)
		}
	}
	if err := requireGit(); err != nil {
		return err
	}
	if err := requireRepoRoot(); err != nil {
		return err
	}
	if err := requireCleanWorktree(); err != nil {
		return err
	}
	if changed, err := updateEmbeddedReleaseVersion(version); err != nil {
		return err
	} else if changed {
		if err := runGit("add", "internal/commands/version/release_version.go"); err != nil {
			return err
		}
		if err := runGit("commit", "-m", "release: embed "+version); err != nil {
			return err
		}
		cliout.Successf("Committed embedded release version %s", version)
	}
	if err := ensureTagAbsent(version); err != nil {
		return err
	}
	message := "Foundry " + version
	if err := runGit("tag", "-a", version, "-m", message); err != nil {
		return err
	}
	cliout.Successf("Created release tag %s", version)
	fmt.Printf("%s %s\n", cliout.Label("Message:"), message)
	if push {
		if err := runGit("push", "origin", "HEAD"); err != nil {
			return err
		}
		if err := runGit("push", "origin", version); err != nil {
			return err
		}
		cliout.Successf("Pushed release tag %s", version)
		fmt.Println("GitHub Actions will now build and publish the release assets from the tagged commit.")
		return nil
	}
	fmt.Println("Next step:")
	fmt.Println("  git push origin HEAD")
	fmt.Printf("  git push origin %s\n", version)
	fmt.Println("Those pushes will trigger the GitHub Release workflow and upload the packaged artifacts.")
	return nil
}

func requireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required to cut a release")
	}
	return nil
}

func requireRepoRoot() error {
	out, err := output("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if filepathClean(out) != filepathClean(wd) {
		return fmt.Errorf("run release cut from the repository root: %s", out)
	}
	return nil
}

func requireCleanWorktree() error {
	out, err := output("git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("git worktree is not clean; commit or stash changes before cutting a release")
	}
	return nil
}

func ensureTagAbsent(version string) error {
	out, err := output("git", "tag", "--list", version)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("git tag %s already exists", version)
	}
	return nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func filepathClean(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
}

func updateEmbeddedReleaseVersion(version string) (bool, error) {
	const target = "internal/commands/version/release_version.go"
	body := "package version\n\n" +
		"// ReleaseVersion is the repo-carried release fallback used when a build does\n" +
		"// not have Git metadata available, such as container builds from a source\n" +
		"// archive or a Docker context without .git.\n" +
		fmt.Sprintf("const ReleaseVersion = %q\n", version)

	current, err := os.ReadFile(target)
	if err == nil && string(current) == body {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
