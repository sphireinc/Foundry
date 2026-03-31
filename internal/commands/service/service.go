package servicecmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/hostservice"
)

type command struct{}

func init() {
	registry.Register(command{})
}

func (command) Name() string { return "service" }

func (command) Summary() string { return "Install and manage Foundry as an OS service" }

func (command) Group() string { return "runtime" }

func (command) Details() []string {
	return []string{
		"foundry service install",
		"foundry service start",
		"foundry service stop",
		"foundry service restart",
		"foundry service status",
		"foundry service uninstall",
	}
}

func (command) RequiresConfig() bool { return false }

func (command) Run(_ *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry service [install|start|stop|restart|status|uninstall]")
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}
	switch strings.TrimSpace(args[2]) {
	case "install":
		meta, err := hostservice.Install(projectDir)
		if err != nil {
			return err
		}
		cliout.Successf("Foundry service installed")
		fmt.Printf("%s %s\n", cliout.Label("Platform:"), meta.Platform)
		fmt.Printf("%s %s\n", cliout.Label("Name:"), meta.Name)
		fmt.Printf("%s %s\n", cliout.Label("Service file:"), meta.ServicePath)
		fmt.Printf("%s %s\n", cliout.Label("Log:"), meta.LogPath)
		if meta.Platform == "linux" {
			fmt.Println("Note: for restart-on-boot after logout, your Linux user may need lingering enabled via `loginctl enable-linger $USER`.")
		}
		return nil
	case "start":
		if err := hostservice.Start(projectDir); err != nil {
			return err
		}
		cliout.Successf("Foundry service started")
		return nil
	case "stop":
		if err := hostservice.Stop(projectDir); err != nil {
			return err
		}
		cliout.Successf("Foundry service stopped")
		return nil
	case "restart":
		if err := hostservice.Restart(projectDir); err != nil {
			return err
		}
		cliout.Successf("Foundry service restarted")
		return nil
	case "status":
		status, err := hostservice.CheckStatus(projectDir)
		if err != nil {
			return err
		}
		fmt.Println(status.Message)
		if status.Metadata != nil {
			fmt.Printf("%s %s\n", cliout.Label("Platform:"), status.Metadata.Platform)
			fmt.Printf("%s %s\n", cliout.Label("Name:"), status.Metadata.Name)
			fmt.Printf("%s %v\n", cliout.Label("Installed:"), status.Installed)
			fmt.Printf("%s %v\n", cliout.Label("Running:"), status.Running)
			fmt.Printf("%s %v\n", cliout.Label("Enabled:"), status.Enabled)
			fmt.Printf("%s %s\n", cliout.Label("Service file:"), status.Metadata.ServicePath)
			fmt.Printf("%s %s\n", cliout.Label("Log:"), status.Metadata.LogPath)
			if !status.Metadata.InstalledAt.IsZero() {
				fmt.Printf("%s %s\n", cliout.Label("Installed at:"), status.Metadata.InstalledAt.Local().Format(time.RFC3339))
			}
		}
		return nil
	case "uninstall":
		if err := hostservice.Uninstall(projectDir); err != nil {
			return err
		}
		cliout.Successf("Foundry service uninstalled")
		return nil
	default:
		return fmt.Errorf("unknown service subcommand: %s", args[2])
	}
}
