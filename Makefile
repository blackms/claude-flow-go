.PHONY: build test lint vet clean install run-doctor bench help

BINARY_NAME := claude-flow
BUILD_DIR := .
GO := go

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the claude-flow binary
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/claude-flow

install: ## Install claude-flow to GOPATH/bin
	$(GO) install ./cmd/claude-flow

test: ## Run all tests
	$(GO) test ./...

test-race: ## Run tests with race detector
	$(GO) test -race ./...

test-v: ## Run tests with verbose output
	$(GO) test -v ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: vet ## Run golangci-lint (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

clean: ## Remove built binary
	rm -f $(BUILD_DIR)/$(BINARY_NAME)

run-doctor: build ## Build and run system diagnostics
	./$(BINARY_NAME) doctor

all: clean lint test build ## Clean, lint, test, and build
