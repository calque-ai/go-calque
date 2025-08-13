package main

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/calque-ai/calque-pipe/pkg/middleware/text"
)

// Anagrams represents a map of sorted characters to words that contain them
type Anagrams map[string]map[string]struct{}

// MoreThanOneChar checks if a word has more than one character
func MoreThanOneChar(word string) bool {
	return len(word) > 1
}

// SingleWordToMap converts a single word to its anagram representation
func SingleWordToMap(word string) Anagrams {
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
func Accumulate(a, b Anagrams) Anagrams {
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

// Baseline implementation (from benchmark)
func Baseline(words []string) map[string]map[string]struct{} {
	var swa []Anagrams
	for _, w := range words {
		if !MoreThanOneChar(w) {
			continue
		}
		swa = append(swa, SingleWordToMap(strings.ToLower(w)))
	}
	if len(swa) == 0 {
		return nil
	}
	seed := swa[0]
	for i := range swa[1:] {
		seed = Accumulate(seed, swa[i])
	}
	return seed
}

// CalquePipe implementation using the calque-pipe framework
func CalquePipe(words []string) map[string]map[string]struct{} {
	if len(words) == 0 {
		return nil
	}

	// Convert words to input stream
	input := strings.Join(words, "\n")

	// Create pipeline that:
	// 1. Filters words (more than one char, lowercase)
	// 2. Maps each word to anagram format
	// 3. Accumulates results
	pipeline := calque.NewFlow().
		Use(filterAndLowercase()).
		Use(mapToAnagramFormat()).
		Use(accumulateAnagrams())

	var outputStr string
	err := pipeline.Run(context.Background(), input, &outputStr)
	if err != nil {
		return nil
	}

	// Parse the accumulated result
	return parseAnagramOutput(outputStr)
}

// filterAndLowercase filters words with more than one character and converts to lowercase
func filterAndLowercase() calque.Handler {
	return text.LineProcessor(func(line string) string {
		word := strings.TrimSpace(line)
		if MoreThanOneChar(word) {
			return strings.ToLower(word)
		}
		return "" // Empty lines will be filtered out
	})
}

// mapToAnagramFormat converts each word line to "sortedkey:word" format
func mapToAnagramFormat() calque.Handler {
	return text.LineProcessor(func(line string) string {
		word := strings.TrimSpace(line)
		if word == "" {
			return ""
		}

		// Sort characters to create anagram key
		chars := []rune(word)
		slices.Sort(chars)
		key := string(chars)

		return fmt.Sprintf("%s:%s", key, word)
	})
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

	// Test calque-pipe implementation
	fmt.Println("\n2. Calque-Pipe Implementation:")
	calquePipe := CalquePipe(words)
	printAnagrams(calquePipe)

	// Verify both produce same results
	fmt.Println("\n3. Results Match:", compareAnagrams(baseline, calquePipe))
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
