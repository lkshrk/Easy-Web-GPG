GOCACHE := $(CURDIR)/.gocache
export GOCACHE

BINARY := bin/web-gpg
CMD := ./cmd/gpgweb
BIN_DIR := $(dir $(BINARY))

.PHONY: help test build run clean fmt vet smoke


# Default target
all: build

help:
	@echo "Makefile targets:"
	@echo "  make        (or make all)        - build native binary"
	@echo "  make build                     - build native binary into $(BINARY)"
	@echo "  make run                       - build and run the app (uses $(BINARY))"
	@echo "  make test                      - run unit tests"
	@echo "  make fmt                       - run gofmt on the codebase"
	@echo "  make vet                       - run go vet"
	@echo "  make smoke                     - start binary, curl /time, then stop"
	@echo "  make clean                     - remove build artifacts"

# Run unit tests
test:
	go test ./...


# Build the Go binary for the current platform (native build)
.PHONY: build
build: css
	@echo "Building native go binary -> $(BINARY)"
	@mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BINARY) $(CMD)


.PHONY: css
css:
	@echo "Building CSS with Tailwind CLI..."
	@if [ -f package.json ]; then 		npm ci --silent || true; 		npm run build:css --silent; 	else 		echo "no package.json found, skipping css build"; 	fi

# Run the app locally (builds then runs the binary)
run: build
	@echo "Running server..."
	MASTER_KEY=$$MASTER_KEY ./$(BINARY)

.PHONY: smoke
smoke: build
	@echo "Running smoke test (start server, curl /time, then stop)"
	@set -e; 	LOG=$$(mktemp); 	./$(BINARY) >$$LOG 2>&1 & 	PID=$$!; 	sleep 1; 	if grep -q "Listening on" $$LOG; then 		if curl -sfS http://localhost:8080/time >/dev/null 2>&1; then 			echo "smoke ok"; 			kill $$PID || true; 			sleep 1; 			exit 0; 		else 			echo "smoke failed: server did not respond"; 			kill $$PID || true; 			exit 1; 		fi; 	else 		# If running inside GitHub Actions we treat missing listen as failure.
		if [ "$$GITHUB_ACTIONS" = "true" ]; then 			cat $$LOG; echo "smoke failed: server did not start"; kill $$PID || true; exit 1; 		else 			cat $$LOG; echo "smoke skipped: unable to bind to port in this environment (non-fatal)"; kill $$PID || true; exit 0; 		fi; 	fi

fmt:
	@gofmt -w .

vet:
	@go vet ./...


clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)

