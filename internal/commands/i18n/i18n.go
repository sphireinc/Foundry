package i18ncmd

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

func (command) Name() string {
	return "i18n"
}

func (command) Summary() string {
	return "Inspect and scaffold translated content"
}

func (command) Group() string {
	return "i18n commands"
}

func (command) Details() []string {
	return []string{
		"foundry i18n list",
		"foundry i18n missing",
		"foundry i18n scaffold <lang> <type> <slug>",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry i18n [list|missing|scaffold]")
	}

	switch args[2] {
	case "list":
		return runList(cfg)
	case "missing":
		return runMissing(cfg)
	case "scaffold":
		return runScaffold(cfg, args)
	}

	return fmt.Errorf("unknown i18n subcommand: %s", args[2])
}

func runList(cfg *config.Config) error {
	graph, err := loadGraph(cfg)
	if err != nil {
		return err
	}

	langs := make([]string, 0, len(graph.ByLang))
	for lang := range graph.ByLang {
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	if len(langs) == 0 {
		fmt.Println("no languages found")
		return nil
	}

	for _, lang := range langs {
		fmt.Printf("%s (%d)\n", lang, len(graph.ByLang[lang]))
	}
	return nil
}

func runMissing(cfg *config.Config) error {
	graph, err := loadGraph(cfg)
	if err != nil {
		return err
	}

	defaultLang := cfg.DefaultLang
	if strings.TrimSpace(defaultLang) == "" {
		return fmt.Errorf("default language is empty")
	}

	defaultDocs := make(map[string]*content.Document)
	for _, doc := range graph.Documents {
		if doc.Lang != defaultLang {
			continue
		}
		key := doc.Type + "|" + doc.Slug
		defaultDocs[key] = doc
	}

	langs := make([]string, 0, len(graph.ByLang))
	for lang := range graph.ByLang {
		if lang == defaultLang {
			continue
		}
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	missingCount := 0
	for _, lang := range langs {
		existing := make(map[string]struct{})
		for _, doc := range graph.ByLang[lang] {
			key := doc.Type + "|" + doc.Slug
			existing[key] = struct{}{}
		}

		for key, doc := range defaultDocs {
			if _, ok := existing[key]; ok {
				continue
			}
			fmt.Printf("missing %s translation for [%s] %s (%s)\n", lang, doc.Type, doc.Slug, doc.SourcePath)
			missingCount++
		}
	}

	if missingCount == 0 {
		fmt.Println("no missing translations")
		return nil
	}

	return fmt.Errorf("found %d missing translation(s)", missingCount)
}

func runScaffold(cfg *config.Config, args []string) error {
	if len(args) < 6 {
		return fmt.Errorf("usage: foundry i18n scaffold <lang> <type> <slug>")
	}

	lang := strings.TrimSpace(args[3])
	typ := strings.TrimSpace(args[4])
	slug := strings.TrimSpace(args[5])

	if lang == "" || typ == "" || slug == "" {
		return fmt.Errorf("lang, type, and slug must not be empty")
	}
	if lang == cfg.DefaultLang {
		return fmt.Errorf("scaffold language must not be the default language")
	}
	if typ != "page" && typ != "post" {
		return fmt.Errorf("type must be page or post")
	}

	srcPath := defaultLanguagePath(cfg, typ, slug)
	body, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source document %s: %w", srcPath, err)
	}

	dstPath := translatedPath(cfg, lang, typ, slug)
	if _, err := os.Stat(dstPath); err == nil {
		return fmt.Errorf("translated file already exists: %s", dstPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dstPath, body, 0o644); err != nil {
		return err
	}

	fmt.Printf("created %s\n", dstPath)
	return nil
}

func loadGraph(cfg *config.Config) (*content.SiteGraph, error) {
	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, true)
	if err != nil {
		return nil, err
	}
	return graph, nil
}

func defaultLanguagePath(cfg *config.Config, typ, slug string) string {
	switch typ {
	case "page":
		return filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, slug+".md")
	default:
		return filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, slug+".md")
	}
}

func translatedPath(cfg *config.Config, lang, typ, slug string) string {
	switch typ {
	case "page":
		return filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, lang, slug+".md")
	default:
		return filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, lang, slug+".md")
	}
}

func init() {
	registry.Register(command{})
}
