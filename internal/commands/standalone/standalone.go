package standalonecmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/standalone"
)

type command struct {
	name    string
	summary string
	details []string
	run     func(*config.Config, []string) error
}

func (c command) Name() string         { return c.name }
func (c command) Summary() string      { return c.summary }
func (c command) Group() string        { return "runtime" }
func (c command) Details() []string    { return c.details }
func (c command) RequiresConfig() bool { return false }
func (c command) Run(cfg *config.Config, args []string) error {
	return c.run(cfg, args)
}

func currentProjectDir() (string, error) {
	return os.Getwd()
}

func runServeStandalone(_ *config.Config, _ []string) error {
	projectDir, err := currentProjectDir()
	if err != nil {
		return err
	}
	state, err := standalone.Start(projectDir, os.Args)
	if err != nil {
		return err
	}
	cliout.Successf("Foundry standalone server started")
	fmt.Printf("%s %d\n", cliout.Label("PID:"), state.PID)
	fmt.Printf("%s %s\n", cliout.Label("Log:"), state.LogPath)
	fmt.Printf("%s %s\n", cliout.Label("Project:"), state.ProjectDir)
	return nil
}

func runStop(_ *config.Config, _ []string) error {
	projectDir, err := currentProjectDir()
	if err != nil {
		return err
	}
	if err := standalone.Stop(projectDir); err != nil {
		return err
	}
	cliout.Successf("Foundry standalone server stopped")
	return nil
}

func runStatus(_ *config.Config, _ []string) error {
	projectDir, err := currentProjectDir()
	if err != nil {
		return err
	}
	state, running, err := standalone.RunningState(projectDir)
	if err != nil {
		return err
	}
	if state == nil {
		fmt.Println("not running")
		return nil
	}
	if !running {
		fmt.Println("not running (stale state found)")
		fmt.Printf("%s %d\n", cliout.Label("PID:"), state.PID)
		fmt.Printf("%s %s\n", cliout.Label("Log:"), state.LogPath)
		return nil
	}
	fmt.Println("running")
	fmt.Printf("%s %d\n", cliout.Label("PID:"), state.PID)
	fmt.Printf("%s %s\n", cliout.Label("Started:"), state.StartedAt.Local().Format(time.RFC3339))
	fmt.Printf("%s %s\n", cliout.Label("Log:"), state.LogPath)
	return nil
}

func runRestart(_ *config.Config, _ []string) error {
	projectDir, err := currentProjectDir()
	if err != nil {
		return err
	}
	state, err := standalone.Restart(projectDir, os.Args)
	if err != nil {
		return err
	}
	cliout.Successf("Foundry standalone server restarted")
	fmt.Printf("%s %d\n", cliout.Label("PID:"), state.PID)
	fmt.Printf("%s %s\n", cliout.Label("Log:"), state.LogPath)
	return nil
}

func runLogs(_ *config.Config, args []string) error {
	projectDir, err := currentProjectDir()
	if err != nil {
		return err
	}
	paths := standalone.ProjectPaths(projectDir)
	lines := 120
	follow := false
	for _, arg := range args[2:] {
		switch {
		case strings.TrimSpace(arg) == "-f" || strings.TrimSpace(arg) == "--follow":
			follow = true
		case strings.HasPrefix(strings.TrimSpace(arg), "--lines="):
			value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(arg), "--lines="))
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid --lines value %q", value)
			}
			lines = n
		default:
			return fmt.Errorf("usage: foundry logs [--lines=N] [-f|--follow]")
		}
	}
	body, err := standalone.ReadLastLines(paths.LogPath, lines)
	if err != nil {
		return err
	}
	if body != "" {
		fmt.Println(body)
	}
	if follow {
		return standalone.FollowLog(paths.LogPath, os.Stdout)
	}
	return nil
}

func init() {
	registry.Register(command{
		name:    "serve-standalone",
		summary: "Run Foundry in the background with PID/log management",
		details: []string{"foundry serve-standalone", "foundry serve-standalone --debug"},
		run:     runServeStandalone,
	})
	registry.Register(command{
		name:    "stop",
		summary: "Stop the standalone Foundry server",
		details: []string{"foundry stop"},
		run:     runStop,
	})
	registry.Register(command{
		name:    "status",
		summary: "Show standalone Foundry server status",
		details: []string{"foundry status"},
		run:     runStatus,
	})
	registry.Register(command{
		name:    "restart",
		summary: "Restart the standalone Foundry server",
		details: []string{"foundry restart"},
		run:     runRestart,
	})
	registry.Register(command{
		name:    "logs",
		summary: "Show standalone Foundry server logs",
		details: []string{"foundry logs", "foundry logs --lines=200", "foundry logs -f"},
		run:     runLogs,
	})
}
