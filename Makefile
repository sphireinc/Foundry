APP_NAME=foundry
BINARY=bin/$(APP_NAME)
MAIN=./cmd/foundry
PLUGIN_SYNC=./cmd/plugin-sync

GO=go

.PHONY: help
help:
	@echo ""
	@echo "Foundry CMS Development Commands"
	@echo ""
	@echo "make plugins-sync    Generate plugin imports from site config"
	@echo "make serve           Start dev server"
	@echo "make preview         Start dev server with drafts"
	@echo "make dev             Start hot-reload Go development server with air"
	@echo "make dev-preview     Start hot-reload preview server with air"
	@echo "make build           Build static site"
	@echo "make run             Run CMS binary"
	@echo "make compile         Compile binary"
	@echo "make clean           Remove build artifacts"
	@echo "make tidy            Run go mod tidy"
	@echo "make test            Run tests"
	@echo "make lint            Run go vet"
	@echo "make fmt             Format Go code"
	@echo "make fmt-web         Format JS/CSS/HTML/Markdown with Prettier"
	@echo "make fmt-all         Format Go and web/docs assets"
	@echo ""

.PHONY: plugins-sync
plugins-sync:
	$(GO) run $(PLUGIN_SYNC)

.PHONY: serve
serve: plugins-sync
	$(GO) run $(MAIN) serve

# -------------------------
# Development
# -------------------------

.PHONY: preview
preview: plugins-sync
	$(GO) run $(MAIN) serve-preview

.PHONY: dev
dev:
	air -c .air.toml

.PHONY: dev-preview
dev-preview:
	air -c .air.preview.toml

.PHONY: run
run: plugins-sync
	$(GO) run $(MAIN)

# -------------------------
# Build
# -------------------------

.PHONY: build
build: plugins-sync
	$(GO) run $(MAIN) build

.PHONY: compile
compile: plugins-sync
	mkdir -p bin
	$(GO) build -o $(BINARY) $(MAIN)

.PHONY: release
release:
	go run ./scripts/build-release.go

# -------------------------
# Code Quality
# -------------------------

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: fmt-web
fmt-web:
	npm run fmt:web

.PHONY: fmt-all
fmt-all: fmt fmt-web

.PHONY: lint
lint:
	$(GO) vet ./...

.PHONY: test
test: plugins-sync
	$(GO) test ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -rf bin
	rm -rf public
	rm -rf tmp
