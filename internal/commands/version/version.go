package version

import (
	"fmt"
	"os"
	"strings"

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

func (command) Details() []string {
	return []string{
		"foundry version",
		"foundry version --short",
		"foundry version --json",
	}
}

func (command) RequiresConfig() bool {
	return false
}

func (command) Run(_ *config.Config, args []string) error {
	projectDir, _ := os.Getwd()
	meta := Current(projectDir)
	switch outputMode(versionArgs(args)) {
	case "short":
		fmt.Println(meta.ShortString())
	case "json":
		fmt.Println(meta.JSON())
	default:
		fmt.Println(meta.String())
	}
	return nil
}

func String() string {
	projectDir, _ := os.Getwd()
	return Current(projectDir).String()
}

func outputMode(args []string) string {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--short":
			return "short"
		case "--json":
			return "json"
		}
	}
	return "default"
}

func versionArgs(args []string) []string {
	if len(args) <= 2 {
		return nil
	}
	return args[2:]
}

func init() {
	registry.Register(command{})
}
