package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

// TestTextOnlyFlow tests the basic text transformation pipeline
func TestTextOnlyFlow(t *testing.T) {
	t.Parallel()
	flow := calque.NewFlow()

	flow.
		Use(inspect.Print("INPUT")).
		Use(text.Transform(strings.ToUpper)).
		Use(inspect.Print("TRANSFORMED")).
		Use(text.Branch(
			func(s string) bool { return strings.Contains(s, "HELLO") },
			text.Transform(func(s string) string { return s + " [GREETING DETECTED]" }),
			text.Transform(func(s string) string { return s + " [GENERAL TEXT]" }),
		)).
		Use(text.Transform(func(s string) string { return s + "\nLength: " + string(rune(len(s))) }))

	// Test with conversation data
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "Greeting message",
			input: "Hello world! This is a test.",
			expected: []string{
				"HELLO WORLD! THIS IS A TEST.",
				"[GREETING DETECTED]",
				"Length:",
			},
		},
		{
			name:  "Business email",
			input: "Please review the quarterly report and provide feedback by Friday.",
			expected: []string{
				"PLEASE REVIEW THE QUARTERLY REPORT AND PROVIDE FEEDBACK BY FRIDAY.",
				"[GENERAL TEXT]",
				"Length:",
			},
		},
		{
			name:  "Technical documentation",
			input: "The API endpoint /api/v1/users accepts POST requests with JSON payload.",
			expected: []string{
				"THE API ENDPOINT /API/V1/USERS ACCEPTS POST REQUESTS WITH JSON PAYLOAD.",
				"[GENERAL TEXT]",
				"Length:",
			},
		},
		{
			name:  "Multilingual text",
			input: "Hello! Bonjour! ¡Hola! 你好! مرحبا!",
			expected: []string{
				"HELLO! BONJOUR! ¡HOLA! 你好! مرحبا!",
				"[GREETING DETECTED]",
				"Length:",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := flow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}

			// Verify all expected transformations
			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

// TestStreamingPipeline tests the streaming capabilities with multi-line data
func TestStreamingPipeline(t *testing.T) {
	t.Parallel()
	flow := calque.NewFlow()
	flow.
		Use(text.LineProcessor(func(line string) string {
			return "[STREAM-" + string(rune(len(line))) + "] " + strings.TrimSpace(line)
		})).
		Use(text.LineProcessor(func(line string) string {
			return "FINAL: " + strings.ToUpper(line)
		}))

	// Test with multi-line data
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "Log file processing",
			input: `2024-01-15 10:30:15 INFO: User login successful
2024-01-15 10:30:16 DEBUG: Loading user preferences
2024-01-15 10:30:17 ERROR: Database connection failed`,
			expected: []string{
				"FINAL:",
				"STREAM-",
				"USER LOGIN SUCCESSFUL",
				"LOADING USER PREFERENCES",
				"DATABASE CONNECTION FAILED",
			},
		},
		{
			name: "CSV-like data",
			input: `Name,Age,City
John,25,New York
Alice,30,London
Bob,35,Tokyo`,
			expected: []string{
				"FINAL:",
				"STREAM-",
				"NAME,AGE,CITY",
				"JOHN,25,NEW YORK",
				"ALICE,30,LONDON",
				"BOB,35,TOKYO",
			},
		},
		{
			name: "Empty lines and whitespace",
			input: `First line

Second line
   Third line with spaces   
`,
			expected: []string{
				"FINAL:",
				"STREAM-",
				"FIRST LINE",
				"SECOND LINE",
				"THIRD LINE WITH SPACES",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := flow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Streaming flow execution failed: %v", err)
			}

			// Verify streaming processed each line
			lines := strings.Split(result, "\n")
			if len(lines) < 2 {
				t.Errorf("Expected multiple lines, got: %s", result)
			}

			// Check that we have the expected content
			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected to contain %q, got: %s", expected, result)
				}
			}

			// Count non-empty lines
			nonEmptyLines := 0
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					nonEmptyLines++
				}
			}

			if nonEmptyLines < 2 {
				t.Errorf("Expected at least 2 non-empty lines, got %d", nonEmptyLines)
			}
		})
	}
}

// TestComposedPipeline tests pipeline composition with business scenarios
func TestComposedPipeline(t *testing.T) {
	t.Parallel()
	// Build reusable sub-pipeline for text preprocessing
	textPreprocessor := calque.NewFlow()
	textPreprocessor.
		Use(text.Transform(strings.TrimSpace)).
		Use(text.Transform(func(s string) string {
			return strings.Join(strings.Fields(s), " ")
		})).
		Use(text.Transform(strings.ToLower))

	// Build reusable sub-pipeline for text analysis
	textAnalyzer := calque.NewFlow()
	textAnalyzer.
		Use(text.Transform(func(s string) string {
			wordCount := len(strings.Fields(s))
			charCount := len(s)
			sentenceCount := len(strings.Split(s, "."))
			return fmt.Sprintf("TEXT: %s\nSTATS: %d words, %d characters, %d sentences", s, wordCount, charCount, sentenceCount)
		}))

	// Build main pipeline that composes the sub-pipelines
	mainFlow := calque.NewFlow()
	mainFlow.
		Use(textPreprocessor).
		Use(text.Branch(
			func(s string) bool { return len(s) > 50 },
			ctrl.Chain(
				text.Transform(func(s string) string { return s + " [LONG TEXT DETECTED]" }),
				textAnalyzer,
			),
			text.Transform(func(s string) string { return s + " [SHORT TEXT - BASIC PROCESSING]" }),
		))

	// Test with business scenarios
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "Long business report",
			input: "  This is a comprehensive quarterly business report that contains detailed financial analysis, market trends, and strategic recommendations for the upcoming fiscal year.   ",
			expected: []string{
				"[LONG TEXT DETECTED]",
				"STATS:",
				"words,",
				"characters,",
				"sentences",
			},
		},
		{
			name:  "Short status update",
			input: "Meeting confirmed",
			expected: []string{
				"[SHORT TEXT - BASIC PROCESSING]",
			},
		},
		{
			name:  "Technical specification",
			input: "  The new microservice architecture implements RESTful APIs with JWT authentication, Redis caching, and PostgreSQL database. It supports horizontal scaling and includes comprehensive monitoring and logging.   ",
			expected: []string{
				"[LONG TEXT DETECTED]",
				"STATS:",
				"words,",
				"characters,",
				"sentences",
			},
		},
		{
			name:  "Empty input with whitespace",
			input: "   \n\t  \n   ",
			expected: []string{
				"[SHORT TEXT - BASIC PROCESSING]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := mainFlow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Pipeline execution failed: %v", err)
			}

			// Verify expected transformations
			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

// TestTimeoutHandling tests timeout middleware with scenarios
func TestTimeoutHandling(t *testing.T) {
	t.Parallel()
	// Create a slow handler that simulates real-world processing
	slowHandler := text.Transform(func(s string) string {
		// Simulate different processing times based on input
		switch {
		case strings.Contains(s, "heavy"):
			time.Sleep(200 * time.Millisecond) // Heavy processing
		case strings.Contains(s, "medium"):
			time.Sleep(100 * time.Millisecond) // Medium processing
		default:
			time.Sleep(20 * time.Millisecond) // Light processing
		}
		return "processed: " + s
	})

	testCases := []struct {
		name        string
		input       string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "Heavy processing with short timeout",
			input:       "heavy data processing task",
			timeout:     50 * time.Millisecond,
			expectError: true,
		},
		{
			name:        "Medium processing with adequate timeout",
			input:       "medium complexity analysis",
			timeout:     150 * time.Millisecond,
			expectError: false,
		},
		{
			name:        "Light processing with short timeout",
			input:       "simple text",
			timeout:     50 * time.Millisecond,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flow := calque.NewFlow()
			flow.Use(ctrl.Timeout(slowHandler, tc.timeout))

			var result string
			err := flow.Run(context.Background(), tc.input, &result)

			if tc.expectError {
				if err == nil {
					t.Error("Expected timeout error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
				if !strings.Contains(result, "processed:") {
					t.Errorf("Expected processed result, got: %s", result)
				}
			}
		})
	}
}

// TestParallelProcessing tests parallel middleware with workloads
func TestParallelProcessing(t *testing.T) {
	t.Parallel()
	// Create handlers that simulate different types of processing
	textProcessor := text.Transform(func(s string) string { return "TEXT: " + s })
	sentimentAnalyzer := text.Transform(func(s string) string { return "SENTIMENT: " + s })
	keywordExtractor := text.Transform(func(s string) string { return "KEYWORDS: " + s })
	languageDetector := text.Transform(func(s string) string { return "LANGUAGE: " + s })

	flow := calque.NewFlow()
	flow.Use(ctrl.Parallel(textProcessor, sentimentAnalyzer, keywordExtractor, languageDetector))

	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "Customer feedback",
			input: "The product is excellent and exceeded my expectations!",
			expected: []string{
				"TEXT: The product is excellent and exceeded my expectations!",
				"SENTIMENT: The product is excellent and exceeded my expectations!",
				"KEYWORDS: The product is excellent and exceeded my expectations!",
				"LANGUAGE: The product is excellent and exceeded my expectations!",
			},
		},
		{
			name:  "Technical support ticket",
			input: "Error 404 when accessing /api/users endpoint",
			expected: []string{
				"TEXT: Error 404 when accessing /api/users endpoint",
				"SENTIMENT: Error 404 when accessing /api/users endpoint",
				"KEYWORDS: Error 404 when accessing /api/users endpoint",
				"LANGUAGE: Error 404 when accessing /api/users endpoint",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := flow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Parallel flow execution failed: %v", err)
			}

			// Verify all handlers processed the input
			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

// TestErrorHandling tests error scenarios with error conditions
func TestErrorHandling(t *testing.T) {
	t.Parallel()
	// Create handlers that simulate different types of errors
	validationError := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		calque.Read(r, &input)
		if len(input) < 5 {
			return fmt.Errorf("validation error: input too short (minimum 5 characters)")
		}
		return calque.Write(w, "valid: "+input)
	})

	processingError := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		calque.Read(r, &input)
		if strings.Contains(input, "error") {
			return fmt.Errorf("processing error: invalid content detected")
		}
		return calque.Write(w, "processed: "+input)
	})

	testCases := []struct {
		name        string
		handler     calque.Handler
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Validation error - too short",
			handler:     validationError,
			input:       "Hi",
			expectError: true,
			errorMsg:    "validation error: input too short",
		},
		{
			name:        "Validation success",
			handler:     validationError,
			input:       "Hello world",
			expectError: false,
		},
		{
			name:        "Processing error - invalid content",
			handler:     processingError,
			input:       "This contains error in the text",
			expectError: true,
			errorMsg:    "processing error: invalid content",
		},
		{
			name:        "Processing success",
			handler:     processingError,
			input:       "This is valid content",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flow := calque.NewFlow()
			flow.Use(tc.handler)

			var result string
			err := flow.Run(context.Background(), tc.input, &result)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				} else if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("Expected error message containing %q, got: %v", tc.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
			}
		})
	}
}

// TestEmptyInput tests handling of empty input with various edge cases
func TestEmptyInput(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace only",
			input:    "   \n\t  ",
			expected: "   \n\t  ",
		},
		{
			name:     "Null bytes",
			input:    "\x00\x00\x00",
			expected: "\x00\x00\x00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flow := calque.NewFlow()
			flow.Use(text.Transform(strings.ToUpper))

			var result string
			err := flow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Empty input flow execution failed: %v", err)
			}

			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestLargeInput tests handling of large input sizes
func TestLargeInput(t *testing.T) {
	t.Parallel()
	// Create large inputs
	largeInputs := []struct {
		name  string
		input string
	}{
		{
			name:  "Large JSON document",
			input: strings.Repeat(`{"id": 123, "name": "Test User", "email": "test@example.com", "data": "This is a large field with lots of content"}, `, 1000),
		},
		{
			name:  "Large log file",
			input: strings.Repeat("2024-01-15 10:30:15 INFO: Processing request for user 12345 with parameters: {param1: value1, param2: value2, param3: value3}\n", 1000),
		},
		{
			name:  "Large text document",
			input: strings.Repeat("This is a comprehensive document that contains detailed information about various topics including technology, business, and science. ", 1000),
		},
	}

	for _, tc := range largeInputs {
		t.Run(tc.name, func(t *testing.T) {
			flow := calque.NewFlow()
			flow.Use(text.Transform(strings.ToUpper))
			flow.Use(text.Transform(func(s string) string {
				// Simulate processing - count words and characters
				wordCount := len(strings.Fields(s))
				charCount := len(s)
				return fmt.Sprintf("Processed: %d words, %d characters, sample: %s...", wordCount, charCount, s[:100])
			}))

			var result string
			err := flow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Large input flow execution failed: %v", err)
			}

			if !strings.Contains(result, "Processed:") {
				t.Errorf("Expected processing indicator, got: %s", result[:100])
			}

			// Verify we get reasonable word and character counts
			if !strings.Contains(result, "words,") || !strings.Contains(result, "characters,") {
				t.Errorf("Expected word and character counts, got: %s", result[:100])
			}
		})
	}
}

// TestUnicodeHandling tests handling of Unicode and special characters
func TestUnicodeHandling(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Emojis",
			input:    "Hello 👋 World 🌍!",
			expected: "HELLO 👋 WORLD 🌍!",
		},
		{
			name:     "Accented characters",
			input:    "café résumé naïve",
			expected: "CAFÉ RÉSUMÉ NAÏVE",
		},
		{
			name:     "CJK characters",
			input:    "你好世界 こんにちは世界 안녕하세요 세계",
			expected: "你好世界 こんにちは世界 안녕하세요 세계",
		},
		{
			name:     "Arabic text",
			input:    "مرحبا بالعالم",
			expected: "مرحبا بالعالم",
		},
		{
			name:     "Special symbols",
			input:    "© ® ™ € £ ¥ $ ¢",
			expected: "© ® ™ € £ ¥ $ ¢",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flow := calque.NewFlow()
			flow.Use(text.Transform(strings.ToUpper))

			var result string
			err := flow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Unicode handling test failed: %v", err)
			}

			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestPerformanceCharacteristics tests performance with workloads
func TestPerformanceCharacteristics(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a processing pipeline
	flow := calque.NewFlow()
	flow.
		Use(text.Transform(strings.TrimSpace)).
		Use(text.Transform(strings.ToLower)).
		Use(text.Transform(func(s string) string {
			// Simulate text analysis
			words := strings.Fields(s)
			wordCount := len(words)
			charCount := len(s)

			// Count vowels for simple analysis
			vowelCount := 0
			for _, char := range s {
				if strings.ContainsRune("aeiou", unicode.ToLower(char)) {
					vowelCount++
				}
			}

			return fmt.Sprintf("Analysis: %d words, %d characters, %d vowels, text: %s",
				wordCount, charCount, vowelCount, s)
		}))

	// Test with various input sizes
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Small text",
			input: "Hello world",
		},
		{
			name:  "Medium text",
			input: strings.Repeat("This is a medium-sized text for testing performance characteristics. ", 10),
		},
		{
			name:  "Large text",
			input: strings.Repeat("This is a large text that will test the performance of the pipeline  volumes. ", 100),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			var result string
			err := flow.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Performance test failed: %v", err)
			}

			duration := time.Since(start)

			// Verify the result contains expected analysis
			if !strings.Contains(result, "Analysis:") {
				t.Errorf("Expected analysis result, got: %s", result[:100])
			}

			// Log performance metrics
			t.Logf("Processed %d characters in %v", len(tc.input), duration)

			// Basic performance assertion (should complete within reasonable time)
			if duration > 5*time.Second {
				t.Errorf("Processing took too long: %v", duration)
			}
		})
	}
}
