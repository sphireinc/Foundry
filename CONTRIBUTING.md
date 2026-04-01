# Contributing to Foundry

Thanks for contributing to Foundry.

This guide is intentionally practical. It focuses on how this repository actually works today: Go for the core CMS, file-based content and themes, JS/CSS/HTML assets under `themes/`, the SDK under `sdk/`, and CI that expects formatting, plugin sync, buildability, and tests to pass.

## Before you start

- Use Go `1.25`
- Install Node.js if you need to format web assets and docs
- Work from the repository root
- Prefer small, focused changes over broad mixed refactors

If you are changing core behavior, open an issue first unless the fix is obviously small, then just go for PR.

## Repository shape

Important directories:

- `cmd/foundry`: main CLI entrypoints
- `cmd/plugin-sync`: regenerates plugin import wiring
- `internal/`: Go application code - this is the CORE of the cms
- `themes/`: frontend themes and the default admin theme
- `plugins/`: built-in plugins
- `sdk/`: official browser SDK modules
- `content/`: example/default project content and config
- `data/`: structured data and admin runtime state
- `docs/`: published docs and coverage output

## Development setup

### Native Go workflow

Install dependencies:

```bash
go mod download
npm install (only needed for frontend/themes development)
```

Common helpeful commands:

```bash
make plugins-sync
make serve
make preview
make build
make test
make lint
make fmt
make fmt-web
make fmt-all
```

Notes:

- `make serve`, `make preview`, `make build`, and `make test` all rely on current geneerated plugin imports.
- If you change plugin registration or plugin config wiring, run `make plugins-sync`.

### Docker workflow

For containerized local development:

```bash
docker compose up -d --build
```

Important detail:

- the repo is bind-mounted into the container
- runtime-writable directories `data/` and `public/` are kept on named Docker volumes

That is required so admin sessions, audit/runtime files, and generated output stay writable inside the container.

If you change Go code, rebuild the image:

```bash
docker compose down
docker compose up -d --build
```

Do not assume a running container is using your latest Go backend changes just because the repo is mounted.

For development, prefer `go run ./cmd/foundry serve` with live-reload turned on - this ensures you are on the latest
build relative to your local branch and ensures no docker fuckery.

## Coding standards

### Go

- run `gofmt`
- keep code idiomatic and direct
- prefer narrow changes over speculative abstraction
- keep errors actionable
- add tests for non-trivial behavior changes

### JS, CSS, HTML, Markdown

- format with Prettier
- keep admin/theme code modular and scoped
- do not break the default admin theme without updating related views/events/state wiring

### Themes and plugins

- do not hardcode theme/plugin assumptions into unrelated core code unless the contract genuinely requires it
- preserve theme/plugin install, validation, and runtime behavior where possible
- if you add contract requirements, update validation and docs in the same change

## Tests and validation

Before opening a PR, run the relevant checks locally.

Minimum expected for most code changes:

```bash
make plugins-sync
make fmt
make fmt-web
make lint
make test
```

For release/update work, also verify:

```bash
go fmt ./...
go test ./...
go vet ./...
```

For admin UI changes, at minimum syntax-check the edited modules if you are not running the full app:

```bash
node --check themes/admin-themes/default/assets/admin.js
```

If you touched multiple admin files, check those specific files too.

## Documentation expectations

Update docs when you change:

- CLI behavior
- config keys or defaults
- Docker/runtime behavior
- backup/update/service workflows
- theme or plugin contracts
- release process

Common files that often need updates:

- `README.md`
- `docs/getting-started/index.html`
- `docs/config/index.html`
- theme/plugin READMEs when applicable

## Releases

Foundry uses semver-style tags in `vX.Y.Z` format.

To cut a release tag:

```bash
foundry release cut v1.3.3
```

To cut and push in one step:

```bash
foundry release cut v1.3.3 --push
```

Or use:

```bash
scripts/release.sh v1.3.3 --push
```

Pushing the tag triggers the GitHub release workflow, which builds and uploads release archives and checksum files.

## Pull requests

A good PR should:

- explain what changed
- explain why it changed
- note any user-facing impact
- mention config, runtime, or migration implications
- include tests or explain why tests were not added

Please keep PRs focused. Avoid combining unrelated refactors with bug fixes.

## Issue reports

Good issue reports include:

- Foundry version
- how Foundry was run:
  - `foundry serve`
  - `serve-standalone`
  - `service install`
  - Docker
- OS/environment
- exact steps to reproduce
- logs or screenshots
- whether the bug reproduces on a fresh clone or fresh Docker compose run

For Docker issues, include:

- `docker compose.yml` changes if any
- whether you rebuilt with `docker compose up -d --build`

## Contributor guidelines

- Do not commit secrets
- Do not check in accidental runtime state unless the change is intentional
- Be careful with generated files and verify whether they belong in the diff
- Preserve backwards compatibility where reasonable, especially for:
  - config
  - admin APIs
  - SDK shape
  - theme/plugin contracts

## When in doubt

If you are unsure whether a change belongs in core, theme, plugin, SDK, or docs:

- choose the narrowest layer that can own the behavior cleanly
- document the contract if the change crosses boundaries
- ask in an issue or draft PR before going broad
