package clean

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
)

type command struct{}

func (command) Name() string {
	return "clean"
}

func (command) Run(cfg *config.Config, _ []string) error {
	paths := []string{
		cfg.PublicDir,
		"bin",
		"tmp",
	}

	for _, p := range paths {
		p = filepath.Clean(p)
		if p == "." || p == "/" || p == "" {
			return fmt.Errorf("refusing to clean unsafe path: %q", p)
		}

		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat %s: %w", p, err)
		}

		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("remove %s: %w", p, err)
		}

		fmt.Printf("removed %s\n", p)
	}

	return nil
}

func init() {
	registry.Register(command{})
}
