package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/config"
)

// Command is the common interface implemented by all Foundry CLI commands.
type Command interface {
	Name() string
	Summary() string
	Group() string
	Details() []string
	RequiresConfig() bool
	Run(cfg *config.Config, args []string) error
}

// Info is the serializable command metadata used to build usage output.
type Info struct {
	Name           string
	Summary        string
	Group          string
	Details        []string
	RequiresConfig bool
}

var commands = map[string]Command{}

// Register adds a command to the global CLI registry.
//
// Registration is expected to happen from package init functions and panics on
// programmer errors such as duplicate names.
func Register(cmd Command) {
	if cmd == nil || cmd.Name() == "" {
		panic("commands: invalid command registration")
	}
	if _, exists := commands[cmd.Name()]; exists {
		panic("commands: duplicate command registration: " + cmd.Name())
	}
	commands[cmd.Name()] = cmd
}

// Lookup resolves a command from os.Args-style input.
func Lookup(args []string) (Command, bool) {
	if len(args) < 2 {
		return nil, false
	}

	cmd, ok := commands[args[1]]
	return cmd, ok
}

// Run looks up and executes a registered command.
//
// The boolean result reports whether a matching command was found.
func Run(cfg *config.Config, args []string) (bool, error) {
	cmd, ok := Lookup(args)
	if !ok {
		return false, nil
	}

	return true, cmd.Run(cfg, args)
}

// List returns all registered commands sorted by group and name.
func List() []Info {
	out := make([]Info, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, Info{
			Name:           cmd.Name(),
			Summary:        cmd.Summary(),
			Group:          cmd.Group(),
			Details:        cmd.Details(),
			RequiresConfig: cmd.RequiresConfig(),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Name < out[j].Name
	})

	return out
}

// Usage renders grouped CLI usage text for all registered commands.
func Usage() string {
	items := List()
	if len(items) == 0 {
		return "usage: foundry <command>"
	}

	grouped := make(map[string][]Info)
	groups := make([]string, 0)

	for _, item := range items {
		group := item.Group
		if group == "" {
			group = "commands"
		}
		if _, ok := grouped[group]; !ok {
			groups = append(groups, group)
		}
		grouped[group] = append(grouped[group], item)
	}

	sort.Strings(groups)

	nameWidth := len("COMMAND")
	for _, item := range items {
		if len(item.Name) > nameWidth {
			nameWidth = len(item.Name)
		}
	}

	var sb strings.Builder
	sb.WriteString(cliout.Heading("usage:"))
	sb.WriteString(" foundry <command>\n")

	for _, group := range groups {
		sb.WriteString("\n")
		sb.WriteString(cliout.Heading(group))
		sb.WriteString(":\n")

		for _, item := range grouped[group] {
			sb.WriteString(fmt.Sprintf("  %-*s  %s\n", nameWidth, cliout.Label(item.Name), item.Summary))
			for _, detail := range item.Details {
				sb.WriteString(fmt.Sprintf("  %-*s  %s\n", nameWidth, "", detail))
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
