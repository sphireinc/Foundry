package mcpcmd

import (
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestRunRejectsWhenDisabled(t *testing.T) {
	err := command{}.Run(&config.Config{}, []string{"foundry", "mcp", "serve", "--transport", "stdio"})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled error, got %v", err)
	}
}

func TestRunRejectsUnsupportedTransport(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.Enabled = true
	err := command{}.Run(cfg, []string{"foundry", "mcp", "serve", "--transport", "http"})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported transport error, got %v", err)
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := command{}.Run(&config.Config{}, []string{"foundry", "mcp", "bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown mcp subcommand") {
		t.Fatalf("expected unknown subcommand error, got %v", err)
	}
}

func TestRunRequiresSubcommand(t *testing.T) {
	if err := (command{}).Run(&config.Config{}, []string{"foundry", "mcp"}); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestCommandMetadata(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "mcp" {
		t.Fatalf("expected mcp command name, got %q", cmd.Name())
	}
	if !cmd.RequiresConfig() {
		t.Fatal("expected mcp command to require config")
	}
}
