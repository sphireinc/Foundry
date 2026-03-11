package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/i18n"
)

func BuildNewContent(cfg *config.Config, kind, slug string) (string, error) {
	return BuildNewContentWithOptions(cfg, NewContentOptions{
		Kind:      kind,
		Slug:      slug,
		Archetype: kind,
		Lang:      cfg.DefaultLang,
	})
}

type NewContentOptions struct {
	Kind      string
	Slug      string
	Archetype string
	Lang      string
}

func BuildNewContentWithOptions(cfg *config.Config, opts NewContentOptions) (string, error) {
	kind := strings.TrimSpace(opts.Kind)
	slug := strings.TrimSpace(opts.Slug)
	archetype := strings.TrimSpace(opts.Archetype)
	lang := normalizeContentLang(cfg, opts.Lang)

	if kind == "" {
		return "", fmt.Errorf("content kind must not be empty")
	}
	if slug == "" {
		return "", fmt.Errorf("slug must not be empty")
	}
	if archetype == "" {
		archetype = kind
	}

	if body, ok, err := loadArchetype(cfg, archetype, kind, slug, lang); err != nil {
		return "", err
	} else if ok {
		return body, nil
	}

	switch kind {
	case "page":
		return defaultPageArchetype(cfg, slug, lang), nil
	case "post":
		return defaultPostArchetype(cfg, slug, lang), nil
	default:
		return "", fmt.Errorf("unknown content type: %s", kind)
	}
}

func loadArchetype(cfg *config.Config, archetype, kind, slug, lang string) (string, bool, error) {
	path := filepath.Join(cfg.ContentDir, "archetypes", archetype+".md")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read archetype %s: %w", path, err)
	}

	return renderArchetype(cfg, kind, slug, lang, string(b)), true, nil
}

func renderArchetype(cfg *config.Config, kind, slug, lang, body string) string {
	title := humanizeSlug(slug)

	layout := cfg.Content.DefaultLayoutPage
	draft := "false"
	date := ""

	if kind == "post" {
		layout = cfg.Content.DefaultLayoutPost
		draft = "true"
		date = time.Now().Format("2006-01-02")
	}

	if strings.TrimSpace(layout) == "" {
		layout = kind
	}

	replacements := map[string]string{
		"{{title}}":  title,
		"{{slug}}":   slug,
		"{{layout}}": layout,
		"{{draft}}":  draft,
		"{{date}}":   date,
		"{{lang}}":   normalizeContentLang(cfg, lang),
		"{{type}}":   kind,
	}

	for old, newValue := range replacements {
		body = strings.ReplaceAll(body, old, newValue)
	}

	return body
}

func defaultPageArchetype(cfg *config.Config, slug, lang string) string {
	title := humanizeSlug(slug)
	layout := cfg.Content.DefaultLayoutPage
	if strings.TrimSpace(layout) == "" {
		layout = "page"
	}

	frontmatter := fmt.Sprintf(`---
title: %s
slug: %s
layout: %s
draft: false
`, title, slug, layout)

	lang = normalizeContentLang(cfg, lang)
	if lang != "" && lang != cfg.DefaultLang {
		frontmatter += fmt.Sprintf("lang: %s\n", lang)
	}

	frontmatter += `---

# ` + title + "\n\n"

	return frontmatter
}

func defaultPostArchetype(cfg *config.Config, slug, lang string) string {
	title := humanizeSlug(slug)
	layout := cfg.Content.DefaultLayoutPost
	if strings.TrimSpace(layout) == "" {
		layout = "post"
	}

	frontmatter := fmt.Sprintf(`---
title: %s
slug: %s
layout: %s
draft: true
date: %s
summary: ""
`, title, slug, layout, time.Now().Format("2006-01-02"))

	lang = normalizeContentLang(cfg, lang)
	if lang != "" && lang != cfg.DefaultLang {
		frontmatter += fmt.Sprintf("lang: %s\n", lang)
	}

	frontmatter += `---

# ` + title + "\n\n"

	return frontmatter
}

func humanizeSlug(slug string) string {
	if slug == "" {
		return ""
	}
	parts := strings.Split(slug, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func normalizeContentLang(cfg *config.Config, lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return cfg.DefaultLang
	}
	lang = i18n.NormalizeTag(lang)
	if !i18n.IsValidTag(lang) {
		return cfg.DefaultLang
	}
	return lang
}
