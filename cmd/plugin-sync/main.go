package main

import (
	"fmt"
	"os"

	"github.com/sphireinc/foundry/internal/plugins"
)

func main() {
	err := plugins.SyncFromConfig(plugins.SyncOptions{
		ConfigPath: plugins.DefaultSyncConfigPath,
		PluginsDir: plugins.DefaultSyncPluginsDir,
		OutputPath: plugins.DefaultSyncOutputPath,
		ModulePath: plugins.DefaultSyncModulePath,
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "sync plugins: %v\n", err)
		os.Exit(1)
	}
}
