.PHONY: all build install install-release uninstall clean ci fmt lint test coverage release rerelease _create_tag

# Variables
APP_NAME := yawn
INSTALL_NAME := yawn-debug
CMD_PATH := ./cmd/$(APP_NAME)
# Append .exe on Windows
ifeq ($(OS),Windows_NT)
INSTALL_NAME := $(INSTALL_NAME).exe
APP_NAME := $(APP_NAME).exe
endif
OUTPUT_DIR ?= $(CURDIR)
GOBIN ?= $(HOME)/.local/bin
# Normalize Windows-style paths (e.g. from GOBIN env var set by mise on Windows)
override GOBIN := $(shell cygpath -u '$(GOBIN)' 2>/dev/null || echo '$(GOBIN)')
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
RELEASE_WAIT_SECONDS ?= 600
RELEASE_WAIT_INTERVAL ?= 10

# Go commands
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOFMT := $(GO) fmt
GOLINT := $(GO) tool golangci-lint run
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

# Install latest release via mise
install-release:
	@echo "==> Installing latest release of yawn via mise..."
	@set -e; \
	current_tag=$$(git describe --tags --abbrev=0 2>/dev/null || true); \
	if [ -z "$$current_tag" ]; then \
		echo "Error: no release tag found"; \
		exit 1; \
	fi; \
	version=$${current_tag#v}; \
	elapsed=0; \
	echo "==> Waiting for GitHub release assets for $$current_tag..."; \
	until curl -fsSL "https://api.github.com/repos/Mayurifag/yawn/releases/tags/$$current_tag" | grep -q '"browser_download_url"'; do \
		if [ $$elapsed -ge $(RELEASE_WAIT_SECONDS) ]; then \
			echo "Error: release assets for $$current_tag were not available after $(RELEASE_WAIT_SECONDS)s"; \
			exit 1; \
		fi; \
		sleep $(RELEASE_WAIT_INTERVAL); \
		elapsed=$$((elapsed + $(RELEASE_WAIT_INTERVAL))); \
	done; \
	config_file=$$(chezmoi source-path "$(HOME)/.config/mise/config.toml"); \
	if [ ! -f "$$config_file" ]; then \
		echo "Error: $$config_file not found"; \
		exit 1; \
	fi; \
	perl -0pi -e 's/("github:Mayurifag\/yawn"\s*=\s*")[^"]+(")/$${1}'"$$version"'$${2}/' "$$config_file"; \
	chezmoi apply "$(HOME)/.config/mise/config.toml"; \
	mise cache clear; \
	mise install github:Mayurifag/yawn@$$version
	@"$(MAKE)" --no-print-directory uninstall

# Uninstall the application from GOBIN
uninstall:
	@echo "==> Uninstalling yawn-debug from $(GOBIN)..."
	@rm -f "$(GOBIN)/yawn-debug" "$(GOBIN)/yawn-debug.exe"
	mise reshim

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
	@set -e; \
	current_tag=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$current_tag | cut -d. -f1 | tr -d v); \
	minor=$$(echo $$current_tag | cut -d. -f2); \
	patch=$$(echo $$current_tag | cut -d. -f3); \
	patch=$$((patch + 1)); \
	new_tag="v$$major.$$minor.$$patch"; \
	echo "Current tag: $$current_tag, New tag: $$new_tag"; \
	"$(MAKE)" _create_tag TAG=$$new_tag; \
	printf "Run make install-release now? [y/N] "; \
	read answer || answer=; \
	case "$$answer" in \
		[Yy]|[Yy][Ee][Ss]) "$(MAKE)" install-release ;; \
		*) echo "Skipping install-release." ;; \
	esac

# Re-release - remove previous tag and create new release
rerelease:
	@echo "==> Re-releasing (removing previous tag and creating new one)..."
	@current_tag=$$(git describe --tags --abbrev=0 2>/dev/null); \
	if [ -z "$$current_tag" ]; then \
		echo "No previous tag found, creating initial release..."; \
		"$(MAKE)" release; \
	else \
		echo "Removing previous tag: $$current_tag"; \
		git tag -d $$current_tag 2>/dev/null || true; \
		git push origin :refs/tags/$$current_tag 2>/dev/null || true; \
		echo "Re-creating the same tag: $$current_tag"; \
		"$(MAKE)" _create_tag TAG=$$current_tag; \
	fi
