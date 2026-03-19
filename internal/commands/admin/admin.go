package admincmd

import (
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
)

type command struct{}

func (command) Name() string         { return "admin" }
func (command) Summary() string      { return "Admin auth helpers" }
func (command) Group() string        { return "utility" }
func (command) RequiresConfig() bool { return false }
func (command) Details() []string {
	return []string{
		"foundry admin hash-password <password>",
		"foundry admin sample-user <username> <name> <email> <password>",
	}
}

func (command) Run(_ *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry admin [hash-password|sample-user] ...")
	}

	switch strings.TrimSpace(args[2]) {
	case "hash-password":
		if len(args) != 4 {
			return fmt.Errorf("usage: foundry admin hash-password <password>")
		}
		hash, err := users.HashPassword(args[3])
		if err != nil {
			return err
		}
		fmt.Println(hash)
		return nil

	case "sample-user":
		if len(args) != 7 {
			return fmt.Errorf("usage: foundry admin sample-user <username> <name> <email> <password>")
		}
		hash, err := users.HashPassword(args[6])
		if err != nil {
			return err
		}
		fmt.Printf("users:\n  - username: %s\n    name: %s\n    email: %s\n    role: admin\n    password_hash: %s\n", args[3], args[4], args[5], hash)
		return nil
	default:
		return fmt.Errorf("unknown admin subcommand: %s", args[2])
	}
}

func init() {
	registry.Register(command{})
}
