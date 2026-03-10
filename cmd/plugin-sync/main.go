package main

import (
	"fmt"
	"os"

	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/diag"
	"github.com/sphireinc/foundry/internal/logx"
	"github.com/sphireinc/foundry/internal/plugins"
)

func main() {
	logx.InitFromEnv()

	project := plugins.NewProject(
		consts.ConfigFilePath,
		consts.PluginsDir,
		consts.GeneratedPluginsFile,
		plugins.DefaultSyncModulePath,
	)

	if err := project.Sync(); err != nil {
		err = diag.Wrap(diag.KindPlugin, "sync plugins", err)
		logx.Error("plugin sync failed", "kind", diag.KindOf(err), "error", err)
		_, _ = fmt.Fprintln(os.Stderr, diag.Present(err))
		os.Exit(diag.ExitCode(err))
	}
}
