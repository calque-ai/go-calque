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

//cpu: VirtualApple @ 2.50GHz

//   BenchmarkBaseline-10    	   		  18306	     63639 ns/op	   76736 B/op	     685 allocs/op
//   BenchmarkGoCalqueFramework-10    	  23707	     48668 ns/op	   32340 B/op	     430 allocs/op
//   BenchmarkBaselineLarge-10    	        296	   3956434 ns/op	 4011701 B/op	   33990 allocs/op
//   BenchmarkGoCalqueFrameworkLarge-10    2193	    499308 ns/op	  469170 B/op	    5489 allocs/op

// Baseline small dataset benchmarks
func BenchmarkBaseline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Baseline(testWords)
	}
}

// Baseline large dataset benchmarks
func BenchmarkBaselineLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Baseline(largeTestWords)
	}
}

// GoCalque small dataset benchmarks
func BenchmarkGoCalqueFramework(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GoCalqueFramework(testWords)
	}
}

// GoCalque large dataset benchmarks
func BenchmarkGoCalqueFrameworkLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GoCalqueFramework(largeTestWords)
	}
}
