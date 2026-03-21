BINARY_NAME=gh-inspect
BUILD_DIR=bin
MAIN_PATH=cmd/gh-inspect/main.go
# Get version from git, default to "dev" if no git
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X 'github.com/mikematt33/gh-inspect/internal/cli.Version=$(VERSION)'"

.PHONY: all build clean test vet fmt lint run-help

all: clean fmt vet lint test build

build:
	@mkdir -p $(BUILD_DIR)
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete. Binary located at $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning build directory..."
	rm -rf $(BUILD_DIR)

test:
	@echo "Running tests..."
	go test ./...

lint:
	@echo "Running linters..."
	@mkdir -p ./bin
	@need_install=0; \
	if [ ! -x ./bin/golangci-lint ]; then \
		need_install=1; \
	else \
		lint_go_ver=$$(./bin/golangci-lint version 2>/dev/null | sed -n 's/.*built with go\([0-9]*\.[0-9]*\).*/\1/p'); \
		host_go_ver=$$(go env GOVERSION | sed -n 's/go\([0-9]*\.[0-9]*\).*/\1/p'); \
		if [ -z "$$lint_go_ver" ] || [ "$$lint_go_ver" != "$$host_go_ver" ]; then \
			need_install=1; \
		fi; \
	fi; \
	if [ $$need_install -eq 1 ]; then \
		echo "Installing golangci-lint to ./bin with local Go toolchain..."; \
		GOBIN=$$(pwd)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
	fi
	@./bin/golangci-lint run

vet:
	@echo "Running go vet..."
	go vet ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

# Helper Targets
run-help: build
	./$(BUILD_DIR)/$(BINARY_NAME) --help

# Run a quick analysis on the tool's own repo
run-self: build
	./$(BUILD_DIR)/$(BINARY_NAME) run mikematt33/gh-inspect

# Run an analysis on a target org (first arg) in deep mode
# Usage: make run-org ORG=cli
run-org: build
	./$(BUILD_DIR)/$(BINARY_NAME) org $(ORG) --deep
