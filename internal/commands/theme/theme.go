package themecmd

import (
	"fmt"
	"strings"

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
		fmt.Println("no themes installed")
		return nil
	}

	nameWidth := len("NAME")
	for _, t := range themes {
		if len(t.Name) > nameWidth {
			nameWidth = len(t.Name)
		}
	}

	fmt.Printf("%-*s  %s\n", nameWidth, "NAME", "STATUS")
	for _, t := range themes {
		status := ""
		if t.Name == cfg.Theme {
			status = "current"
		}
		fmt.Printf("%-*s  %s\n", nameWidth, t.Name, status)
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

	fmt.Printf("Theme %q is valid\n", name)
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

	fmt.Printf("Scaffolded theme %q at %s\n", name, path)
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

	fmt.Printf("Switched theme to %q\n", name)
	fmt.Println("Next steps:")
	fmt.Println("1. Run foundry build or foundry serve")
	return nil
}

func init() {
	registry.Register(command{})
}
