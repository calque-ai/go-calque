#!/bin/bash
set -e

echo "Starting go-calque workflow..."

# Build
echo "Building..."
go build -v -o /dev/null ./...
for example in examples/*/; do
    if [ -f "$example/main.go" ]; then
        (cd "$example" && go build -v -o /dev/null .)
    fi
done

# Lint
echo "Linting..."
go vet ./...
if command -v golangci-lint >/dev/null; then
    golangci-lint run
fi

# Unit tests with coverage
echo "Unit tests with coverage..."
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
go test -v -race -coverprofile=pkg-coverage.out -covermode=atomic ./pkg/...

# Print coverage data
echo ""
echo "=== COVERAGE DATA ==="
if [ -f "coverage.out" ]; then
    echo "Main packages coverage:"
    go tool cover -func=coverage.out | tail -1
fi

if [ -f "pkg-coverage.out" ]; then
    echo "Package coverage:"
    go tool cover -func=pkg-coverage.out | tail -1
fi

# Integration tests with benchmarks
echo "Integration tests with benchmarks..."
chmod +x examples/run_integration_tests.sh
examples/run_integration_tests.sh

# Run integration test benchmarks
echo ""
echo "=== INTEGRATION TEST BENCHMARKS ==="
for example in examples/*/; do
    if [ -f "$example/integration_test.go" ]; then
        echo "Benchmarking $example..."
        (cd "$example" && go test -bench=. -benchmem -run=^$ > /dev/null 2>&1 && echo "  ✓ Benchmarks completed" || echo "  ⚠ No benchmarks found")
    fi
done

echo ""
echo "Workflow completed successfully!"
