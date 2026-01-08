#!/bin/bash
# Generate benchmark analysis report for go-calque
# Usage: ./scripts/generate_benchmark_report.sh [--update] [--profile]

set -e

REPORT_FILE="docs/BENCHMARK_ANALYSIS_REPORT.md"
TEMP_FILE=$(mktemp)
PROFILE_DIR="benchmark_profiles"

echo "=============================================="
echo "  Go-Calque Benchmark Suite"
echo "=============================================="
echo ""

# Run comprehensive analysis benchmarks
echo "1. Running analysis benchmarks (baseline, overhead, scaling)..."
go test -bench="Benchmark(Baseline|FlowSetup|SingleHandler|MultiHandler|ChainVsFlow|AILatency|Memory|Goroutine|Concurrent|DataSize|Streaming|Realistic)" \
    -benchmem -timeout 10m ./pkg/calque/... 2>/dev/null | tee "$TEMP_FILE"

echo ""
echo "2. Running core flow benchmarks..."
go test -bench="BenchmarkFlow" -benchmem ./pkg/calque/... 2>/dev/null | tee -a "$TEMP_FILE"

echo ""
echo "3. Running anagram benchmarks (framework vs baseline)..."
go test -bench=. -benchmem ./examples/anagram/... 2>/dev/null | tee -a "$TEMP_FILE"

echo ""
echo "4. Running ctrl middleware benchmarks..."
go test -bench=. -benchmem ./pkg/middleware/ctrl/... 2>/dev/null | tee -a "$TEMP_FILE"

echo ""
echo "=============================================="
echo "  Benchmark Summary"
echo "=============================================="

# Extract and display key metrics
echo ""
echo "BASELINE (direct function calls):"
grep "BenchmarkBaseline" "$TEMP_FILE" 2>/dev/null | head -5 || echo "  (not found)"

echo ""
echo "SINGLE HANDLER OVERHEAD:"
grep "BenchmarkSingleHandler" "$TEMP_FILE" 2>/dev/null || echo "  (not found)"

echo ""
echo "CHAIN VS FLOW:"
grep "BenchmarkChainVsFlow" "$TEMP_FILE" 2>/dev/null || echo "  (not found)"

echo ""
echo "AI LATENCY IMPACT:"
grep "BenchmarkAILatency" "$TEMP_FILE" 2>/dev/null || echo "  (not found)"

echo ""
echo "FRAMEWORK VS BASELINE (anagram):"
grep -E "Benchmark(Baseline|GoCalque)" "$TEMP_FILE" 2>/dev/null | head -4 || echo "  (not found)"

echo ""
echo "=============================================="
echo ""
echo "Raw results saved to: $TEMP_FILE"
echo "Report location: $REPORT_FILE"
echo ""

# Handle --profile flag
if [[ "$1" == "--profile" ]] || [[ "$2" == "--profile" ]]; then
    echo ""
    echo "5. Running CPU and memory profiling..."
    mkdir -p "$PROFILE_DIR"
    
    # CPU profile
    echo "  → CPU profile..."
    go test -bench="BenchmarkRealisticWorkflow" -benchmem -cpuprofile="$PROFILE_DIR/cpu.prof" \
        ./pkg/calque/... 2>/dev/null
    
    # Memory profile
    echo "  → Memory profile..."
    go test -bench="BenchmarkMemory" -benchmem -memprofile="$PROFILE_DIR/mem.prof" \
        ./pkg/calque/... 2>/dev/null
    
    echo ""
    echo "Profiles saved to $PROFILE_DIR/"
    echo "  - cpu.prof: CPU profile"
    echo "  - mem.prof: Memory profile"
    echo ""
    echo "To analyze profiles:"
    echo "  go tool pprof -http=:8080 $PROFILE_DIR/cpu.prof"
    echo "  go tool pprof -http=:8081 $PROFILE_DIR/mem.prof"
fi

# Handle --update flag
if [[ "$1" == "--update" ]] || [[ "$2" == "--update" ]]; then
    echo ""
    echo "Updating report with raw benchmark data..."
    
    # Backup current report
    cp "$REPORT_FILE" "${REPORT_FILE}.bak"
    
    # Remove old raw data section if exists
    sed -i '/^## Raw Benchmark Data$/,$d' "$REPORT_FILE"
    
    # Add new raw data section
    cat >> "$REPORT_FILE" << EOF

---

## Raw Benchmark Data

> Last updated: $(date "+%B %d, %Y")
> System: $(uname -s) $(uname -m)
> Go: $(go version | cut -d' ' -f3)

\`\`\`
$(cat "$TEMP_FILE")
\`\`\`
EOF
    
    echo "✓ Report updated: $REPORT_FILE"
    echo "✓ Backup saved: ${REPORT_FILE}.bak"
fi

if [[ "$1" != "--update" ]] && [[ "$1" != "--profile" ]]; then
    echo ""
    echo "Options:"
    echo "  ./scripts/generate_benchmark_report.sh --update   # Update report with results"
    echo "  ./scripts/generate_benchmark_report.sh --profile  # Generate CPU/memory profiles"
    echo "  ./scripts/generate_benchmark_report.sh --update --profile  # Both"
fi
