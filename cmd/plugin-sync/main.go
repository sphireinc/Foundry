package main

import (
	"fmt"
	"os"

	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/plugins"
)

func main() {
	project := plugins.NewProject(
		consts.ConfigFilePath,
		consts.PluginsDir,
		consts.GeneratedPluginsFile,
		plugins.DefaultSyncModulePath,
	)

	if err := project.Sync(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "sync plugins: %v\n", err)
		os.Exit(1)
	}
}
