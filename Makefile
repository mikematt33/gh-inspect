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
	@if [ ! -f ./bin/golangci-lint ]; then \
		echo "Installing golangci-lint to ./bin..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./bin v1.63.4; \
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
