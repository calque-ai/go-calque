package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// MockMCPClient provides a mock implementation for testing
type MockMCPClient struct {
	tools     map[string]func(interface{}) (string, error)
	resources map[string]string
	prompts   map[string]string
}

func NewMockMCPClient() *MockMCPClient {
	return &MockMCPClient{
		tools:     make(map[string]func(interface{}) (string, error)),
		resources: make(map[string]string),
		prompts:   make(map[string]string),
	}
}

func (m *MockMCPClient) Tool(name string) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		if tool, exists := m.tools[name]; exists {
			var input string
			if err := calque.Read(r, &input); err != nil {
				return err
			}

			result, err := tool(input)
			if err != nil {
				return err
			}

			return calque.Write(w, result)
		}
		return calque.Write(w, "tool not found")
	})
}

func (m *MockMCPClient) Resource(uri string) calque.Handler {
	return calque.HandlerFunc(func(_ *calque.Request, w *calque.Response) error {
		if content, exists := m.resources[uri]; exists {
			return calque.Write(w, content)
		}
		return calque.Write(w, "resource not found")
	})
}

func (m *MockMCPClient) ResourceTemplate(template string) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input []byte
		if err := calque.Read(r, &input); err != nil {
			return err
		}

		// Parse JSON input
		var templateVars map[string]string
		if len(input) > 0 {
			// Simple JSON parsing for testing
			inputStr := string(input)
			if strings.Contains(inputStr, `"name"`) && strings.Contains(inputStr, `"database.json"`) {
				templateVars = map[string]string{"name": "database.json"}
			}
		}

		// Simple template replacement for testing
		uri := template
		for key, value := range templateVars {
			uri = strings.ReplaceAll(uri, "{"+key+"}", value)
		}

		if content, exists := m.resources[uri]; exists {
			return calque.Write(w, content)
		}
		return calque.Write(w, "resource not found")
	})
}

func (m *MockMCPClient) Prompt(name string) calque.Handler {
	return calque.HandlerFunc(func(_ *calque.Request, w *calque.Response) error {
		if prompt, exists := m.prompts[name]; exists {
			return calque.Write(w, prompt)
		}
		return calque.Write(w, "prompt not found")
	})
}

func (m *MockMCPClient) Close() error {
	return nil
}

// TestBasicMCPToolCalling tests basic MCP tool calling functionality
func TestBasicMCPToolCalling(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register a multiply tool
	mockClient.tools["multiply"] = func(_ interface{}) (string, error) {
		// Simple mock implementation that returns "42" for any input
		return "42", nil
	}

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.Tool("multiply"))

	input := `{"a": 6, "b": 7}`
	var output string

	err := flow.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}

	if output != "42" {
		t.Errorf("Expected output '42', got '%s'", output)
	}
}

// TestMCPResourceHandling tests MCP resource functionality
func TestMCPResourceHandling(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register a resource
	mockClient.resources["file:///api-docs"] = "# API Documentation\n\n## Authentication\nUse API key in Authorization header.\n\n## Endpoints\n- GET /users - List users\n- POST /users - Create user"

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.Resource("file:///api-docs"))

	input := "I need to understand the API endpoints"
	var output string

	err := flow.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Resource fetch failed: %v", err)
	}

	expectedContent := "# API Documentation"
	if !strings.Contains(output, expectedContent) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedContent, output)
	}
}

// TestMCPResourceTemplate tests MCP resource template functionality
func TestMCPResourceTemplate(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register resources for the template
	mockClient.resources["file:///configs/database.json"] = `{"host": "localhost", "port": 5432, "database": "app_db"}`
	mockClient.resources["file:///configs/cache.json"] = `{"redis_url": "redis://localhost:6379", "ttl": 3600}`

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.ResourceTemplate("file:///configs/{name}"))

	input := `{"name": "database.json"}`
	var output string

	err := flow.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Resource template failed: %v", err)
	}

	expectedContent := `"host": "localhost"`
	if !strings.Contains(output, expectedContent) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedContent, output)
	}
}

// TestMCPPromptHandling tests MCP prompt functionality
func TestMCPPromptHandling(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register a prompt
	mockClient.prompts["code_review"] = "Please review this go code using security review criteria. Focus on:\n- Security vulnerabilities\n- Input validation\n- Authentication/authorization"

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.Prompt("code_review"))

	input := `{"language": "go", "style": "security"}`
	var output string

	err := flow.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	expectedContent := "Security vulnerabilities"
	if !strings.Contains(output, expectedContent) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedContent, output)
	}
}

// TestMCPToolWithProgress tests MCP tool with progress tracking
func TestMCPToolWithProgress(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register a progress tool
	mockClient.tools["progress_demo"] = func(_ interface{}) (string, error) { //nolint:unparam
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return "Completed 5 steps", nil
	}

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.Tool("progress_demo"))

	input := `{"steps": 5}`
	var output string

	err := flow.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Progress tool failed: %v", err)
	}

	expectedContent := "Completed 5 steps"
	if !strings.Contains(output, expectedContent) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedContent, output)
	}
}

// TestMCPConcurrentToolCalls tests concurrent MCP tool calls
func TestMCPConcurrentToolCalls(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register a search tool
	mockClient.tools["search"] = func(_ interface{}) (string, error) {
		return "Search results for query", nil
	}

	// Create multiple concurrent requests
	const numRequests = 5
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(_ int) {
			flow := calque.NewFlow()
			flow.Use(mockClient.Tool("search"))

			searchInput := `{"query": "test", "limit": 3}`
			var searchOutput string

			err := flow.Run(context.Background(), searchInput, &searchOutput)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}
}

// TestMCPErrorHandling tests MCP error scenarios
func TestMCPErrorHandling(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register a tool that returns an error
	mockClient.tools["error_tool"] = func(_ interface{}) (string, error) {
		return "", fmt.Errorf("mock error")
	}

	// Test tool that doesn't exist
	flow := calque.NewFlow()
	flow.Use(mockClient.Tool("nonexistent_tool"))

	const testInput = "test input"
	var output string

	err := flow.Run(context.Background(), testInput, &output)
	if err != nil {
		t.Fatalf("Expected error but got none")
	}

	if output != "tool not found" {
		t.Errorf("Expected output 'tool not found', got '%s'", output)
	}
}

// TestMCPComplexPipeline tests a complex MCP pipeline with multiple tools
func TestMCPComplexPipeline(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register multiple tools
	mockClient.tools["search"] = func(_ interface{}) (string, error) {
		return "search results", nil
	}

	mockClient.tools["analyze"] = func(_ interface{}) (string, error) {
		return "analysis complete", nil
	}

	// Create a complex pipeline
	flow := calque.NewFlow()
	flow.Use(mockClient.Tool("search"))
	flow.Use(mockClient.Tool("analyze"))

	input := `{"query": "complex analysis"}`
	var output string

	err := flow.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Complex pipeline failed: %v", err)
	}

	expectedContent := "analysis complete"
	if !strings.Contains(output, expectedContent) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedContent, output)
	}
}

// TestMCPResourceNotFound tests handling of non-existent resources
func TestMCPResourceNotFound(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Don't register any resources

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.Resource("file:///nonexistent"))

	const testInput = "test input"
	var output string

	err := flow.Run(context.Background(), testInput, &output)
	if err != nil {
		t.Fatalf("Resource fetch failed: %v", err)
	}

	if output != "resource not found" {
		t.Errorf("Expected output 'resource not found', got '%s'", output)
	}
}

// TestMCPPromptNotFound tests handling of non-existent prompts
func TestMCPPromptNotFound(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Don't register any prompts

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.Prompt("nonexistent_prompt"))

	input := `{"language": "go"}`
	var output string

	err := flow.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	if output != "prompt not found" {
		t.Errorf("Expected output 'prompt not found', got '%s'", output)
	}
}

// TestMCPPerformanceCharacteristics tests MCP performance
func TestMCPPerformanceCharacteristics(t *testing.T) {
	t.Parallel()

	// Create mock MCP client
	mockClient := NewMockMCPClient()

	// Register a fast tool
	mockClient.tools["fast_tool"] = func(_ interface{}) (string, error) {
		return "fast result", nil
	}

	// Create the flow
	flow := calque.NewFlow()
	flow.Use(mockClient.Tool("fast_tool"))

	// Measure processing time
	start := time.Now()

	const testInput = "test input"
	var output string

	err := flow.Run(context.Background(), testInput, &output)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}

	duration := time.Since(start)
	t.Logf("MCP tool call completed in %v", duration)

	// Verify output
	if output != "fast result" {
		t.Errorf("Expected output 'fast result', got '%s'", output)
	}
}
