package plugins

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type CommandContext struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
}

type CLIHook interface {
	Commands() []Command
}

type Command struct {
	Name        string
	Summary     string
	Description string
	Run         func(ctx CommandContext) error
}

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
