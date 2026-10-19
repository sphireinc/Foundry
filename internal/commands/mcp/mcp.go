package mcpcmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	foundryversion "github.com/sphireinc/foundry/internal/commands/version"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/logx"
	"github.com/sphireinc/foundry/internal/mcp"
	"github.com/sphireinc/foundry/internal/site"
)

type command struct{}

func (command) Name() string {
	return "mcp"
}

func (command) Summary() string {
	return "Run the governed Model Context Protocol server"
}

func (command) Group() string {
	return "core commands"
}

func (command) Details() []string {
	return []string{
		"foundry mcp serve --transport stdio",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry mcp serve --transport stdio")
	}

	switch args[2] {
	case "serve":
		return runServe(cfg, args[3:])
	default:
		return fmt.Errorf("unknown mcp subcommand: %s", args[2])
	}
}

func runServe(cfg *config.Config, args []string) error {
	transport := "stdio"

	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "":
			continue
		case arg == "--transport":
			if i+1 >= len(args) {
				return fmt.Errorf("--transport requires a value")
			}
			// #nosec G602 -- the length check above guarantees i+1 is in range.
			transport = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--transport="):
			transport = strings.TrimSpace(strings.TrimPrefix(arg, "--transport="))
		default:
			return fmt.Errorf("unknown mcp serve flag: %s", arg)
		}
	}

	if transport != "stdio" {
		return fmt.Errorf("unsupported mcp transport %q (supported: stdio)", transport)
	}

	if !cfg.MCP.Enabled {
		return fmt.Errorf("mcp is disabled; set mcp.enabled to true in %s to enable it", consts.ConfigFilePath)
	}

	policy, err := mcp.PolicyFromConfig(cfg)
	if err != nil {
		return err
	}

	manager, err := site.NewPluginManager(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = manager.Close() }()

	env, err := mcp.NewProjectEnvironment(cfg, manager)
	if err != nil {
		return err
	}

	reg := mcp.NewRegistry()
	if err := mcp.RegisterCapabilities(reg, env); err != nil {
		return err
	}

	meta := foundryversion.Current("")
	server, err := mcp.NewServer(mcp.ServerOptions{
		Registry:   reg,
		Authorizer: mcp.NewAuthorizer(policy),
		Auditor:    mcp.NewAuditor(cfg, cfg.MCP.Audit),
		Actor:      mcp.LocalActor(),
		Info:       mcp.Implementation{Name: "foundry", Version: meta.DisplayVersion},
	})
	if err != nil {
		return err
	}

	logx.Info("foundry mcp server starting",
		"transport", transport,
		"protocol", mcp.ProtocolVersion,
		"scopes", scopesToStrings(policy.EnabledScopes()),
	)

	return server.ServeStdio(context.Background(), os.Stdin, os.Stdout)
}

func scopesToStrings(scopes []mcp.Scope) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, string(scope))
	}
	return out
}

func init() {
	registry.Register(command{})
}
