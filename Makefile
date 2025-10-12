.PHONY: help build run test clean docker-build docker-run test-visual test-visual-setup test-visual-cleanup

help:
	@echo "Easy Web GPG"
	@echo ""
	@echo "Available targets:"
	@echo "  build             - Build using Docker"
	@echo "  run               - Run the application"
	@echo "  test              - Run Go tests"
	@echo "  test-visual       - Run visual regression tests (full comparison)"
	@echo "  test-visual-setup - Setup containers for visual testing"
	@echo "  test-visual-cleanup - Stop and remove visual test containers"
	@echo "  clean             - Clean build artifacts"
	@echo "  docker-build      - Build Docker image"
	@echo "  docker-run        - Run Docker container"

build:
	docker build --target binary-export --output bin/ .

run:
	docker compose up

test:
	go test ./...

clean:
	rm -rf bin/ static/dist/

docker-build:
	docker build -t easy-web-gpg:latest .

docker-run:
	docker run -d --name easy-web-gpg -p 8080:8080 \
		-e MASTER_PASSWORD="${MASTER_PASSWORD}" \
		-e PORT="${PORT}" \
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
	@echo "✓ Visual test environment ready!"
	@echo "  Main branch:    http://localhost:8081"
	@echo "  Current branch: http://localhost:8080"

test-visual-cleanup:
	@echo "Cleaning up visual regression test environment..."
	@docker stop easy-web-gpg-main easy-web-gpg-current 2>/dev/null || true
	@docker rm easy-web-gpg-main easy-web-gpg-current 2>/dev/null || true
	@echo "✓ Cleanup complete"

test-visual: test-visual-setup
	@echo ""
	@echo "Running visual regression tests..."
	@cd tests && \
		npm install --silent && \
		npx playwright install --with-deps chromium --quiet && \
		BASELINE_URL=http://localhost:8081 BASE_URL=http://localhost:8080 npm run test:visual
	@$(MAKE) test-visual-cleanup
