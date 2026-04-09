.PHONY: all build test lint clean help pre-commit-install pre-commit-run

BINARY_NAME := pageindex

all: build

build:
	go build -o $(BINARY_NAME) ./cmd/pageindex/

clean:
	rm -f $(BINARY_NAME)

help:
	@echo "Available targets:"
	@echo "  build              Build the binary to project root"
	@echo "  test               Run all tests with race detection and coverage"
	@echo "  lint               Run golangci-lint"
	@echo "  clean              Remove build artifacts"
	@echo "  help               Show this help message"
	@echo "  pre-commit-install Install pre-commit hooks"
	@echo "  pre-commit-run     Run pre-commit on all files"

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

pre-commit-install:
	pip install pre-commit
	pre-commit install

pre-commit-run:
	pre-commit run --all-files
