# Makefile for PDF to Markdown MCP Server

# Build configuration
BINARY_NAME=pdf-md-mcp
BUILD_DIR=./bin
MAIN_PACKAGE=.

# Go build flags
GOFLAGS=-ldflags="-s -w"
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build: clean
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)

# Install dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Run the server (requires configuration)
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run

# Create example environment file
.PHONY: config
config:
	@if [ ! -f .env ]; then \
		echo "Creating .env from pdf_md_mcp.env..."; \
		cp pdf_md_mcp.env .env; \
		echo "Please edit .env with your specific configuration."; \
	else \
		echo ".env file already exists."; \
	fi

# Development setup
.PHONY: setup
setup: deps config
	@echo "Development setup complete!"
	@echo "Next steps:"
	@echo "1. Edit .env file with your configuration"
	@echo "2. Run 'make build' to build the server"
	@echo "3. Run 'make run' to start the server"

# Cross-compilation targets
.PHONY: build-linux
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

.PHONY: build-windows
build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

.PHONY: build-darwin
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

.PHONY: build-all
build-all: build-linux build-windows build-darwin
	@echo "Cross-compilation complete!"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary for current platform"
	@echo "  clean       - Remove build artifacts"
	@echo "  deps        - Download and tidy dependencies"
	@echo "  run         - Build and run the server"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format Go code"
	@echo "  lint        - Run linter"
	@echo "  config      - Create .env configuration file"
	@echo "  setup       - Complete development setup"
	@echo "  build-all   - Cross-compile for all platforms"
	@echo "  help        - Show this help message"