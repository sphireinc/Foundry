package themecmd

import (
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/theme"
)

type command struct{}

func (command) Name() string {
	return "theme"
}

func (command) Summary() string {
	return "Manage themes"
}

func (command) Group() string {
	return "theme commands"
}

func (command) Details() []string {
	return []string{
		"foundry theme list",
		"foundry theme current",
		"foundry theme validate <name>",
		"foundry theme scaffold <name>",
		"foundry theme switch <name>",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry theme [list|current|validate|scaffold|switch]")
	}

	switch args[2] {
	case "list":
		return runList(cfg)
	case "current":
		return runCurrent(cfg)
	case "validate":
		return runValidate(cfg, args)
	case "scaffold":
		return runScaffold(cfg, args)
	case "switch":
		return runSwitch(cfg, args)
	default:
		return fmt.Errorf("unknown theme subcommand: %s", args[2])
	}
}

func runList(cfg *config.Config) error {
	themes, err := theme.ListInstalled(cfg.ThemesDir)
	if err != nil {
		return err
	}

	if len(themes) == 0 {
		cliout.Println(cliout.Warning("no themes installed"))
		return nil
	}

	nameWidth := len("NAME")
	versionWidth := len("VERSION")

	type row struct {
		Name    string
		Version string
		Title   string
		Status  string
	}

	rows := make([]row, 0, len(themes))
	for _, t := range themes {
		manifest, err := theme.LoadManifest(cfg.ThemesDir, t.Name)
		title := t.Name
		version := "-"
		if err == nil {
			title = manifest.Title
			version = manifest.Version
		}

		status := ""
		if t.Name == cfg.Theme {
			status = "current"
		}

		rows = append(rows, row{
			Name:    t.Name,
			Version: version,
			Title:   title,
			Status:  status,
		})

		if len(t.Name) > nameWidth {
			nameWidth = len(t.Name)
		}
		if len(version) > versionWidth {
			versionWidth = len(version)
		}
	}

	fmt.Printf("%-*s  %-*s  %-20s  %s\n", nameWidth, cliout.Label("NAME"), versionWidth, cliout.Label("VERSION"), cliout.Label("TITLE"), cliout.Label("STATUS"))
	for _, row := range rows {
		fmt.Printf("%-*s  %-*s  %-20s  %s\n", nameWidth, row.Name, versionWidth, row.Version, row.Title, row.Status)
	}

	return nil
}

func runCurrent(cfg *config.Config) error {
	fmt.Println(cfg.Theme)
	return nil
}

func runValidate(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry theme validate <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := theme.ValidateInstalled(cfg.ThemesDir, name); err != nil {
		return err
	}

	manifest, err := theme.LoadManifest(cfg.ThemesDir, name)
	if err != nil {
		return err
	}

	cliout.Successf("Theme %q is valid", name)
	fmt.Printf("%s %s\n", cliout.Label("Title:"), manifest.Title)
	fmt.Printf("%s %s\n", cliout.Label("Version:"), manifest.Version)
	fmt.Printf("%s %s\n", cliout.Label("Min Foundry Version:"), manifest.MinFoundryVersion)
	return nil
}

func runScaffold(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry theme scaffold <name>")
	}

	name := strings.TrimSpace(args[3])
	path, err := theme.Scaffold(cfg.ThemesDir, name)
	if err != nil {
		return err
	}

	cliout.Successf("Scaffolded theme %q at %s", name, path)
	return nil
}

func runSwitch(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry theme switch <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := theme.ValidateInstalled(cfg.ThemesDir, name); err != nil {
		return err
	}

	if err := theme.SwitchInConfig(consts.ConfigFilePath, name); err != nil {
		return err
	}

	cliout.Successf("Switched theme to %q", name)
	cliout.Println(cliout.Heading("Next steps:"))
	fmt.Println("1. Run foundry build or foundry serve")
	return nil
}

func init() {
	registry.Register(command{})
}
