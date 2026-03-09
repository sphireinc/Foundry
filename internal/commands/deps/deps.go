package depscmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/deps"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/router"
)

type command struct{}

func (command) Name() string {
	return "deps"
}

func (command) Summary() string {
	return "Inspect dependency and rebuild relationships"
}

func (command) Group() string {
	return "dependency commands"
}

func (command) Details() []string {
	return []string{
		"foundry deps graph",
		"foundry deps explain <url>",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry deps [graph|explain]")
	}

	switch args[2] {
	case "graph":
		return runGraph(cfg)
	case "explain":
		if len(args) < 4 {
			return fmt.Errorf("usage: foundry deps explain <url>")
		}
		return runExplain(cfg, args[3])
	default:
		return fmt.Errorf("unknown deps subcommand: %s", args[2])
	}
}

func runGraph(cfg *config.Config) error {
	graph, depGraph, err := loadGraphs(cfg)
	if err != nil {
		return err
	}

	exported := depGraph.Export()

	rawNodes, _ := exported["nodes"].([]*deps.Node)
	rawEdges, _ := exported["edges"].([]deps.Edge)

	fmt.Println("Dependency graph")
	fmt.Println("----------------")
	fmt.Printf("documents: %d\n", len(graph.Documents))
	fmt.Printf("nodes: %d\n", len(rawNodes))
	fmt.Printf("edges: %d\n", len(rawEdges))
	fmt.Println("")

	sort.Slice(rawNodes, func(i, j int) bool {
		if rawNodes[i].Type != rawNodes[j].Type {
			return rawNodes[i].Type < rawNodes[j].Type
		}
		return rawNodes[i].ID < rawNodes[j].ID
	})

	fmt.Println("Nodes:")
	for _, node := range rawNodes {
		fmt.Printf("- %s [%s]", node.ID, node.Type)
		if len(node.Meta) > 0 {
			fmt.Printf(" %s", formatMeta(node.Meta))
		}
		fmt.Println("")
	}

	fmt.Println("")
	fmt.Println("Edges:")
	sort.Slice(rawEdges, func(i, j int) bool {
		if rawEdges[i].From != rawEdges[j].From {
			return rawEdges[i].From < rawEdges[j].From
		}
		return rawEdges[i].To < rawEdges[j].To
	})
	for _, edge := range rawEdges {
		fmt.Printf("- %s -> %s\n", edge.From, edge.To)
	}

	return nil
}

func runExplain(cfg *config.Config, targetURL string) error {
	graph, depGraph, err := loadGraphs(cfg)
	if err != nil {
		return err
	}

	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return fmt.Errorf("url must not be empty")
	}
	if !strings.HasPrefix(targetURL, "/") {
		targetURL = "/" + targetURL
	}

	if _, exists := graph.ByURL[targetURL]; !exists {
		return fmt.Errorf("route not found: %s", targetURL)
	}

	outputID := "output:" + targetURL
	node, ok := depGraph.Node(outputID)
	if !ok {
		return fmt.Errorf("dependency node not found for route: %s", targetURL)
	}

	fmt.Printf("Explain %s\n", targetURL)
	fmt.Println("")

	fmt.Printf("Node ID: %s\n", node.ID)
	fmt.Printf("Type:    %s\n", node.Type)
	if len(node.Meta) > 0 {
		fmt.Printf("Meta:    %s\n", formatMeta(node.Meta))
	}
	fmt.Println("")

	exported := depGraph.Export()
	rawEdges, _ := exported["edges"].([]deps.Edge)

	dependsOn := make([]string, 0)
	rebuilds := depGraph.DependentsOf(outputID)

	for _, edge := range rawEdges {
		if edge.To == outputID {
			dependsOn = append(dependsOn, edge.From)
		}
	}

	sort.Strings(dependsOn)
	sort.Strings(rebuilds)

	if len(dependsOn) > 0 {
		fmt.Println("Depends on:")
		for _, dep := range dependsOn {
			if n, ok := depGraph.Node(dep); ok {
				fmt.Printf("- %s [%s]", n.ID, n.Type)
				if len(n.Meta) > 0 {
					fmt.Printf(" %s", formatMeta(n.Meta))
				}
				fmt.Println("")
			} else {
				fmt.Printf("- %s\n", dep)
			}
		}
		fmt.Println("")
	}

	if len(rebuilds) > 0 {
		fmt.Println("Dependents:")
		for _, rebuild := range rebuilds {
			if n, ok := depGraph.Node(rebuild); ok {
				fmt.Printf("- %s [%s]", n.ID, n.Type)
				if len(n.Meta) > 0 {
					fmt.Printf(" %s", formatMeta(n.Meta))
				}
				fmt.Println("")
			} else {
				fmt.Printf("- %s\n", rebuild)
			}
		}
		fmt.Println("")
	}

	fmt.Println("If this output route changes, it is itself a rebuild target.")
	return nil
}

func loadGraphs(cfg *config.Config) (*content.SiteGraph, *deps.Graph, error) {
	pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		return nil, nil, err
	}
	if err := pm.OnConfigLoaded(cfg); err != nil {
		return nil, nil, err
	}

	loader := content.NewLoader(cfg, pm, true)
	graph, err := loader.Load(context.Background())
	if err != nil {
		return nil, nil, err
	}

	resolver := router.NewResolver(cfg)
	if err := resolver.AssignURLs(graph); err != nil {
		return nil, nil, err
	}
	if err := pm.OnRoutesAssigned(graph); err != nil {
		return nil, nil, err
	}

	depGraph := deps.BuildSiteDependencyGraph(graph, cfg.Theme)
	return graph, depGraph, nil
}

func formatMeta(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}

	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, meta[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func init() {
	registry.Register(command{})
}
