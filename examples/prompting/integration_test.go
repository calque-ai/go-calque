package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/logger"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
)

// TestBasicPromptTemplate tests basic prompt template functionality
func TestBasicPromptTemplate(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Hello! I'm a helpful AI assistant. How can I help you today?")
	agent := ai.Agent(mockClient)

	// Create basic prompt template
	template := prompt.Template("Hello! {{.Input}}")

	// Create pipeline
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("INPUT")).
		Use(template).
		Use(logger.Print("PROMPT")).
		Use(agent)

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "I need help with Go programming", &result)
	if err != nil {
		t.Fatalf("Basic prompt template test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Hello!") {
		t.Errorf("Expected greeting in response, got: %s", result)
	}
}

// TestSystemPrompt tests system prompt functionality
func TestSystemPrompt(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("I'm a coding expert. I can help you with Go programming, debugging, and best practices.")
	agent := ai.Agent(mockClient)

	// Create system prompt
	systemPrompt := prompt.System("You are a coding expert specializing in Go programming. Provide clear, concise answers with code examples.")

	// Create pipeline
	flow := calque.NewFlow()
	flow.
		Use(systemPrompt).
		Use(agent)

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "How do I handle errors in Go?", &result)
	if err != nil {
		t.Fatalf("System prompt test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "coding expert") {
		t.Errorf("Expected coding expert response, got: %s", result)
	}
}

// TestUserPrompt tests user prompt functionality
func TestUserPrompt(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("I understand you want to learn about Go programming. Let me help you get started.")
	agent := ai.Agent(mockClient)

	// Create user prompt using Chat function
	userPrompt := prompt.Chat("user", "I want to learn Go programming")

	// Create pipeline
	flow := calque.NewFlow()
	flow.
		Use(userPrompt).
		Use(agent)

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "additional context", &result)
	if err != nil {
		t.Fatalf("User prompt test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "learn about Go programming") {
		t.Errorf("Expected learning response, got: %s", result)
	}
}

// TestAssistantPrompt tests assistant prompt functionality
func TestAssistantPrompt(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("I'm here to help you with Go programming. What specific topic would you like to explore?")
	agent := ai.Agent(mockClient)

	// Create assistant prompt using Chat function
	assistantPrompt := prompt.Chat("assistant", "I'm here to help you with Go programming. What specific topic would you like to explore?")

	// Create pipeline
	flow := calque.NewFlow()
	flow.
		Use(assistantPrompt).
		Use(agent)

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "I want to learn about goroutines", &result)
	if err != nil {
		t.Fatalf("Assistant prompt test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "help you with Go programming") {
		t.Errorf("Expected helpful response, got: %s", result)
	}
}

// TestComplexPromptTemplate tests complex prompt template with multiple variables
func TestComplexPromptTemplate(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Based on the context provided, here's a comprehensive answer to your question about Go programming.")
	agent := ai.Agent(mockClient)

	// Create complex prompt template that works with the input
	template := prompt.Template(`Context: Go is a statically typed programming language
Question: What are the benefits of static typing?
Please provide a detailed answer based on the context above.`)

	// Create pipeline
	flow := calque.NewFlow()
	flow.
		Use(template).
		Use(agent)

	// Test with simple input
	var result string
	err := flow.Run(context.Background(), "test input", &result)
	if err != nil {
		t.Fatalf("Complex prompt template test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "comprehensive answer") {
		t.Errorf("Expected comprehensive answer, got: %s", result)
	}
}

// TestPromptPipeline tests prompt in a complex pipeline
func TestPromptPipeline(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Pipeline processed: I've analyzed your Go code and provided suggestions for improvement.")
	agent := ai.Agent(mockClient)

	// Create complex pipeline with multiple prompts
	flow := calque.NewFlow()
	flow.
		Use(logger.Print("START")).
		Use(prompt.System("You are a Go code reviewer. Analyze code and provide constructive feedback.")).
		Use(prompt.Template("Review this Go code:\n{{.Input}}\n\nProvide feedback on:")).
		Use(prompt.Template("1. Code structure\n2. Best practices\n3. Potential improvements\n4. Performance considerations")).
		Use(agent).
		Use(logger.Print("END"))

	// Test the pipeline
	var result string
	err := flow.Run(context.Background(), "func main() { fmt.Println(\"Hello, World!\") }", &result)
	if err != nil {
		t.Fatalf("Prompt pipeline test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Pipeline processed") {
		t.Errorf("Expected pipeline processed response, got: %s", result)
	}
}

// TestPromptErrorHandling tests prompt error handling
func TestPromptErrorHandling(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Error handling test: Successfully processed the request despite template issues.")
	agent := ai.Agent(mockClient)

	// Create pipeline with potentially problematic template
	flow := calque.NewFlow()
	flow.
		Use(prompt.Template("{{.NonExistentField}}")).
		Use(agent)

	// Test the pipeline - should handle template errors gracefully
	var result string
	err := flow.Run(context.Background(), "test input", &result)
	if err != nil {
		t.Fatalf("Prompt error handling test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Error handling test") {
		t.Errorf("Expected error handling test response, got: %s", result)
	}
}

// TestPromptConcurrency tests prompt processing under concurrent access
func TestPromptConcurrency(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Concurrent processing: Successfully handled multiple prompt requests simultaneously.")
	agent := ai.Agent(mockClient)

	// Create pipeline
	flow := calque.NewFlow()
	flow.
		Use(prompt.System("You are a helpful assistant.")).
		Use(agent)

	// Test concurrent processing
	const numRequests = 3
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
			if strings.Contains(result, "Concurrent processing") {
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
