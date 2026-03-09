package version

import (
	"fmt"
	"runtime"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type command struct{}

func (command) Name() string {
	return "version"
}

func (command) Summary() string {
	return "Print Foundry version information"
}

func (command) Group() string {
	return "core commands"
}

func (command) Run(_ *config.Config, _ []string) error {
	fmt.Println(String())
	return nil
}

func String() string {
	return fmt.Sprintf(
		"Foundry %s\ncommit: %s\nbuilt: %s\ngo: %s",
		Version,
		Commit,
		Date,
		runtime.Version(),
	)
}

func init() {
	registry.Register(command{})
}
