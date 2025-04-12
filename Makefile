.PHONY: all build install clean ci fmt lint test coverage release

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
GOTEST := $(GO) test
GOTOOL := $(GO) tool
GORUN := $(GO) run

# Default target
all: build

# Build the application
build: ci
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME) $(CMD_PATH)

# Install the application to GOBIN
install: ci
	@echo "==> Installing $(APP_NAME) to $(GOBIN)..."
	@mkdir -p $(GOBIN)
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(GOBIN)/$(APP_NAME) $(CMD_PATH)

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(OUTPUT_DIR)/$(APP_NAME) coverage.*

ci: fmt lint test

fmt:
	$(GOFMT) ./...

# Lint code
lint:
	$(GOLINT) ./...

# Run tests
test:
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...

# Run tests with coverage report
coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOTOOL) cover -html=coverage.out

# Release - bump version and push new tag
release:
	@echo "==> Creating new release..."
	@current_tag=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$current_tag | cut -d. -f1 | tr -d v); \
	minor=$$(echo $$current_tag | cut -d. -f2); \
	patch=$$(echo $$current_tag | cut -d. -f3); \
	patch=$$((patch + 1)); \
	new_tag="v$$major.$$minor.$$patch"; \
	echo "Current tag: $$current_tag, New tag: $$new_tag"; \
	git tag -a $$new_tag -m "Release $$new_tag"; \
	git push origin $$new_tag
