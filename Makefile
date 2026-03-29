BINARY     := knet
MODULE     := github.com/themark007/knew
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X $(MODULE)/cmd.Version=$(VERSION) \
  -X $(MODULE)/cmd.Commit=$(COMMIT) \
  -X $(MODULE)/cmd.BuildDate=$(BUILD_DATE)

GOTESTFLAGS ?= -v -race -count=1
INSTALL_DIR ?= /usr/local/bin

.PHONY: all build install uninstall test lint fmt vet tidy \
        snapshot release clean help

all: build

## build: compile the binary for the current platform
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## install: build and copy binary to INSTALL_DIR (default /usr/local/bin)
install: build
	@if [ ! -w "$(INSTALL_DIR)" ]; then \
		echo "→ Installing with sudo to $(INSTALL_DIR)"; \
		sudo install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY); \
	else \
		install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY); \
	fi
	@echo "✓ Installed $(INSTALL_DIR)/$(BINARY)"

## uninstall: remove the installed binary
uninstall:
	@if [ -f "$(INSTALL_DIR)/$(BINARY)" ]; then \
		if [ ! -w "$(INSTALL_DIR)" ]; then \
			sudo rm -f $(INSTALL_DIR)/$(BINARY); \
		else \
			rm -f $(INSTALL_DIR)/$(BINARY); \
		fi; \
		echo "✓ Removed $(INSTALL_DIR)/$(BINARY)"; \
	else \
		echo "$(INSTALL_DIR)/$(BINARY) not found"; \
	fi

## test: run all unit tests
test:
	go test $(GOTESTFLAGS) ./...

## lint: run golangci-lint (must be installed separately)
lint:
	golangci-lint run ./...

## fmt: gofmt all source files
fmt:
	gofmt -w -s .

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy go modules
tidy:
	go mod tidy

## snapshot: build multi-platform binaries locally (no release)
snapshot:
	goreleaser build --snapshot --clean

## release: create and publish a GitHub release (requires GITHUB_TOKEN)
release:
	goreleaser release --clean

## clean: remove build artifacts
clean:
	rm -f $(BINARY) dist/

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## /  /'
