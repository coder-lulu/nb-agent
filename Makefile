# NewBee Agent Makefile
# Build configuration for NewBee Agent server management platform

# Variables
SHELL := /bin/bash
APP_NAME := nb-agent
CLI_NAME := nb-cli
MODULE := github.com/coder-lulu/nb-agent
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo '0.0.0')
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')
GO_VERSION := $(shell go version | cut -d' ' -f3)
BUILD_ID := $(shell echo $$RANDOM)
BUILD_USER := $(shell whoami)
BUILD_HOST := $(shell hostname)

# Build flags
LDFLAGS := -s -w --extldflags "-static"
LDFLAGS += -X '$(MODULE)/internal/app.Version=$(VERSION)'
LDFLAGS += -X '$(MODULE)/internal/app.BuildTime=$(BUILD_TIME)'
LDFLAGS += -X '$(MODULE)/internal/app.CommitHash=$(COMMIT_HASH)'
LDFLAGS += -X '$(MODULE)/internal/app.GoVersion=$(GO_VERSION)'
LDFLAGS += -X '$(MODULE)/internal/app.BuildID=$(BUILD_ID)'
LDFLAGS += -X '$(MODULE)/internal/app.BuildUser=$(BUILD_USER)'
LDFLAGS += -X '$(MODULE)/internal/app.BuildHost=$(BUILD_HOST)'

# Build environment
export CGO_ENABLED := 0
export GOOS := linux
export GOARCH := amd64

# Default target
.DEFAULT_GOAL := help

# Help target
.PHONY: help
help: ## Show this help message
	@echo "NewBee Agent Build System"
	@echo "========================="
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Build info:"
	@echo "  Version:     $(VERSION)"
	@echo "  Go version:  $(GO_VERSION)"
	@echo "  Build time:  $(BUILD_TIME)"
	@echo "  Commit:      $(COMMIT_HASH)"

# Build targets
.PHONY: agent
agent: ## Build the main NewBee Agent server binary
	@echo "Building NewBee Agent server..."
	@echo "Version: $(VERSION)"
	@echo "Target: $(GOOS)/$(GOARCH)"
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) ./cmd/nb
	@echo "✅ Built $(APP_NAME) successfully!"

.PHONY: cli
cli: ## Build the NewBee Agent CLI tool binary
	@echo "Building NewBee Agent CLI..."
	@echo "Version: $(VERSION)"
	@echo "Target: $(GOOS)/$(GOARCH)"
	go build -ldflags "$(LDFLAGS)" -o $(CLI_NAME) ./cmd/cli
	@echo "✅ Built $(CLI_NAME) successfully!"

.PHONY: all
all: agent cli ## Build both agent and cli binaries
	@echo "✅ All binaries built successfully!"

# Cross-platform builds
.PHONY: agent-amd64
agent-amd64: ## Build agent for Linux AMD64
	@$(MAKE) agent GOARCH=amd64 APP_NAME=$(APP_NAME)-amd64

.PHONY: agent-arm64
agent-arm64: ## Build agent for Linux ARM64
	@$(MAKE) agent GOARCH=arm64 APP_NAME=$(APP_NAME)-arm64

.PHONY: cli-amd64
cli-amd64: ## Build CLI for Linux AMD64
	@$(MAKE) cli GOARCH=amd64 CLI_NAME=$(CLI_NAME)-amd64

.PHONY: cli-arm64
cli-arm64: ## Build CLI for Linux ARM64
	@$(MAKE) cli GOARCH=arm64 CLI_NAME=$(CLI_NAME)-arm64

.PHONY: build-all-arch
build-all-arch: agent-amd64 agent-arm64 cli-amd64 cli-arm64 ## Build for all architectures
	@echo "✅ All architectures built successfully!"

# Development targets
.PHONY: dev
dev: ## Build for development (with race detector)
	@echo "Building for development..."
	CGO_ENABLED=1 go build -race -ldflags "$(LDFLAGS)" -o $(APP_NAME)-dev ./cmd/nb
	CGO_ENABLED=1 go build -race -ldflags "$(LDFLAGS)" -o $(CLI_NAME)-dev ./cmd/cli
	@echo "✅ Development builds completed!"

.PHONY: run
run: agent ## Build and run the agent
	@echo "Starting NewBee Agent..."
	@if [ "$$(id -u)" -ne 0 ]; then \
		echo "⚠️  Warning: NewBee Agent requires root privileges"; \
		echo "Please run: sudo make run"; \
		exit 1; \
	fi
	./$(APP_NAME)

# Dependency and module management
.PHONY: deps
deps: ## Download and tidy Go modules
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies updated!"

.PHONY: deps-update
deps-update: ## Update all dependencies to latest versions
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy
	@echo "✅ Dependencies updated to latest versions!"

# Code quality targets
.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted!"

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...
	@echo "✅ Vet completed!"

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "✅ Linting completed!"; \
	else \
		echo "⚠️  golangci-lint not found. Install it with:"; \
		echo "    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Test targets
.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	go test -v ./...
	@echo "✅ Tests completed!"

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	go test -race -v ./...
	@echo "✅ Race tests completed!"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Frontend targets
.PHONY: frontend
frontend: ## Build frontend (requires Node.js and pnpm)
	@echo "Building frontend..."
	@if [ -d "web" ]; then \
		cd web && \
		cp .env.production .env && \
		pnpm install && \
		pnpm run gettext:compile && \
		pnpm build && \
		echo "✅ Frontend built successfully!"; \
	else \
		echo "⚠️  Frontend directory not found"; \
	fi

# Release targets
.PHONY: release
release: ## Build release binaries with compression
	@echo "Building release binaries..."
	@$(MAKE) build-all-arch
	@echo "Compressing binaries..."
	@if command -v upx >/dev/null 2>&1; then \
		upx --best --lzma $(APP_NAME)-amd64 $(APP_NAME)-arm64 $(CLI_NAME)-amd64 $(CLI_NAME)-arm64; \
		echo "✅ Release binaries compressed!"; \
	else \
		echo "⚠️  UPX not found. Binaries not compressed."; \
		echo "    Install UPX for smaller binaries: apt-get install upx"; \
	fi

# Cleanup targets
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f $(APP_NAME) $(CLI_NAME)
	rm -f $(APP_NAME)-* $(CLI_NAME)-*
	rm -f coverage.out coverage.html
	rm -rf dist/
	@echo "✅ Cleaned!"

.PHONY: clean-all
clean-all: clean ## Clean everything including dependencies
	@echo "Cleaning dependencies..."
	go clean -modcache
	@echo "✅ Everything cleaned!"

# Docker targets (optional)
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@if [ -f "Dockerfile" ]; then \
		docker build -t newbee-agent:$(VERSION) -t newbee-agent:latest .; \
		echo "✅ Docker image built!"; \
	else \
		echo "⚠️  Dockerfile not found"; \
	fi

# Install targets
.PHONY: install
install: all ## Install binaries to /usr/local/bin
	@echo "Installing binaries..."
	@if [ "$$(id -u)" -ne 0 ]; then \
		echo "⚠️  Installation requires root privileges"; \
		echo "Please run: sudo make install"; \
		exit 1; \
	fi
	install -m 755 $(APP_NAME) /usr/local/bin/
	install -m 755 $(CLI_NAME) /usr/local/bin/
	@echo "✅ Binaries installed to /usr/local/bin/"

.PHONY: uninstall
uninstall: ## Remove installed binaries
	@echo "Uninstalling binaries..."
	@if [ "$$(id -u)" -ne 0 ]; then \
		echo "⚠️  Uninstallation requires root privileges"; \
		echo "Please run: sudo make uninstall"; \
		exit 1; \
	fi
	rm -f /usr/local/bin/$(APP_NAME)
	rm -f /usr/local/bin/$(CLI_NAME)
	@echo "✅ Binaries uninstalled!"

# Development workflow
.PHONY: check
check: fmt vet test ## Run code formatting, vetting, and tests
	@echo "✅ All checks passed!"

.PHONY: pre-commit
pre-commit: check lint ## Run all pre-commit checks
	@echo "✅ Pre-commit checks completed!"

# Quick targets
.PHONY: quick
quick: agent ## Quick build of main agent (alias for 'agent')

.PHONY: build
build: all ## Alias for 'all' target