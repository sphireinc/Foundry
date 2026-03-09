# Foundry

[![Build Status](https://img.shields.io/github/actions/workflow/status/sphireinc/foundry/go.yml?branch=main)](https://github.com/sphireinc/foundry/actions)
[![Repo Size](https://img.shields.io/github/repo-size/sphireinc/foundry)](https://github.com/sphireinc/foundry)
[![Go Report Card](https://goreportcard.com/badge/github.com/sphireinc/foundry)](https://goreportcard.com/report/github.com/sphireinc/foundry)
![Last Commit](https://img.shields.io/github/last-commit/sphireinc/foundry)
![Contributors](https://img.shields.io/github/contributors/sphireinc/foundry)
![Stars](https://img.shields.io/github/stars/sphireinc/foundry)
[//]: # ([![Issues]&#40;https://img.shields.io/github/issues/sphireinc/foundry&#41;]&#40;https://github.com/sphireinc/foundry/issues&#41;)

### Foundry is a Markdown-driven CMS written in Go

It is designed for developers, startups, internal platforms, and 
content-heavy products that want the flexibility of a filesystem-driven
workflow, the structure of a real CMS, and the extensibility of a plugin 
and theme system inspired by platforms like WordPress, but reimagined in
Go. It utilizes frontmatter-enabled Markdown for it's content.

### Foundry supports:

- Markdown-based content
- Frontmatter metadata
- Config-driven active themes
- Multi-language content with English as the default language
- Hook-based plugins
- Taxonomies such as tags and categories
- Custom fields similar in spirit to Wordpress ACF (Advanced Custom Fields)
- Dynamic RSS and sitemap generation
- Live reload during development
- Asset syncing and CSS bundling
- Incremental rebuild planning through a dependency graph
- Static site output as well as local preview serving

Foundry is architected in an explicitly layered manner so the system can evolve
cleanly from a simple Markdown CMS into a much more capable platform over time.

---

# Table of Contents

- [Goals](#goals)
- [Current Capabilities](#current-capabilities)
- [High-Level Architecture](#high-level-architecture)
- [Project Structure](#project-structure)
- [Installing the Foundry CLI](#installing-the-foundry-cli)
- [How Foundry Thinks About Content](#how-foundry-thinks-about-content)
- [Language and Localization Model](#language-and-localization-model)
- [Configuration](#configuration)
- [Themes](#themes)
- [Template Rendering Model](#template-rendering-model)
- [Navigation System](#navigation-system)
- [Content Frontmatter](#content-frontmatter)
- [Taxonomies](#taxonomies)
- [Custom Fields](#custom-fields)
- [Data Files](#data-files)
- [Plugin Architecture](#plugin-architecture)
- [Dependency Graph and Incremental Rebuilds](#dependency-graph-and-incremental-rebuilds)
- [Asset Pipeline](#asset-pipeline)
- [Feeds and SEO](#feeds-and-seo)
- [Development Workflow](#development-workflow)
- [Commands](#commands)
- [Routes](#routes)
- [Design Principles](#design-principles)
- [Roadmap](#roadmap)

---

# Goals

Foundry was created to solve a problem that many teams run into: choosing between a 
traditional database-driven CMS or a Markdown-based static site generator

Traditional CMS platforms provide many features out of the box but they often 
come with problems: operational overhead, tightly coupled systems, and infrastructure complexity.

Static site generators take the opposite approach. They offer simplicity, performance, 
and a workflow that fits naturally with version control, but frequently require developers
to build or extend features themselves.

Foundry exists to bridge that gap.

It combines a Markdown-first content workflow with the architecture of a modern
CMS. Content lives in plain files and works naturally with version control, while the 
system still provides familiar capabilities such as interchangeable themes, extensible
plugins, structured configuration, taxonomy support, and scalable performance.

Foundrys goal is simple: keep the clarity and durability of a file-based system while 
preserving the flexibility and extensibility that people expect from a traditional CMS.

Its goals are:

- Keep content in plain files
- Keep architecture modular and testable
- Keep configuration explicit
- Keep themes isolatedd
- Keep plugins extensible BUT predictable
- Make local development easy
- Make future expansion possible without rewriting the core model

---

# Current Capabilities

Foundry currently supports the following core capabilities.

## Content

- Markdown pages and posts
- YAML frontmatter
- Multi language content with i18n support
- Translated content by language subdirectory
- Draft filtering in normal serve and build modes
- Draft inclusion in preview mode

## Rendering

- Theme-selected layouts
- Partial templates
- Dynamic page rendering in serve mode
- Static HTML build output
- Taxonomy archive pagesd
- Config-driven or data-driven navigation

## Platform Features

- Theme selection through configuration
- Plugin manager with hook-based extension points (hi WordPress!)
- Dynamic RSS feed generation
- Dynamic sitemap generation
- Dependency graph debug endpoints
- Asset syncing
- CSS bundle generation
- Live reload support
- Auto-open browser on startup
- Air-based hot reload workflow for Go code in development

---

# High-Level Architecture

Foundry separates the system into distinct subsystems.

The core flow is:

```text
start
  -> config loader
    -> content loader
      -> data loader
        -> plugin hooks
          -> normalized site graph
            -> route assignment
              -> dependency graph generation
                -> renderer
                  -> HTTP serve or static build output
```

There are two different graphs in the system.

### Site Graph

The content-centric site graph is the **in-memory** representation of the site itself. It contains (in no particular order);

- markdown documents 
- generated URLs 
- language groupings 
- taxonomy indexes 
- loaded data files 
- site config

### Dependency Graph

The rebuild-centric dependency graph is used for rebuild planning. It tracks relationships such as:

- source file -> document
- document -> taxonomy
- document, template, data file -> output

Keeping those two graphs separate is intentional. It makes the architecture easier to 
think about and easier to evolve in the future

---

# Project Structure

The Foundry repository looks like this:

```text
/── cmd/
│   └── cms/
│       └── main.go
/── internal/
│   ├── assets/
│   ├── cache/
│   ├── config/
│   ├── content/
│   ├── data/
│   ├── deps/
│   ├── fields/
│   ├── markup/
│   ├── plugins/
│   ├── renderer/
│   ├── router/
│   ├── server/
│   ├── taxonomy/
│   └── theme/
/── themes/
│   └── default/
│       ├── assets/
│       └── layouts/
/── content/
│   ├── assets/
│   ├── config/
│   ├── images/
│   ├── pages/
│   ├── posts/
│   └── uploads/
/── data/
/── plugins/
/── public/
/── bin/
/── scripts/
/── Makefile
/── .air.toml
/── .air.preview.toml
/── go.mod
```

1. **cmd/foundry**: contains the application entrypoint. It parses commands, loads config, etc.
2. **internal/config**: loads teh onfiguration, exposes the unified config object
3. **internal/content**: scans content, resolves translation, parsing, and rendering, as well as  bilding the in-memory document model.
4. **internal/data**: loads teh data files from /data, exposes them as simple KV stores to the site graph
5. **internal/theme**: resolves the layout file paths, loads the active theme, etc.
6. **internal/renderer**: loads partials, builds HTML output, renders dynamic responses, resolves taxonmy and navigation
7. **internal/router**: assigns URLs to document and taxonomy
8. **internal/server**: live reloading, http serving, feed and debug endpoints, rebuild orchestration, etc
9. **internal/plugins**: plugin registration and instantification, hook dispatch
10. **internal/deps**: building the dependency graph and managing rebuild plan
11. **internal/assets**: syncing static assets to the public dir, copying images, generating css bundles
12. **internal/taxonomy**: indexing taxonomy, storing entries by taxonomy name
13. **internal/fields**: normalizing custom field maps
14. **internal/markup**: Markdown-to-HTML conversion and helpers
15. **internal/cache**: in-memory caching of parsed content, etc

---

# Installing the Foundry CLI

Foundry provides a command-line interface used to create, build, and manage Foundry projects.

The CLI follows a familair command structure:

```bash
foundry <command> <subcommand> <options>
```

This design is similar in spirit to tools such as Composer and Hugo.

## Installation

The Foundry CLI can be installed directly using Go:

```bash
go install github.com/sphireinc/foundry/cmd/foundry@latest
```

This installs the Foundry binary into your Go bin directory (typically $GOPATH/bin or $HOME/go/bin).

Make sure this directory is included in your system PATH.

After installation, verify the CLI is available: `foundry version`

---

# How Foundry Thinks About Content

Foundry treats content as typed documents - simple as that. 

- Markdown files in `content/pages/*.md` become page documents
- Markdown files in `content/posts/*.md` become post documents
- Markdown files in `content/pages/<lang>/*.md` become translated page documents
- Markdown files in content/posts/<lang>/*.md` become translated post documents

The Document model has the following fields:

- `ID`
- `Type`
- `Language`
- `IsDefault`
- `Title`
- `Slug`
- `URL`
- `Layout`
- `SourcePath`
- `RawBody`
- `HTMLBody`
- `Summary`
- `Date`
- `UpdatedAt`
- `Draft`
- `Params`
- `Fields`
- `Taxonomies`

---

# Language and Localization Model

Foundry uses a English-default content translation model

English is treated as the default language in content, and translated content lives in subdirectories

| Content File                      | URI/Route                |
|-----------------------------------|--------------------------|
| `content/pages/index.md`          | `/`                      |
| `content/pages/es/index.md`       | `/es/`                   |
| `content/pages/about.md`          | `/about/`                |
| `content/posts/hello-world.md`    | `/posts/hello-world/`    | 
| `content/pages/es/about.md`       | `/es/about/`             |
| `content/posts/es/hello-world.md` | `/es/posts/hello-world/` |
| `content/pages/gb/about.md`       | `/gb/about/`             |
| `content/posts/gb/hello-world.md` | `/gb/posts/hello-world/` |


I went with this structure because it gives the default language cleaner URLs while still allowing 
explicit translated variants. It also avoids forcing the default language into a
language prefix.

In the future, subdomain language prefixing is a feature Foundry will
support. For now, subdomain routing must be done at the reverse-proxy level, such with an  Nginx config:

```nginx 
server {
    listen 80;
    server_name example.com www.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}

server {
    listen 80;
    server_name es.example.com;

    location / {
        rewrite ^/(.*)$ /es/$1 break;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

---

# Configuration

Foundry is configured through `content/config/site.yaml`

The config file controls the entire site runtime.

## Major config groups

| Config Group    | Keys                                                                                                                                         |
|-----------------|----------------------------------------------------------------------------------------------------------------------------------------------|
| Core identity   | `name`, `title`, `base_url`, `theme`, `default_lang`                                                                                         |
| Directory paths | `content_dir`, `public_dir`, `themes_dir`, `data_dir`, `plugins_dir`                                                                         |
| Server          | `addr`, `live_reload`, `auto_open_browser`, `debug_routes`                                                                                   |
| Build           | `clean_public_dir`, `include_drafts`, `minify_html`, `copy_assets`, `copy_images`, `copy_uploads`                                            |
| Content         | `pages_dir`, `posts_dir`, `images_dir`, `assets_dir`, `uploads_dir`, `default_layout_page`, `default_layout_post`, `default_page_slug_index` |
| Permalinks      | `page_default`, `page_i18n`, `post_default`, `post_i18n`                                                                                     |
| Taxonomies      | `enabled`, `default_set`                                                                                                                     |
| Plugins         | `enabled`                                                                                                                                    |
| Fields          | `enabled`, `allow_anything`                                                                                                                  |
| SEO             | `enabled`, `default_title_sep`                                                                                                               |
| Cache           | `enabled`                                                                                                                                    |
| Security        | `allow_unsafe_html`                                                                                                                          |
| Feed            | `rss_path`, `sitemap_path`, `rss_limit`, `rss_title`, `rss_description`                                                                      |

## Params

Arbitrary config-level parameters can be stored under `params`

## Menus

Navigation menus may be defined under `menus`.

The main menu is typically stored as:

```yaml
menus:
  main:
    - name: "Home"
      url: "/"
    - name: "Posts"
      url: "/posts/"
```

# Themes

Themes in Foundry live under the `/themes` directory.

Only one theme is active at a time, and the active theme is selected in the config file:

```yaml
theme: "default"
```

This mirrors the conceptual model used by WordPress

## Theme structure

A theme generally looks like this:

```text
themes/default/
├── assets/
│   ├── css/
│   ├── js/
│   └── images/
└── layouts/
    ├── base.html
    ├── index.html
    ├── page.html
    ├── post.html
    ├── list.html
    └── partials/
        ├── head.html
        ├── header.html
        ├── footer.html
        ├── post_meta.html
        └── taxonomy_chips.html
```

## Theme responsibilities

A theme controls:
- HTML structure
- visual design
- template partial composition
- theme-provided static assets

A theme does not own:
- content loading
- routing logic
- plugin execution
- rebuild planning

Layout conventions
- `base.html` defines the outer shell
- `index.html` defines the homepage
- `page.html` defines general pages
- `post.html` defines posts
- `list.html` defines archives such as taxonomy pages

## Partials

Partials are reusable templates parsed from:

```text
themes/<theme-name>/layouts/partials/*.html
```

These are included in the template parse list and can be referenced with `{{ template "name" . }}`

# Template Rendering Model

Foundry uses Go's stdlib `html/template`

The renderer builds a `ViewData` struct and passes it into the selected layout, which looks like this:

```go
type ViewData struct {
	Site         *config.Config
	Page         *content.Document
	Documents    []*content.Document
	Data         map[string]any
	Lang         string
	Title        string
	LiveReload   bool
	TaxonomyName string
	TaxonomyTerm string
	Nav          []NavItem
}
```

## Template philosophy

Foundry templates should remain presentation-focused

Content indexing, language grouping, taxonomy lookup, navigation generation, 
and similar logic should live in Go rather than in templates. This keeps themes 
cleaner and avoids burying business logic inside template files. 

However, I understand that this may not be possible for all use cases, so it is 
a flexible rule. Use your best judgement, though if publishing the theme publically, you
should disclose any such rule transgressions.

## Template helper functions

The renderer currently exposes helpers such as `safeHTML`, `field` and `data`.

`safeHTML` safely renders already-prepared HTML content.

`field` accesses custom document fields. Example:

```bash
{{ field .Page "hero_title" }}
```

`data` accesses values loaded from `/data`. Example:

```bash
{{ data "navigation" }}
```

# Navigation System

Navigation in Foundry is data-driven. It resolves in this priority order:
 
1. `site.yaml` -> `menus.main`
2. `data/navigation.yaml` -> `main`
3. built-in fallback defaults

This gives site owners flexibility - simple sites can keep navigation in config, and 
more complex sites can move it into data files.

### Active nav state

The renderer computes an Active boolean for each nav item based on the current request or 
build target. This enables the theme to highlight the current section or route. The
active-state logic normalizes URLs and uses exact or prefix matches depending on the route.

--- 

# Content Frontmatter

Foundry heavily uses Frontmatter. A typical document may look like this, and this example
uses all of the built-in frontmatter fields:

```bash
---
title: Hello World
slug: hello-world
layout: post
draft: false
summary: A short summary for previews and feeds.
date: 2026-03-06
updated_at: 2026-03-07
tags: [go, cms]
categories: [engineering]
taxonomies:
  audience: [developers]
fields:
  hero_title: "Build with confidence"
---

# Hello World

Welcome to Foundry.
```

## Additional params

Any other values are preserved in Params. This allows future features or 
theme-specific values without changing the core Frontmatter schema every time

---

# Taxonomies

Foundry supports taxonomies as a first-class content organization mechanism.

## Built-in default taxonomies

By default, the system expects `tags` and `categories`

These come from frontmatter and are normalized into a taxonomy map on the document.

## Custom taxonomies

Additional taxonomies may be declared under `taxonomies` in frontmatter.

Example:

```bash
taxonomies:
  audience: [developers, founders]
  product: [foundry]
```

## Taxonomy indexing

The taxonomy subsystem stores content entries by:
- taxonomy name
- term

For each term, it stores lightweight entries containing:
- document ID
- URL
- language
- type
- title
- slug

## Taxonomy archive routes

Foundry dynamically supports routes like:
- `/tags/go/`
- `/categories/engineering/`
- `/<lang>/tags/go/`
- `/<lang>/categories/go`

These are accessible in and rendered by `list.html`

---

# Custom Fields

Foundry supports custom fields through the `fields` frontmatter key.

This is conceptually similar to **Advanced Custom Fields** in WordPress, though 
intentionally simpler and more transparent. They look like this in Frontmatter:

```bash
fields:
  hero_title: "Build with confidence"
  hero_subtitle: "Modern publishing in Go"
  featured: true
```

Themes can access these values through the `field` template helper, as such:

```bash
{{ field .Page "hero_title" }}
```

THe custom fields allow authors to drive layout behavior and design content without 
changing the core schema. They can control things like:

- hero copy
- featured flags
- call-to-action settings
- image references
- sidebar blocks
- product-specific metadata

---

# Data Files

Foundry supports structured data in `/data`. It supports both YAML and JSON file types, or a mix of both:

```bash
data/navigation.yaml
data/authors.yaml
data/features/home.json
```

Each file is loaded into a store using a normalized key based on its path.

| Filepath                | Key to access |
|-------------------------|---------------|
| data/navigation.yaml    | navigation    |
| data/authors/jane.yaml  | authors/jane  | 
| data/features/home.json | features/home |

Data files are ideal for:
- navigation
- author metadata
- reusable content blocks 
- feature lists
- marketing content
- structured reference data

---

# Plugin Architecture

Foundry uses a hook-based plugin architecture designed to feel familiar 
to WordPress users while remaining idiomatic Go.


Plugins in Foundry are designed to be:
- Explicit
- Composable
- Testable
- Easy to register
- Straightforward to reason about

Foundry is architected in a way that favors clarity and determinism over unnecessary magic.

## Current plugin model

Plugins in Foundry are registered at compile time and enabled at runtime through configuration.

This means plugins are linked into the application binary, rather than dynamically loaded.

***This is intentional***

In Go, compile-time linking provides several advantages:
- simpler dependency management
- type safety
- easier debugging
- predictable builds
- no fragile runtime plugin loading

Dynamic plugin systems in Go tend to introduce platform limitations 
and operational complexity. Foundry avoids those pitfalls by using a registry-based
plugin system instead

The plugin API is intentionally minimal.

## Installing a plugin

Plugins can be installed in two ways

The first is to copy the plugin into the `plugins/` directory manually.

The second is to install directly from a git repository:

```bash
cms plugin install https://github.com/someOwner/someRepo
cms plugin install someOwner/somePlugin toc
```

If no explicit name is provided, Foundry will determine the plugin 
directory name from the repository URL. It supports fully qualified URLs
such as `https://github.com/someOwner/someRepo`, as well as shorthand (Github only)
such as `someOwner/somePlugin`

After installation, enable the plugin in `content/config/site.yaml`. Foundry
does not by default enable install plugins.

Then regenerate plugin imports and rebuild using `make plugins-sync`

Conversely, you can uninstall a plugin via `cms plugin uninstall <name>`

## Plugin dependencies

Plugins may declare dependencies in `plugin.yaml` metadata using repository idents such as:

```yaml
repo: github.com/someOwner/someOtherPlugin
requires:
  - github.com/someOwner/somePlugin
```

Dependencies are validated against the repo field of other
enabled plugins. If a required dependency is missing, Foundry
will fail during plugin loading with a clear error.

IMPORTANT: Repository identifiers should be written in canonical form (`github.com/owner/repo`)




## Registration model

Plugins register themselves with the global plugin registry

```bash
func init() {
    plugins.Register("my-plugin", func() plugins.Plugin {
        return &MyPlugin{}
    })
}
```

The registry stores a factory function, allowing the plugin manager to instantiate plugins as needed

## Plugin enablement

Plugins are enabled in config:

```yaml
plugins:
  enabled:
    - my-plugin
    - sitemap-enhancer
```

The plugin manager looks up registered factories and instantiates only the enabled plugins.

During startup, the plugin manager:

1. Reads the enabled plugin list 
2. Looks up each plugin in the registry 
3. Instantiates the plugin via its factory 
4. Initializes the plugin lifecycle

Only enabled plugins are instantiated - others are ignored.

Plugins listed under "enabled" that do not exist cause the application to error out

## Theme injection slots

Foundry themes can expose injection slots that plugins can target to insert HTML.

This allows plugins to extend theme templates without modifying the theme itself.

For example, a theme template might define an injection point:

```go
{{ slot "post.footer" }}
```

This creates a named slot where plugins can insert content. These are the minimum suggested slots
a theme should expose:

- `head.end`
- `body.end`
- `page.before_content`
- `page.after_content`
- `post.before_content`
- `post.after_content`
- `post.sidebar.top`
- `post.sidebar.bottom`
- `list.before_items`
- `list.after_items`

I don't think they need explanations as they are quite clear in their intention

## Plugin HTML injection

Plugins can inject HTML into specific slots exposed by the theme.

```go
func (p *MyPlugin) Init(ctx *plugins.Context) {
    ctx.InjectHTML("post.footer", "<div class='related-posts'></div>")
}
```

At render time, Foundry collects all HTML registered for a slot and injects it into the template.

Multiple plugins may target the same slot.

Injection order is deterministic and follows plugin load order (in YAML config file)

## Asset pipeline

Plugin assets live with the plugin.

To register assets, inside a plugin do this:

```go
func (p *Plugin) OnAssets(ctx *renderer.ViewData, assets *renderer.AssetSet) error {
	assets.AddStyle("/plugins/myplugin/css/widget.css")
	assets.AddScript("/plugins/myplugin/js/widget.js", renderer.ScriptPositionBodyEnd)
	return nil
}
```

The renderer injects those assets automatically into `head.end` or `body.end` HTML slots

### Why Slots Instead of Template Hooks?

Slots provide a clear contract between themes and plugins:

- Themes define where extensions are allowed
- Plugins define what they want to add and where

This prevents plugins from modifying templates and keeps themes predictable and maintainable.

## Plugin lifecycle

Plugins follow a simple lifecycle managed by the plugin manager during application startup.

The lifecycle is intentionally minimal to keep plugin behavior predictable and easy to test.

### 1. Registration

At program startup, plugins register themselves with the global registry via `init()`

```go
func init() {
    plugins.Register("my-plugin", func() plugins.Plugin {
        return &MyPlugin{}
    })
}
```

Registration only stores a factory function. No plugin code is executed yet.

### 2. Instantiation

During application startup, the plugin manager does the following:

1. Reads the list of enabled plugins from config 
2. Looks up each plugin in the registry 
3. Instantiates the plugin using its factory function

### 3. Initialization

After instantiation, the plugin manager calls the plugins Init method.

```go
func (p *MyPlugin) Init(ctx *plugins.Context) error {
    return nil
}
```

The Context provides access to services the plugin may interact with, such as HTML injection, 
configuration, and logging

Plugins should use this phase to register injections, initialize internal state, or attach to extension points

Initialization happens **once** at startup.

### 4. Runtime Operation

After initialization, plugins operate passively through the mechanisms they registered during Init.

Plugins do not run continuously unless explicitly designed to.

## Minimal example ppugin

Below is a minimal working plugin that injects HTML into a theme slot.

### File structure

This file structure shows the possible base files for assets, metadata, and the plugin itself.

Realistically, the only necessary file is the entrypoint. 

Within the plugins directory, should exist `<pluginName>/<entrypoint>.go` at 
a minimum (in the example below, `plugin.go`). In the config YAML `config.plugin.enabled`,
it references the **folder name**, in this case `<pluginName>`.

```text
plugins/
  related-posts/
    assets/
      css/
        style.css
      js/
        script.js
    plugin.yaml
    plugin.go
```

### Plugin implementation

```go
package someplugin

import (	
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

type Plugin struct{}

func init() {
    plugins.Register("someplugin", func() plugins.Plugin {
        return &Plugin{}
    })
}

func (p *Plugin) Init(ctx *plugins.Context) error {
    ctx.InjectHTML(
        "post.footer",
        `<div class="someplugin"><h3>Some HTML</h3></div>`,
    )
    return nil
}

func (p *Plugin) Commands() []plugins.Command {
	return []plugins.Command{
		{
			Name:        "someplugin",
			Summary:     "Runs someplugin command",
			Description: "Shows information about this plugin.",
			Run: func(ctx plugins.CommandContext) error {
				_, err := fmt.Fprintln(ctx.Stdout, "Plugin is installed and available.")
				return err
			},
		},
	}
}
```

## Current hook types

The plugin system currently supports these hooks:

- `OnConfigLoaded(*config.Config) error`
- `OnContentDiscovered(path string) error`
- `OnFrontmatterParsed(*content.Document) error`
- `OnMarkdownRendered(*content.Document) error`
- `OnDocumentParsed(*content.Document) error`
- `OnDataLoaded(map[string]any) error`
- `OnGraphBuilding(*content.SiteGraph) error`
- `OnGraphBuilt(*content.SiteGraph) error`
- `OnTaxonomyBuilt(*content.SiteGraph) error`
- `OnRoutesAssigned(*content.SiteGraph) error`
- `OnContext(*renderer.ViewData) error`
- `OnBeforeRender(*renderer.ViewData) error`
- `OnAfterRender(url string, html []byte) ([]byte, error)`
- `OnAssetsBuilding(*config.Config) error`
- `OnBuildStarted() error`
- `OnBuildCompleted(*content.SiteGraph) error`
- `OnServerStarted(addr string) error`
- `RegisterRoutes(mux *http.ServeMux)`

These are invoked by the content loader.

The content loader depends only on a narrow hook interface rather than the 
concrete plugin manager type. This avoids cyclic imports and keeps subsystems loosely coupled.

---

# Dependency Graph and Incremental Rebuilds

Foundry includes a dependency graph subsystem to support smarter rebuild
behavior. Without a dependency graph, every content or template change tends 
to trigger a full rebuild. That is wasteful and becomes painful as the site 
grows. The dependency graph lets Foundry reason about which outputs are 
affected by which sources.

## Node types

The graph includes node types such as:
- `source`
- `document`
- `template`
- `data`
- `taxonomy`
- `output`

Example relationships
- a Markdown file produces a document
- a document produces an output
- a layout template affects outputs rendered with that layout
- base.html affects every rendered output
- a data file can affect many outputs

A rebuild change set may include:
- source changes
- template changes
- data changes
- asset changes
- full rebuild triggers

The resolver then computes a rebuild plan, which can request:
- a full rebuild
- rebuilding specific URLs

## Current state

The dependency graph is already used to support incremental rebuild planning and a debug endpoint.

There is still room for future refinement, such as more precise taxonomy 
archive targeting and partial dependency tracing.

---

# Asset Pipeline

Foundry includes a lightweight asset pipeline. It currently does the following:

- copies content/assets to public/assets
- copies content/images to public/images
- copies content/uploads to public/uploads
- copies themes/<active>/assets to public/theme
- bundles CSS files into public/assets/css/foundry.bundle.css

The CSS bundler currently concatenates the following into a single bundle file:

- theme CSS files
- content CSS files

This provides a simple default development and build workflow without 
introducing a heavyweight frontend toolchain.

Future asset pipeline ideas

- minification
- asset hashing
- JS bundling
- Sass support
- image transformations
- PostCSS pipeline
- theme asset manifests

---

# Feeds and SEO

Foundry dynamically generates RSS and sitemap endpoints.

## RSS

Configurable via:

- `feed.rss_path`
- `feed.rss_limit`
- `feed.rss_title`
- `feed.rss_description`

RSS items are generated from posts and include:

- title
- link
- GUID
- summary
- publication date

## Sitemap

Configurable via:

- `feed.sitemap_path`

The sitemap includes all non-draft documents and uses `updated_at` or `date` where available.

---

# Development Workflow

Foundry supports a strong local development workflow with the following serve modes:

| Serve mode      | Description                                                                |
|-----------------|----------------------------------------------------------------------------|
| `serve`         | Runs the site locally, excludes drafts, enables live development features. |
| `serve-preview` | Runs the site locally and includes drafts.                                 |
| `build`         | Builds static output into `public/`                                        |

## Air-based hot reload

Foundry includes .air.toml and .air.preview.toml so Go code changes trigger 
rebuilds and restarts automatically.

This works well alongside Foundrys own watcher, which handles content, 
theme, data, and asset changes and triggers browser reloads.

## Browser auto-open

When enabled in config, Foundry opens the browser automatically on startup.

## Live reload

Foundry exposes an internal SSE endpoint for browser reloads and injects a small 
client snippet into pages during serve mode.

---

# Commands

The following commands are in the Makefile:


| Command          | Meaning                                   |
|------------------|-------------------------------------------|
| make serve       | runs `go run ./cmd/foundry serve`         |
| make preview     | runs `go run ./cmd/foundry serve-preview` |
| make dev         | uses `air` for hot reload                 |
| make dev-preview | uses `air` in preview mode                |
| make build       | builds static output                      |
| make compile     | produces a binary under bin/              |
| make fmt         | runs `go fmt`                             |
| make lint        |                                           |
| make test        |                                           |
| make tidy        |                                           |
| make clean       |                                           |

--- 

# Routes

Foundry currently supports a mix of document, system, and debug routes.

## Taxonomy routes

- `/tags/`
- `/categories/`

## Feed routes

- `/rss.xml`
- `/sitemap.xml`

## Debug and development routes

- `/__reload`
- `/__debug/deps`

## Static asset routes

- `/assets/...`
- `/images/...`
- `/uploads/...`
- `/theme/...`

--- 

# Design Principles

Foundry is built around several core principles:

1. Content should be durable 
2. Architecture should be layered 
3. Each subsystem should have a clear job. 
4. Themes should be easily replaceable
5. Themes should not own business logic. 
6. Plugins should be explicit and deterministic
7. A hook-based plugin system is easier to understand than hidden lifecycle magic. 
8. Defaults should be polished 
9. The out-of-the-box experience should feel like a product, not a scaffold or starting point.
10. Complexity should be earned 
11. Complicated systems are fine when they provide real tangible value, but not before.

--- 

# Roadmap

The current architecture is intentionally designed so the following 
features can be added without rewriting the core.

### Near-term
- posts index page
- tags index page
- categories index page
- built-in pagination
- theme partial expansion
- richer navigation structures to enable submenus
- built-in example plugins
- safer and richer custom field helpers

### Mid-term
- plugin render hooks
- image metadata extraction
- content search indexing
- taxonomy archive optimization in rebuild planning
- asset hashing and minification
- dark mode and theme variants
- author pages
- related content logic

### Long-term
- admin/editor UI
- content workflows
- draft scheduling
- content versioning
- API output modes
- remote theme packages
- installable plugin ecosystem
- structured schema layer on top of fields

--- 

That's all :) 

Welcome to Foundry - enjoy it, use it, give me feedback, and help extend it and make it better 
