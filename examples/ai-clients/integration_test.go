package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
)

// TestMockClient tests basic mock AI client functionality
func TestMockClient(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Hello! I'm a mock AI assistant. How can I help you today?")
	agent := ai.Agent(mockClient)

	// Create pipeline
	flow := calque.NewFlow()
	flow.
		Use(inspect.Print("INPUT")).
		Use(agent).
		Use(inspect.Print("RESPONSE"))

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "Hello, can you help me?", &result)
	if err != nil {
		t.Fatalf("Mock client test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Hello!") {
		t.Errorf("Expected greeting in response, got: %s", result)
	}
	if !strings.Contains(result, "mock AI assistant") {
		t.Errorf("Expected mock AI assistant response, got: %s", result)
	}
}

// TestMockClientWithOptions tests mock AI client with various options
func TestMockClientWithOptions(t *testing.T) {
	t.Parallel()

	// Create mock AI client with custom options
	mockClient := ai.NewMockClient("Custom response with specific behavior")
	mockClient = mockClient.WithStreamDelay(50) // 50ms delay

	agent := ai.Agent(mockClient)

	// Create pipeline
	flow := calque.NewFlow()
	flow.Use(agent)

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "Test request", &result)
	if err != nil {
		t.Fatalf("Mock client with options test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Custom response") {
		t.Errorf("Expected custom response, got: %s", result)
	}
}

// TestMockClientErrorHandling tests mock AI client error handling
func TestMockClientErrorHandling(t *testing.T) {
	t.Parallel()

	// Create mock AI client that always returns an error
	mockClient := ai.NewMockClientWithError("Mock error for testing error handling")

	agent := ai.Agent(mockClient)

	// Create pipeline
	flow := calque.NewFlow()
	flow.Use(agent)

	// Test the pipeline - should handle errors gracefully
	var result string
	err := flow.Run(context.Background(), "Test request", &result)
	if err == nil {
		t.Error("Expected error from mock client with 100% error rate")
	}
}

// TestMockClientStreaming tests mock AI client streaming behavior
func TestMockClientStreaming(t *testing.T) {
	t.Parallel()

	// Create mock AI client with streaming
	mockClient := ai.NewMockClient("Streaming response: This is a test of streaming capabilities.")
	mockClient = mockClient.WithStreamDelay(10) // Small delay for streaming effect

	agent := ai.Agent(mockClient)

	// Create pipeline
	flow := calque.NewFlow()
	flow.Use(agent)

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "Test streaming", &result)
	if err != nil {
		t.Fatalf("Mock client streaming test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Streaming response") {
		t.Errorf("Expected streaming response, got: %s", result)
	}
}

// TestMockClientConcurrency tests mock AI client under concurrent access
func TestMockClientConcurrency(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Concurrent response: Successfully handled multiple requests.")
	agent := ai.Agent(mockClient)

	// Create pipeline
	flow := calque.NewFlow()
	flow.Use(agent)

	// Test concurrent processing
	const numRequests = 5
	results := make(chan string, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			var result string
			err := flow.Run(context.Background(), fmt.Sprintf("Request %d", id), &result)
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
			if strings.Contains(result, "Concurrent response") {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent request failed: %v", err)
		}
	}

	// Verify we got successful responses
	if successCount == 0 {
		t.Error("Expected at least one successful concurrent response")
	}
}

// TestMockClientWithPrompt tests mock AI client with prompt middleware
func TestMockClientWithPrompt(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Prompted response: I understand your request and can help you with Go programming.")
	agent := ai.Agent(mockClient)

	// Create pipeline with prompt
	flow := calque.NewFlow()
	flow.
		Use(prompt.System("You are a helpful Go programming assistant.")).
		Use(agent)

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "How do I create a goroutine?", &result)
	if err != nil {
		t.Fatalf("Mock client with prompt test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Prompted response") {
		t.Errorf("Expected prompted response, got: %s", result)
	}
	if !strings.Contains(result, "Go programming") {
		t.Errorf("Expected Go programming response, got: %s", result)
	}
}

// TestMockClientPipeline tests mock AI client in a complex pipeline
func TestMockClientPipeline(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Pipeline processed: I've analyzed your request through multiple processing steps and provided a comprehensive response.")
	agent := ai.Agent(mockClient)

	// Create complex pipeline
	flow := calque.NewFlow()
	flow.
		Use(inspect.Print("START")).
		Use(prompt.System("You are an expert AI assistant.")).
		Use(prompt.Template("Process this request: {{.Input}}")).
		Use(agent).
		Use(inspect.Print("END"))

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "Complex request for analysis", &result)
	if err != nil {
		t.Fatalf("Mock client pipeline test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Pipeline processed") {
		t.Errorf("Expected pipeline processed response, got: %s", result)
	}
	if !strings.Contains(result, "comprehensive response") {
		t.Errorf("Expected comprehensive response, got: %s", result)
	}
}

// TestMockClientDifferentResponses tests mock AI client with different response patterns
func TestMockClientDifferentResponses(t *testing.T) {
	t.Parallel()

	// Create multiple mock clients with different responses
	responses := []string{
		"Response 1: Basic information about Go",
		"Response 2: Advanced Go concepts and patterns",
		"Response 3: Go best practices and recommendations",
	}

	// Test each response
	for i, expectedResponse := range responses {
		t.Run(fmt.Sprintf("Response_%d", i+1), func(t *testing.T) {
			mockClient := ai.NewMockClient(expectedResponse)
			agent := ai.Agent(mockClient)

			flow := calque.NewFlow()
			flow.Use(agent)

			var result string
			err := flow.Run(context.Background(), "Test request", &result)
			if err != nil {
				t.Fatalf("Mock client response %d test failed: %v", i+1, err)
			}

			// Verify the result contains the expected response
			if !strings.Contains(result, fmt.Sprintf("Response %d", i+1)) {
				t.Errorf("Expected response %d, got: %s", i+1, result)
			}
		})
	}
}
