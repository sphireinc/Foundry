package validate

import (
	"context"
	"fmt"

	"github.com/sphireinc/foundry/internal/commands/registry"
	foundryconfig "github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/router"
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
		fmt.Println("config:")
		for _, err := range errs {
			fmt.Printf("- %v\n", err)
		}
		errCount += len(errs)
	}

	if err := theme.NewManager(cfg.ThemesDir, cfg.Theme).MustExist(); err != nil {
		fmt.Printf("theme: %v\n", err)
		errCount++
	}

	pluginManager, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		fmt.Printf("plugins: %v\n", err)
		errCount++
	} else {
		loader := content.NewLoader(cfg, pluginManager, true)
		graph, loadErr := loader.Load(context.Background())
		if loadErr != nil {
			fmt.Printf("content load: %v\n", loadErr)
			errCount++
		} else {
			resolver := router.NewResolver(cfg)
			if routeErr := resolver.AssignURLs(graph); routeErr != nil {
				fmt.Printf("routes: %v\n", routeErr)
				errCount++
			} else {
				seen := make(map[string]string)
				for _, doc := range graph.Documents {
					if doc.URL == "" {
						fmt.Printf("document %s has empty URL\n", doc.SourcePath)
						errCount++
						continue
					}
					if other, ok := seen[doc.URL]; ok {
						fmt.Printf("duplicate URL %s for %s and %s\n", doc.URL, other, doc.SourcePath)
						errCount++
						continue
					}
					seen[doc.URL] = doc.SourcePath
				}

				fmt.Printf("validated %d document(s)\n", len(graph.Documents))
				fmt.Printf("validated %d route(s)\n", len(seen))
			}
		}
	}

	if errCount > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errCount)
	}

	fmt.Println("validation OK")
	return nil
}

func init() {
	registry.Register(command{})
}
