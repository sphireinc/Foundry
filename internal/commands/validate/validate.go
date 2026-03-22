package validate

import (
	"context"
	"fmt"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	foundryconfig "github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/ops"
	"github.com/sphireinc/foundry/internal/site"
	"github.com/sphireinc/foundry/internal/theme"
)

type command struct{}

func (command) Name() string {
	return "validate"
}

func (command) Summary() string {
	return "Validate config, plugins, content, and routes"
}

func (command) Group() string {
	return "core commands"
}

func (command) Details() []string {
	return nil
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *foundryconfig.Config, _ []string) error {
	errCount := 0

	if errs := foundryconfig.Validate(cfg); len(errs) > 0 {
		cliout.Println(cliout.Heading("config:"))
		for _, err := range errs {
			fmt.Printf("%s %v\n", cliout.Fail("-"), err)
		}
		errCount += len(errs)
	}

	if err := theme.NewManager(cfg.ThemesDir, cfg.Theme).MustExist(); err != nil {
		fmt.Printf("%s %v\n", cliout.Fail("theme:"), err)
		errCount++
	}

	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, true)
	if err != nil {
		fmt.Printf("%s %v\n", cliout.Fail("site:"), err)
		errCount++
	} else {
		report := ops.AnalyzeSite(cfg, graph)
		fmt.Printf("%s %d document(s)\n", cliout.Label("validated"), len(graph.Documents))
		fmt.Printf("%s %d route(s)\n", cliout.Label("validated"), len(graph.ByURL))
		for _, msg := range report.Messages() {
			fmt.Println(msg)
		}
		errCount += len(report.Messages())
	}

	if errCount > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errCount)
	}

	cliout.Successf("validation OK")
	return nil
}

func init() {
	registry.Register(command{})
}
