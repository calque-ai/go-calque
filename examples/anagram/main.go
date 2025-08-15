package main

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

// Anagrams represents a map of sorted characters to words that contain them
type Anagrams map[string]map[string]struct{}

func main() {
	// Sample words for testing
	words := []string{
		"listen", "silent", "hello", "world", "act", "cat", "tac",
		"race", "care", "acre", "programming", "go", "og", "a", "b",
		"stressed", "desserts", "evil", "live", "vile",
	}

	fmt.Println("Anagram Processing Example")
	fmt.Println("==========================")

	// Test baseline implementation
	fmt.Println("\n1. Baseline Implementation:")
	baseline := Baseline(words)
	printAnagrams(baseline)

	// Test framework-heavy implementation
	fmt.Println("\n3. GoCalque Framework Implementation:")
	goCalqueFramework := GoCalqueFramework(words)
	printAnagrams(goCalqueFramework)

	// Verify all produce same results
	fmt.Println("\n4. Results Match:")
	fmt.Printf("  Baseline vs Framework: %v\n", compareAnagrams(baseline, goCalqueFramework))
}

// GoCalqueFramework implementation using maximum framework leverage
// This version uses the calque framework to process words into anagrams
// using a series of middleware handlers for filtering, transforming, and accumulating results.
func GoCalqueFramework(words []string) map[string]map[string]struct{} {
	if len(words) == 0 {
		return nil
	}

	// Convert words to input stream
	input := strings.Join(words, "\n")

	// Create flow using optimal middleware for each task:
	// 1. Filter out invalid words (text.Filter)
	// 2. Convert to lowercase (text.Transform)
	// 3. Convert each word to sorted key format (text.Transform)
	// 4. Accumulate and group results (custom handler)
	flow := calque.NewFlow().
		Use(filterValidWords()).
		Use(lowercaseTransform()).
		Use(wordsToSortedKeys()).
		Use(accumulateAnagrams())

	var outputStr string
	err := flow.Run(context.Background(), input, &outputStr)
	if err != nil {
		return nil
	}

	// Parse the accumulated result
	return parseAnagramOutput(outputStr)
}

// Baseline implementation (from benchmark)
// This is a simple implementation that processes words into anagrams
func Baseline(words []string) map[string]map[string]struct{} {
	var swa []Anagrams
	for _, w := range words {
		if !moreThanOneChar(w) {
			continue
		}
		swa = append(swa, singleWordToMap(strings.ToLower(w)))
	}
	if len(swa) == 0 {
		return nil
	}
	seed := swa[0]
	for i := range swa[1:] {
		seed = accumulate(seed, swa[i])
	}
	return seed
}

// MoreThanOneChar checks if a word has more than one character
func moreThanOneChar(word string) bool {
	return len(word) > 1
}

// SingleWordToMap converts a single word to its anagram representation
func singleWordToMap(word string) Anagrams {
	// Sort the characters in the word to create the key
	chars := []rune(word)
	sort.Slice(chars, func(i, j int) bool {
		return chars[i] < chars[j]
	})
	key := string(chars)

	// Create the anagram map
	result := make(Anagrams)
	result[key] = make(map[string]struct{})
	result[key][word] = struct{}{}

	return result
}

// Accumulate merges two anagram maps
func accumulate(a, b Anagrams) Anagrams {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	result := make(Anagrams)

	// Copy from a
	for key, words := range a {
		result[key] = make(map[string]struct{})
		for word := range words {
			result[key][word] = struct{}{}
		}
	}

	// Merge from b
	for key, words := range b {
		if result[key] == nil {
			result[key] = make(map[string]struct{})
		}
		for word := range words {
			result[key][word] = struct{}{}
		}
	}

	return result
}

// accumulateAnagrams collects all anagram mappings
func accumulateAnagrams() calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		if err := calque.Read(r, &input); err != nil {
			return err
		}

		result := make(Anagrams)
		lines := strings.SplitSeq(string(input), "\n")

		for line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			key, word := parts[0], parts[1]
			if result[key] == nil {
				result[key] = make(map[string]struct{})
			}
			result[key][word] = struct{}{}
		}

		// Output in a parseable format
		var lines_out []string
		for key, wordSet := range result {
			var wordList []string
			for word := range wordSet {
				wordList = append(wordList, word)
			}
			sort.Strings(wordList)
			lines_out = append(lines_out, fmt.Sprintf("%s=%s", key, strings.Join(wordList, ",")))
		}

		return calque.Write(w, strings.Join(lines_out, "\n"))
	})
}

// parseAnagramOutput converts the pipeline output back to Anagrams
func parseAnagramOutput(output string) map[string]map[string]struct{} {
	result := make(Anagrams)
	lines := strings.SplitSeq(output, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		words := strings.Split(parts[1], ",")

		result[key] = make(map[string]struct{})
		for _, word := range words {
			if word != "" {
				result[key][word] = struct{}{}
			}
		}
	}

	return result
}

// Optimized framework middleware functions

// filterValidWords filters out empty lines and single characters using text.Filter
func filterValidWords() calque.Handler {
	return text.Filter(
		func(s string) bool {
			// Check if input contains any valid words (>1 char, non-empty)
			lines := strings.Split(s, "\n")
			for _, line := range lines {
				word := strings.TrimSpace(line)
				if len(word) > 1 {
					return true
				}
			}
			return false
		},
		text.Transform(func(s string) string {
			// Filter and clean lines
			lines := strings.Split(s, "\n")
			var validLines []string
			for _, line := range lines {
				word := strings.TrimSpace(line)
				if len(word) > 1 {
					validLines = append(validLines, word)
				}
			}
			return strings.Join(validLines, "\n")
		}),
	)
}

// lowercaseTransform converts entire input to lowercase using text.Transform
func lowercaseTransform() calque.Handler {
	return text.Transform(strings.ToLower)
}

// wordsToSortedKeys converts words to "sortedkey:word" format using text.Transform
func wordsToSortedKeys() calque.Handler {
	return text.Transform(func(s string) string {
		lines := strings.Split(s, "\n")
		var keyValuePairs []string

		for _, line := range lines {
			word := strings.TrimSpace(line)
			if word == "" {
				continue
			}

			// Sort characters to create anagram key
			chars := []rune(word)
			slices.Sort(chars)
			key := string(chars)

			keyValuePairs = append(keyValuePairs, fmt.Sprintf("%s:%s", key, word))
		}

		return strings.Join(keyValuePairs, "\n")
	})
}

func printAnagrams(anagrams map[string]map[string]struct{}) {
	if anagrams == nil {
		fmt.Println("  No anagrams found")
		return
	}

	for key, words := range anagrams {
		if len(words) > 1 { // Only show groups with multiple anagrams
			var wordList []string
			for word := range words {
				wordList = append(wordList, word)
			}
			sort.Strings(wordList)
			fmt.Printf("  %s: %v\n", key, wordList)
		}
	}
}

func compareAnagrams(a, b map[string]map[string]struct{}) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}

	if len(a) != len(b) {
		return false
	}

	for key, wordsA := range a {
		wordsB, exists := b[key]
		if !exists || len(wordsA) != len(wordsB) {
			return false
		}

		for word := range wordsA {
			if _, exists := wordsB[word]; !exists {
				return false
			}
		}
	}

	return true
}
