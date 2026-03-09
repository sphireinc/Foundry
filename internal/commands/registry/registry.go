package registry

import (
	"sort"

	"github.com/sphireinc/foundry/internal/config"
)

type Command interface {
	Name() string
	Run(cfg *config.Config, args []string) error
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

func Names() []string {
	out := make([]string, 0, len(commands))
	for name := range commands {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
