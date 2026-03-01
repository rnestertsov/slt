# ============================================================================ #
# SHELL CONFIGURATION
# ============================================================================ #

SHELL := /bin/bash

# ============================================================================ #
# HELP
# ============================================================================ #

## help: Show this help message
.PHONY: help
help:
	@echo 'slt - SQL Logic Test Framework'
	@echo ''
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/  /'
	@echo ''
	@echo 'Examples:'
	@echo '  make test       # Run all tests'
	@echo '  make fmt        # Format code'
	@echo ''

# ============================================================================ #
# DEVELOPMENT
# ============================================================================ #

## build: Verify package compiles
.PHONY: build
build:
	@go build ./...

## test: Run all tests
.PHONY: test
test:
	@go test ./... -v

## test-coverage: Generate test coverage report
.PHONY: test-coverage
test-coverage:
	@go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out
	@rm -f coverage.out

## fmt: Format code
.PHONY: fmt
fmt:
	@gofmt -w .

## vet: Run go vet
.PHONY: vet
vet:
	@go vet ./...

# ============================================================================ #
# UTILITIES
# ============================================================================ #

## clean: Clean build artifacts
.PHONY: clean
clean:
	@rm -f coverage.out
