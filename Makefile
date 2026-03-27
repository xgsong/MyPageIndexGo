.PHONY: all build test lint pre-commit-install pre-commit-run

all: build

build:
	go build -o pageindex ./cmd/pageindex/

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

pre-commit-install:
	pip install pre-commit
	pre-commit install

pre-commit-run:
	pre-commit run --all-files
