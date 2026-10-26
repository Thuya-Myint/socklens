GO ?= go
GOFMT ?= gofmt
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
DIST ?= dist
LDFLAGS = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: help build test check install release

help:
	@printf '%s\n' 'make build    Build bin/socklens with version metadata' 'make test     Run tests with the race detector' 'make check    Check formatting and run go vet' 'make install  Build and install into ~/.local/bin' 'make release  Package Linux amd64/arm64 archives and SHA256SUMS'

build:
	@mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/socklens ./cmd/socklens

test:
	$(GO) test -race ./...

check:
	sh -n install.sh
	@files="$$($(GOFMT) -l cmd internal tools)" && test -z "$$files"
	$(GO) vet ./...

install: build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 0755 bin/socklens "$(DESTDIR)$(BINDIR)/socklens"

release:
	$(GO) run ./tools/release --go "$(GO)" --version "$(VERSION)" --commit "$(COMMIT)" --date "$(BUILD_DATE)" --out "$(DIST)"
