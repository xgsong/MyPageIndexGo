# PageIndex Go Makefile

BINARY := pageindex
SOURCE := ./cmd/pageindex
VERSION := 1.0.0
BUILD_DATE := $(shell date -u +%Y-%m-%d)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)"

.PHONY: all build clean test vet fmt lint coverage help

all: build test vet

# Build the binary for current platform
build:
	@go build $(LDFLAGS) -o $(BINARY) $(SOURCE)

# Build with race detector
build-race:
	@go build -race $(LDFLAGS) -o $(BINARY) $(SOURCE)

# Clean build artifacts
clean:
	@rm -f $(BINARY)
	@rm -f $(BINARY)-*.zip
	@rm -f *.prof
	@rm -rf dist/

# Run all tests
test:
	@go test -race ./...

# Run tests with coverage output
coverage:
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

# Run go vet
vet:
	@go vet ./...

# Format all code
fmt:
	@go fmt ./...

# Run golangci-lint
lint:
	@golangci-lint run ./...

# Install dependencies
deps:
	@go mod download
	@go mod tidy

# Cross-compilation targets
.PHONY: build-linux build-darwin build-windows release

build-linux:
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 $(SOURCE)
	@zip -j dist/$(BINARY)-linux-amd64.zip dist/$(BINARY)-linux-amd64

build-darwin:
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 $(SOURCE)
	@zip -j dist/$(BINARY)-darwin-amd64.zip dist/$(BINARY)-darwin-amd64

build-darwin-arm64:
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 $(SOURCE)
	@zip -j dist/$(BINARY)-darwin-arm64.zip dist/$(BINARY)-darwin-arm64

build-windows:
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe $(SOURCE)
	@zip -j dist/$(BINARY)-windows-amd64.zip dist/$(BINARY)-windows-amd64.exe

# Build release archives for all platforms
release: clean build-linux build-darwin build-darwin-arm64 build-windows
	@echo "Release archives created in dist/"

help:
	@echo "PageIndex Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all            - Build and run tests"
	@echo "  build          - Build binary for current platform"
	@echo "  build-race     - Build with race detector enabled"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run all tests with race detection"
	@echo "  coverage       - Generate coverage report"
	@echo "  vet            - Run go vet"
	@echo "  fmt            - Format all code"
	@echo "  lint           - Run golangci-lint"
	@echo "  deps           - Download dependencies"
	@echo "  release        - Build release archives for all platforms"
	@echo "  help           - Show this help"
