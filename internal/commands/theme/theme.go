package themecmd

import (
	"fmt"
	"strings"

	adminui "github.com/sphireinc/foundry/internal/admin/ui"
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
		"foundry theme install <git-url|owner/repo> [name] [--admin]",
		"foundry theme scaffold <name>",
		"foundry theme switch <name>",
		"foundry theme switch --admin <name>",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry theme [list|current|validate|install|scaffold|switch]")
	}

	switch args[2] {
	case "list":
		return runList(cfg)
	case "current":
		return runCurrent(cfg)
	case "validate":
		return runValidate(cfg, args)
	case "install":
		return runInstall(cfg, args)
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

func runInstall(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry theme install <git-url|owner/repo> [name] [--admin]")
	}

	kind := theme.InstallKindFrontend
	values := make([]string, 0, len(args)-3)
	for _, arg := range args[3:] {
		if strings.TrimSpace(arg) == "--admin" {
			kind = theme.InstallKindAdmin
			continue
		}
		values = append(values, arg)
	}
	if len(values) == 0 {
		return fmt.Errorf("usage: foundry theme install <git-url|owner/repo> [name] [--admin]")
	}

	repoURL := strings.TrimSpace(values[0])
	name := ""
	if len(values) >= 2 {
		name = strings.TrimSpace(values[1])
	}

	meta, err := theme.Install(theme.InstallOptions{
		ThemesDir: cfg.ThemesDir,
		URL:       repoURL,
		Name:      name,
		Kind:      kind,
	})
	if err != nil {
		return err
	}

	switch m := meta.(type) {
	case *theme.Manifest:
		cliout.Successf("Installed frontend theme: %s", m.Name)
		fmt.Printf("%s %s\n", cliout.Label("Directory:"), filepathJoin(cfg.ThemesDir, m.Name))
		fmt.Printf("%s %s\n", cliout.Label("Version:"), m.Version)
		fmt.Println("")
		cliout.Println(cliout.Heading("Next steps:"))
		fmt.Printf("1. Run foundry theme validate %q\n", m.Name)
		fmt.Printf("2. Run foundry theme switch %q\n", m.Name)
		fmt.Println("3. Run foundry build or foundry serve")
	case *adminui.Manifest:
		cliout.Successf("Installed admin theme: %s", m.Name)
		fmt.Printf("%s %s\n", cliout.Label("Directory:"), filepathJoin(cfg.ThemesDir, "admin-themes", m.Name))
		fmt.Printf("%s %s\n", cliout.Label("Version:"), m.Version)
		fmt.Println("")
		cliout.Println(cliout.Heading("Next steps:"))
		fmt.Printf("1. Validate the theme from the admin UI or with internal tooling\n")
		fmt.Printf("2. Set admin.theme to %q in %s\n", m.Name, consts.ConfigFilePath)
		fmt.Println("3. Run foundry build or foundry serve")
	default:
		return fmt.Errorf("unexpected installed theme metadata type")
	}
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
		return fmt.Errorf("usage: foundry theme switch [--admin] <name>")
	}

	adminKind := false
	name := ""
	for _, arg := range args[3:] {
		if strings.TrimSpace(arg) == "--admin" {
			adminKind = true
			continue
		}
		if name == "" {
			name = strings.TrimSpace(arg)
		}
	}
	if name == "" {
		return fmt.Errorf("usage: foundry theme switch [--admin] <name>")
	}

	if adminKind {
		validation, err := adminui.ValidateTheme(cfg.ThemesDir, name)
		if err != nil {
			return err
		}
		if !validation.Valid {
			return fmt.Errorf("admin theme %q is invalid", name)
		}
		if err := config.UpsertNestedScalar(consts.ConfigFilePath, []string{"admin", "theme"}, name); err != nil {
			return err
		}
		cliout.Successf("Switched admin theme to %q", name)
		cliout.Println(cliout.Heading("Next steps:"))
		fmt.Println("1. Run foundry build or foundry serve")
		return nil
	}

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

func filepathJoin(parts ...string) string {
	return strings.ReplaceAll(strings.Join(parts, "/"), "//", "/")
}

func init() {
	registry.Register(command{})
}
