.PHONY: help build run test test-docker clean css css-watch docker-build docker-run test-visual test-visual-setup test-visual-cleanup dev dev-build dev-down dev-logs demo-gif

help:
	@echo "Easy Web GPG"
	@echo ""
	@echo "Available targets:"
	@echo "  build             - Build using Docker"
	@echo "  run               - Run the application (builds CSS first)"
	@echo "  dev               - Run development environment with live reload"
	@echo "  dev-build         - Build development Docker images"
	@echo "  dev-down          - Stop development environment"
	@echo "  dev-logs          - Follow development logs"
	@echo "  test              - Run Go tests (requires local Go)"
	@echo "  test-docker       - Run all tests in Docker (no dependencies)"
	@echo "  css               - Build Tailwind CSS (minified)"
	@echo "  css-watch         - Watch and rebuild Tailwind CSS"
	@echo "  demo-gif          - Regenerate .github/assets/demo.gif (requires Go, Node, ImageMagick)"
	@echo "  test-visual       - Run visual regression tests (full comparison)"
	@echo "  test-visual-setup - Setup containers for visual testing"
	@echo "  test-visual-cleanup - Stop and remove visual test containers"
	@echo "  clean             - Clean build artifacts"
	@echo "  docker-build      - Build Docker image"
	@echo "  docker-run        - Build and run Docker container (port 8080, set MASTER_PASSWORD env)"

# Tailwind CSS standalone CLI
TAILWIND_BIN := bin/tailwindcss
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
  ifeq ($(UNAME_M),arm64)
    TAILWIND_PLATFORM := macos-arm64
  else
    TAILWIND_PLATFORM := macos-x64
  endif
else
  ifeq ($(UNAME_M),aarch64)
    TAILWIND_PLATFORM := linux-arm64
  else
    TAILWIND_PLATFORM := linux-x64
  endif
endif

$(TAILWIND_BIN):
	@mkdir -p bin
	@echo "Downloading Tailwind CSS standalone CLI..."
	@curl -sLo $(TAILWIND_BIN) "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-$(TAILWIND_PLATFORM)"
	@chmod +x $(TAILWIND_BIN)

css: $(TAILWIND_BIN)
	@mkdir -p static/dist
	$(TAILWIND_BIN) -i static/css/input.css -o static/dist/styles.css --minify

css-watch: $(TAILWIND_BIN)
	@mkdir -p static/dist
	$(TAILWIND_BIN) -i static/css/input.css -o static/dist/styles.css --watch

demo-gif: css
	@bash scripts/demo.sh

build:
	docker build --target binary-export --output bin/ .

run: css
	go run ./cmd/easywebgpg

test:
	go test -race ./...

test-docker:
	@echo "Running all tests in Docker (no local dependencies)..."
	@echo ""
	@echo "Building application image..."
	docker build -t easy-web-gpg:latest .
	@echo ""
	@echo "Running Go tests..."
	docker build --target go-test -t easy-web-gpg:test-go -f Dockerfile.test .
	@echo ""
	@echo "Running E2E tests..."
	@docker run -d --name easy-web-gpg-test-app \
		-p 8080:8080 \
		-e MASTER_PASSWORD="test-password" \
		easy-web-gpg:latest; \
	for i in $$(seq 1 30); do \
		if curl -sf http://localhost:8080/ > /dev/null 2>&1; then \
			echo "Application ready"; \
			break; \
		fi; \
		sleep 1; \
	done; \
	docker build --target playwright-test -t easy-web-gpg:test-e2e -f Dockerfile.test .; \
	docker stop easy-web-gpg-test-app > /dev/null 2>&1; \
	docker rm easy-web-gpg-test-app > /dev/null 2>&1; \
	echo ""
	@echo "✓ All tests completed"

clean:
	rm -rf bin/ static/dist/

docker-build:
	docker build -t easy-web-gpg:latest .

docker-run: docker-build
	@docker rm -f easy-web-gpg 2>/dev/null || true
	@mkdir -p data
	docker run --rm --name easy-web-gpg \
		-p 8080:8080 \
		-e MASTER_PASSWORD="$${MASTER_PASSWORD}" \
		-v "$(CURDIR)/data:/data" \
		easy-web-gpg:latest

test-visual-setup:
	@echo "Setting up visual regression test environment..."
	@echo "Building main branch Docker image..."
	@if [ ! -d "/tmp/easy-web-gpg-main" ]; then \
		git clone . /tmp/easy-web-gpg-main 2>/dev/null || true; \
	fi
	@cd /tmp/easy-web-gpg-main && git fetch origin && git checkout main && git pull
	@docker build -t easy-web-gpg:main /tmp/easy-web-gpg-main
	@echo "Building current branch Docker image..."
	@docker build -t easy-web-gpg:current .
	@echo "Starting main branch application on port 8081..."
	@docker run -d --name easy-web-gpg-main -p 8081:8080 \
		-e MASTER_PASSWORD="test-password" \
		easy-web-gpg:main
	@echo "Waiting for main branch application to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -sf http://localhost:8081/time > /dev/null 2>&1; then \
			echo "Main branch application is ready"; \
			break; \
		fi; \
		echo "Waiting for main branch application... ($$i/30)"; \
		sleep 1; \
	done
	@echo "Starting current branch application on port 8080..."
	@docker run -d --name easy-web-gpg-current -p 8080:8080 \
		-e MASTER_PASSWORD="test-password" \
		easy-web-gpg:current
	@echo "Waiting for current branch application to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -sf http://localhost:8080/time > /dev/null 2>&1; then \
			echo "Current branch application is ready"; \
			break; \
		fi; \
		echo "Waiting for current branch application... ($$i/30)"; \
		sleep 1; \
	done
	@echo ""
	@echo "Visual test environment ready!"
	@echo "  Main branch:    http://localhost:8081"
	@echo "  Current branch: http://localhost:8080"

test-visual-cleanup:
	@echo "Cleaning up visual regression test environment..."
	@docker stop easy-web-gpg-main easy-web-gpg-current 2>/dev/null || true
	@docker rm easy-web-gpg-main easy-web-gpg-current 2>/dev/null || true
	@echo "Cleanup complete"

test-visual: test-visual-setup
	@echo ""
	@echo "Running visual regression tests..."
	@cd tests && \
		npm install --silent && \
		npx playwright install --with-deps chromium --quiet && \
		BASELINE_URL=http://localhost:8081 BASE_URL=http://localhost:8080 npm run test:visual
	@$(MAKE) test-visual-cleanup

dev-build:
	@echo "Building development Docker images..."
	@docker compose -f docker-compose.dev.yml build

dev:
	@echo "Starting development environment with live reload..."
	@echo ""
	@echo "  App:        http://localhost:8080"
	@echo "  Password:   $${MASTER_PASSWORD:-changeme}"
	@echo ""
	@echo "Changes to Go files, templates, and CSS will trigger automatic reloads."
	@echo "Press Ctrl+C to stop."
	@echo ""
	@docker compose -f docker-compose.dev.yml up

dev-down:
	@docker compose -f docker-compose.dev.yml down

dev-logs:
	@docker compose -f docker-compose.dev.yml logs -f
