.PHONY: help build run test clean docker-build docker-run

help:
	@echo "Easy Web GPG"
	@echo ""
	@echo "Available targets:"
	@echo "  build        - Build using Docker"
	@echo "  run          - Run the application"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"

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
