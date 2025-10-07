package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestToolHandlerDeduplicationFix specifically tests the bug fix for duplicate content
// This test replicates the exact scenario from the user's bug report where
// both text content and structured content contained the same data, causing duplication
func TestToolHandlerDeduplicationFix(t *testing.T) {
	t.Parallel()

	// Test the exact bug scenario: same JSON data in both Content and StructuredContent
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test the deduplication behavior directly using the existing greet tool
	handler := client.Tool("greet")

	// Input: tool arguments as JSON
	toolArgs := map[string]any{"name": "TestUser"}
	argsJSON, _ := json.Marshal(toolArgs)

	req := calque.NewRequest(context.Background(), strings.NewReader(string(argsJSON)))
	var output strings.Builder
	res := calque.NewResponse(&output)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Tool handler failed: %v", err)
	}

	result := output.String()

	// Verify the output appears only once (no duplication)
	expectedContent := "Hello, TestUser!"
	if !strings.Contains(result, expectedContent) {
		t.Errorf("Expected output to contain '%s', got: %s", expectedContent, result)
	}

	// Most importantly: verify there's no duplication
	// Count occurrences of the expected content
	count := strings.Count(result, expectedContent)
	if count != 1 {
		t.Errorf("Expected exactly 1 occurrence of '%s', found %d occurrences in: %s", expectedContent, count, result)
	}

	t.Logf("✅ Deduplication fix working correctly - content appears exactly once")
}

func TestToolHandlerContentDeduplication(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		toolResult     *mcp.CallToolResult
		expectedOutput string
		expectError    bool
		errorContains  string
		description    string
	}{
		{
			name:  "text content only - no structured content",
			input: `{"query": "test"}`,
			toolResult: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Hello, World!"},
				},
				StructuredContent: nil,
			},
			expectedOutput: "Hello, World!",
			expectError:    false,
			description:    "Should output text content when no structured content is available",
		},
		{
			name:  "multiple text content pieces - no structured content",
			input: `{"query": "test"}`,
			toolResult: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Part 1 "},
					&mcp.TextContent{Text: "Part 2"},
				},
				StructuredContent: nil,
			},
			expectedOutput: "Part 1 Part 2",
			expectError:    false,
			description:    "Should concatenate multiple text content pieces",
		},
		{
			name:  "no content at all",
			input: `{"query": "test"}`,
			toolResult: &mcp.CallToolResult{
				Content:           []mcp.Content{},
				StructuredContent: nil,
			},
			expectedOutput: "",
			expectError:    false,
			description:    "Should handle empty content gracefully",
		},
		{
			name:  "tool error response",
			input: `{"query": "test"}`,
			toolResult: &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Tool failed"},
				},
			},
			expectedOutput: "",
			expectError:    true,
			errorContains:  "returned error",
			description:    "Should handle tool errors properly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mock client with test tool
			client, cleanup := setupMockClientWithTool(t, "test_tool", tt.toolResult)
			defer cleanup()

			// Create handler
			handler := client.Tool("test_tool")

			// Execute handler
			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check output
			actualOutput := output.String()
			if actualOutput != tt.expectedOutput {
				t.Errorf("Output mismatch.\nExpected: %s\nActual: %s", tt.expectedOutput, actualOutput)
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

// TestToolErrorMessageExtraction specifically tests the error message formatting fix
// This test ensures that when an MCP tool returns an error, the error message
// contains actual text content instead of memory addresses like [0x7feaf45974e0]
func TestToolErrorMessageExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		errorContent  []mcp.Content
		expectedError string
		description   string
	}{
		{
			name: "single text error content",
			errorContent: []mcp.Content{
				&mcp.TextContent{Text: "Authentication failed: invalid API key"},
			},
			expectedError: "Authentication failed: invalid API key",
			description:   "Should extract single text error message correctly",
		},
		{
			name: "multiple text error content pieces",
			errorContent: []mcp.Content{
				&mcp.TextContent{Text: "Error: "},
				&mcp.TextContent{Text: "Database connection failed"},
			},
			expectedError: "Error: Database connection failed",
			description:   "Should concatenate multiple text error pieces",
		},
		{
			name:          "empty error content",
			errorContent:  []mcp.Content{},
			expectedError: "unknown error (no text content in error response)",
			description:   "Should provide fallback message for empty error content",
		},
		{
			name: "non-text error content",
			errorContent: []mcp.Content{
				// Simulate non-text content that would previously show as memory address
				&mcp.ImageContent{},
			},
			expectedError: "unknown error (no text content in error response)",
			description:   "Should provide fallback message when no text content is available",
		},
		{
			name: "mixed text and non-text error content",
			errorContent: []mcp.Content{
				&mcp.TextContent{Text: "Validation failed: "},
				&mcp.ImageContent{}, // This would previously show as memory address
				&mcp.TextContent{Text: "Invalid file format"},
			},
			expectedError: "Validation failed: Invalid file format",
			description:   "Should extract only text content and ignore non-text content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mock client with tool that returns error with specific content
			toolResult := &mcp.CallToolResult{
				IsError: true,
				Content: tt.errorContent,
			}

			client, cleanup := setupMockClientWithTool(t, "error_tool", toolResult)
			defer cleanup()

			// Create handler
			handler := client.Tool("error_tool")

			// Execute handler
			req := calque.NewRequest(context.Background(), strings.NewReader(`{"test": "input"}`))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Should always return an error for IsError=true tools
			if err == nil {
				t.Fatalf("Expected error but got none")
			}

			// Check that error message contains the expected text (not memory addresses)
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error to contain '%s', got: %s", tt.expectedError, err.Error())
			}

			// Ensure error message doesn't contain memory addresses like [0x...]
			if strings.Contains(err.Error(), "[0x") {
				t.Errorf("Error message contains memory address instead of text: %s", err.Error())
			}

			// Verify the error message format
			expectedPrefix := "tool error_tool returned error: "
			if !strings.HasPrefix(err.Error(), expectedPrefix) {
				t.Errorf("Expected error to start with '%s', got: %s", expectedPrefix, err.Error())
			}

			t.Logf("✅ %s - Error message: %s", tt.description, err.Error())
		})
	}
}

func TestResourceHandlerAugmentation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		resourceURI    string
		resourceResult *mcp.ReadResourceResult
		expectedOutput string
		expectError    bool
		errorContains  string
		description    string
	}{
		{
			name:        "basic resource augmentation",
			input:       "How do I use the API?",
			resourceURI: "file:///docs/api.md",
			resourceResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:  "file:///docs/api.md",
						Text: "# API Documentation\nUse POST /api/v1/endpoint",
					},
				},
			},
			expectedOutput: "=== User Query ===\nHow do I use the API?\n\n=== Resource 1 ===\n# API Documentation\nUse POST /api/v1/endpoint\n\n",
			expectError:    false,
			description:    "Should augment user query with resource content",
		},
		{
			name:        "binary resource content",
			input:       "What's in the image?",
			resourceURI: "file:///images/diagram.png",
			resourceResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "file:///images/diagram.png",
						Blob:     []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
						MIMEType: "image/png",
					},
				},
			},
			expectedOutput: "=== User Query ===\nWhat's in the image?\n\n=== Resource 1 ===\n[Binary content: 4 bytes, type: image/png]\n\n",
			expectError:    false,
			description:    "Should handle binary content appropriately",
		},
		{
			name:        "multiple resource contents",
			input:       "Show me all configs",
			resourceURI: "file:///configs/",
			resourceResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:  "file:///configs/app.json",
						Text: `{"name": "myapp"}`,
					},
					{
						URI:  "file:///configs/db.json",
						Text: `{"host": "localhost"}`,
					},
				},
			},
			expectedOutput: "=== User Query ===\nShow me all configs\n\n=== Resource 1.1 ===\n{\"name\": \"myapp\"}\n\n=== Resource 1.2 ===\n{\"host\": \"localhost\"}\n\n",
			expectError:    false,
			description:    "Should handle multiple resource contents",
		},
		{
			name:        "empty input",
			input:       "",
			resourceURI: "file:///docs/help.md",
			resourceResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:  "file:///docs/help.md",
						Text: "Help documentation",
					},
				},
			},
			expectedOutput: "=== User Query ===\n\n\n=== Resource 1 ===\nHelp documentation\n\n",
			expectError:    false,
			description:    "Should handle empty input gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mock client with test resource
			client, cleanup := setupMockClientWithResource(t, tt.resourceURI, tt.resourceResult)
			defer cleanup()

			// Create handler
			handler := client.Resource(tt.resourceURI)

			// Execute handler
			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check output
			actualOutput := output.String()
			if actualOutput != tt.expectedOutput {
				t.Errorf("Output mismatch.\nExpected: %s\nActual: %s", tt.expectedOutput, actualOutput)
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

func TestResourceTemplateSecurityValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		uriTemplate   string
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:          "path traversal attack prevention",
			input:         `{"path": "../../../etc/passwd"}`,
			uriTemplate:   "file:///{path}",
			expectError:   true,
			errorContains: "path traversal not allowed",
			description:   "Should prevent path traversal attacks",
		},
		{
			name:          "control characters prevention",
			input:         `{"path": "config\n.json"}`,
			uriTemplate:   "file:///{path}",
			expectError:   true,
			errorContains: "control characters not allowed",
			description:   "Should prevent control characters in paths",
		},
		{
			name:          "invalid JSON input",
			input:         `{invalid json}`,
			uriTemplate:   "file:///{path}",
			expectError:   true,
			errorContains: "invalid template variables JSON",
			description:   "Should handle invalid JSON gracefully",
		},
		{
			name:        "valid template resolution",
			input:       `{"path": "test/doc.md"}`,
			uriTemplate: "file:///{path}",
			expectError: false,
			description: "Should allow valid template resolution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a client (we'll test the template resolution, not actual resource fetching)
			client, cleanup := setupTestServer(t)
			defer cleanup()

			// Create handler
			handler := client.ResourceTemplate(tt.uriTemplate)

			// Execute handler
			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

func TestPromptHandlerFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		promptName     string
		promptResult   *mcp.GetPromptResult
		expectedOutput string
		expectError    bool
		errorContains  string
		description    string
	}{
		{
			name:       "basic prompt execution",
			input:      `{"language": "Go"}`,
			promptName: "code_review",
			promptResult: &mcp.GetPromptResult{
				Description: "Code review prompt",
				Messages: []*mcp.PromptMessage{
					{
						Role: "user",
						Content: &mcp.TextContent{
							Text: "Please review this Go code for best practices.",
						},
					},
				},
			},
			expectedOutput: "user: Please review this Go code for best practices.",
			expectError:    false,
			description:    "Should execute prompt with arguments and return formatted messages",
		},
		{
			name:       "multiple messages",
			input:      `{"topic": "API design"}`,
			promptName: "conversation",
			promptResult: &mcp.GetPromptResult{
				Description: "Conversation prompt",
				Messages: []*mcp.PromptMessage{
					{
						Role: "system",
						Content: &mcp.TextContent{
							Text: "You are an API design expert.",
						},
					},
					{
						Role: "user",
						Content: &mcp.TextContent{
							Text: "Help me design a REST API.",
						},
					},
				},
			},
			expectedOutput: "system: You are an API design expert.\nuser: Help me design a REST API.",
			expectError:    false,
			description:    "Should handle multiple messages with different roles",
		},
		{
			name:          "invalid JSON arguments",
			input:         `{invalid json}`,
			promptName:    "test",
			expectError:   true,
			errorContains: "invalid prompt arguments JSON",
			description:   "Should handle invalid JSON gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mock client with test prompt
			client, cleanup := setupMockClientWithPrompt(t, tt.promptName, tt.promptResult)
			defer cleanup()

			// Create handler
			handler := client.Prompt(tt.promptName)

			// Execute handler
			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check output
			actualOutput := output.String()
			if actualOutput != tt.expectedOutput {
				t.Errorf("Output mismatch.\nExpected: %s\nActual: %s", tt.expectedOutput, actualOutput)
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

// Helper functions for setting up mock clients

func setupMockClientWithTool(t *testing.T, toolName string, result *mcp.CallToolResult) (*Client, func()) {
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

	// Add mock tool that returns the specified result
	mcp.AddTool(server, &mcp.Tool{
		Name:        toolName,
		Description: "Test tool",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {
		// Return the result as-is, whether it's an error or not
		// When result.IsError is true, the MCP client should handle it properly
		return result, nil, nil
	})

	// Start server session
	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"tools"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
	}

	// Connect
	if err := client.connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	return client, func() {
		if client.session != nil {
			client.session.Close()
		}
	}
}

func setupMockClientWithResource(t *testing.T, resourceURI string, result *mcp.ReadResourceResult) (*Client, func()) {
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

	server.AddResource(&mcp.Resource{
		URI:         resourceURI,
		Name:        "Test Resource",
		Description: "Test resource for handler testing",
	}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if req.Params.URI == resourceURI {
			return result, nil
		}
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	})

	// Start server session
	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"resources"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
	}

	if err := client.connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	return client, func() {
		if client.session != nil {
			client.session.Close()
		}
	}
}

func setupMockClientWithPrompt(t *testing.T, promptName string, result *mcp.GetPromptResult) (*Client, func()) {
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

	server.AddPrompt(&mcp.Prompt{
		Name:        promptName,
		Description: "Test prompt",
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		if req.Params.Name == promptName {
			return result, nil
		}
		return nil, fmt.Errorf("prompt not found: %s", req.Params.Name)
	})

	// Start server session
	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"prompts"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
	}

	if err := client.connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	return client, func() {
		if client.session != nil {
			client.session.Close()
		}
	}
}

func TestResourceHandlerCaching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		resourceURI   string
		input1        string
		input2        string
		expectedCalls int // How many times the MCP server should be called
		description   string
	}{
		{
			name:          "cache hit on identical input",
			resourceURI:   "file:///docs/api.md",
			input1:        "How do I use the API?",
			input2:        "How do I use the API?",
			expectedCalls: 1, // Should only call server once
			description:   "Identical inputs should result in cache hit",
		},
		{
			name:          "cache miss on different input",
			resourceURI:   "file:///docs/api.md",
			input1:        "How do I use the API?",
			input2:        "What are the best practices?",
			expectedCalls: 2, // Should call server twice
			description:   "Different inputs should result in cache miss",
		},
		{
			name:          "cache hit on empty input",
			resourceURI:   "file:///docs/help.md",
			input1:        "",
			input2:        "",
			expectedCalls: 1,
			description:   "Empty inputs should cache correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callCount := 0
			ctx := context.Background()
			clientTransport, serverTransport := mcp.NewInMemoryTransports()

			server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

			server.AddResource(&mcp.Resource{
				URI:         tt.resourceURI,
				Name:        "Test Resource",
				Description: "Test resource for caching",
			}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				callCount++ // Track how many times server is called
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{
							URI:  tt.resourceURI,
							Text: "API Documentation",
						},
					},
				}, nil
			})

			// Start server
			_, err := server.Connect(ctx, serverTransport, nil)
			if err != nil {
				t.Fatalf("Failed to start test server: %v", err)
			}

			// Create client with caching enabled
			mcpClient := mcp.NewClient(defaultImplementation(), nil)
			client := &Client{
				client:            mcpClient,
				transport:         clientTransport,
				timeout:           30 * time.Second,
				capabilities:      []string{"resources"},
				implementation:    defaultImplementation(),
				progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
				subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
			}

			// Apply caching with default config
			cacheStore := cache.NewInMemoryStore()
			cacheConfig := &CacheConfig{
				ResourceTTL: 5 * time.Minute,
			}
			WithCache(cacheStore, cacheConfig)(client)

			if err := client.connect(ctx); err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer client.session.Close()

			// Create handler
			handler := client.Resource(tt.resourceURI)

			// First request
			req1 := calque.NewRequest(context.Background(), strings.NewReader(tt.input1))
			var output1 strings.Builder
			res1 := calque.NewResponse(&output1)

			err = handler.ServeFlow(req1, res1)
			if err != nil {
				t.Fatalf("First request failed: %v", err)
			}

			// Second request
			req2 := calque.NewRequest(context.Background(), strings.NewReader(tt.input2))
			var output2 strings.Builder
			res2 := calque.NewResponse(&output2)

			err = handler.ServeFlow(req2, res2)
			if err != nil {
				t.Fatalf("Second request failed: %v", err)
			}

			// Verify call count
			if callCount != tt.expectedCalls {
				t.Errorf("Expected %d server calls, got %d", tt.expectedCalls, callCount)
			}

			// For cache hits, outputs should be identical
			if tt.expectedCalls == 1 && output1.String() != output2.String() {
				t.Errorf("Cache hit should produce identical outputs.\nFirst: %s\nSecond: %s",
					output1.String(), output2.String())
			}

			t.Logf("✅ %s - Server called %d times", tt.description, callCount)
		})
	}
}

func TestPromptHandlerCaching(t *testing.T) {
	t.Parallel()

	callCount := 0
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

	server.AddPrompt(&mcp.Prompt{
		Name:        "code_review",
		Description: "Code review prompt",
	}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		callCount++ // Track server calls
		return &mcp.GetPromptResult{
			Description: "Code review prompt",
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please review this code.",
					},
				},
			},
		}, nil
	})

	// Start server
	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	// Create client with caching
	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"prompts"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
	}

	cacheStore := cache.NewInMemoryStore()
	WithCache(cacheStore)(client) // Use defaults

	if err := client.connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.session.Close()

	handler := client.Prompt("code_review")

	// First request
	req1 := calque.NewRequest(context.Background(), strings.NewReader(`{"language": "Go"}`))
	var output1 strings.Builder
	res1 := calque.NewResponse(&output1)

	err = handler.ServeFlow(req1, res1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	// Second request with same input
	req2 := calque.NewRequest(context.Background(), strings.NewReader(`{"language": "Go"}`))
	var output2 strings.Builder
	res2 := calque.NewResponse(&output2)

	err = handler.ServeFlow(req2, res2)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}

	// Should only call server once due to caching
	if callCount != 1 {
		t.Errorf("Expected 1 server call (cache hit), got %d", callCount)
	}

	// Outputs should be identical
	if output1.String() != output2.String() {
		t.Errorf("Cached outputs should be identical.\nFirst: %s\nSecond: %s",
			output1.String(), output2.String())
	}

	t.Logf("✅ Prompt caching working correctly - Server called %d time(s)", callCount)
}

func TestToolHandlerCaching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		toolTTL       time.Duration
		expectedCalls int
		description   string
	}{
		{
			name:          "caching disabled by default (TTL=0)",
			toolTTL:       0,
			expectedCalls: 2, // No caching
			description:   "Tools should not be cached by default",
		},
		{
			name:          "caching enabled with TTL > 0",
			toolTTL:       5 * time.Minute,
			expectedCalls: 1, // Cached
			description:   "Tools should be cached when TTL > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callCount := 0
			ctx := context.Background()
			clientTransport, serverTransport := mcp.NewInMemoryTransports()

			server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

			mcp.AddTool(server, &mcp.Tool{
				Name:        "test_tool",
				Description: "Test tool",
			}, func(_ context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {
				callCount++
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "Tool result"},
					},
				}, nil, nil
			})

			// Start server
			_, err := server.Connect(ctx, serverTransport, nil)
			if err != nil {
				t.Fatalf("Failed to start test server: %v", err)
			}

			// Create client
			mcpClient := mcp.NewClient(defaultImplementation(), nil)
			client := &Client{
				client:            mcpClient,
				transport:         clientTransport,
				timeout:           30 * time.Second,
				capabilities:      []string{"tools"},
				implementation:    defaultImplementation(),
				progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
				subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
			}

			// Apply caching with specific TTL
			cacheStore := cache.NewInMemoryStore()
			cacheConfig := &CacheConfig{
				ToolTTL: tt.toolTTL,
			}
			WithCache(cacheStore, cacheConfig)(client)

			if err := client.connect(ctx); err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer client.session.Close()

			handler := client.Tool("test_tool")

			// Make two identical requests
			input := `{"query": "test"}`

			req1 := calque.NewRequest(context.Background(), strings.NewReader(input))
			var output1 strings.Builder
			res1 := calque.NewResponse(&output1)

			err = handler.ServeFlow(req1, res1)
			if err != nil {
				t.Fatalf("First request failed: %v", err)
			}

			req2 := calque.NewRequest(context.Background(), strings.NewReader(input))
			var output2 strings.Builder
			res2 := calque.NewResponse(&output2)

			err = handler.ServeFlow(req2, res2)
			if err != nil {
				t.Fatalf("Second request failed: %v", err)
			}

			// Verify call count
			if callCount != tt.expectedCalls {
				t.Errorf("Expected %d server calls, got %d", tt.expectedCalls, callCount)
			}

			t.Logf("✅ %s - Server called %d time(s)", tt.description, callCount)
		})
	}
}

func TestCachingWithNilConfig(t *testing.T) {
	t.Parallel()

	// Test that cache works without config (uses defaults)
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

	callCount := 0
	server.AddResource(&mcp.Resource{
		URI:         "file:///test.md",
		Name:        "Test",
		Description: "Test",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		callCount++
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "file:///test.md", Text: "content"},
			},
		}, nil
	})

	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"resources"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
	}

	// Apply cache with no config (should use defaults)
	WithCache(cache.NewInMemoryStore())(client)

	if err := client.connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.session.Close()

	// Verify default config was applied
	if client.cacheConfig == nil {
		t.Fatal("Expected default cache config to be applied")
	}

	if client.cacheConfig.ResourceTTL != 5*time.Minute {
		t.Errorf("Expected default ResourceTTL of 5 minutes, got %v", client.cacheConfig.ResourceTTL)
	}

	handler := client.Resource("file:///test.md")

	// Make two requests
	for i := 0; i < 2; i++ {
		req := calque.NewRequest(context.Background(), strings.NewReader("test"))
		var output strings.Builder
		res := calque.NewResponse(&output)

		err := handler.ServeFlow(req, res)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
	}

	// Should only call server once (cached)
	if callCount != 1 {
		t.Errorf("Expected 1 server call with default config, got %d", callCount)
	}

	t.Logf("✅ Default cache config working correctly")
}

func TestMultipleResourcesCaching(t *testing.T) {
	t.Parallel()

	// Test caching with variadic Resource() function
	callCount1 := 0
	callCount2 := 0

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

	server.AddResource(&mcp.Resource{
		URI:         "file:///doc1.md",
		Name:        "Doc 1",
		Description: "First doc",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		callCount1++
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "file:///doc1.md", Text: "Content 1"},
			},
		}, nil
	})

	server.AddResource(&mcp.Resource{
		URI:         "file:///doc2.md",
		Name:        "Doc 2",
		Description: "Second doc",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		callCount2++
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "file:///doc2.md", Text: "Content 2"},
			},
		}, nil
	})

	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"resources"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
	}

	WithCache(cache.NewInMemoryStore())(client)

	if err := client.connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.session.Close()

	// Create handler with multiple resources
	handler := client.Resource("file:///doc1.md", "file:///doc2.md")

	// Make two identical requests
	for i := range 2 {
		req := calque.NewRequest(context.Background(), strings.NewReader("query"))
		var output strings.Builder
		res := calque.NewResponse(&output)

		err := handler.ServeFlow(req, res)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		// Verify output contains both resources
		result := output.String()
		if !strings.Contains(result, "Content 1") {
			t.Errorf("Request %d: Expected output to contain 'Content 1'", i+1)
		}
		if !strings.Contains(result, "Content 2") {
			t.Errorf("Request %d: Expected output to contain 'Content 2'", i+1)
		}
	}

	// Both resources should only be called once (cache hit on second request)
	if callCount1 != 1 {
		t.Errorf("Expected doc1 to be called 1 time, got %d", callCount1)
	}
	if callCount2 != 1 {
		t.Errorf("Expected doc2 to be called 1 time, got %d", callCount2)
	}

	t.Logf("✅ Multiple resources caching working correctly")
}

func TestCachingWithoutCacheEnabled(t *testing.T) {
	t.Parallel()

	// Verify that handlers work correctly when caching is NOT enabled
	callCount := 0
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)

	server.AddResource(&mcp.Resource{
		URI:         "file:///test.md",
		Name:        "Test",
		Description: "Test",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		callCount++
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "file:///test.md", Text: "content"},
			},
		}, nil
	})

	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"resources"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
	}

	// NOTE: No caching enabled!

	if err := client.connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.session.Close()

	handler := client.Resource("file:///test.md")

	// Make two requests
	for i := range 2 {
		req := calque.NewRequest(context.Background(), strings.NewReader("test"))
		var output strings.Builder
		res := calque.NewResponse(&output)

		err := handler.ServeFlow(req, res)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
	}

	// Should call server twice (no caching)
	if callCount != 2 {
		t.Errorf("Expected 2 server calls without caching, got %d", callCount)
	}

	t.Logf("✅ Handlers work correctly without caching enabled")
}
