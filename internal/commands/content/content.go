package contentcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/site"
)

type command struct{}

type contentRow struct {
	Type   string
	Lang   string
	Title  string
	Slug   string
	URL    string
	Draft  bool
	Source string
}

func (command) Name() string {
	return "content"
}

func (command) Summary() string {
	return "Manage and inspect content"
}

func (command) Group() string {
	return "content commands"
}

func (command) Details() []string {
	return []string{
		"foundry content lint",
		"foundry content new page <slug>",
		"foundry content new post <slug>",
		"foundry content list",
		"foundry content graph",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry content [lint|new|list|graph]")
	}

	switch args[2] {
	case "lint":
		return runLint(cfg)
	case "new":
		return runNew(cfg, args)
	case "list":
		return runList(cfg)
	case "graph":
		return runGraph(cfg)
	default:
		return fmt.Errorf("unknown content subcommand: %s", args[2])
	}
}

func runLint(cfg *config.Config) error {
	graph, err := loadGraph(cfg, true)
	if err != nil {
		return err
	}

	errCount := 0
	seenSource := make(map[string]struct{})
	seenSlugByTypeLang := make(map[string]string)

	for _, doc := range graph.Documents {
		if strings.TrimSpace(doc.Title) == "" {
			fmt.Printf("missing title: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Slug) == "" {
			fmt.Printf("missing slug: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Layout) == "" {
			fmt.Printf("missing layout: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Type) == "" {
			fmt.Printf("missing type: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Lang) == "" {
			fmt.Printf("missing lang: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.URL) == "" {
			fmt.Printf("missing URL: %s\n", doc.SourcePath)
			errCount++
		}

		if _, ok := seenSource[doc.SourcePath]; ok {
			fmt.Printf("duplicate source path: %s\n", doc.SourcePath)
			errCount++
		}
		seenSource[doc.SourcePath] = struct{}{}

		key := doc.Type + "|" + doc.Lang + "|" + doc.Slug
		if other, ok := seenSlugByTypeLang[key]; ok {
			fmt.Printf("duplicate slug within type/lang %q for %s and %s\n", key, other, doc.SourcePath)
			errCount++
		} else {
			seenSlugByTypeLang[key] = doc.SourcePath
		}
	}

	if errCount > 0 {
		return fmt.Errorf("content lint failed with %d error(s)", errCount)
	}

	fmt.Printf("content lint OK (%d document(s))\n", len(graph.Documents))
	return nil
}

func runNew(cfg *config.Config, args []string) error {
	if len(args) < 5 {
		return fmt.Errorf("usage: foundry content new [page|post] <slug>")
	}

	kind := strings.TrimSpace(args[3])
	slug := normalizeSlug(args[4])
	if slug == "" {
		return fmt.Errorf("slug must not be empty")
	}

	var path string
	switch kind {
	case "page":
		path = filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, slug+".md")
	case "post":
		path = filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, slug+".md")
	default:
		return fmt.Errorf("unknown content type: %s", kind)
	}

	body, err := content.BuildNewContent(cfg, kind, slug)
	if err != nil {
		return err
	}

	return writeNewContentFile(path, body)
}

func runList(cfg *config.Config) error {
	graph, err := loadGraph(cfg, true)
	if err != nil {
		return err
	}

	rows := make([]contentRow, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		rows = append(rows, contentRow{
			Type:   doc.Type,
			Lang:   doc.Lang,
			Title:  doc.Title,
			Slug:   doc.Slug,
			URL:    doc.URL,
			Draft:  doc.Draft,
			Source: doc.SourcePath,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Type != rows[j].Type {
			return rows[i].Type < rows[j].Type
		}
		if rows[i].Lang != rows[j].Lang {
			return rows[i].Lang < rows[j].Lang
		}
		if rows[i].URL != rows[j].URL {
			return rows[i].URL < rows[j].URL
		}
		return rows[i].Source < rows[j].Source
	})

	typeWidth := len("TYPE")
	langWidth := len("LANG")
	slugWidth := len("SLUG")
	draftWidth := len("DRAFT")

	for _, row := range rows {
		if len(row.Type) > typeWidth {
			typeWidth = len(row.Type)
		}
		if len(row.Lang) > langWidth {
			langWidth = len(row.Lang)
		}
		if len(row.Slug) > slugWidth {
			slugWidth = len(row.Slug)
		}
	}

	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
		typeWidth, "TYPE",
		langWidth, "LANG",
		slugWidth, "SLUG",
		draftWidth, "DRAFT",
		"TITLE",
	)

	for _, row := range rows {
		draft := "false"
		if row.Draft {
			draft = "true"
		}
		fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
			typeWidth, row.Type,
			langWidth, row.Lang,
			slugWidth, row.Slug,
			draftWidth, draft,
			row.Title,
		)
	}

	fmt.Println("")
	fmt.Printf("%d document(s)\n", len(rows))
	return nil
}

func runGraph(cfg *config.Config) error {
	graph, err := loadGraph(cfg, true)
	if err != nil {
		return err
	}

	fmt.Println("Site graph")
	fmt.Println("----------")
	fmt.Printf("documents: %d\n", len(graph.Documents))
	fmt.Printf("urls: %d\n", len(graph.ByURL))
	fmt.Printf("languages: %d\n", len(graph.ByLang))
	fmt.Printf("types: %d\n", len(graph.ByType))
	fmt.Println("")

	fmt.Println("By language:")
	langs := sortedKeysDocs(graph.ByLang)
	for _, lang := range langs {
		fmt.Printf("- %s: %d\n", lang, len(graph.ByLang[lang]))
	}
	fmt.Println("")

	fmt.Println("By type:")
	types := sortedKeysDocs(graph.ByType)
	for _, typ := range types {
		fmt.Printf("- %s: %d\n", typ, len(graph.ByType[typ]))
	}
	fmt.Println("")

	if graph.Taxonomies.Values != nil && len(graph.Taxonomies.Values) > 0 {
		fmt.Println("Taxonomies:")
		for _, name := range graph.Taxonomies.OrderedNames() {
			def := graph.Taxonomies.Definition(name)
			terms := graph.Taxonomies.Values[name]
			fmt.Printf("- %s (%s): %d term(s)\n", name, def.DisplayTitle(cfg.DefaultLang), len(terms))

			for _, term := range graph.Taxonomies.OrderedTerms(name) {
				fmt.Printf("  - %s: %d document(s)\n", term, len(terms[term]))
			}
		}
		fmt.Println("")
	}

	fmt.Println("Documents:")
	rows := make([]contentRow, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		rows = append(rows, contentRow{
			Type:   doc.Type,
			Lang:   doc.Lang,
			Title:  doc.Title,
			Slug:   doc.Slug,
			URL:    doc.URL,
			Draft:  doc.Draft,
			Source: doc.SourcePath,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].URL != rows[j].URL {
			return rows[i].URL < rows[j].URL
		}
		return rows[i].Source < rows[j].Source
	})

	for _, row := range rows {
		fmt.Printf("- %s [%s/%s] %s\n", row.URL, row.Type, row.Lang, row.Source)
	}

	return nil
}

func loadGraph(cfg *config.Config, includeDrafts bool) (*content.SiteGraph, error) {
	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, includeDrafts)
	if err != nil {
		return nil, err
	}
	return graph, nil
}

func normalizeSlug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

func writeNewContentFile(path, body string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path must not be empty")
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}

	fmt.Printf("created %s\n", path)
	return nil
}

func sortedKeysDocs[T any](m map[string][]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func init() {
	registry.Register(command{})
}
