.PHONY: help test test-race test-coverage lint fmt bench install clean

# Default target
help:
	@echo "Available targets:"
	@echo "  make test          - Run tests"
	@echo "  make test-race     - Run tests with race detector"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make coverage      - Generate and view coverage report"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make bench         - Run benchmarks"
	@echo "  make install       - Install dependencies"
	@echo "  make clean         - Clean build artifacts"

# Run tests
test:
	go test -v ./...

# Run tests with race detector
test-race:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# Generate and view coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Run linter
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run --timeout=5m

# Format code
fmt:
	go fmt ./...
	@which goimports > /dev/null && goimports -w . || true

# Run benchmarks
bench:
	go test -bench=. -benchmem -run=^$$ ./...

# Install dependencies
install:
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	rm -f coverage.out
	go clean -cache -testcache
