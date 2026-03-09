package routes

import (
	"context"
	"fmt"
	"sort"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/router"
)

type command struct{}

type routeRow struct {
	URL    string
	Type   string
	Lang   string
	Title  string
	Source string
}

func (command) Name() string {
	return "routes"
}

func (command) Summary() string {
	return "List and validate generated routes"
}

func (command) Group() string {
	return "routing commands"
}

func (command) Details() []string {
	return []string{
		"foundry routes list",
		"foundry routes check",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry routes [list|check]")
	}

	switch args[2] {
	case "list":
		return runList(cfg)
	case "check":
		return runCheck(cfg)
	default:
		return fmt.Errorf("unknown routes subcommand: %s", args[2])
	}
}

func runList(cfg *config.Config) error {
	graph, err := loadGraph(cfg)
	if err != nil {
		return err
	}

	rows := make([]routeRow, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		rows = append(rows, routeRow{
			URL:    doc.URL,
			Type:   doc.Type,
			Lang:   doc.Lang,
			Title:  doc.Title,
			Source: doc.SourcePath,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].URL != rows[j].URL {
			return rows[i].URL < rows[j].URL
		}
		if rows[i].Lang != rows[j].Lang {
			return rows[i].Lang < rows[j].Lang
		}
		return rows[i].Source < rows[j].Source
	})

	urlWidth := len("URL")
	typeWidth := len("TYPE")
	langWidth := len("LANG")

	for _, row := range rows {
		if len(row.URL) > urlWidth {
			urlWidth = len(row.URL)
		}
		if len(row.Type) > typeWidth {
			typeWidth = len(row.Type)
		}
		if len(row.Lang) > langWidth {
			langWidth = len(row.Lang)
		}
	}

	fmt.Printf("%-*s  %-*s  %-*s  %s\n", urlWidth, "URL", typeWidth, "TYPE", langWidth, "LANG", "TITLE")
	for _, row := range rows {
		fmt.Printf("%-*s  %-*s  %-*s  %s\n", urlWidth, row.URL, typeWidth, row.Type, langWidth, row.Lang, row.Title)
	}

	fmt.Println("")
	fmt.Printf("%d route(s)\n", len(rows))
	return nil
}

func runCheck(cfg *config.Config) error {
	graph, err := loadGraph(cfg)
	if err != nil {
		return err
	}

	errCount := 0
	seen := make(map[string]string)

	for _, doc := range graph.Documents {
		if doc.URL == "" {
			fmt.Printf("empty URL: %s\n", doc.SourcePath)
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

	if errCount > 0 {
		return fmt.Errorf("route check failed with %d error(s)", errCount)
	}

	fmt.Printf("route check OK (%d route(s))\n", len(graph.Documents))
	return nil
}

func loadGraph(cfg *config.Config) (*content.SiteGraph, error) {
	pluginManager, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		return nil, fmt.Errorf("load plugins: %w", err)
	}

	if err := pluginManager.OnConfigLoaded(cfg); err != nil {
		return nil, fmt.Errorf("plugin config hook failed: %w", err)
	}

	loader := content.NewLoader(cfg, pluginManager, true)
	graph, err := loader.Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load content: %w", err)
	}

	resolver := router.NewResolver(cfg)
	if err := resolver.AssignURLs(graph); err != nil {
		return nil, fmt.Errorf("assign urls: %w", err)
	}

	if err := pluginManager.OnRoutesAssigned(graph); err != nil {
		return nil, fmt.Errorf("route hook failed: %w", err)
	}

	return graph, nil
}

func init() {
	registry.Register(command{})
}
