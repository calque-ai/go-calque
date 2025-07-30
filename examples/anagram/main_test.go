package main

import (
	"testing"
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
		CalquePipe(testWords)
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
		CalquePipe(largeTestWords)
	}
}
