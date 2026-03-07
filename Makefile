APP_NAME := foundry
BINARY := bin/$(APP_NAME)
MAIN := ./cmd/cms

GO := go

.PHONY: help
help:
	@echo ""
	@echo "Foundry CMS Development Commands"
	@echo ""
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
	@echo ""

# -------------------------
# Development
# -------------------------

.PHONY: serve
serve:
	$(GO) run $(MAIN) serve

.PHONY: preview
preview:
	$(GO) run $(MAIN) serve-preview

.PHONY: dev
dev:
	air -c .air.toml

.PHONY: dev-preview
dev-preview:
	air -c .air.preview.toml

.PHONY: run
run:
	$(GO) run $(MAIN)

# -------------------------
# Build
# -------------------------

.PHONY: build
build:
	$(GO) run $(MAIN) build

.PHONY: compile
compile:
	mkdir -p bin
	$(GO) build -o $(BINARY) $(MAIN)

# -------------------------
# Code Quality
# -------------------------

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: lint
lint:
	$(GO) vet ./...

.PHONY: test
test:
	$(GO) test ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -rf bin
	rm -rf public
	rm -rf tmp