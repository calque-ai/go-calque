#!/bin/bash

set -e

# Simple configuration
TIMEOUT=${TIMEOUT:-30s}
PARALLEL_JOBS=${PARALLEL_JOBS:-4}

echo "Running integration tests in parallel..."
echo "Parallel jobs: $PARALLEL_JOBS"
echo "Timeout: $TIMEOUT"

# Find test directories
test_dirs=()
while IFS= read -r dir; do
    if [ -n "$dir" ]; then
        test_dirs+=("$dir")
        echo "Found: $dir"
    fi
done < <(find . -name "integration_test.go" -exec dirname {} \; | sort | uniq)

if [ ${#test_dirs[@]} -eq 0 ]; then
    echo "No integration tests found"
    exit 0
fi

echo "Starting ${#test_dirs[@]} test suites..."

# Run tests in parallel using background jobs
pids=()
failed=0

for dir in "${test_dirs[@]}"; do
    (
        echo "[$dir] Starting..."
        cd "$dir"
        if go test -v -race -parallel="$PARALLEL_JOBS" -timeout="$TIMEOUT" -short .; then
            echo "[$dir] PASSED"
        else
            echo "[$dir] FAILED"
            exit 1
        fi
    ) &
    pids+=($!)
done

# Wait for all tests
for pid in "${pids[@]}"; do
    if ! wait "$pid"; then
        failed=1
    fi
done

if [ $failed -eq 0 ]; then
    echo "All integration tests passed!"
else
    echo "Some integration tests failed!"
    exit 1
fi
