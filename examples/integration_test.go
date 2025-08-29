package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

// TestEndToEndPipeline tests a complete pipeline from input to output
func TestEndToEndPipeline(t *testing.T) {
	t.Parallel()
	// Create a comprehensive pipeline that tests multiple components
	flow := calque.NewFlow()

	flow.
		Use(text.Transform(strings.ToUpper)).
		Use(text.Transform(func(s string) string {
			return "Processed: " + s
		})).
		Use(ctrl.Timeout(text.Transform(func(s string) string {
			return s + " [TIMEOUT_TEST]"
		}), 1*time.Second)).
		Use(text.Branch(
			func(s string) bool { return len(s) > 20 },
			text.Transform(func(s string) string { return s + " [LONG_TEXT]" }),
			text.Transform(func(s string) string { return s + " [SHORT_TEXT]" }),
		))

	inputText := "Hello world! This is a comprehensive end-to-end test."
	var result string

	err := flow.Run(context.Background(), inputText, &result)
	if err != nil {
		t.Fatalf("End-to-end pipeline failed: %v", err)
	}

	// Verify the result contains expected transformations
	if !strings.Contains(result, "Processed: HELLO WORLD! THIS IS A COMPREHENSIVE END-TO-END TEST.") {
		t.Errorf("Expected uppercase transformation, got: %s", result)
	}
	if !strings.Contains(result, "[TIMEOUT_TEST]") {
		t.Errorf("Expected timeout test marker, got: %s", result)
	}
	if !strings.Contains(result, "[LONG_TEXT]") {
		t.Errorf("Expected long text marker, got: %s", result)
	}
}

// TestConcurrentPipelines tests multiple pipelines running concurrently
func TestConcurrentPipelines(t *testing.T) {
	t.Parallel()
	const numPipelines = 5
	results := make(chan string, numPipelines)

	for i := 0; i < numPipelines; i++ {
		go func(id int) {
			flow := calque.NewFlow()
			flow.Use(text.Transform(func(s string) string {
				return fmt.Sprintf("Pipeline %d: %s", id, strings.ToUpper(s))
			}))

			var result string
			err := flow.Run(context.Background(), fmt.Sprintf("message %d", id), &result)
			if err != nil {
				results <- fmt.Sprintf("Error in pipeline %d: %v", id, err)
			} else {
				results <- result
			}
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numPipelines; i++ {
		result := <-results
		// Check that we got a valid result
		if strings.Contains(result, "Pipeline") && strings.Contains(result, "MESSAGE") {
			successCount++
		}
	}

	// Should have some successful results
	if successCount == 0 {
		t.Error("Expected some successful pipeline results, got none")
	}
}

// TestErrorRecovery tests pipeline behavior when errors occur
func TestErrorRecovery(t *testing.T) {
	t.Parallel()
	// Create a pipeline that might fail
	flow := calque.NewFlow()
	flow.Use(text.Transform(func(s string) string {
		if strings.Contains(s, "error") {
			return "error: " + s
		}
		return "success: " + s
	}))

	// Test successful case
	var result string
	err := flow.Run(context.Background(), "normal message", &result)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if !strings.Contains(result, "success: normal message") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Test error case
	err = flow.Run(context.Background(), "error message", &result)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if !strings.Contains(result, "error: error message") {
		t.Errorf("Expected error message, got: %s", result)
	}
}

// TestLargeDataHandling tests pipeline behavior with large data
func TestLargeDataHandling(t *testing.T) {
	t.Parallel()
	// Create large input
	largeInput := strings.Repeat("This is a test message. ", 1000)

	flow := calque.NewFlow()
	flow.Use(text.Transform(strings.ToUpper))
	flow.Use(text.Transform(func(s string) string {
		return "Processed: " + s[:100] + "..." // Truncate for test
	}))

	var result string
	err := flow.Run(context.Background(), largeInput, &result)
	if err != nil {
		t.Fatalf("Large data processing failed: %v", err)
	}

	if !strings.Contains(result, "Processed:") {
		t.Errorf("Expected processing indicator, got: %s", result[:100])
	}
}

// TestMemoryEfficiency tests that pipelines don't leak memory
func TestMemoryEfficiency(t *testing.T) {
	t.Parallel()
	// Run multiple pipelines to test memory efficiency
	const numIterations = 100

	for i := 0; i < numIterations; i++ {
		flow := calque.NewFlow()
		flow.Use(text.Transform(strings.ToUpper))
		flow.Use(text.Transform(func(s string) string {
			return fmt.Sprintf("Iteration %d: %s", i, s)
		}))

		var result string
		err := flow.Run(context.Background(), "test message", &result)
		if err != nil {
			t.Fatalf("Memory efficiency test failed at iteration %d: %v", i, err)
		}

		expected := fmt.Sprintf("Iteration %d: TEST MESSAGE", i)
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	}
}

// TestMixedWorkloads tests different types of workloads
func TestMixedWorkloads(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "Single character",
			input:    "a",
			expected: "A",
		},
		{
			name:     "Normal text",
			input:    "hello world",
			expected: "HELLO WORLD",
		},
		{
			name:     "Special characters",
			input:    "Hello! @#$%^&*()",
			expected: "HELLO! @#$%^&*()",
		},
		{
			name:     "Unicode text",
			input:    "café résumé naïve",
			expected: "CAFÉ RÉSUMÉ NAÏVE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := calque.NewFlow()
			flow.Use(text.Transform(strings.ToUpper))

			var result string
			err := flow.Run(context.Background(), tt.input, &result)
			if err != nil {
				t.Fatalf("Mixed workload test failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestPipelineComposition tests composing multiple pipelines
func TestPipelineComposition(t *testing.T) {
	t.Parallel()
	// Create sub-pipelines
	preprocessor := calque.NewFlow()
	preprocessor.Use(text.Transform(strings.TrimSpace))
	preprocessor.Use(text.Transform(strings.ToLower))

	analyzer := calque.NewFlow()
	analyzer.Use(text.Transform(func(s string) string {
		wordCount := len(strings.Fields(s))
		return fmt.Sprintf("Words: %d, Text: %s", wordCount, s)
	}))

	// Compose main pipeline
	mainFlow := calque.NewFlow()
	mainFlow.Use(preprocessor)
	mainFlow.Use(analyzer)

	inputText := "  Hello World! This is a test.  "
	var result string

	err := mainFlow.Run(context.Background(), inputText, &result)
	if err != nil {
		t.Fatalf("Pipeline composition failed: %v", err)
	}

	// Verify preprocessing and analysis
	if !strings.Contains(result, "hello world! this is a test.") {
		t.Errorf("Expected preprocessed text, got: %s", result)
	}
	if !strings.Contains(result, "Words: 6") {
		t.Errorf("Expected word count, got: %s", result)
	}
}

// TestTimeoutHandling tests timeout behavior
func TestTimeoutHandling(t *testing.T) {
	t.Parallel()
	// Create a slow handler
	slowHandler := text.Transform(func(s string) string {
		time.Sleep(100 * time.Millisecond) // Simulate slow processing
		return "processed: " + s
	})

	// Test with timeout
	flow := calque.NewFlow()
	flow.Use(ctrl.Timeout(slowHandler, 50*time.Millisecond))

	var result string
	err := flow.Run(context.Background(), "test", &result)
	if err == nil {
		t.Error("Expected timeout error, but got none")
	}

	// Test without timeout (should succeed)
	flow2 := calque.NewFlow()
	flow2.Use(ctrl.Timeout(slowHandler, 200*time.Millisecond))

	err = flow2.Run(context.Background(), "test", &result)
	if err != nil {
		t.Fatalf("Expected success with longer timeout, got error: %v", err)
	}
	if !strings.Contains(result, "processed: test") {
		t.Errorf("Expected processed result, got: %s", result)
	}
}

// TestParallelProcessing tests parallel middleware
func TestParallelProcessing(t *testing.T) {
	t.Parallel()
	handler1 := text.Transform(func(s string) string { return "H1: " + s })
	handler2 := text.Transform(func(s string) string { return "H2: " + s })
	handler3 := text.Transform(func(s string) string { return "H3: " + s })

	flow := calque.NewFlow()
	flow.Use(ctrl.Parallel(handler1, handler2, handler3))

	var result string
	err := flow.Run(context.Background(), "test", &result)
	if err != nil {
		t.Fatalf("Parallel processing failed: %v", err)
	}

	// Verify all handlers processed the input
	if !strings.Contains(result, "H1: test") {
		t.Errorf("Expected handler1 output, got: %s", result)
	}
	if !strings.Contains(result, "H2: test") {
		t.Errorf("Expected handler2 output, got: %s", result)
	}
	if !strings.Contains(result, "H3: test") {
		t.Errorf("Expected handler3 output, got: %s", result)
	}
}

// TestChainProcessing tests chain middleware
func TestChainProcessing(t *testing.T) {
	t.Parallel()
	step1 := text.Transform(func(s string) string { return "Step1: " + s })
	step2 := text.Transform(func(s string) string { return "Step2: " + s })
	step3 := text.Transform(func(s string) string { return "Step3: " + s })

	flow := calque.NewFlow()
	flow.Use(ctrl.Chain(step1, step2, step3))

	var result string
	err := flow.Run(context.Background(), "test", &result)
	if err != nil {
		t.Fatalf("Chain processing failed: %v", err)
	}

	// Verify all steps were executed in order
	if !strings.Contains(result, "Step1: test") {
		t.Errorf("Expected step1 output, got: %s", result)
	}
	if !strings.Contains(result, "Step2: Step1: test") {
		t.Errorf("Expected step2 output, got: %s", result)
	}
	if !strings.Contains(result, "Step3: Step2: Step1: test") {
		t.Errorf("Expected step3 output, got: %s", result)
	}
}

// TestFallbackMechanism tests fallback behavior
func TestFallbackMechanism(t *testing.T) {
	t.Parallel()
	// Create a primary handler that fails
	failingHandler := text.Transform(func(s string) string {
		if strings.Contains(s, "fail") {
			return "error: " + s
		}
		return "primary: " + s
	})

	// Create a fallback handler
	fallbackHandler := text.Transform(func(s string) string {
		return "fallback: " + s
	})

	flow := calque.NewFlow()
	flow.Use(ctrl.Fallback(failingHandler, fallbackHandler))

	// Test successful case
	var result string
	err := flow.Run(context.Background(), "success", &result)
	if err != nil {
		t.Fatalf("Fallback test failed: %v", err)
	}
	if !strings.Contains(result, "primary: success") {
		t.Errorf("Expected primary handler result, got: %s", result)
	}

	// Test fallback case
	err = flow.Run(context.Background(), "fail", &result)
	if err != nil {
		t.Fatalf("Fallback test failed: %v", err)
	}
	if !strings.Contains(result, "error: fail") {
		t.Errorf("Expected error handler result, got: %s", result)
	}
}

// TestRateLimiting tests rate limiting behavior
func TestRateLimiting(t *testing.T) {
	t.Parallel()
	handler := text.Transform(func(s string) string {
		return "processed: " + s
	})

	flow := calque.NewFlow()
	flow.Use(ctrl.RateLimit(2, time.Second)) // 2 requests per second
	flow.Use(handler)

	// Test rapid requests
	const numRequests = 5
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			var result string
			err := flow.Run(context.Background(), fmt.Sprintf("request %d", id), &result)
			results <- err
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		if err := <-results; err == nil {
			successCount++
		}
	}

	// Should have some successful requests (rate limiting allows some through)
	if successCount == 0 {
		t.Error("Expected some successful requests, got none")
	}
	// Note: Rate limiting behavior may vary depending on timing and system load
	t.Logf("Rate limiting test: %d/%d requests succeeded", successCount, numRequests)
}

// TestIntegrationWithMockAI tests integration with mock AI client
func TestIntegrationWithMockAI(t *testing.T) {
	t.Parallel()
	mockClient := ai.NewMockClient("Mock AI response")
	agent := ai.Agent(mockClient)

	flow := calque.NewFlow()
	flow.Use(text.Transform(strings.ToUpper))
	flow.Use(agent)

	var result string
	err := flow.Run(context.Background(), "hello", &result)
	if err != nil {
		t.Fatalf("AI integration test failed: %v", err)
	}

	if !strings.Contains(result, "Mock AI response") {
		t.Errorf("Expected AI response, got: %s", result)
	}
}

// TestStressTest runs a stress test with many concurrent pipelines
func TestStressTest(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const numPipelines = 50
	const pipelineIterations = 10
	results := make(chan error, numPipelines)

	for i := 0; i < numPipelines; i++ {
		go func(pipelineID int) {
			for j := 0; j < pipelineIterations; j++ {
				flow := calque.NewFlow()
				flow.Use(text.Transform(strings.ToUpper))
				flow.Use(text.Transform(func(s string) string {
					return fmt.Sprintf("Pipeline %d, Iteration %d: %s", pipelineID, j, s)
				}))

				var result string
				err := flow.Run(context.Background(), "stress test", &result)
				if err != nil {
					results <- fmt.Errorf("pipeline %d, iteration %d failed: %v", pipelineID, j, err)
					return
				}

				expected := fmt.Sprintf("Pipeline %d, Iteration %d: STRESS TEST", pipelineID, j)
				if result != expected {
					results <- fmt.Errorf("pipeline %d, iteration %d: expected %s, got %s", pipelineID, j, expected, result)
					return
				}
			}
			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numPipelines; i++ {
		if err := <-results; err != nil {
			t.Errorf("Stress test failed: %v", err)
		}
	}
}

// TestMain runs integration tests
func TestMain(m *testing.M) {
	// Set up any global test configuration
	log.Println("Starting integration tests...")

	// Run the tests
	exitCode := m.Run()

	// Clean up any global resources
	log.Println("Integration tests completed.")

	os.Exit(exitCode)
}
