package backupcmd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/backup"
	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
)

type command struct{}

func init() {
	registry.Register(command{})
}

func (command) Name() string { return "backup" }

func (command) Summary() string { return "Create and inspect content backups" }

func (command) Group() string { return "backup commands" }

func (command) Details() []string {
	return []string{
		"foundry backup create [target.zip]",
		"foundry backup list",
		"foundry backup git-snapshot [message] [--push]",
		"foundry backup git-log [limit]",
	}
}

func (command) RequiresConfig() bool { return true }

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry backup [create|list|git-snapshot|git-log]")
	}
	switch strings.TrimSpace(args[2]) {
	case "create":
		return runCreate(cfg, args)
	case "list":
		return runList(cfg)
	case "git-snapshot":
		return runGitSnapshot(cfg, args)
	case "git-log":
		return runGitLog(cfg, args)
	default:
		return fmt.Errorf("unknown backup subcommand: %s", args[2])
	}
}

func runCreate(cfg *config.Config, args []string) error {
	var (
		snapshot *backup.Snapshot
		err      error
	)
	if len(args) >= 4 && strings.TrimSpace(args[3]) != "" {
		target := strings.TrimSpace(args[3])
		if filepath.Ext(target) == "" {
			target += ".zip"
		}
		snapshot, err = backup.CreateZipSnapshot(cfg, target)
	} else {
		snapshot, err = backup.CreateManagedSnapshot(cfg)
	}
	if err != nil {
		return err
	}
	cliout.Successf("Backup created")
	fmt.Printf("%s %s\n", cliout.Label("Path:"), snapshot.Path)
	fmt.Printf("%s %d bytes\n", cliout.Label("Archive:"), snapshot.SizeBytes)
	fmt.Printf("%s %d bytes\n", cliout.Label("Source:"), snapshot.SourceBytes)
	fmt.Printf("%s %s\n", cliout.Label("Created:"), snapshot.CreatedAt.Format(time.RFC3339))
	return nil
}

func runList(cfg *config.Config) error {
	items, err := backup.List(cfg.Backup.Dir)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No backups found.")
		return nil
	}
	for _, item := range items {
		fmt.Printf("- %s (%d bytes, %s)\n", item.Path, item.SizeBytes, item.CreatedAt.Format(time.RFC3339))
	}
	return nil
}

func runGitSnapshot(cfg *config.Config, args []string) error {
	message := ""
	push := false
	for _, arg := range args[3:] {
		switch strings.TrimSpace(arg) {
		case "":
		case "--push":
			push = true
		default:
			if message == "" {
				message = strings.TrimSpace(arg)
				continue
			}
			return fmt.Errorf("usage: foundry backup git-snapshot [message] [--push]")
		}
	}
	snapshot, err := backup.CreateGitSnapshot(cfg, message, push)
	if err != nil {
		return err
	}
	if !snapshot.Changed {
		cliout.Successf("No content changes to snapshot")
	} else {
		cliout.Successf("Git snapshot created")
	}
	fmt.Printf("%s %s\n", cliout.Label("Repo:"), snapshot.RepoDir)
	fmt.Printf("%s %s\n", cliout.Label("Revision:"), snapshot.Revision)
	if snapshot.RemoteURL != "" {
		fmt.Printf("%s %s\n", cliout.Label("Remote:"), snapshot.RemoteURL)
		fmt.Printf("%s %s\n", cliout.Label("Branch:"), snapshot.Branch)
		fmt.Printf("%s %t\n", cliout.Label("Pushed:"), snapshot.Pushed)
	}
	fmt.Printf("%s %s\n", cliout.Label("Created:"), snapshot.CreatedAt.Format(time.RFC3339))
	return nil
}

func runGitLog(cfg *config.Config, args []string) error {
	limit := 10
	if len(args) >= 4 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(args[3])); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := backup.ListGitSnapshots(cfg, limit)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No git snapshots found.")
		return nil
	}
	for _, item := range items {
		fmt.Printf("- %s %s %s\n", item.Revision, item.CreatedAt.Format(time.RFC3339), item.Message)
	}
	return nil
}
