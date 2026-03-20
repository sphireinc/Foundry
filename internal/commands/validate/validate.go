package validate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	foundryconfig "github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/media"
	"github.com/sphireinc/foundry/internal/site"
	"github.com/sphireinc/foundry/internal/theme"
)

var (
	markdownLinkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	htmlHrefRE     = regexp.MustCompile(`(?i)\bhref="([^"]+)"`)
	htmlSrcRE      = regexp.MustCompile(`(?i)\bsrc="([^"]+)"`)
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

	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, true)
	if err != nil {
		fmt.Printf("site: %v\n", err)
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

		brokenLinks := validateReferences(cfg, graph, seen)
		for _, msg := range brokenLinks {
			fmt.Println(msg)
		}
		errCount += len(brokenLinks)
	}

	if errCount > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errCount)
	}

	fmt.Println("validation OK")
	return nil
}

func validateReferences(cfg *foundryconfig.Config, graph *content.SiteGraph, seen map[string]string) []string {
	var errs []string
	for _, doc := range graph.Documents {
		if doc == nil {
			continue
		}
		for _, ref := range collectReferences(doc.RawBody) {
			switch {
			case strings.HasPrefix(ref, media.ReferenceScheme):
				if err := validateMediaReference(cfg, ref); err != nil {
					errs = append(errs, fmt.Sprintf("document %s has broken media reference %s: %v", doc.SourcePath, ref, err))
				}
			case isInternalLink(ref):
				if err := validateInternalReference(cfg, ref, seen); err != nil {
					errs = append(errs, fmt.Sprintf("document %s has broken internal link %s: %v", doc.SourcePath, ref, err))
				}
			}
		}
	}
	return errs
}

func collectReferences(raw string) []string {
	refs := make([]string, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		refs = append(refs, value)
	}
	for _, match := range markdownLinkRE.FindAllStringSubmatch(raw, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}
	for _, match := range htmlHrefRE.FindAllStringSubmatch(raw, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}
	for _, match := range htmlSrcRE.FindAllStringSubmatch(raw, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}
	return refs
}

func isInternalLink(ref string) bool {
	ref = strings.TrimSpace(ref)
	return strings.HasPrefix(ref, "/") && !strings.HasPrefix(ref, "//")
}

func validateMediaReference(cfg *foundryconfig.Config, ref string) error {
	resolved, err := media.ResolveReference(ref)
	if err != nil {
		return err
	}
	root := ""
	switch resolved.Collection {
	case "images":
		root = filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir)
	case "uploads":
		root = filepath.Join(cfg.ContentDir, cfg.Content.UploadsDir)
	case "assets":
		root = filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir)
	}
	if root == "" {
		return fmt.Errorf("unknown media collection")
	}
	_, err = os.Stat(filepath.Join(root, filepath.FromSlash(resolved.Path)))
	return err
}

func validateInternalReference(cfg *foundryconfig.Config, ref string, seen map[string]string) error {
	ref = strings.TrimSpace(ref)
	if idx := strings.Index(ref, "#"); idx >= 0 {
		ref = ref[:idx]
	}
	if ref == "" {
		return nil
	}
	if _, ok := seen[ref]; ok {
		return nil
	}
	if ref == cfg.Feed.RSSPath || ref == cfg.Feed.SitemapPath {
		return nil
	}
	switch {
	case strings.HasPrefix(ref, "/images/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, strings.TrimPrefix(ref, "/images/")))
		return err
	case strings.HasPrefix(ref, "/uploads/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.UploadsDir, strings.TrimPrefix(ref, "/uploads/")))
		return err
	case strings.HasPrefix(ref, "/assets/"):
		_, err := os.Stat(filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, strings.TrimPrefix(ref, "/assets/")))
		return err
	default:
		return fmt.Errorf("route not found")
	}
}

func init() {
	registry.Register(command{})
}
