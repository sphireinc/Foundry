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
	"github.com/sphireinc/foundry/internal/site"
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

	nodes := depGraph.Nodes()
	edges := depGraph.Edges()

	fmt.Println("Dependency graph")
	fmt.Println("----------------")
	fmt.Printf("documents: %d\n", len(graph.Documents))
	fmt.Printf("nodes: %d\n", len(nodes))
	fmt.Printf("edges: %d\n", len(edges))
	fmt.Println("")

	fmt.Println("Nodes:")
	for _, node := range nodes {
		fmt.Printf("- %s [%s]", node.ID, node.Type)
		if len(node.Meta) > 0 {
			fmt.Printf(" %s", formatMeta(node.Meta))
		}
		fmt.Println("")
	}

	fmt.Println("")
	fmt.Println("Edges:")
	for _, edge := range edges {
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

	dependencies := depGraph.DependenciesOf(outputID)
	directDependents := depGraph.DirectDependentsOf(outputID)
	transitiveDependents := depGraph.DependentsOf(outputID)

	if len(dependencies) > 0 {
		fmt.Println("Depends on:")
		for _, dep := range dependencies {
			printNode(depGraph, dep)
		}
		fmt.Println("")
	}

	if len(directDependents) > 0 {
		fmt.Println("Direct dependents:")
		for _, dep := range directDependents {
			printNode(depGraph, dep)
		}
		fmt.Println("")
	}

	if len(transitiveDependents) > 0 {
		fmt.Println("Transitive dependents:")
		for _, dep := range transitiveDependents {
			printNode(depGraph, dep)
		}
		fmt.Println("")
	}

	fmt.Println("This route is itself a rebuild target when its inputs change.")
	return nil
}

func loadGraphs(cfg *config.Config) (*content.SiteGraph, *deps.Graph, error) {
	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, true)
	if err != nil {
		return nil, nil, err
	}

	depGraph := deps.BuildSiteDependencyGraph(graph, cfg.Theme)
	return graph, depGraph, nil
}

func printNode(g *deps.Graph, id string) {
	if n, ok := g.Node(id); ok {
		fmt.Printf("- %s [%s]", n.ID, n.Type)
		if len(n.Meta) > 0 {
			fmt.Printf(" %s", formatMeta(n.Meta))
		}
		fmt.Println("")
		return
	}

	fmt.Printf("- %s\n", id)
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
