# Foundry

[![Build Status](https://img.shields.io/github/actions/workflow/status/sphireinc/foundry/build.yml?branch=main)](https://github.com/sphireinc/foundry/actions)
[![CodeQL](https://github.com/sphireinc/Foundry/actions/workflows/github-code-scanning/codeql/badge.svg?branch=main)](https://github.com/sphireinc/Foundry/actions/workflows/github-code-scanning/codeql)
[![Pages](https://github.com/sphireinc/Foundry/actions/workflows/pages/pages-build-deployment/badge.svg)](https://github.com/sphireinc/Foundry/actions/workflows/pages/pages-build-deployment)
[![Go Report Card](https://goreportcard.com/badge/github.com/sphireinc/foundry)](https://goreportcard.com/report/github.com/sphireinc/foundry)
![Last Commit](https://img.shields.io/github/last-commit/sphireinc/foundry)

<p align="center">
  <img height="300" src="logo.png" alt="logo">
</p>

Foundry is a Markdown-first CMS written in Go. It keeps content in files, renders through themes, extends through plugins, and supports both static output and local preview serving.

The project is aimed at teams that want a file-based workflow without giving up CMS-style features such as taxonomies, fields, feeds, plugin hooks, and an admin surface.

## What Foundry does

- Stores pages and posts as Markdown with frontmatter
- Supports language-aware routing and content grouping
- Builds a normalized site graph in memory
- Uses themes for layouts, partials, and theme assets
- Uses plugins for hooks, asset injection, and runtime extensions
- Generates RSS and sitemap output
- Publishes static output to `public/`
- Serves the site locally with live reload during development
- Tracks document, template, data, and taxonomy dependencies for incremental rebuilds

## Project layout

```text
cmd/
  foundry/         main CLI entrypoint
  plugin-sync/     generated plugin import synchronizer

internal/
  admin/           admin auth, HTTP handlers, service layer, UI templates
  assets/          asset sync and CSS bundling
  commands/        CLI command implementations
  config/          config loading, editing, validation
  content/         document parsing, loading, site graph assembly
  data/            data file loading
  deps/            dependency graph and incremental rebuild planning
  feed/            RSS and sitemap generation
  markup/          Markdown rendering
  plugins/         plugin metadata, loading, lifecycle, sync
  renderer/        template rendering and output writing
  router/          URL assignment
  server/          preview server, watcher, incremental rebuild orchestration
  site/            higher-level graph loading helpers
  taxonomy/        taxonomy indexing and archive helpers
  theme/           theme management and validation

plugins/           built-in plugins
themes/            installed themes
content/           project content
data/              project data files
docs/              GitHub Pages site and published coverage
scripts/           release and maintenance utilities
```

## Architecture

Foundry keeps a clean separation between content loading, route assignment, rendering, and runtime orchestration.

The main pipeline is:

```text
config -> content/data load -> site graph -> route assignment
      -> dependency graph -> renderer -> build output or preview server
```

Two graph types matter:

- `SiteGraph`: the in-memory representation of documents, routes, taxonomies, config, and loaded data
- `DependencyGraph`: the rebuild graph used to decide which outputs must be regenerated after a change

The dependency graph includes taxonomy archive outputs, so incremental rebuilds can target both document pages and taxonomy pages.

## Quick start

### Prerequisites

- Go `1.22` or newer
- A working `PATH` that includes `$(go env GOPATH)/bin` if you install with `go install`

### Install

Install the CLI:

```bash
go install github.com/sphireinc/foundry/cmd/foundry@latest
```

Verify the install:

```bash
foundry version
```

If you are working from a local checkout instead of a global install, you can run:

```bash
go run ./cmd/foundry version
```

### Start a site from the repo layout

Foundry expects a file-based project layout. The quickest way to get running is to start with this shape:

```text
content/
  config/
    site.yaml
  pages/
    index.md
  posts/
  images/
  uploads/
data/
themes/
plugins/
```

Minimal `content/config/site.yaml`:

```yaml
title: My Site
base_url: http://localhost:8080
theme: default

content_dir: content
public_dir: public
themes_dir: themes
data_dir: data
plugins_dir: plugins

server:
  addr: :8080
  live_reload: true
  live_reload_mode: stream
```

Minimal `content/pages/index.md`:

```md
---
title: Home
---

# Hello from Foundry
```

### Run it

Start the local preview server from the project root:

```bash
foundry serve
```

Then open `http://localhost:8080/`.

To produce static output:

```bash
foundry build
```

Generated files will be written to `public/`.

### Common commands

```bash
foundry version
foundry build
foundry serve
foundry serve --debug
foundry serve-preview
foundry plugin list --enabled
foundry theme list
foundry routes check
foundry admin hash-password your-password
```

### Typical local workflow

1. Update `content/config/site.yaml`.
2. Add pages and posts under `content/pages` and `content/posts`.
3. Put images under `content/images` and videos or other uploaded files under `content/uploads`.
4. Reference media from Markdown with the `media:` scheme.
5. Run `foundry serve` during development.
6. Run `foundry build` before publishing or checking generated output.

Embedded media uses normal Markdown image syntax:

```md
![Hero image](media:images/hero/banner.jpg)
![Walkthrough video](media:uploads/demo.mp4)
```

File links use normal Markdown link syntax:

```md
[Download the spec](media:uploads/spec-sheet.pdf)
```

Admin uploads return stable references in the same format, for example:

```text
media:images/posts/launch/diagram.png
media:uploads/posts/launch/demo.mp4
```

If a page appears to hang during local preview, run `foundry serve --debug` to emit per-request timing plus runtime snapshots, including:

- heap allocation and in-use heap
- stack and total runtime memory
- goroutine count
- active request count
- GC count
- process user/system CPU time and request CPU percentage estimates

If live reload causes browser connection stalls in development, switch `server.live_reload_mode` from `stream` to `poll`. `stream` uses Server-Sent Events and refreshes immediately. `poll` trades a small delay for simpler connection behavior.

## Content model

Foundry currently supports two primary document types:

- `page`
- `post`

Content is loaded from:

- `content/pages`
- `content/posts`
- `content/images`
- `content/uploads`

Language variants are represented by a leading language directory. For example:

```text
content/pages/about.md
content/pages/fr/about.md
content/posts/launch.md
content/posts/fr/launch.md
```

Markdown files use frontmatter for metadata such as:

- `title`
- `slug`
- `layout`
- `draft`
- `summary`
- `date`
- `updated_at`
- `tags`
- `categories`
- custom `taxonomies`
- `fields`
- arbitrary `params`

### Multimedia

Foundry supports images, video, audio, and downloadable files through the `media:` reference scheme.

- `media:images/...` resolves to `/images/...`
- `media:uploads/...` resolves to `/uploads/...`
- `media:assets/...` resolves to `/assets/...`

The renderer infers the output element from the target file extension:

- image files render as `<img>`
- video files render as `<video controls>`
- audio files render as `<audio controls>`
- other files remain standard links

## Configuration

The main config file is typically:

```text
content/config/site.yaml
```

Important config groups:

- `admin`: admin service settings
- `server`: preview server settings
- `content`: content directory conventions and default layouts
- `taxonomies`: taxonomy definitions and archive layouts
- `plugins`: enabled plugins
- `security`: security-sensitive rendering settings
- `feed`: RSS and sitemap output paths

### Admin

`/__admin` serves a themeable admin shell with a login form. The shell itself is public so the browser can load HTML, CSS, and JavaScript. Authenticated API access is session-based by default.

Admin users live in a filesystem-backed YAML file, which defaults to:

```text
content/config/admin-users.yaml
```

Example:

```yaml
users:
  - username: admin
    name: Admin User
    email: admin@example.com
    role: admin
    password_hash: pbkdf2-sha256$...
```

Generate a password hash with:

```bash
foundry admin hash-password "your-password"
```

Or generate a starter YAML snippet with:

```bash
foundry admin sample-user admin "Admin User" admin@example.com "your-password"
```

Browser sessions are stored in a secure cookie under `/__admin`, expire after 30 minutes of inactivity by default, and are renewed while the user remains active.

`admin.access_token` is now optional. If set, it still works for API automation with:

- `Authorization: Bearer <token>`
- `X-Foundry-Admin-Token: <token>`

Admin themes live under:

```text
themes/admin-themes/<name>/
  index.html
  assets/
    admin.css
    admin.js
```

Set the active admin theme with:

```yaml
admin:
  enabled: true
  theme: default
  users_file: content/config/admin-users.yaml
  session_ttl_minutes: 30
```

`admin.local_only` is a convenience restriction for local development. It should not be treated as the only security boundary in front of a reverse proxy.

### Security

`security.allow_unsafe_html` controls whether raw HTML in Markdown is preserved in rendered output.

### Server

`server.live_reload` turns live reload on during local preview.

`server.live_reload_mode` controls the transport:

- `stream`: uses a long-lived SSE connection to `/__reload`
- `poll`: polls `/__reload/poll` every 1.5 seconds and reloads when the rebuild version changes

Use `poll` if your browser or proxy environment is sensitive to long-lived local connections.

The preview/admin server uses explicit read, write, and idle timeouts by default.

## Themes

Themes live under `themes/<name>/`.

A theme typically contains:

```text
themes/default/
  assets/
    css/
  layouts/
    base.html
    index.html
    page.html
    post.html
    list.html
    partials/
      head.html
      header.html
      footer.html
  theme.yaml
```

Themes are responsible for:

- page and post presentation
- shared base layout
- taxonomy archive templates
- theme-specific assets

Launch themes are also expected to support the current minimum slot contract:

- `head.end`
- `body.start`
- `body.end`
- `page.before_main`
- `page.after_main`
- `page.before_content`
- `page.after_content`
- `post.before_header`
- `post.after_header`
- `post.before_content`
- `post.after_content`
- `post.sidebar.top`
- `post.sidebar.overview`
- `post.sidebar.bottom`

Theme validation now checks both of these conditions:

- the theme manifest declares the required slots
- the corresponding layouts actually render those slots

## Plugins

Plugins live under `plugins/<name>/` and are registered through generated imports.

Current plugin capabilities include:

- lifecycle hooks during load/build/serve
- route and rendering hooks
- HTML slot injection
- asset injection
- plugin validation and dependency checks

Plugin installation is intentionally conservative now: install sources are restricted to GitHub over `https` or `git@github.com`. Installing a plugin still means trusting third-party code, so treat it as a supply-chain boundary.

## Incremental rebuilds

Foundry maintains a dependency graph that relates:

- source files to documents
- templates to outputs
- data keys to outputs
- documents to taxonomy archives
- taxonomy archives to rendered URLs

When possible, the preview server rebuilds only the affected outputs instead of running a full site rebuild.

## Asset pipeline

The asset pipeline can:

- copy content assets
- copy images
- copy uploads
- copy theme assets
- copy enabled plugin assets
- build a bundled CSS file in `public/assets/css/foundry.bundle.css`

Asset roots and plugin/theme names are validated as safe paths, and symlinked asset files are rejected.

## GitHub Pages

The repository publishes a small docs site from `docs/` that includes:

- a project overview
- the CLI contract
- the latest HTML coverage report generated in CI

## Development

Useful commands while working on the repo:

```bash
go test ./...
go vet ./...
go run ./cmd/plugin-sync
```

The main CI workflow also verifies formatting, syncs generated plugin imports, builds the project, runs tests, and publishes the coverage report to GitHub Pages on `main`.

