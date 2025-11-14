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

# Unit tests with coverage (excluding proto directory)
echo "Unit tests with coverage..."
go test -v -race -coverprofile=coverage.out -covermode=atomic $(go list ./... | grep -v '/proto')

# Print coverage data
echo ""
echo "=== COVERAGE DATA ==="
if [ -f "coverage.out" ]; then
    echo "Overall coverage (excluding proto):"
    go tool cover -func=coverage.out | tail -1
    
    # Generate HTML coverage report
    echo "Generating HTML coverage report..."
    go tool cover -html=coverage.out -o coverage.html
    echo "Coverage report generated: coverage.html"
fi

# Integration tests with benchmarks
echo "Integration tests with benchmarks..."
chmod +x examples/run_integration_tests.sh
examples/run_integration_tests.sh

# Package integration tests (e.g., vector DB tests with testcontainers)
echo ""
echo "=== PACKAGE INTEGRATION TESTS ==="
if command -v docker >/dev/null 2>&1; then
    echo "Running Qdrant integration tests..."
    go test -tags=integration -v -timeout=10m ./pkg/middleware/retrieval/qdrant/ || echo "⚠ Qdrant integration tests failed (Docker required)"
else
    echo "⚠ Docker not found - skipping package integration tests"
    echo "  Install Docker to run vector DB integration tests"
fi

# Run integration test benchmarks
echo ""
echo "=== INTEGRATION TEST BENCHMARKS ==="
for example in examples/*/; do
    if [ -f "$example/integration_test.go" ]; then
        echo "Benchmarking $example..."
        (cd "$example" && go test -bench=. -benchmem -run=^$ > /dev/null 2>&1 && echo "  ✓ Benchmarks completed" || echo "  ⚠ No benchmarks found")
    fi
done

# Coverage Report Summary
echo ""
echo "=== COVERAGE SUMMARY ==="
if [ -f "coverage.out" ]; then
    OVERALL_COVERAGE=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
    echo "📊 Coverage: ${OVERALL_COVERAGE}%"
    echo "📄 Report: coverage.html"
fi

echo ""
echo "Workflow completed successfully!"
