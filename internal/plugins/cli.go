package plugins

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// CommandContext is the execution context passed to plugin CLI commands.
//
// Args contains only the arguments that follow the plugin command name.
type CommandContext struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
}

// CLIHook lets a plugin contribute subcommands to the Foundry CLI.
//
// Commands are discovered during command dispatch and names must be unique
// across all enabled plugins.
type CLIHook interface {
	Commands() []Command
}

// Command describes a single plugin-owned CLI command.
type Command struct {
	Name        string
	Summary     string
	Description string
	Run         func(ctx CommandContext) error
}

// Commands returns all valid CLI commands exposed by a enabled plugins, sorted by
// command name.
func (m *Manager) Commands() []Command {
	commands := make([]Command, 0)

	for _, p := range m.plugins {
		hook, ok := p.(CLIHook)
		if !ok {
			continue
		}

		for _, cmd := range hook.Commands() {
			if strings.TrimSpace(cmd.Name) == "" || cmd.Run == nil {
				continue
			}
			commands = append(commands, cmd)
		}
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	return commands
}

// RunCommand executes a plugin CLI command by name.
func (m *Manager) RunCommand(name string, ctx CommandContext) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("plugin command name cannot be empty")
	}

	for _, cmd := range m.Commands() {
		if cmd.Name == name {
			return cmd.Run(ctx)
		}
	}

	return fmt.Errorf("unknown plugin command: %s", name)
}
