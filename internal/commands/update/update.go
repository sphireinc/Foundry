package updatecmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/updater"
)

type command struct{}

func init() {
	registry.Register(command{})
}

func (command) Name() string         { return "update" }
func (command) Summary() string      { return "Check and apply Foundry releases" }
func (command) Group() string        { return "runtime" }
func (command) RequiresConfig() bool { return false }
func (command) Details() []string {
	return []string{
		"foundry update check",
		"foundry update apply",
	}
}

func (command) Run(_ *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry update [check|apply|__helper]")
	}
	projectDir, _ := os.Getwd()
	switch strings.TrimSpace(args[2]) {
	case "check":
		return runCheck(projectDir)
	case "apply":
		return runApply(projectDir)
	case "__helper":
		return runHelper(args)
	default:
		return fmt.Errorf("unknown update subcommand: %s", args[2])
	}
}

func runCheck(projectDir string) error {
	info, err := updater.Check(context.Background(), projectDir)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", cliout.Label("Current:"), info.CurrentVersion)
	fmt.Printf("%s %s\n", cliout.Label("Latest:"), info.LatestVersion)
	fmt.Printf("%s %s\n", cliout.Label("Install mode:"), info.InstallMode)
	fmt.Printf("%s %t\n", cliout.Label("Update available:"), info.HasUpdate)
	fmt.Printf("%s %t\n", cliout.Label("Apply supported:"), info.ApplySupported)
	if info.ReleaseURL != "" {
		fmt.Printf("%s %s\n", cliout.Label("Release:"), info.ReleaseURL)
	}
	if info.Instructions != "" {
		fmt.Printf("%s %s\n", cliout.Label("Notes:"), info.Instructions)
	}
	return nil
}

func runApply(projectDir string) error {
	info, err := updater.ScheduleApply(context.Background(), projectDir)
	if err != nil {
		return err
	}
	cliout.Successf("Update scheduled")
	fmt.Printf("%s %s\n", cliout.Label("Current:"), info.CurrentVersion)
	fmt.Printf("%s %s\n", cliout.Label("Latest:"), info.LatestVersion)
	fmt.Printf("%s %s\n", cliout.Label("Asset:"), info.AssetName)
	fmt.Println("Foundry will restart after the release binary is replaced.")
	return nil
}

func runHelper(args []string) error {
	var (
		projectDir string
		target     string
		source     string
		pid        int
	)
	for _, arg := range args[3:] {
		switch {
		case strings.HasPrefix(arg, "--project-dir="):
			projectDir = strings.TrimPrefix(arg, "--project-dir=")
		case strings.HasPrefix(arg, "--target="):
			target = strings.TrimPrefix(arg, "--target=")
		case strings.HasPrefix(arg, "--source="):
			source = strings.TrimPrefix(arg, "--source=")
		case strings.HasPrefix(arg, "--pid="):
			parsed, _ := strconv.Atoi(strings.TrimPrefix(arg, "--pid="))
			pid = parsed
		}
	}
	if projectDir == "" || target == "" || source == "" {
		return fmt.Errorf("update helper requires --project-dir, --target, and --source")
	}
	time.Sleep(300 * time.Millisecond)
	return updater.RunHelper(projectDir, target, source, pid)
}
