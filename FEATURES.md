# Foundry CMS Features

Foundry is a Markdown-first, file-based CMS written in Go for teams that want the control of a codebase with the authoring experience of a modern CMS.

## Core CMS capabilities

- **Markdown-first content model** for pages and posts
- **File-based content storage** instead of a database-first architecture
- **Frontmatter-driven documents** with support for title, slug, layout, summary, dates, draft state, archived state, taxonomies, params, and custom fields
- **Clean page and post separation** with dedicated content directories and default layouts
- **Static site generation** to a public output directory
- **Local preview server** for real-time development and editing
- **Route-aware content graph** that maps documents, URLs, languages, and content types
- **Document summaries, detail views, and raw source access** in the admin and CLI
- **Draft and published workflows** built directly into content state
- **Expanded editorial workflow states** including draft, in review, scheduled, published, and archived
- **Archived content support** without deleting the source document state
- **In-place soft delete and versioning** using `*.trash.<timestamp>.*` and `*.version.<timestamp>.*` file conventions

## Authoring and editorial workflow

- **Create new pages from the admin**
- **Create new posts from the admin**
- **Create content from archetypes/templates**
- **Optional language selection at document creation time**
- **Structured frontmatter editor** in the admin for title, slug, layout, date, summary, tags, categories, language, draft, and archived state
- **Edit raw Markdown directly** from the admin
- **Two-way sync between structured fields and raw frontmatter**
- **Version comments** captured alongside document revisions
- **Live document preview** before publishing
- **Restore-preview workflow** that loads a document diff before committing a restore
- **Save content back to the filesystem** with validation and safe path handling
- **Document listing with filters** for drafts, type, language, and text query
- **Status switching** between draft, in review, scheduled, published, and archived
- **Scheduled publish and unpublish windows**
- **Author, last-editor, created-at, and updated-at attribution** surfaced in admin and frontmatter
- **Editorial notes** stored with workflow metadata
- **Document history, diff, restore, and purge flows** in the admin
- **Trash and history views** for editorial recovery workflows
- **Document locking with heartbeat and collision prevention**
- **Word count support** in preview responses and plugin data

## Multilingual and routing features

- **Language-aware content organization** using language-prefixed directories
- **Language-aware routing** for both default and translated content
- **Content grouping by language** inside the site graph
- **Configurable permalink patterns** for pages and posts
- **Default-language and i18n-aware route generation**
- **Basic translation manager support** for theme and runtime usage

## Taxonomies and content relationships

- **Built-in taxonomy system**
- **Default support for tags and categories**
- **Custom taxonomy definitions** through config
- **Localized taxonomy labels** per language
- **Taxonomy archive generation**
- **Per-term archive pages**
- **Taxonomy-aware dependency tracking** for rebuilds
- **Related content support via plugin hooks and shared taxonomy scoring**

## Rendering and presentation

- **Theme-based rendering system**
- **Separate layouts for base, index, page, post, and list views**
- **Taxonomy archive templates**
- **HTML slot system** for controlled extension points inside themes
- **Context-aware rendering data** passed to templates
- **Menu support in config** for theme navigation
- **Site params support** for theme-level configuration and reusable values
- **Optional HTML minification during builds**
- **Heading ID generation** for Markdown content
- **Built-in support for rendered navigation state**

## Media and asset handling

- **First-class media reference scheme** using `media:` URLs
- **Dedicated collections for images, uploads, and assets**
- **Automatic public URL resolution** from media references
- **Support for images, video, audio, and downloadable files**
- **Markdown media embedding** that renders to the appropriate HTML element
- **Automatic rewriting of media references in rendered HTML**
- **Admin media uploads** with stable reusable references
- **Automatic image-vs-upload classification** based on file type
- **Filename sanitization for uploaded media**
- **Media library in admin**
- **Media detail views in admin**
- **Media thumbnails and previews** for images, video, audio, and files
- **Editable media metadata** including title, alt text, caption, description, credit, and tags
- **Metadata sidecars for media records**
- **Media history, restore, and purge flows**
- **Media replacement** that preserves canonical `media:` references
- **Where-used detection** for media references across documents
- **Media deletion from admin**
- **Asset pipeline for content assets, theme assets, images, uploads, and plugin assets**
- **Bundled CSS output** into a single Foundry stylesheet
- **Safe asset copying with symlink rejection and path validation**

## Preview, rebuilds, and developer experience

- **Incremental rebuild engine** for local serving
- **Dependency graph tracking** across documents, templates, data, and taxonomy outputs
- **Targeted rebuilds** when only affected URLs need to be regenerated
- **Filesystem watch mode** for local development
- **Live reload support** during preview
- **Two live reload transports**: streaming (SSE) and polling
- **Optional browser auto-open** on local serve
- **Preview server diagnostics** and debug output for route and timing inspection
- **Explicit server read, write, and idle timeouts**
- **Frontend search index generation** written to `public/search.json`
- **Weighted frontend search ranking** with normalized snippets in both live and static search payloads
- **Public frontend data artifacts** written under `public/__foundry/` for stable theme-side JS integration
- **Public preview manifest artifacts** written under `public/__foundry/preview.json`
- **Preview build manifest generation** written to `public/preview-links.json`
- **Environment-specific config layering** with `site.<env>.yaml` overlays
- **Named deploy target overrides** for build-time base URL, public dir, theme, preview mode, and related output settings
- **Default deploy-target application** when `deploy.default_target` is set and no `--target` flag is passed

## SEO and syndication

- **RSS feed generation**
- **Sitemap generation**
- **Configurable RSS path, sitemap path, title, description, and item limit**
- **Post-date ordering for feed output**
- **Absolute URL generation from site base URL**
- **SEO configuration surface** for site-wide defaults

## Admin interface

- **Built-in admin UI** served under `/__admin`
- **Themeable admin shell** with swappable admin themes
- **Session-based authentication** by default
- **Optional API token authentication** for automation and tooling
- **Filesystem-backed admin user accounts**
- **Capability-based API access control** with admin, editor, author, and reviewer roles
- **Login, logout, and session endpoints**
- **Password reset, session revocation, and optional TOTP-based 2FA** on the backend auth surface
- **Dedicated admin UI flows** for password reset tokens, TOTP setup/disable, and session revocation
- **Config editing from the admin**
- **Theme listing and theme switching from the admin**
- **Plugin listing plus install, update, rollback, enable, and disable controls from the admin**
- **User listing, create/update, disable, and delete from the admin**
- **Dashboard-style system overview** including content counts, draft counts, users, themes, plugins, and media
- **System status endpoint** with health and configuration visibility
- **Audit log view** for authentication and admin-management activity
- **Official Admin JavaScript SDK** for admin frontends and future plugin-provided admin UI
- **Admin settings-section metadata endpoint** for core and plugin-defined settings groups
- **Plugin admin-extension registry endpoint** for future pages, widgets, slots, and settings sections
- **Plugin-defined admin page bundles** with stable module/style URLs under the admin base path
- **Automatic admin extension-page mounting** through the default admin shell plus `window.FoundryAdmin`
- **Automatic admin widget mounting** in stable default-theme slots through the same extension contract
- **Keyboard shortcuts** for save, preview, shortcut help, and section jumps
- **Command palette** for fast admin navigation and common creation actions
- **Unsaved-change warnings, toasts, breadcrumbs, and better error surfaces**
- **Client-side pagination, filtering, and sorting** across major admin tables
- **Authenticated debug dashboard** with runtime, content, storage, integrity, activity, build-report, and `pprof` visibility

## Security and operational safeguards

- **Local-only admin mode** for development-safe access restrictions
- **Loopback-only enforcement** for local admin access
- **Forwarded-header rejection** when local-only admin is enabled
- **Secure cookie-based browser sessions** scoped to the admin area
- **Persistent session storage** instead of memory-only session state
- **Session TTL controls** with rolling renewal during active use
- **Password hashing with PBKDF2-SHA256**
- **Login throttling** to slow repeated auth attempts
- **CSRF protection** for state-changing cookie-authenticated admin routes
- **Safe path validation** across themes, plugins, content operations, and assets
- **Supply-chain-conscious plugin installation rules** restricted to GitHub sources
- **Optional unsafe HTML toggle** for Markdown rendering when explicitly enabled
- **Validation for broken `media:` references, broken internal links, missing templates, orphaned media, duplicate routes/slugs, and taxonomy inconsistencies**
- **Schema-driven field validation surfaced in preview responses** before publish/save decisions

## Theme system

- **Frontend theme manager**
- **Theme manifests with metadata and compatibility info**
- **Theme manifests with supported layouts, config schema, screenshots, and compatibility version**
- **Theme validation** against Foundry requirements
- **Theme validation diagnostics** for missing layouts/partials, slot completeness, template references, and parse failures
- **Theme scaffolding from the CLI**
- **Theme switching from the CLI and admin**
- **Theme asset support** copied into public output
- **Required slot contract enforcement** for launch-compatible themes
- **Theme-owned layouts, partials, and assets**
- **Admin theme manifests and validation** with a stable component contract for the admin shell
- **Admin theme widget-slot manifests and validation** for plugin widget placement
- **Official Frontend JavaScript SDK** for JS-powered themes and hybrid presentation layers
- **Preview-manifest access in the Frontend SDK** for draft/review preview link discovery

## Platform SDKs and client contracts

- **Shared SDK core** for request handling, error normalization, and capability helpers
- **Official Admin SDK** with session, capabilities, status, documents, media, settings, users, themes, plugins, and audit modules
- **Official Frontend SDK** with site, navigation, routes, content, collections, search, media, and preview modules
- **Capability discovery endpoints** for both admin and frontend clients
- **Live frontend platform API** under `/__foundry/api`
- **Published browser-consumable SDK modules** under `/__foundry/sdk/`
- **Static frontend JSON contract** emitted under `public/__foundry/`
- **Shipped default themes consume the official SDKs** instead of private fetch logic

## Plugin system

- **Hook-based plugin architecture**
- **Compile-time plugin registration model** for predictable deployments
- **Generated plugin import syncing**
- **Plugin metadata manifests**
- **Plugin dependency metadata and config schema support**
- **Plugin-declared admin page/widget/settings metadata** in `plugin.yaml`
- **Plugin lifecycle hooks** spanning config load, content discovery, parsing, graph building, routing, rendering, asset injection, build, server start, and custom CLI commands
- **Plugin validation and dependency checks**
- **Plugin health/diagnostic reporting** surfaced in admin
- **Plugin rollback snapshots** preserved across updates for safer recovery
- **Plugin install, update, uninstall, enable, disable, info, deps, and sync commands**
- **Plugin asset injection into rendered pages**
- **Plugin-defined HTML slot output**
- **Plugin-defined CLI commands**

## Built-in plugins included right now

- **Reading Time** plugin for estimated reading time and word count
- **Table of Contents** plugin for heading extraction and in-page navigation
- **Related Posts** plugin for automatic same-language, same-type related content recommendations

## Data and project structure

- **Structured project layout** for content, config, data, themes, plugins, and public output
- **Data directory support** for external structured data loading
- **Config-driven project behavior** with sensible defaults
- **Filesystem-native operation** that works well with Git-based workflows

## CLI and operations tooling

- **Build command** for static output generation
- **Serve command** for local site serving
- **Serve-preview command** for preview-oriented development
- **Validate command** for config, content, routes, and theme checks
- **Doctor command** for project health checks and timing breakdowns
- **Clean command** for generated artifact cleanup
- **Version command** for build metadata
- **Config check command**
- **Content commands** for linting, listing, graph inspection, generating new pages/posts, import/export, and migration tasks
- **Dry-run migration support** for layout and field-rename content migrations
- **Routes commands** for route inspection
- **Feed commands** for feed generation and inspection
- **Assets commands** for build, list, and clean
- **Theme commands** for list, current, validate, scaffold, and switch
- **Plugin commands** for list, info, install, uninstall, enable, disable, validate, deps, update, and sync
- **Admin commands** for password hashing and sample user generation
- **Debug commands** for routes, plugins, and config output
- **Dependency inspection commands** for rebuild planning visibility
- **i18n commands** for language-related inspection helpers
- **Environment and deploy-target aware builds** through `foundry build --env` and `--target`
- **Preview builds** through `foundry build --preview`
