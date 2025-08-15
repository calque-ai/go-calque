package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Test data - mix of small and larger word sets
var testWords = []string{
	"listen", "silent", "hello", "world", "act", "cat", "tac",
	"race", "care", "acre", "programming", "go", "og", "a", "b",
	"stressed", "desserts", "evil", "live", "vile", "elbow", "below",
	"study", "dusty", "night", "thing", "angel", "glean", "angle",
}

var largeTestWords = func() []string {
	words := make([]string, 0, 1000)
	baseWords := []string{
		"listen", "silent", "enlist", "tinsel",
		"race", "care", "acre", "scare",
		"evil", "live", "vile", "levi",
		"stressed", "desserts",
		"angel", "glean", "angle", "lange",
		"study", "dusty", "sudan",
		"night", "thing",
		"below", "elbow", "bowel",
	}

	// Duplicate words to create a larger dataset
	for i := 0; i < 40; i++ {
		words = append(words, baseWords...)
	}
	return words
}()

//   BenchmarkBaseline-10              189362             63148 ns/op           76736 B/op        685 allocs/op
//   BenchmarkCalquePipe-10            130064             90928 ns/op           28586 B/op        479 allocs/op
//   BenchmarkBaselineLarge-10           2991           3979004 ns/op         4011708 B/op      33990 allocs/op
//   BenchmarkCalquePipeLarge-10         5352           2213417 ns/op          248295 B/op       9574 allocs/op

// Benchmark tests
func BenchmarkBaseline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Baseline(testWords)
	}
}

func BenchmarkCalquePipe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GoCalque(testWords)
	}
}

// Large dataset benchmarks
func BenchmarkBaselineLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Baseline(largeTestWords)
	}
}

func BenchmarkCalquePipeLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GoCalque(largeTestWords)
	}
}

// Benchmark the new framework-heavy implementation
func BenchmarkGoCalqueFramework(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GoCalqueFramework(testWords)
	}
}

func BenchmarkGoCalqueFrameworkLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GoCalqueFramework(largeTestWords)
	}
}

// GoCalqueWithConfig creates a pipeline with specific concurrency configuration
func GoCalqueWithConfig(words []string, config calque.FlowConfig) map[string]map[string]struct{} {
	if len(words) == 0 {
		return nil
	}

	input := strings.Join(words, "\n")

	flow := calque.NewFlow(config).
		Use(filterAndLowercase()).
		Use(mapToAnagramFormat()).
		Use(accumulateAnagrams())

	var outputStr string
	err := flow.Run(context.Background(), input, &outputStr)
	if err != nil {
		return nil
	}

	return parseAnagramOutput(outputStr)
}

// Benchmark with unlimited concurrency (old behavior)
func BenchmarkCalquePipeUnlimited(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: calque.ConcurrencyUnlimited,
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(testWords, config)
	}
}

func BenchmarkCalquePipeUnlimitedLarge(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: calque.ConcurrencyUnlimited,
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(largeTestWords, config)
	}
}

// Benchmark with fixed concurrency limit
func BenchmarkCalquePipeFixed(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: 10, // Fixed limit of 10 concurrent pipelines
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(testWords, config)
	}
}

func BenchmarkCalquePipeFixedLarge(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: 10, // Fixed limit of 10 concurrent pipelines
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(largeTestWords, config)
	}
}

// Benchmark with auto CPU-based limiting (current default)
func BenchmarkCalquePipeAuto(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: calque.ConcurrencyAuto,
		CPUMultiplier: 4, // 4x CPU cores
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(testWords, config)
	}
}

func BenchmarkCalquePipeAutoLarge(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: calque.ConcurrencyAuto,
		CPUMultiplier: 4, // 4x CPU cores
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(largeTestWords, config)
	}
}

// Benchmark with high CPU multiplier
func BenchmarkCalquePipeHighCPU(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: calque.ConcurrencyAuto,
		CPUMultiplier: 8, // 8x CPU cores
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(testWords, config)
	}
}

func BenchmarkCalquePipeHighCPULarge(b *testing.B) {
	config := calque.FlowConfig{
		MaxConcurrent: calque.ConcurrencyAuto,
		CPUMultiplier: 8, // 8x CPU cores
	}
	for i := 0; i < b.N; i++ {
		GoCalqueWithConfig(largeTestWords, config)
	}
}

// Benchmark with goroutine counting
func BenchmarkGoroutineUsage(b *testing.B) {
	fmt.Printf("\n=== Goroutine Usage Analysis ===\n")

	// Baseline goroutine count
	runtime.GC()
	runtime.GC() // Run twice to ensure cleanup
	baselineGoroutines := runtime.NumGoroutine()
	fmt.Printf("Baseline goroutines: %d\n", baselineGoroutines)

	configs := []struct {
		name   string
		config *calque.FlowConfig
	}{
		{"Default", nil},
		{"Unlimited", &calque.FlowConfig{MaxConcurrent: calque.ConcurrencyUnlimited}},
		{"Fixed3", &calque.FlowConfig{MaxConcurrent: 3}},
		{"Fixed5", &calque.FlowConfig{MaxConcurrent: 5}},
		{"Auto1x", &calque.FlowConfig{MaxConcurrent: calque.ConcurrencyAuto, CPUMultiplier: 1}},
		{"Auto2x", &calque.FlowConfig{MaxConcurrent: calque.ConcurrencyAuto, CPUMultiplier: 2}},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			var maxGoroutines, minGoroutines int = 0, 999999
			var totalGoroutines int64 = 0

			for i := 0; i < b.N; i++ {
				// Measure goroutines before
				beforeGoroutines := runtime.NumGoroutine()

				// Run the pipeline
				if cfg.config == nil {
					GoCalque(testWords) // Default
				} else {
					GoCalqueWithConfig(testWords, *cfg.config)
				}

				// Measure goroutines after
				afterGoroutines := runtime.NumGoroutine()

				// Track statistics
				if afterGoroutines > maxGoroutines {
					maxGoroutines = afterGoroutines
				}
				if beforeGoroutines < minGoroutines {
					minGoroutines = beforeGoroutines
				}
				totalGoroutines += int64(afterGoroutines)
			}

			avgGoroutines := float64(totalGoroutines) / float64(b.N)
			fmt.Printf("  %s: min=%d, max=%d, avg=%.1f goroutines\n",
				cfg.name, minGoroutines, maxGoroutines, avgGoroutines)
		})
	}
}

// Test concurrent pipeline execution to see semaphore effects
func TestConcurrentPipelineExecution(t *testing.T) {
	fmt.Printf("\n=== Concurrent Execution Test ===\n")

	configs := []struct {
		name   string
		config calque.FlowConfig
	}{
		{"Unlimited", calque.FlowConfig{MaxConcurrent: calque.ConcurrencyUnlimited}},
		{"Fixed3", calque.FlowConfig{MaxConcurrent: 3}},
		{"Fixed5", calque.FlowConfig{MaxConcurrent: 5}},
		{"Auto1x", calque.FlowConfig{MaxConcurrent: calque.ConcurrencyAuto, CPUMultiplier: 1}},
		{"Auto2x", calque.FlowConfig{MaxConcurrent: calque.ConcurrencyAuto, CPUMultiplier: 2}},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			// Run 20 concurrent pipelines
			const numPipelines = 20

			// Measure baseline
			runtime.GC()
			baselineGoroutines := runtime.NumGoroutine()

			results := make(chan int, numPipelines)

			// Launch concurrent pipelines
			for i := 0; i < numPipelines; i++ {
				go func(id int) {
					// Measure goroutines during concurrent execution
					duringGoroutines := runtime.NumGoroutine()
					results <- duringGoroutines

					// Run pipeline
					GoCalqueWithConfig(testWords, cfg.config)
				}(i)
			}

			// Collect results
			var maxConcurrentGoroutines int
			for i := 0; i < numPipelines; i++ {
				goroutineCount := <-results
				if goroutineCount > maxConcurrentGoroutines {
					maxConcurrentGoroutines = goroutineCount
				}
			}

			// Wait a moment for cleanup
			runtime.GC()
			finalGoroutines := runtime.NumGoroutine()

			fmt.Printf("  %s: baseline=%d, peak=%d, final=%d, increase=%d goroutines\n",
				cfg.name, baselineGoroutines, maxConcurrentGoroutines, finalGoroutines,
				maxConcurrentGoroutines-baselineGoroutines)
		})
	}
}
