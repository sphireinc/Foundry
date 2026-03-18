package configcmd

import (
	"testing"

	foundryconfig "github.com/sphireinc/foundry/internal/config"
)

func TestCommandMetadataAndRun(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "config" || !cmd.RequiresConfig() || len(cmd.Details()) == 0 {
		t.Fatalf("unexpected command metadata")
	}

	cfg := &foundryconfig.Config{}
	cfg.ApplyDefaults()
	if err := cmd.Run(cfg, []string{"foundry", "config", "check"}); err != nil {
		t.Fatalf("expected config check success, got %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "config"}); err == nil {
		t.Fatal("expected usage error")
	}

	bad := &foundryconfig.Config{}
	if err := cmd.Run(bad, []string{"foundry", "config", "check"}); err == nil {
		t.Fatal("expected config validation error")
	}
}
