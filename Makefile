.PHONY: all build install uninstall clean ci fmt lint test coverage release rerelease _create_tag

# Variables
APP_NAME := yawn
INSTALL_NAME := yawn-debug
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
	@echo "==> Installing $(INSTALL_NAME) to $(GOBIN)..."
	@mkdir -p $(GOBIN)
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(GOBIN)/$(INSTALL_NAME) $(CMD_PATH)

# Uninstall the application from GOBIN
uninstall:
	@echo "==> Uninstalling $(INSTALL_NAME) from $(GOBIN)..."
	@if [ -f "$(GOBIN)/$(INSTALL_NAME)" ]; then \
		rm -f $(GOBIN)/$(INSTALL_NAME) && \
		echo "$(INSTALL_NAME) successfully removed from $(GOBIN)"; \
	else \
		echo "$(INSTALL_NAME) not found in $(GOBIN)"; \
	fi

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
	$(GOTEST) ./...

# Run tests with coverage report
coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOTOOL) cover -html=coverage.out

# Internal function to create and push a tag
_create_tag:
	@if [ -z "$(TAG)" ]; then \
		echo "Error: TAG variable is required"; \
		exit 1; \
	fi
	@echo "Creating tag: $(TAG)"
	@git tag -a $(TAG) -m "Release $(TAG)"
	@git push origin $(TAG)

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
	$(MAKE) _create_tag TAG=$$new_tag

# Re-release - remove previous tag and create new release
rerelease:
	@echo "==> Re-releasing (removing previous tag and creating new one)..."
	@current_tag=$$(git describe --tags --abbrev=0 2>/dev/null); \
	if [ -z "$$current_tag" ]; then \
		echo "No previous tag found, creating initial release..."; \
		$(MAKE) release; \
	else \
		echo "Removing previous tag: $$current_tag"; \
		git tag -d $$current_tag 2>/dev/null || true; \
		git push origin :refs/tags/$$current_tag 2>/dev/null || true; \
		echo "Re-creating the same tag: $$current_tag"; \
		$(MAKE) _create_tag TAG=$$current_tag; \
	fi
