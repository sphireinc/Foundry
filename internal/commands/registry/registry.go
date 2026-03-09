package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
)

type Command interface {
	Name() string
	Summary() string
	Group() string
	Details() []string
	Run(cfg *config.Config, args []string) error
}

type Info struct {
	Name    string
	Summary string
	Group   string
	Details []string
}

var commands = map[string]Command{}

func Register(cmd Command) {
	if cmd == nil || cmd.Name() == "" {
		panic("commands: invalid command registration")
	}
	if _, exists := commands[cmd.Name()]; exists {
		panic("commands: duplicate command registration: " + cmd.Name())
	}
	commands[cmd.Name()] = cmd
}

func Run(cfg *config.Config, args []string) (bool, error) {
	if len(args) < 2 {
		return false, nil
	}

	cmd, ok := commands[args[1]]
	if !ok {
		return false, nil
	}

	return true, cmd.Run(cfg, args)
}

func List() []Info {
	out := make([]Info, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, Info{
			Name:    cmd.Name(),
			Summary: cmd.Summary(),
			Group:   cmd.Group(),
			Details: cmd.Details(),
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
	sb.WriteString("usage: foundry <command>\n")

	for _, group := range groups {
		sb.WriteString("\n")
		sb.WriteString(group)
		sb.WriteString(":\n")

		for _, item := range grouped[group] {
			sb.WriteString(fmt.Sprintf("  %-*s  %s\n", nameWidth, item.Name, item.Summary))
			for _, detail := range item.Details {
				sb.WriteString(fmt.Sprintf("  %-*s  %s\n", nameWidth, "", detail))
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
