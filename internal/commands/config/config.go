package configcmd

import (
	"fmt"

	"github.com/sphireinc/foundry/internal/commands/registry"
	foundryconfig "github.com/sphireinc/foundry/internal/config"
)

type command struct{}

func (command) Name() string {
	return "config"
}

func (command) Summary() string {
	return "Validate config, plugins, content, and routes"
}

func (command) Group() string {
	return "core commands"
}

func (command) Run(cfg *foundryconfig.Config, args []string) error {
	if len(args) < 3 || args[2] != "check" {
		return fmt.Errorf("usage: foundry config check")
	}

	errs := foundryconfig.Validate(cfg)
	if len(errs) == 0 {
		fmt.Println("config OK")
		return nil
	}

	fmt.Println("config check failed:")
	for _, err := range errs {
		fmt.Printf("- %v\n", err)
	}

	return fmt.Errorf("config validation failed with %d error(s)", len(errs))
}

func init() {
	registry.Register(command{})
}
