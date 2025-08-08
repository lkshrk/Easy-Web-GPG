.PHONY: all build test run clean install

BINARY=bin/gpgweb
CMD=./cmd/gpgweb

all: build

install:
	@echo "no-op: frontend removed"

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
