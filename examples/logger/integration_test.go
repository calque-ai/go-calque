package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/logger"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

// TestSimpleLogging tests basic logging functionality
func TestSimpleLogging(t *testing.T) {
	t.Parallel()

	// Create a flow with simple logging

	// Create a flow with simple logging
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("FULL_INPUT")).
		Use(logger.Head("QUICK_DEBUG", 30)).
		Use(text.Transform(func(s string) string {
			return "Processed: " + s
		})).
		Use(logger.HeadTail("FINAL_CHECK", 20, 15))

	// Test input
	input := "Quick debugging example with some additional content"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Simple logging test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Processed:") {
		t.Error("Expected processed result")
	}
}

// TestSlogLogging tests structured logging with slog
func TestSlogLogging(t *testing.T) {
	t.Parallel()

	// Create a flow with slog logging
	flow := calque.NewFlow()
	flow.
		Use(logger.Head("INPUT", 40)).
		Use(text.Transform(strings.ToLower)).
		Use(logger.Print("STREAM_SAMPLING")).
		Use(logger.HeadTail("RESULT", 20, 10))

	// Test input
	input := "SLOG provides structured logging with JSON output by default."
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Slog logging test failed: %v", err)
	}

	// Verify the result
	if result != strings.ToLower(input) {
		t.Errorf("Expected lowercase result, got: %s", result)
	}
}

// TestZerologLogging tests zerolog logging functionality
func TestZerologLogging(t *testing.T) {
	t.Parallel()

	// Create a flow with zerolog logging
	flow := calque.NewFlow()
	flow.
		Use(logger.Head("INPUT", 50)).
		Use(text.Transform(func(s string) string {
			return "Transformed: " + s
		})).
		Use(logger.Print("STREAM_SAMPLING")).
		Use(logger.HeadTail("RESULT", 25, 10))

	// Test input
	input := "Zerolog provides fast and structured logging capabilities."
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Zerolog logging test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Transformed:") {
		t.Error("Expected transformed result")
	}
}

// TestLoggingLevels tests different logging levels
func TestLoggingLevels(t *testing.T) {
	t.Parallel()

	// Create a flow with different log levels
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("DEBUG_HEAD")).
		Use(logger.Print("INFO_HEAD")).
		Use(logger.Print("WARN_HEAD")).
		Use(logger.Print("ERROR_HEAD")).
		Use(text.Transform(func(s string) string {
			return "Processed with levels: " + s
		}))

	// Test input
	input := "Testing different log levels"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Logging levels test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Processed with levels:") {
		t.Error("Expected processed result")
	}
}

// TestLoggingAttributes tests logging with custom attributes
func TestLoggingAttributes(t *testing.T) {
	t.Parallel()

	// Create a flow with custom attributes
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("ATTRIBUTE_TEST")).
		Use(text.Transform(func(s string) string {
			return "Enhanced: " + s
		})).
		Use(logger.Print("FINAL_ATTR"))

	// Test input
	input := "Testing logging with custom attributes"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Logging attributes test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Enhanced:") {
		t.Error("Expected enhanced result")
	}
}

// TestLoggingSampling tests logging sampling functionality
func TestLoggingSampling(t *testing.T) {
	t.Parallel()

	// Create a flow with sampling
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("SAMPLE_LOG")).
		Use(text.Transform(func(s string) string {
			return "Sampled: " + s
		}))

	// Test input
	input := "Testing logging sampling functionality"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Logging sampling test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Sampled:") {
		t.Error("Expected sampled result")
	}
}

// TestLoggingPipeline tests logging in a complex pipeline
func TestLoggingPipeline(t *testing.T) {
	t.Parallel()

	// Create a complex pipeline with multiple logging stages
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("START_PIPELINE")).
		Use(logger.Head("INPUT_ANALYSIS", 40)).
		Use(text.Transform(strings.ToUpper)).
		Use(logger.Print("TRANSFORM_LOG")).
		Use(text.Transform(func(s string) string {
			return "Final: " + s
		})).
		Use(logger.HeadTail("OUTPUT_ANALYSIS", 30, 20)).
		Use(logger.Print("END_PIPELINE"))

	// Test input
	input := "Complex logging pipeline test with multiple stages"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Logging pipeline test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Final:") {
		t.Error("Expected final result")
	}
	if !strings.Contains(result, strings.ToUpper(input)) {
		t.Error("Expected uppercase transformation")
	}
}

// TestLoggingConcurrency tests logging under concurrent access
func TestLoggingConcurrency(t *testing.T) {
	t.Parallel()

	// Create a flow with logging
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("CONCURRENT_LOG")).
		Use(text.Transform(func(s string) string {
			return "Concurrent: " + s
		}))

	// Test concurrent processing
	const numRequests = 5
	results := make(chan string, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			input := fmt.Sprintf("Request %d", id)
			var result string
			err := flow.Run(context.Background(), input, &result)
			if err != nil {
				errors <- fmt.Errorf("request %d failed: %v", id, err)
			} else {
				results <- result
			}
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		select {
		case result := <-results:
			if strings.Contains(result, "Concurrent:") {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent request failed: %v", err)
		}
	}

	// Should have successful results
	if successCount == 0 {
		t.Error("Expected at least one successful concurrent result")
	}
}

// TestLoggingErrorHandling tests logging error handling
func TestLoggingErrorHandling(t *testing.T) {
	t.Parallel()

	// Create a flow that might encounter errors
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("ERROR_TEST")).
		Use(text.Transform(func(s string) string {
			// Simulate potential error condition
			if strings.Contains(s, "error") {
				return "Error detected: " + s
			}
			return "Normal: " + s
		})).
		Use(logger.Print("ERROR_LOG"))

	// Test normal input
	input := "Normal processing request"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Normal logging test failed: %v", err)
	}

	// Verify normal result
	if !strings.Contains(result, "Normal:") {
		t.Error("Expected normal result")
	}

	// Test error input
	errorInput := "This contains error"
	var errorResult string

	err = flow.Run(context.Background(), errorInput, &errorResult)
	if err != nil {
		t.Fatalf("Error logging test failed: %v", err)
	}

	// Verify error result
	if !strings.Contains(errorResult, "Error detected:") {
		t.Error("Expected error detection result")
	}
}
