.PHONY: all build test clean cross-build checksums install

GO ?= $(shell command -v go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
BINARY_NAME=lapdog
BUILD_DIR=bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS=-trimpath -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

all: build test

build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go

test:
	$(GO) test -v ./...

clean:
	rm -rf $(BUILD_DIR)

cross-build:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe main.go
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 main.go

checksums: cross-build
	cd $(BUILD_DIR) && sha256sum lapdog-* > checksums.txt

install: build
	./scripts/install.sh

