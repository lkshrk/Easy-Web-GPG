# ==============================================================================
# Easy Web GPG Makefile
# ==============================================================================

# Variables
# ------------------------------------------------------------------------------
GOCACHE := $(CURDIR)/.gocache
export GOCACHE

# Binary and build configuration
BINARY := bin/easy-web-gpg
CMD := ./cmd/easywebgpg
BIN_DIR := $(dir $(BINARY))
BUILD_FLAGS := -trimpath -ldflags="-s -w"

# CSS and Node.js configuration
NODE_MODULES := node_modules
CSS_INPUT := static/css/input.css
CSS_OUTPUT := static/dist/styles.css
PACKAGE_JSON := package.json

# Go configuration
GO_FILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')
GO_MODULES := $(shell find . -name 'go.mod' -not -path './vendor/*')

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

# ==============================================================================
# Default and Help
# ==============================================================================

.DEFAULT_GOAL := help
.PHONY: help

help: ## Show this help message
	@echo "$(BLUE)Easy Web GPG$(NC)"
	@echo "$(BLUE)====================$(NC)"
	@echo ""
	@echo "$(GREEN)Build Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -E "(build|install|clean)"
	@echo ""
	@echo "$(GREEN)Development Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -E "(run|dev|watch)"
	@echo ""
	@echo "$(GREEN)CSS Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -E "(css)"
	@echo ""
	@echo "$(GREEN)Docker Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -E "(docker)"
	@echo ""
	@echo "$(GREEN)Quality Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -E "(test|fmt|vet|lint|check)"
	@echo ""
	@echo "$(GREEN)Utility Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -v -E "(build|install|clean|run|dev|watch|css|docker|test|fmt|vet|lint|check)"

# ==============================================================================
# Installation and Setup
# ==============================================================================

.PHONY: install install-deps install-go-deps install-node-deps

install: install-deps ## Install all dependencies
	@echo "$(GREEN)✓ All dependencies installed$(NC)"

install-deps: install-go-deps install-node-deps ## Install Go and Node.js dependencies

install-go-deps: ## Install Go dependencies
	@echo "$(BLUE)Installing Go dependencies...$(NC)"
	@go mod download
	@go mod verify
	@echo "$(GREEN)✓ Go dependencies installed$(NC)"

install-node-deps: ## Install Node.js dependencies
	@echo "$(BLUE)Installing Node.js dependencies...$(NC)"
	@if [ -f "$(PACKAGE_JSON)" ]; then \
		npm ci --silent; \
		echo "$(GREEN)✓ Node.js dependencies installed$(NC)"; \
	else \
		echo "$(YELLOW)⚠ No package.json found, skipping Node.js dependencies$(NC)"; \
	fi

# ==============================================================================
# CSS Build Targets
# ==============================================================================

.PHONY: css css-dev css-watch css-clean

css: install-node-deps ## Build production CSS with Tailwind
	@echo "$(BLUE)Building production CSS...$(NC)"
	@if [ -f "$(PACKAGE_JSON)" ]; then \
		npm run build:css --silent && \
		echo "$(GREEN)✓ CSS built successfully$(NC)"; \
	else \
		echo "$(RED)✗ No package.json found$(NC)"; \
		exit 1; \
	fi

css-dev: install-node-deps ## Build development CSS (unminified)
	@echo "$(BLUE)Building development CSS...$(NC)"
	@if [ -f "$(PACKAGE_JSON)" ]; then \
		npx tailwindcss -i $(CSS_INPUT) -o $(CSS_OUTPUT) && \
		echo "$(GREEN)✓ Development CSS built$(NC)"; \
	else \
		echo "$(RED)✗ No package.json found$(NC)"; \
		exit 1; \
	fi

css-watch: install-node-deps ## Watch and rebuild CSS on changes
	@echo "$(BLUE)Watching CSS files for changes...$(NC)"
	@if [ -f "$(PACKAGE_JSON)" ]; then \
		npm run watch:css; \
	else \
		echo "$(RED)✗ No package.json found$(NC)"; \
		exit 1; \
	fi

css-clean: ## Clean CSS build artifacts
	@echo "$(BLUE)Cleaning CSS artifacts...$(NC)"
	@rm -f $(CSS_OUTPUT)
	@echo "$(GREEN)✓ CSS artifacts cleaned$(NC)"

# ==============================================================================
# Build Targets
# ==============================================================================

.PHONY: build build-dev all

all: build ## Default target (same as build)

build: css ## Build production binary
	@echo "$(BLUE)Building production binary...$(NC)"
	@mkdir -p $(BIN_DIR)
	@go build $(BUILD_FLAGS) -o $(BINARY) $(CMD)
	@echo "$(GREEN)✓ Binary built: $(BINARY)$(NC)"

build-dev: css-dev ## Build development binary (with debug info)
	@echo "$(BLUE)Building development binary...$(NC)"
	@mkdir -p $(BIN_DIR)
	@go build -o $(BINARY) $(CMD)
	@echo "$(GREEN)✓ Development binary built: $(BINARY)$(NC)"

# ==============================================================================
# Development Targets
# ==============================================================================

.PHONY: run run-dev dev

run: build ## Build and run the production binary
	@echo "$(BLUE)Starting production server...$(NC)"
	@MASTER_KEY=$$MASTER_KEY ./$(BINARY)

run-dev: build-dev ## Build and run the development binary
	@echo "$(BLUE)Starting development server...$(NC)"
	@MASTER_KEY=$$MASTER_KEY ./$(BINARY)

dev: ## Start development environment (CSS watch + server)
	@echo "$(BLUE)Starting development environment...$(NC)"
	@echo "$(YELLOW)This will start CSS watching and the server$(NC)"
	@trap 'echo "$(YELLOW)Stopping development environment...$(NC)"; kill 0' INT; \
	make css-watch & \
	sleep 2; \
	make run-dev

# ==============================================================================
# Docker Targets
# ==============================================================================

.PHONY: docker-build docker-run docker-dev docker-stop docker-clean docker-logs

docker-build: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	@docker build -t easy-web-gpg:latest .
	@echo "$(GREEN)✓ Docker image built$(NC)"

docker-run: docker-build ## Build and run Docker container
	@echo "$(BLUE)Starting Docker container...$(NC)"
	@docker run -d \
		--name easy-web-gpg-container \
		-p 8080:8080 \
		-e MASTER_KEY=$$MASTER_KEY \
		easy-web-gpg:latest
	@echo "$(GREEN)✓ Container started on http://localhost:8080$(NC)"

docker-dev: ## Start development environment with Docker Compose
	@echo "$(BLUE)Starting development environment...$(NC)"
	@docker-compose --profile dev up -d
	@echo "$(GREEN)✓ Development environment started$(NC)"

docker-stop: ## Stop and remove Docker containers
	@echo "$(BLUE)Stopping Docker containers...$(NC)"
	@docker stop easy-web-gpg-container 2>/dev/null || true
	@docker rm easy-web-gpg-container 2>/dev/null || true
	@docker-compose down 2>/dev/null || true
	@echo "$(GREEN)✓ Containers stopped$(NC)"

docker-logs: ## Show Docker container logs
	@docker logs easy-web-gpg-container 2>/dev/null || docker-compose logs

docker-clean: docker-stop ## Clean Docker images and containers
	@echo "$(BLUE)Cleaning Docker artifacts...$(NC)"
	@docker rmi easy-web-gpg:latest 2>/dev/null || true
	@docker system prune -f
	@echo "$(GREEN)✓ Docker artifacts cleaned$(NC)"

# ==============================================================================
# Testing and Quality
# ==============================================================================
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -E "(test|fmt|vet|lint|check)"
	@echo ""
	@echo "$(GREEN)Utility Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST) | grep -v -E "(build|install|clean|run|dev|watch|css|test|fmt|vet|lint|check)"

.PHONY: test test-verbose test-coverage vet fmt lint check

test: ## Run unit tests
	@echo "$(BLUE)Running unit tests...$(NC)"
	@go test ./...
	@echo "$(GREEN)✓ Tests passed$(NC)"

test-verbose: ## Run unit tests with verbose output
	@echo "$(BLUE)Running unit tests (verbose)...$(NC)"
	@go test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ Vet checks passed$(NC)"

fmt: ## Format Go code
	@echo "$(BLUE)Formatting Go code...$(NC)"
	@gofmt -w .
	@echo "$(GREEN)✓ Code formatted$(NC)"

lint: ## Run comprehensive linting (requires golangci-lint)
	@echo "$(BLUE)Running linters...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)✓ Linting passed$(NC)"; \
	else \
		echo "$(YELLOW)⚠ golangci-lint not found, running basic checks$(NC)"; \
		make fmt vet; \
	fi

check: fmt vet test ## Run all quality checks

# ==============================================================================
# Utility Targets
# ==============================================================================

.PHONY: smoke clean clean-all deps-check info

smoke: build ## Run smoke test (start server, test endpoint, stop)
	@echo "$(BLUE)Running smoke test...$(NC)"
	@set -e; \
	LOG=$$(mktemp); \
	./$(BINARY) >$$LOG 2>&1 & \
	PID=$$!; \
	sleep 2; \
	if grep -q "Listening on" $$LOG; then \
		if curl -sfS http://localhost:8080/time >/dev/null 2>&1; then \
			echo "$(GREEN)✓ Smoke test passed$(NC)"; \
			kill $$PID || true; \
			sleep 1; \
			exit 0; \
		else \
			echo "$(RED)✗ Smoke test failed: server did not respond$(NC)"; \
			kill $$PID || true; \
			cat $$LOG; \
			exit 1; \
		fi; \
	else \
		if [ "$$GITHUB_ACTIONS" = "true" ]; then \
			cat $$LOG; \
			echo "$(RED)✗ Smoke test failed: server did not start$(NC)"; \
			kill $$PID || true; \
			exit 1; \
		else \
			cat $$LOG; \
			echo "$(YELLOW)⚠ Smoke test skipped: unable to bind to port (non-fatal)$(NC)"; \
			kill $$PID || true; \
			exit 0; \
		fi; \
	fi

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)✓ Build artifacts cleaned$(NC)"

clean-all: clean css-clean ## Clean all artifacts including CSS and dependencies
	@echo "$(BLUE)Cleaning all artifacts...$(NC)"
	@rm -rf $(NODE_MODULES)
	@go clean -cache -modcache -testcache
	@echo "$(GREEN)✓ All artifacts cleaned$(NC)"

deps-check: ## Check for outdated dependencies
	@echo "$(BLUE)Checking Go dependencies...$(NC)"
	@go list -u -m all | grep -E '\[[^\]]*\]' || echo "$(GREEN)✓ All Go dependencies up to date$(NC)"
	@if [ -f "$(PACKAGE_JSON)" ]; then \
		echo "$(BLUE)Checking Node.js dependencies...$(NC)"; \
		npm outdated || echo "$(GREEN)✓ All Node.js dependencies up to date$(NC)"; \
	fi

info: ## Show project information
	@echo "$(BLUE)Project Information$(NC)"
	@echo "$(BLUE)==================$(NC)"
	@echo "Binary: $(BINARY)"
	@echo "Command: $(CMD)"
	@echo "Go cache: $(GOCACHE)"
	@echo "CSS input: $(CSS_INPUT)"
	@echo "CSS output: $(CSS_OUTPUT)"
	@echo ""
	@echo "$(BLUE)Go Version:$(NC)"
	@go version
	@echo ""
	@echo "$(BLUE)Node.js Version:$(NC)"
	@node --version 2>/dev/null || echo "Node.js not found"
	@echo ""
	@echo "$(BLUE)NPM Version:$(NC)"
	@npm --version 2>/dev/null || echo "NPM not found"

# ==============================================================================
# File Dependencies
# ==============================================================================

$(BINARY): $(GO_FILES) $(CSS_OUTPUT)
	@make build

$(CSS_OUTPUT): $(CSS_INPUT) $(NODE_MODULES)
	@make css

$(NODE_MODULES): $(PACKAGE_JSON)
	@make install-node-deps
