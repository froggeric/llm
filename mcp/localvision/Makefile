# Makefile for localvision.
#
# This module lives in a subdirectory of a larger monorepo. Run all targets
# from this directory:
#
#	cd mcp/localvision && make <target>
#
# Variables can be overridden from the environment. The version metadata is
# injected via -ldflags so that `localvision version` reports the same
# information that was baked in at release time.

SHELL            := /usr/bin/env bash
GO               ?= go
GORELEASER       ?= goreleaser

MODULE           := github.com/froggeric/llm/mcp/localvision
VERSION_PKG      := $(MODULE)/internal/version

# Build-time metadata. Defaults produce an obviously-dev build; goreleaser
# overrides these via env vars at release time.
VERSION          ?= dev
GIT_COMMIT       ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE       ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS          := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).GitCommit=$(GIT_COMMIT) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)

# Detect Windows when running under MinGW/Cygwin/MSYS so `make build` produces
# a `.exe` even on non-Windows hosts when GOOS=windows is set.
BIN_NAME         := localvision
ifeq ($(OS),Windows_NT)
	BIN_NAME := $(BIN_NAME).exe
endif

# Allow GOOS/GOARCH overrides for cross builds.
bin_dir          := bin
dist_dir         := dist
bin_path         := $(bin_dir)/$(BIN_NAME)

.PHONY: all build test test-race lint vet clean install release-snapshot release doctor version dist-install-sh help

all: build

## build: compile the binary into ./bin/localvision
build:
	@mkdir -p $(bin_dir)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(bin_path) ./cmd/localvision
	@echo "built $(bin_path)"

## test: run the unit test suite
test:
	$(GO) test ./...

## test-race: run unit tests with the race detector enabled
test-race:
	$(GO) test -race ./...

## vet: go vet across the whole module
vet:
	$(GO) vet ./...

## lint: run golangci-lint if installed, otherwise fall back to go vet
lint: vet
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "note: golangci-lint not installed; falling back to 'go vet'"; \
		exit 0; \
	}
	golangci-lint run ./...

## clean: remove build artifacts (bin/ and dist/)
clean:
	rm -rf $(bin_dir) $(dist_dir)

## install: install the binary into $$GOPATH/bin using the current ldflags
install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" ./cmd/localvision

## dist-install-sh: stage scripts/install.sh at dist/install.sh for local testing
# The source of truth is scripts/install.sh; this target lets you run
# `bash dist/install.sh --dry-run` without doing a full release, because
# goreleaser wipes dist/ on every run.
dist-install-sh:
	@mkdir -p $(dist_dir)
	cp scripts/install.sh $(dist_dir)/install.sh
	chmod +x $(dist_dir)/install.sh
	@echo "staged $(dist_dir)/install.sh (run with: bash $(dist_dir)/install.sh --dry-run)"

## release-snapshot: run goreleaser as a local snapshot (no tag/push required)
release-snapshot: dist-install-sh
	$(GORELEASER) release --snapshot --clean
	# goreleaser wipes dist/, so re-stage install.sh after the snapshot.
	@cp scripts/install.sh $(dist_dir)/install.sh && chmod +x $(dist_dir)/install.sh
	@echo "re-staged $(dist_dir)/install.sh"

## release: run a real goreleaser release (requires a git tag + GITHUB_TOKEN)
release:
	$(GORELEASER) release --clean

## doctor: print build metadata and a few quick environment diagnostics
doctor:
	@echo "localvision build doctor"
	@echo "  go version     : $(shell $(GO) version)"
	@echo "  module         : $(MODULE)"
	@echo "  version        : $(VERSION)"
	@echo "  git commit     : $(GIT_COMMIT)"
	@echo "  build date     : $(BUILD_DATE)"
	@echo "  GOOS/GOARCH    : $(shell $(GO) env GOOS)/$(shell $(GO) env GOARCH)"
	@echo "  goreleaser     : $(shell command -v $(GORELEASER) 2>/dev/null || echo '(not installed)')"

## version: print the version string that would be stamped into a build
version:
	@echo "$(VERSION) (commit $(GIT_COMMIT), built $(BUILD_DATE))"

## help: list the available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //; s/: /	/' | sort
