package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
)

func BuildNewContent(cfg *config.Config, kind, slug string) (string, error) {
	kind = strings.TrimSpace(kind)
	slug = strings.TrimSpace(slug)

	if kind == "" {
		return "", fmt.Errorf("content kind must not be empty")
	}
	if slug == "" {
		return "", fmt.Errorf("slug must not be empty")
	}

	if body, ok, err := loadArchetype(cfg, kind, slug); err != nil {
		return "", err
	} else if ok {
		return body, nil
	}

	switch kind {
	case "page":
		return defaultPageArchetype(cfg, slug), nil
	case "post":
		return defaultPostArchetype(cfg, slug), nil
	default:
		return "", fmt.Errorf("unknown content type: %s", kind)
	}
}

func loadArchetype(cfg *config.Config, kind, slug string) (string, bool, error) {
	path := filepath.Join(cfg.ContentDir, "archetypes", kind+".md")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read archetype %s: %w", path, err)
	}

	return renderArchetype(cfg, kind, slug, string(b)), true, nil
}

func renderArchetype(cfg *config.Config, kind, slug, body string) string {
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
		"{{lang}}":   cfg.DefaultLang,
		"{{type}}":   kind,
	}

	for old, newValue := range replacements {
		body = strings.ReplaceAll(body, old, newValue)
	}

	return body
}

func defaultPageArchetype(cfg *config.Config, slug string) string {
	title := humanizeSlug(slug)
	layout := cfg.Content.DefaultLayoutPage
	if strings.TrimSpace(layout) == "" {
		layout = "page"
	}

	return fmt.Sprintf(`---
title: %s
slug: %s
layout: %s
draft: false
---

# %s

`, title, slug, layout, title)
}

func defaultPostArchetype(cfg *config.Config, slug string) string {
	title := humanizeSlug(slug)
	layout := cfg.Content.DefaultLayoutPost
	if strings.TrimSpace(layout) == "" {
		layout = "post"
	}

	return fmt.Sprintf(`---
title: %s
slug: %s
layout: %s
draft: true
date: %s
summary: ""
---

# %s

`, title, slug, layout, time.Now().Format("2006-01-02"), title)
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
