package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	foundryconfig "github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/safepath"
	"gopkg.in/yaml.v3"
)

type Info struct {
	Name string
	Path string
}

type Manifest struct {
	Name              string   `yaml:"name"`
	Title             string   `yaml:"title"`
	Version           string   `yaml:"version"`
	Description       string   `yaml:"description"`
	Author            string   `yaml:"author"`
	License           string   `yaml:"license"`
	MinFoundryVersion string   `yaml:"min_foundry_version"`
	Layouts           []string `yaml:"layouts"`
	Slots             []string `yaml:"slots"`
}

var requiredLaunchSlots = []string{
	"head.end",
	"body.start",
	"body.end",
	"page.before_main",
	"page.after_main",
	"page.before_content",
	"page.after_content",
	"post.before_header",
	"post.after_header",
	"post.before_content",
	"post.after_content",
	"post.sidebar.top",
	"post.sidebar.overview",
	"post.sidebar.bottom",
}

var requiredLaunchSlotFiles = map[string]string{
	"head.end":              filepath.Join("layouts", "partials", "head.html"),
	"body.start":            filepath.Join("layouts", "base.html"),
	"body.end":              filepath.Join("layouts", "base.html"),
	"page.before_main":      filepath.Join("layouts", "base.html"),
	"page.after_main":       filepath.Join("layouts", "base.html"),
	"page.before_content":   filepath.Join("layouts", "page.html"),
	"page.after_content":    filepath.Join("layouts", "page.html"),
	"post.before_header":    filepath.Join("layouts", "post.html"),
	"post.after_header":     filepath.Join("layouts", "post.html"),
	"post.before_content":   filepath.Join("layouts", "post.html"),
	"post.after_content":    filepath.Join("layouts", "post.html"),
	"post.sidebar.top":      filepath.Join("layouts", "post.html"),
	"post.sidebar.overview": filepath.Join("layouts", "post.html"),
	"post.sidebar.bottom":   filepath.Join("layouts", "post.html"),
}

func ListInstalled(themesDir string) ([]Info, error) {
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Info{}, nil
		}
		return nil, err
	}

	out := make([]Info, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		out = append(out, Info{
			Name: name,
			Path: filepath.Join(themesDir, name),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out, nil
}

func LoadManifest(themesDir, name string) (*Manifest, error) {
	var err error
	name, err = safepath.ValidatePathComponent("theme name", name)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(themesDir, name, "theme.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("theme %q is missing theme.yaml", name)
		}
		return nil, err
	}

	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if strings.TrimSpace(m.Name) == "" {
		m.Name = name
	}
	if strings.TrimSpace(m.Title) == "" {
		m.Title = m.Name
	}
	if strings.TrimSpace(m.Version) == "" {
		m.Version = "0.0.0"
	}

	return &m, nil
}

func ValidateInstalled(themesDir, name string) error {
	var err error
	name, err = safepath.ValidatePathComponent("theme name", name)
	if err != nil {
		return err
	}

	root := filepath.Join(themesDir, name)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("theme %q does not exist", name)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("theme path %q is not a directory", root)
	}

	manifest, err := LoadManifest(themesDir, name)
	if err != nil {
		return err
	}

	if strings.TrimSpace(manifest.Name) != name {
		return fmt.Errorf("theme manifest name %q must match directory %q", manifest.Name, name)
	}

	requiredLayouts := manifest.Layouts
	if len(requiredLayouts) == 0 {
		requiredLayouts = []string{"base", "index", "page", "post", "list"}
	}

	for _, layout := range requiredLayouts {
		path := filepath.Join(root, "layouts", layout+".html")
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("missing required theme layout: %s", path)
			}
			return err
		}
	}

	requiredPartials := []string{
		filepath.Join(root, "layouts", "partials", "head.html"),
		filepath.Join(root, "layouts", "partials", "header.html"),
		filepath.Join(root, "layouts", "partials", "footer.html"),
	}

	for _, path := range requiredPartials {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("missing required theme partial: %s", path)
			}
			return err
		}
	}

	if err := validateRequiredLaunchSlots(root, manifest); err != nil {
		return err
	}

	return nil
}

func Scaffold(themesDir, name string) (string, error) {
	var err error
	name, err = safepath.ValidatePathComponent("theme name", name)
	if err != nil {
		return "", err
	}

	root := filepath.Join(themesDir, name)
	if _, err := os.Stat(root); err == nil {
		return "", fmt.Errorf("theme already exists: %s", root)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	dirs := []string{
		filepath.Join(root, "assets", "css"),
		filepath.Join(root, "layouts"),
		filepath.Join(root, "layouts", "partials"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	files := map[string]string{
		filepath.Join(root, "theme.yaml"):                         scaffoldManifest(name),
		filepath.Join(root, "assets", "css", "base.css"):          scaffoldCSS(),
		filepath.Join(root, "layouts", "base.html"):               scaffoldBase(),
		filepath.Join(root, "layouts", "index.html"):              scaffoldIndex(),
		filepath.Join(root, "layouts", "page.html"):               scaffoldPage(),
		filepath.Join(root, "layouts", "post.html"):               scaffoldPost(),
		filepath.Join(root, "layouts", "list.html"):               scaffoldList(),
		filepath.Join(root, "layouts", "partials", "head.html"):   scaffoldHead(),
		filepath.Join(root, "layouts", "partials", "header.html"): scaffoldHeader(),
		filepath.Join(root, "layouts", "partials", "footer.html"): scaffoldFooter(),
	}

	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
	}

	return root, nil
}

func SwitchInConfig(configPath, themeName string) error {
	var err error
	themeName, err = safepath.ValidatePathComponent("theme name", themeName)
	if err != nil {
		return err
	}

	return foundryconfig.UpsertTopLevelScalar(configPath, "theme", themeName)
}

func scaffoldManifest(name string) string {
	return fmt.Sprintf(`name: %s
title: %s
version: 0.1.0
description: A Foundry theme.
author: Unknown
license: MIT
min_foundry_version: 0.1.0
layouts:
  - base
  - index
  - page
  - post
  - list
slots:
  - head.end
  - body.start
  - body.end
  - page.before_main
  - page.after_main
  - page.before_content
  - page.after_content
  - post.before_header
  - post.after_header
  - post.before_content
  - post.after_content
  - post.sidebar.top
  - post.sidebar.overview
  - post.sidebar.bottom
`, name, humanizeName(name))
}

func humanizeName(name string) string {
	parts := strings.Split(strings.ReplaceAll(name, "_", "-"), "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func scaffoldCSS() string {
	return `html, body {
  margin: 0;
  padding: 0;
  font-family: system-ui, sans-serif;
  color: #111;
  background: #fff;
}

a {
  color: inherit;
}

.container {
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem;
}

.site-header,
.site-footer {
  border-bottom: 1px solid #ddd;
}

.site-footer {
  border-top: 1px solid #ddd;
  border-bottom: 0;
  margin-top: 3rem;
}

.site-header .container,
.site-footer .container {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.prose {
  line-height: 1.7;
}

.prose img {
  max-width: 100%;
  height: auto;
}
`
}

func scaffoldBase() string {
	return `{{ define "base" }}
<!doctype html>
<html lang="{{ .Lang }}">
  {{ template "head" . }}
  <body>
    {{ pluginSlot "body.start" }}

    {{ template "header" . }}

    {{ pluginSlot "page.before_main" }}

    <main class="site-main">
      <div class="container">
        {{ template "content" . }}
      </div>
    </main>

    {{ pluginSlot "page.after_main" }}

    {{ template "footer" . }}

    {{ pluginSlot "body.end" }}

    {{ if .LiveReload }}
    <script>
      (() => {
        const es = new EventSource('/__reload');
        es.onmessage = () => window.location.reload();
      })();
    </script>
    {{ end }}
  </body>
</html>
{{ end }}
`
}

func scaffoldHead() string {
	return `{{ define "head" }}
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{ .Title }}</title>
  <link rel="stylesheet" href="/assets/css/foundry.bundle.css">
  {{ pluginSlot "head.end" }}
</head>
{{ end }}
`
}

func scaffoldHeader() string {
	return `{{ define "header" }}
<header class="site-header">
  <div class="container">
    <strong>{{ .Site.Title }}</strong>
    <nav>
      {{ range .Nav }}
        <a href="{{ .URL }}">{{ .Name }}</a>
      {{ end }}
    </nav>
  </div>
</header>
{{ end }}
`
}

func scaffoldFooter() string {
	return `{{ define "footer" }}
<footer class="site-footer">
  <div class="container">
    <small>{{ .Site.Title }}</small>
  </div>
</footer>
{{ end }}
`
}

func scaffoldIndex() string {
	return `{{ define "content" }}
<h1>{{ .Site.Title }}</h1>

{{ if .Documents }}
  <ul>
    {{ range .Documents }}
      <li><a href="{{ .URL }}">{{ .Title }}</a></li>
    {{ end }}
  </ul>
{{ else }}
  <p>No content found.</p>
{{ end }}
{{ end }}
`
}

func scaffoldPage() string {
	return `{{ define "content" }}
<article class="prose">
  <h1>{{ .Page.Title }}</h1>
  {{ pluginSlot "page.before_content" }}
  {{ safeHTML .Page.HTMLBody }}
  {{ pluginSlot "page.after_content" }}
</article>
{{ end }}
`
}

func scaffoldPost() string {
	return `{{ define "content" }}
<div class="split-layout">
  <section class="prose">
    {{ pluginSlot "post.before_header" }}
    <h1>{{ .Page.Title }}</h1>
    {{ pluginSlot "post.after_header" }}
    {{ pluginSlot "post.before_content" }}
    {{ safeHTML .Page.HTMLBody }}
    {{ pluginSlot "post.after_content" }}
  </section>

  <aside>
    {{ pluginSlot "post.sidebar.top" }}
    <div>
      {{ pluginSlot "post.sidebar.overview" }}
    </div>
    {{ pluginSlot "post.sidebar.bottom" }}
  </aside>
</div>
{{ end }}
`
}

func scaffoldList() string {
	return `{{ define "content" }}
<h1>{{ .Title }}</h1>

{{ if .Documents }}
  <ul>
    {{ range .Documents }}
      <li><a href="{{ .URL }}">{{ .Title }}</a></li>
    {{ end }}
  </ul>
{{ else }}
  <p>No entries found.</p>
{{ end }}
{{ end }}
`
}

func validateRequiredLaunchSlots(root string, manifest *Manifest) error {
	declared := make(map[string]struct{}, len(manifest.Slots))
	for _, slot := range manifest.Slots {
		slot = strings.TrimSpace(slot)
		if slot == "" {
			continue
		}
		declared[slot] = struct{}{}
	}

	for _, slot := range requiredLaunchSlots {
		if _, ok := declared[slot]; !ok {
			return fmt.Errorf("theme manifest is missing required slot %q", slot)
		}

		relPath := requiredLaunchSlotFiles[slot]
		path := filepath.Join(root, relPath)
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read slot template %s: %w", path, err)
		}
		call := fmt.Sprintf(`pluginSlot %q`, slot)
		if !strings.Contains(string(body), call) {
			return fmt.Errorf("theme must render required slot %q in %s", slot, path)
		}
	}

	return nil
}
