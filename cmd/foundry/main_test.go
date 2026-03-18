package main

import (
	"testing"

	_ "github.com/sphireinc/foundry/internal/commands/imports"
	"github.com/sphireinc/foundry/internal/config"
)

func TestHandleConfigFreeCLI(t *testing.T) {
	if !handleConfigFreeCLI([]string{"foundry", "version"}) {
		t.Fatal("expected version command to be handled without config")
	}
	if handleConfigFreeCLI([]string{"foundry", "theme", "list"}) {
		t.Fatal("expected config-bound command to be skipped")
	}
	if handleConfigFreeCLI([]string{"foundry", "unknown"}) {
		t.Fatal("expected unknown command to be skipped")
	}
}

func TestHandleConfigBoundCLINoCommand(t *testing.T) {
	handleConfigBoundCLI(&config.Config{}, []string{"foundry", "unknown"})
}

func TestParseServeDebugFlag(t *testing.T) {
	debug, err := parseServeDebugFlag([]string{"--debug"})
	if err != nil || !debug {
		t.Fatalf("expected serve debug flag to be parsed, got debug=%v err=%v", debug, err)
	}

	debug, err = parseServeDebugFlag(nil)
	if err != nil || debug {
		t.Fatalf("expected empty serve args to be accepted, got debug=%v err=%v", debug, err)
	}

	if _, err := parseServeDebugFlag([]string{"--nope"}); err == nil {
		t.Fatal("expected unknown serve flag to fail")
	}
}
