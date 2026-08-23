.PHONY: all build test clean cross-build install

GO ?= $(shell command -v go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
BINARY_NAME=lapdog
BUILD_DIR=bin

all: build test

build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) main.go

test:
	$(GO) test -v ./...

clean:
	rm -rf $(BUILD_DIR)

cross-build:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 main.go

install: build
	./scripts/install.sh
