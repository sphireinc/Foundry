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
- **Save content back to the filesystem** with validation and safe path handling
- **Document listing with filters** for drafts, type, language, and text query
- **Status switching** between published, draft, and archived
- **Document history, diff, restore, and purge flows** in the admin
- **Trash and history views** for editorial recovery workflows
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
- **Role-aware API access** with admin and editor scopes
- **Login, logout, and session endpoints**
- **Config editing from the admin**
- **Theme listing and theme switching from the admin**
- **Plugin listing and enable/disable controls from the admin**
- **User listing, create/update, disable, and delete from the admin**
- **Dashboard-style system overview** including content counts, draft counts, users, themes, plugins, and media
- **System status endpoint** with health and configuration visibility
- **Audit log view** for authentication and admin-management activity
- **Keyboard shortcuts** for save, preview, shortcut help, and section jumps
- **Unsaved-change warnings, toasts, breadcrumbs, and better error surfaces**
- **Client-side pagination, filtering, and sorting** across major admin tables

## Security and operational safeguards

- **Local-only admin mode** for development-safe access restrictions
- **Loopback-only enforcement** for local admin access
- **Forwarded-header rejection** when local-only admin is enabled
- **Secure cookie-based browser sessions** scoped to the admin area
- **Session TTL controls** with rolling renewal during active use
- **Password hashing with PBKDF2-SHA256**
- **Login throttling** to slow repeated auth attempts
- **Safe path validation** across themes, plugins, content operations, and assets
- **Supply-chain-conscious plugin installation rules** restricted to GitHub sources
- **Optional unsafe HTML toggle** for Markdown rendering when explicitly enabled
- **Validation for broken `media:` references and broken internal links**

## Theme system

- **Frontend theme manager**
- **Theme manifests with metadata and compatibility info**
- **Theme validation** against Foundry requirements
- **Theme scaffolding from the CLI**
- **Theme switching from the CLI and admin**
- **Theme asset support** copied into public output
- **Required slot contract enforcement** for launch-compatible themes
- **Theme-owned layouts, partials, and assets**

## Plugin system

- **Hook-based plugin architecture**
- **Compile-time plugin registration model** for predictable deployments
- **Generated plugin import syncing**
- **Plugin metadata manifests**
- **Plugin lifecycle hooks** spanning config load, content discovery, parsing, graph building, routing, rendering, asset injection, build, server start, and custom CLI commands
- **Plugin validation and dependency checks**
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
- **Doctor command** for project health checks
- **Clean command** for generated artifact cleanup
- **Version command** for build metadata
- **Config check command**
- **Content commands** for linting, listing, graph inspection, and generating new pages/posts
- **Routes commands** for route inspection
- **Feed commands** for feed generation and inspection
- **Assets commands** for build, list, and clean
- **Theme commands** for list, current, validate, scaffold, and switch
- **Plugin commands** for list, info, install, uninstall, enable, disable, validate, deps, update, and sync
- **Admin commands** for password hashing and sample user generation
- **Debug commands** for routes, plugins, and config output
- **Dependency inspection commands** for rebuild planning visibility
- **i18n commands** for language-related inspection helpers

## What this adds up to

Foundry already ships as a real CMS, not just a site generator. It gives you file-based content, a browser admin, multilingual routing, taxonomies, themes, plugins, media management, live preview, incremental rebuilds, and deployment-friendly static output - all in a Go-native architecture designed for teams that want control, transparency, and extensibility.
