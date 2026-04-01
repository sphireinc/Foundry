# Foundry

[![Build Status](https://img.shields.io/github/actions/workflow/status/sphireinc/foundry/build.yml?branch=main)](https://github.com/sphireinc/foundry/actions)
[![CodeQL](https://github.com/sphireinc/Foundry/actions/workflows/github-code-scanning/codeql/badge.svg?branch=main)](https://github.com/sphireinc/Foundry/actions/workflows/github-code-scanning/codeql)
[![Pages](https://github.com/sphireinc/Foundry/actions/workflows/pages/pages-build-deployment/badge.svg)](https://github.com/sphireinc/Foundry/actions/workflows/pages/pages-build-deployment)
[![Go Report Card](https://goreportcard.com/badge/github.com/sphireinc/foundry)](https://goreportcard.com/report/github.com/sphireinc/foundry)
![Last Commit](https://img.shields.io/github/last-commit/sphireinc/foundry)

<p align="center">
  <img height="300" src="readme-assets/logo.png" alt="logo">
</p>

Foundry is a Markdown-first CMS written in Go. It keeps content in files, renders through themes, extends through plugins, and supports both static output and local preview serving.

The project is aimed at teams that want a file-based workflow without giving up CMS-style features such as taxonomies, theme-owned custom fields, feeds, plugin hooks, and an admin surface.

See more of Foundry here: [Foundry Screenshots](README_SCREENSHOTS.md)

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

## Quick Start

The fastest way to get the project running locally is via Docker:

```bash
docker compose up -d --build
```

The compose setup bind-mounts the project into the container for source-driven
development, but keeps `data/` and `public/` on named Docker volumes so the
runtime can write sessions, admin state, and generated output without host-file
permission issues.

Otherwise, see the [Getting Started](#getting-started) section for how to install the `foundry` command, run Foundry locally, or run it in portable standalone mode without Docker.

Foundry will run on `http://localhost:8080/` by default, and the admin panel
is reachable at `http://localhost:8080/__admin`. The default login on a 
new install is `admin:admin`.

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

## Formatting

Foundry uses:

- `go fmt` for Go code
- `prettier` for JS, CSS, HTML, and Markdown assets/docs

Install the formatter tooling once:

```bash
npm install
```

Then run:

```bash
make fmt
make fmt-web
make fmt-all
```

## Official JavaScript SDKs

Foundry now ships two official framework-agnostic JavaScript SDKs under `sdk/`:

- `sdk/admin`
- `sdk/frontend`

They exist to give admin frontends, plugin UIs, and JS-powered themes a supported client contract instead of forcing each consumer to hand-roll fetch logic against unstable internal endpoints.

The Admin SDK targets the authenticated admin API under `admin.path + /api`.

The Frontend SDK targets the public Foundry platform surface under `/__foundry`, with a live JSON API in preview/server mode and generated static artifacts under `public/__foundry/` for built sites.

The shared SDK core handles:

- request construction
- normalized JSON/error handling
- capability discovery helpers
- common client configuration

The current official browser entrypoints are:

```text
/__foundry/sdk/admin/index.js
/__foundry/sdk/frontend/index.js
```

Example:

```js
import { createAdminClient } from '/__foundry/sdk/admin/index.js';
import { createFrontendClient } from '/__foundry/sdk/frontend/index.js';

const admin = createAdminClient({ baseURL: '/__admin' });
const frontend = createFrontendClient({ mode: 'auto' });
```

Current capability discovery endpoints:

- admin: `<admin.path>/api/capabilities`
- frontend: `/__foundry/api/capabilities`

Built sites also emit frontend SDK data artifacts under:

```text
public/__foundry/
  capabilities.json
  site.json
  navigation.json
  routes.json
  collections.json
  search.json
  preview.json
  content/<id>.json
  sdk/...
```

The shipped themes use these SDKs too:

- the default admin theme imports the Admin SDK from `/__foundry/sdk/admin/index.js`
- the default frontend theme boots a small SDK-based runtime from `/theme/js/foundry-theme.js`

Plugin-defned admin pages and widgets can also target a stable shell contract now. A plugin can declare admin page and widget bundles in `plugin.yaml`, Foundry exposes those bundles under `<admin.path>/extensions/<plugin>/...`, and the default admin shell will automatically import them when their page or widget slot is active. The shell dispatches `foundry:admin-extension-page` and `foundry:admin-extension-widget` and exposes `window.FoundryAdmin` so plugin code can mount against a supported runtime surface instead of private admin internals. The built-in Extensions admin page itself uses a separate route, `<admin.path>/a-extensions`, so it does not collide with the plugin asset namespace.

## Getting Started

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

Minimal `content/config/site.yaml` (though you can just cope the `example.site.yaml`):

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

If you want Foundry to stay running after you close the terminal, use standalone mode:

```bash
foundry serve-standalone
foundry status
foundry logs -f
foundry stop
foundry update check
foundry service install
foundry service status
foundry service restart
```

Standalone mode stores its runtime files under `.foundry/run/` in the project:

- `standalone.json`
- `standalone.log`

To produce static output:

```bash
foundry build
```

Generated files will be written to `public/`.

For preview-oriented output that includes non-published workflow states:

```bash
foundry build --preview
```

That also writes a preview manifest at `public/preview-links.json`.

### Common commands

```bash
foundry version
foundry build
foundry build --preview
foundry build --env preview --target production
foundry serve
foundry serve --debug
foundry serve-standalone
foundry serve-preview
foundry status
foundry restart
foundry stop
foundry logs -f
foundry service install
foundry service status
foundry service restart
foundry service uninstall
foundry release cut v1.3.3
foundry update check
foundry update apply
foundry plugin list --enabled
foundry theme list
foundry theme migrate field-contracts
foundry routes check
foundry admin hash-password your-password
```

### Typical local workflow

1. Update `content/config/site.yaml`.
2. Add pages and posts under `content/pages` and `content/posts`.
3. Put media under the dedicated collection roots: `content/images`, `content/videos`, `content/audio`, and `content/documents`.
4. Reference media from Markdown with the `media:` scheme.
5. Run `foundry serve` during development.
6. Run `foundry build` before publishing or checking generated output.

### Standalone mode

For simple self-hosting or local development where you want Foundry to keep running in the background without Docker, use:

```bash
foundry serve-standalone
```

Then manage it with:

```bash
foundry status
foundry logs
foundry logs -f
foundry restart
foundry stop
foundry update check
foundry update apply
```

Notes:

- standalone mode is designed for macOS and Linux
- it writes state and logs under `.foundry/run/`
- if you launch standalone from source with `go run`, Foundry first builds a managed binary under `.foundry/run/` and keeps that binary running instead of keeping `go run` alive after logout
- it is a portable convenience runtime, not a replacement for Docker, `systemd`, or `launchd` in more serious production setups

### Managed service mode

For a more durable self-hosted runtime that behaves more like `nginx`, install
Foundry as a user-level OS service:

```bash
foundry service install
foundry service status
foundry service restart
foundry service uninstall
```

Behavior by platform:

- Linux: installs a user service under `~/.config/systemd/user/`
- macOS: installs a LaunchAgent under `~/Library/LaunchAgents/`

Foundry stores service metadata and logs under `.foundry/run/` in the project.
If Foundry was launched from source via `go run`, service installation first
builds a managed binary under `.foundry/run/` and points the service at that
binary.

On Linux, user services may need lingering enabled to survive logout and reboot:

```bash
loginctl enable-linger "$USER"
```
Embedded media uses normal Markdown image syntax:

```md
![Hero image](media:images/hero/banner.jpg)
![Walkthrough video](media:videos/demo.mp4)
```

File links use normal Markdown link syntax:

```md
[Download the spec](media:documents/spec-sheet.pdf)
```

Admin uploads return stable references in the same format, for example:

```text
media:images/posts/launch/diagram.png
media:videos/posts/launch/demo.mp4
media:documents/posts/launch/spec-sheet.pdf
```

If a page appears to hang during local preview, run `foundry serve --debug` to emit per-request timing plus runtime snapshots, including:

- heap allocation and in-use heap
- stack and total runtime memory
- goroutine count
- active request count
- GC count
- process user/system CPU time and request CPU percentage estimates

If live reload causes browser connection stalls in development, switch `server.live_reload_mode` from `stream` to `poll`. `stream` uses Server-Sent Events and refreshes immediately. `poll` trades a small delay for simpler connection behavior.

## Deploy and operations

Foundry supports environment-specific config overlays and named deploy targets.

If `content/config/site.preview.yaml` exists, it can be layered on top of the base config with:

```bash
foundry build --env preview
```

Named targets are configured under `deploy.targets` and applied with:

```bash
foundry build --target production
foundry build --env staging --target edge
```

If `deploy.default_target` is set, Foundry applies that target automatically when no explicit `--target` flag is provided.

`foundry doctor` now reports timing breakdowns for:

- plugin config hooks
- content/data loading
- route assignment
- route hooks
- asset sync
- renderer
- feed generation

`foundry validate` now checks:

- broken internal links
- broken `media:` references
- missing layout templates
- orphaned media
- duplicate URLs
- duplicate type/lang slug combinations
- taxonomy inconsistencies

The content command set also includes portability and migration helpers:

```bash
foundry content export bundle.zip
foundry backup create
foundry backup list
foundry content import markdown ./legacy-markdown
foundry content import wordpress ./wordpress.xml
foundry content migrate layout page landing --dry-run
foundry content migrate field-rename marketing old_field new_field --dry-run
```

## Backups

Foundry now has a dedicated backup flow for the content tree:

```bash
foundry backup create
foundry backup create ./archives/manual-snapshot.zip
foundry backup list
foundry backup git-snapshot "before launch"
foundry backup git-log 10
```

By default, managed backups are written under:

```text
.foundry/backups
```

Backups are zip archives of `content/` plus a small `backup-manifest.json`
entry. Before writing the archive, Foundry checks free disk space on the target
filesystem and refuses to start the backup if there is not enough headroom.

Foundry also supports local Git-backed snapshots as a second backup format:

- the snapshot repo lives under `.foundry/backups/git`
- each snapshot copies the current `content/` tree into that repo
- Foundry commits only when there are actual content changes
- this is a local history mechanism today, not a remote Git push integration yet

You can also enable debounced on-change backups in `content/config/site.yaml`:

```yaml
backup:
  enabled: true
  dir: ".foundry/backups"
  on_change: true
  debounce_seconds: 45
  retention_count: 20
  min_free_mb: 256
  headroom_percent: 125
```

When `backup.on_change` is enabled, `foundry serve` and `foundry serve-preview`
will create a backup after the content tree goes quiet for the configured
debounce window. Foundry also prunes older backups beyond `retention_count`.

The admin Themes screen now also exposes manual zip backup creation, download,
and restore actions for the same archive set.

## Release updates

Foundry includes a release-aware updater for managed standalone installs:

```bash
foundry update check
foundry update apply
```

Behavior depends on install mode:

- `standalone`: self-update supported
- `docker`: update availability only, roll out a new image manually
- `source`: update availability only, pull and rebuild manually
- `binary`: update availability only unless you run it under `serve-standalone`

Self-update uses GitHub Releases metadata, compares the running version against
the latest release tag, downloads the matching release asset, verifies a
published `.sha256` asset when available, replaces the binary, and restarts the
standalone runtime.

This repository also includes a GitHub Actions release workflow at
`.github/workflows/release.yml`. Pushing a `v*.*.*` tag now generates and uploads:

- `foundry-linux-amd64.tar.gz`
- `foundry-linux-arm64.tar.gz`
- `foundry-darwin-amd64.tar.gz`
- `foundry-darwin-arm64.tar.gz`
- `foundry-windows-amd64.tar.gz`
- matching `.sha256` files for each archive

The admin Themes screen also shows current version, latest release version,
install mode, and whether self-update is supported.

## Cutting a release

To cut a release tag locally:

```bash
foundry release cut v1.3.3
```

To cut and push the tag in one step:

```bash
foundry release cut v1.3.3 --push
```

You can also use the repo-local wrapper:

```bash
scripts/release.sh v1.3.3 --push
```

Release cutting rules:

- run it from the repository root
- the worktree must be clean
- versions must be in `vX.Y.Z` format

When the tag is pushed, `.github/workflows/release.yml` builds and uploads the
`foundry-*.tar.gz` archives and matching `.sha256` files to GitHub Releases.

## Content model

Foundry currently supports two primary document types:

- `page`
- `post`

Content is loaded from:

- `content/pages`
- `content/posts`
- `content/images`
- `content/videos`
- `content/audio`
- `content/documents`

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

Foundry supports images, video, audo, and downloadable files through the `media:` reference scheme.

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

The admin `Settings` area edits this same file through structured forms and an
`Advanced YAML` tab. Foundry keeps YAML as the storage format on disk, but the
admin works with a structured config object so common settings can be edited as
forms and then written back safely to `site.yaml`.

Important config groups:

- `admin`: admin service settings
- `server`: preview server settings
- `content`: content directory conventions and default layouts
- `taxonomies`: taxonomy definitions and archive layouts
- `plugins`: enabled plugins
- `security`: security-sensitive rendering settings
- `feed`: RSS and sitemap output paths

### Advanced custom fields

Foundry handles advanced custom fields through theme-owned contracts, not through `content/config/site.yaml`.

- Themes declare supported fields in `theme.yaml` under `field_contracts`
- Page-specific field values stay in page frontmatter under `fields:`
- Shared/global field values live in `content/custom-fields.yaml`
- The admin editor resolves the current page's available fields from the active theme
- The admin `Custom Fields` section edits shared/global field groups declared by the active theme

Example `theme.yaml`:

```yaml
field_contracts:
  - key: marketing-page
    title: Marketing Page
    description: Fields for standard marketing pages.
    target:
      scope: document
      types: [page]
      layouts: [page]
      slugs: [pricing, about, contact]
    fields:
      - name: hero_title
        type: text
        label: Hero Title
        required: true
      - name: hero_body
        type: textarea
        label: Hero Body
  - key: site_marketing
    title: Site Marketing
    target:
      scope: shared
      key: site_marketing
    fields:
      - name: primary_cta_label
        type: text
        label: Primary CTA Label
```

Example page frontmatter:

```yaml
---
title: Pricing
slug: pricing
layout: page
fields:
  hero_title: Clear pricing for modern teams
  hero_body: Foundry keeps publishing infrastructure calm and predictable.
---
```

Example shared values:

```yaml
values:
  site_marketing:
    primary_cta_label: Start with Foundry
```

If you still have legacy config-owned field schemas, migrate them with:

```bash
foundry theme migrate field-contracts
```

### Admin

`admin.path` controls where the themeable admin shell is mounted. By default it is `/__admin`. The shell itself is public so the browser can load HTML, CSS, and JavaScript. Authenticated API access is session-based by default.

Admin users live in a filesystem-backed YAM file, which defaults to:

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

Browser sessions are stored in a secure cookie scoped to `admin.path`, expire after 30 minutes of inactivity by default, and are renewed while the user remains active.

`admin.access_token` is now optional. If set, it still works for API automation with:

- `Authorization: Bearer <token>`
- `X-Foundry-Admin-Token: <token>`

Admin themes live under:

```text
themes/admin-themes/<name>/
  admin-theme.yaml
  index.html
  assets/
    admin.css
    admin.js
```

`admin-theme.yaml` is the admin-theme manifest. It now supports:

- `admin_api`
- `sdk_version`
- `compatibility_version`
- `components`
- `widget_slots`
- `screenshots`

Both shipped themes now declare and validate against the current SDK contract:

- frontend theme `sdk_version: v1`
- admin theme `sdk_version: v1`

The current stable admin-theme component contract is:

- `shell`
- `login`
- `navigation`
- `documents`
- `media`
- `users`
- `config`
- `plugins`
- `themes`
- `audit`

The current stable admin-theme widget-slot contract is:

- `overview.after`
- `documents.sidebar`
- `media.sidebar`
- `plugins.sidebar`

Set the active admin theme with:

```yaml
admin:
  enabled: true
  path: /__admin
  theme: default
  users_file: content/config/admin-users.yaml
  session_ttl_minutes: 30
```

`admin.local_only` is a convenience restriction for local development. It should not be treated as the only security boundary in front of a reverse proxy.

The dfault admin theme now includes:

- a structured frontmatter editor that stays in sync with raw Markdown
- media-picker insertion for stable `media:` references
- document and media history/trash views with restore and purge actions
- restore-preview flows that load a diff before a document restore is committed
- media replacement while preserving the canonical reference path
- an audit log view
- dedicated user-security flows for password reset tokens, TOTP setup/disable, and session revocation
- a Debug page with runtime, content, storage, integrity, activity, and persisted build-report visibility when `admin.debug.pprof` is enabled
- keyboard shortcuts:
  - `Cmd/Ctrl+S` save the current form
  - `Cmd/Ctrl+Enter` preview the current document
  - `Cmd/Ctrl+K` open the command palette
  - `Shift+/` toggle shortcut help
  - `g d`, `g m`, `g u`, `g a` jump to Documents, Media, Users, and Audit

The admin UI also includes breadcrumbs, toast notifications, unsaved-change warnings, clearer error panels, a command palette for fast navigation and creation shortcuts, review/scheduled overview queues, and client-side pagination/sorting for the major management tables.

### Security

`security.allow_unsafe_html` controls whether raw HTML in Markdown is preserved in rendered output.

### Server

`server.live_reload` turns live reload on during local preview.

`server.live_reload_mode` controls the transport:

- `stream`: uses a long-lived SSE connection to `/__reload`
- `poll`: polls `/__reload/poll` every 1.5 seconds and reloads when the rebuild version changes

Use `poll` if your browser or proxy environment is sensitive to long-lived local connections.

The preview/admin server uses explicit read, write, and idle timeouts by default.

Static builds now also emit a frontend search index at:

```text
public/search.json
```

The generated and live search surfaces now include snippets, and the search APIs apply simple weighted ranking so title and summary matches are promoted ahead of body-only matches.

`foundry validate` now checks for:

- broken `media:` references
- broken internal links to routes and static files

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

Theme manifests now support richer metadata:

- `supported_layouts`
- `config_schema`
- `field_contracts`
- `screenshots`
- `compatibility_version`

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

It also checks:

- required layouts and partials
- template references to missing partials/layouts
- template parse failures with diagnostics suitable for admin reporting

## Plugins

Plugins live under `plugins/<name>/` and are registered through generated imports.

Current plugin capabilities include:

- lifecycle hooks during load/build/serve
- route and rendering hooks
- HTML slot injection
- asset injection
- plugin validation and dependency checks

Plugin manifests now also support:

- `dependencies`
- `compatibility_version`
- `config_schema`
- `screenshots`

Plugin installation is intentionally conservative now: install sources are restricted to GitHub over `https` or `git@github.com`. Installing a plugin still means trusting third-party code, so treat it as a supply-chain boundary.

Plugin management is safer now:

- updates keep rollback snapshots under `plugins/.rollback/<name>/...`
- plugins can be rolled back to the latest preserved snapshot
- admin plugin records now include health/diagnostic reporting, dependency/config metadata, and rollback availability

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


# License

Please see `LICENSE`, `LICENSE.PLUGIN.EXCEPTION`, and `LICENSE.THEMES.EXCEPTION` for full license information.

```text
Foundry includes additional permissions under AGPLv3 section 7 for
third-party plugins and themes. Plugins, extensions, modules, themes,
templates, skins, style packages, and similar customizations that work
with Foundry solely through its documented public extension and theming
interfaces are treated as separate and independent works. They are not
subject to the AGPL solely because they interoperate with Foundry
through those public interfaces, and their authors may license them
under terms of their choice, including proprietary terms.

These permissions do not apply to Foundry itself, to modifications of
Foundry, to works based on non-public or internal interfaces, or to
works that copy code from Foundry except as otherwise permitted.
```
