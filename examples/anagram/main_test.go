package main

import (
	"strings"
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

// Mariomac-style implementation (simulated)
func Mariomac(words []string) map[string]map[string]struct{} {
	filtered := make([]string, 0, len(words))
	for _, w := range words {
		if MoreThanOneChar(w) {
			filtered = append(filtered, strings.ToLower(w))
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	mapped := make([]Anagrams, 0, len(filtered))
	for _, w := range filtered {
		mapped = append(mapped, SingleWordToMap(w))
	}

	seed := mapped[0]
	for i := range mapped[1:] {
		seed = Accumulate(seed, mapped[i])
	}
	return seed
}

// Lambda-style implementation (simulated)
func Lambda(words []string) map[string]map[string]struct{} {
	filtered := make([]string, 0, len(words))
	for _, w := range words {
		if MoreThanOneChar(w) {
			filtered = append(filtered, strings.ToLower(w))
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	mapped := make([]Anagrams, 0, len(filtered))
	for _, w := range filtered {
		mapped = append(mapped, SingleWordToMap(w))
	}

	seed := mapped[0]
	for i := range mapped[1:] {
		seed = Accumulate(seed, mapped[i])
	}
	return seed
}

// Test correctness of all implementations
func TestImplementations(t *testing.T) {
	baseline := Baseline(testWords)
	calquePipe := CalquePipe(testWords)
	mariomac := Mariomac(testWords)
	lambda := Lambda(testWords)

	if !compareAnagrams(baseline, calquePipe) {
		t.Error("CalquePipe implementation doesn't match baseline")
	}

	if !compareAnagrams(baseline, mariomac) {
		t.Error("Mariomac implementation doesn't match baseline")
	}

	if !compareAnagrams(baseline, lambda) {
		t.Error("Lambda implementation doesn't match baseline")
	}
}

// Test with empty input
func TestEmptyInput(t *testing.T) {
	empty := []string{}

	if result := Baseline(empty); result != nil {
		t.Error("Baseline should return nil for empty input")
	}

	if result := CalquePipe(empty); result != nil {
		t.Error("CalquePipe should return nil for empty input")
	}
}

// Test with single character words only
func TestSingleCharWords(t *testing.T) {
	singleChars := []string{"a", "b", "c", "d"}

	if result := Baseline(singleChars); result != nil {
		t.Error("Baseline should return nil for single char words only")
	}

	if result := CalquePipe(singleChars); result != nil {
		t.Error("CalquePipe should return nil for single char words only")
	}
}

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

func BenchmarkMariomac(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Mariomac(testWords)
	}
}

func BenchmarkLambda(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Lambda(testWords)
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

func BenchmarkMariomacLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Mariomac(largeTestWords)
	}
}

func BenchmarkLambdaLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Lambda(largeTestWords)
	}
}

// Utility function to verify anagram correctness
func TestAnagramDetection(t *testing.T) {
	words := []string{"listen", "silent", "enlist"}
	result := Baseline(words)

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// All three words should be in the same anagram group
	var foundGroup map[string]struct{}
	var groupCount int

	for _, wordSet := range result {
		if len(wordSet) > 1 {
			groupCount++
			foundGroup = wordSet
		}
	}

	if groupCount != 1 {
		t.Errorf("Expected 1 anagram group, got %d", groupCount)
	}

	expectedWords := []string{"listen", "silent", "enlist"}
	for _, word := range expectedWords {
		if _, exists := foundGroup[word]; !exists {
			t.Errorf("Word %s not found in anagram group", word)
		}
	}
}
