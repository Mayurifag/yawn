.PHONY: all build install clean fmt lint test coverage generate-config help

# Variables
APP_NAME := yawn
CMD_PATH := ./cmd/$(APP_NAME)
OUTPUT_DIR ?= $(CURDIR)
GOBIN ?= $(HOME)/.local/bin
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Go commands
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOFMT := $(GO) fmt
GOLINT := golangci-lint run
# GOTEST := $(GO) test
GOTOOL := $(GO) tool
GORUN := $(GO) run

# Default target
all: build

# Build the application
build: fmt lint
	@echo "==> Building $(APP_NAME) $(VERSION)..."
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME) $(CMD_PATH)

# Install the application to GOBIN
install: fmt lint
	@echo "==> Installing $(APP_NAME) to $(GOBIN)..."
	@mkdir -p $(GOBIN)
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(GOBIN)/$(APP_NAME) $(CMD_PATH)
	@echo "$(APP_NAME) installed to $(GOBIN)/$(APP_NAME)"

# Clean build artifacts
clean:
	@echo "==> Cleaning..."
	$(GOCLEAN)
	rm -f $(OUTPUT_DIR)/$(APP_NAME) coverage.*

# Format code
fmt:
	@echo "==> Formatting code..."
	$(GOFMT) ./...

# Lint code
lint:
	@echo "==> Linting code..."
	$(GOLINT) ./...

# # Run tests
# test:
# 	@echo "==> Running tests..."
# 	$(GOTEST) -race -covermode=atomic ./...

# Run tests with coverage report
# coverage:
# 	@echo "==> Running tests with coverage..."
# 	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
# 	@echo "==> Opening coverage report..."
# 	$(GOTOOL) cover -html=coverage.out

# Generate default configuration file
generate-config:
	@echo "==> Generating default .yawn.toml configuration file..."
	$(GORUN) $(CMD_PATH) --generate-config > .yawn.toml
	@echo "Default config written to .yawn.toml"

# Show help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all              Build the application (default)"
	@echo "  build            Build the application binary"
	@echo "  install          Build and install the application to $(GOBIN)"
	@echo "  clean            Remove build artifacts"
	@echo "  fmt              Format Go source code"
	@echo "  lint             Run golangci-lint"
	@echo "  test             Run tests"
	@echo "  coverage         Run tests and show coverage report"
	@echo "  generate-config  Generate a default .yawn.toml configuration file"
	@echo "  help             Show this help message"
