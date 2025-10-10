package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecuteHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		hasTools       bool
		selectedTool   string
		expectedOutput string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "no tool selected - pass through",
			input:          "hello world",
			hasTools:       true,
			selectedTool:   "",
			expectedOutput: "hello world",
			expectError:    false,
		},
		{
			name:          "tool selected but not found",
			input:         "search for something",
			hasTools:      true,
			selectedTool:  "nonexistent",
			expectError:   true,
			errorContains: "not found in registry",
		},
		{
			name:          "tool selected but no tools in context",
			input:         "search for something",
			hasTools:      false,
			selectedTool:  "search",
			expectError:   true,
			errorContains: "not found in registry",
		},
		{
			name:          "valid tool selected - execution attempted",
			input:         "search for golang",
			hasTools:      true,
			selectedTool:  "search",
			expectError:   true,                // Will fail because we have no real client
			errorContains: "MCP client is nil", // Expected from nil client
		},
		{
			name:           "empty input with no tool selected",
			input:          "",
			hasTools:       true,
			selectedTool:   "",
			expectedOutput: "",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasTools {
				ctx = createTestContextForExecute()
			} else {
				ctx = context.Background()
			}

			// Add selected tool to context if specified
			if tt.selectedTool != "" {
				ctx = context.WithValue(ctx, selectedToolContextKey{}, tt.selectedTool)
			}

			// Create handler
			handler := ExecuteTool()

			// Execute
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check output for non-error cases
			if output.String() != tt.expectedOutput {
				t.Errorf("Output = %q, expected %q", output.String(), tt.expectedOutput)
			}
		})
	}
}

func TestExecuteHandlerPanicRecovery(t *testing.T) {
	t.Parallel()

	// This test ensures that Execute handles nil clients gracefully
	// without panicking the entire application

	ctx := createTestContextForExecute()
	ctx = context.WithValue(ctx, selectedToolContextKey{}, "search")

	handler := ExecuteTool()
	req := calque.NewRequest(ctx, strings.NewReader("test input"))
	var output strings.Builder
	res := calque.NewResponse(&output)

	// This should not panic, even with nil client
	err := handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error when executing with nil client")
	}

	// The application should still be running (no panic)
	t.Log("✅ Execute handler handled nil client gracefully")
}

func TestExecuteHandlerInputReading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expectedErr bool
	}{
		{
			name:        "normal input",
			input:       "test input",
			expectedErr: false,
		},
		{
			name:        "empty input",
			input:       "",
			expectedErr: false,
		},
		{
			name:        "large input",
			input:       strings.Repeat("a", 10000),
			expectedErr: false,
		},
		{
			name:        "unicode input",
			input:       "🔍 search for 中文 content",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background() // No tools, should pass through
			handler := ExecuteTool()

			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			if tt.expectedErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectedErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !tt.expectedErr && output.String() != tt.input {
				t.Errorf("Output = %q, expected %q", output.String(), tt.input)
			}
		})
	}
}

// Helper function to create test context with mock tools for execute tests
func createTestContextForExecute() context.Context {
	tools := []*Tool{
		{
			Name:        "search",
			Description: "Search for information",
			MCPTool: &mcp.Tool{
				Name:        "search",
				Description: "Search for information",
			},
			Client: nil, // Nil client will cause controlled errors
		},
		{
			Name:        "connect",
			Description: "Connect to server",
			MCPTool: &mcp.Tool{
				Name:        "connect",
				Description: "Connect to server",
			},
			Client: nil,
		},
	}

	return context.WithValue(context.Background(), mcpToolsContextKey{}, tools)
}

func TestExecuteResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		input            string
		hasResources     bool
		selectedResource string
		expectedOutput   string
		expectError      bool
		errorContains    string
	}{
		{
			name:             "no resource selected - pass through",
			input:            "hello world",
			hasResources:     true,
			selectedResource: "",
			expectedOutput:   "hello world",
			expectError:      false,
		},
		{
			name:             "resource selected - execution attempted",
			input:            "summarize this",
			hasResources:     true,
			selectedResource: "file:///docs/api.md",
			expectError:      true, // Will fail because we have no real client
			errorContains:    "failed to connect",
		},
		{
			name:           "empty input with no resource selected",
			input:          "",
			hasResources:   true,
			expectedOutput: "",
			expectError:    false,
		},
		{
			name:           "unicode input - pass through",
			input:          "🔍 search for 中文 content",
			hasResources:   false,
			expectedOutput: "🔍 search for 中文 content",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasResources {
				ctx = createTestResourceContextForExecute()
			} else {
				ctx = context.Background()
			}

			// Add selected resource to context if specified
			if tt.selectedResource != "" {
				ctx = context.WithValue(ctx, selectedResourceContextKey{}, tt.selectedResource)
			}

			// Create handler with nil client (will error on execution)
			client := &Client{} // Nil session will cause errors
			handler := ExecuteResource(client)

			// Execute
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check output for non-error cases
			if output.String() != tt.expectedOutput {
				t.Errorf("Output = %q, expected %q", output.String(), tt.expectedOutput)
			}
		})
	}
}

func TestExecutePrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		hasPrompts     bool
		selectedPrompt string
		expectedOutput string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "no prompt selected - pass through",
			input:          "hello world",
			hasPrompts:     true,
			selectedPrompt: "",
			expectedOutput: "hello world",
			expectError:    false,
		},
		{
			name:           "prompt selected - execution attempted",
			input:          "{}",
			hasPrompts:     true,
			selectedPrompt: "blog_writer",
			expectError:    true, // Will fail because we have no real client
			errorContains:  "failed to connect",
		},
		{
			name:           "empty input with no prompt selected",
			input:          "",
			hasPrompts:     true,
			expectedOutput: "",
			expectError:    false,
		},
		{
			name:           "large input - pass through",
			input:          strings.Repeat("a", 10000),
			hasPrompts:     false,
			expectedOutput: strings.Repeat("a", 10000),
			expectError:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasPrompts {
				ctx = createTestPromptContextForExecute()
			} else {
				ctx = context.Background()
			}

			// Add selected prompt to context if specified
			if tt.selectedPrompt != "" {
				ctx = context.WithValue(ctx, selectedPromptContextKey{}, tt.selectedPrompt)
			}

			// Create handler with nil client (will error on execution)
			client := &Client{} // Nil session will cause errors
			handler := ExecutePrompt(client)

			// Execute
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check output for non-error cases
			if output.String() != tt.expectedOutput {
				t.Errorf("Output = %q, expected %q", output.String(), tt.expectedOutput)
			}
		})
	}
}

// Helper function to create test context with mock resources for execute tests
func createTestResourceContextForExecute() context.Context {
	resources := []*mcp.Resource{
		{
			URI:         "file:///docs/api.md",
			Name:        "API Documentation",
			Description: "Complete API documentation",
		},
		{
			URI:         "file:///config/settings.json",
			Name:        "Settings",
			Description: "Application settings",
		},
	}

	return context.WithValue(context.Background(), mcpResourcesContextKey{}, resources)
}

// Helper function to create test context with mock prompts for execute tests
func createTestPromptContextForExecute() context.Context {
	prompts := []*mcp.Prompt{
		{
			Name:        "blog_writer",
			Description: "Help write blog posts",
		},
		{
			Name:        "code_review",
			Description: "Review code for best practices",
		},
	}

	return context.WithValue(context.Background(), mcpPromptsContextKey{}, prompts)
}

func TestExecuteCaching(t *testing.T) {
	t.Parallel()

	t.Run("ExecuteResource caching", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "./examples/mcp/cmd/server"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				ResourceTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		// Setup pipeline: ResourceRegistry → select resource → ExecuteResource
		resourceRegistry := ResourceRegistry(client)
		executeHandler := ExecuteResource(client)

		// First call - cache miss
		// Setup context with resource registry
		req1 := calque.NewRequest(context.Background(), strings.NewReader("test input 1"))
		var registryOutput1 strings.Builder
		res1 := calque.NewResponse(&registryOutput1)

		err = resourceRegistry.ServeFlow(req1, res1)
		if err != nil {
			t.Skipf("Skipping test - MCP server connection failed: %v", err)
		}

		resources := GetResources(req1.Context)
		if len(resources) == 0 {
			t.Skip("No resources available from MCP server")
		}

		// Select first resource
		selectedURI := resources[0].URI
		ctx1 := context.WithValue(req1.Context, selectedResourceContextKey{}, selectedURI)

		// Execute with selected resource - cache miss
		execReq1 := calque.NewRequest(ctx1, strings.NewReader("fetch this resource"))
		var execOutput1 strings.Builder
		execRes1 := calque.NewResponse(&execOutput1)

		err = executeHandler.ServeFlow(execReq1, execRes1)
		if err != nil {
			t.Fatalf("First ExecuteResource call failed: %v", err)
		}

		result1 := execOutput1.String()
		if result1 == "" {
			t.Error("Expected resource content on cache miss")
		}

		// Second call - cache hit
		ctx2 := context.WithValue(req1.Context, selectedResourceContextKey{}, selectedURI)

		execReq2 := calque.NewRequest(ctx2, strings.NewReader("fetch this resource again"))
		var execOutput2 strings.Builder
		execRes2 := calque.NewResponse(&execOutput2)

		err = executeHandler.ServeFlow(execReq2, execRes2)
		if err != nil {
			t.Fatalf("Second ExecuteResource call failed: %v", err)
		}

		result2 := execOutput2.String()
		if result2 == "" {
			t.Error("Expected resource content on cache hit (this was the bug!)")
		}

		// Verify cache hit returned same content
		if result1 != result2 {
			t.Error("Cache hit should return same content as cache miss")
		}

		t.Log("✅ ExecuteResource caching works correctly - content retrieved on both cache miss and hit")
	})

	t.Run("ExecutePrompt caching", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "./examples/mcp/cmd/server"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				PromptTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		// Setup pipeline: PromptRegistry → select prompt → ExecutePrompt
		promptRegistry := PromptRegistry(client)
		executeHandler := ExecutePrompt(client)

		// First call - cache miss
		// Setup context with prompt registry
		req1 := calque.NewRequest(context.Background(), strings.NewReader("test input 1"))
		var registryOutput1 strings.Builder
		res1 := calque.NewResponse(&registryOutput1)

		err = promptRegistry.ServeFlow(req1, res1)
		if err != nil {
			t.Skipf("Skipping test - MCP server connection failed: %v", err)
		}

		prompts := GetPrompts(req1.Context)
		if len(prompts) == 0 {
			t.Skip("No prompts available from MCP server")
		}

		// Select first prompt
		selectedName := prompts[0].Name
		ctx1 := context.WithValue(req1.Context, selectedPromptContextKey{}, selectedName)

		// Execute with selected prompt - cache miss
		execReq1 := calque.NewRequest(ctx1, strings.NewReader("{}"))
		var execOutput1 strings.Builder
		execRes1 := calque.NewResponse(&execOutput1)

		err = executeHandler.ServeFlow(execReq1, execRes1)
		if err != nil {
			t.Fatalf("First ExecutePrompt call failed: %v", err)
		}

		result1 := execOutput1.String()
		if result1 == "" {
			t.Error("Expected prompt content on cache miss")
		}

		// Second call - cache hit
		ctx2 := context.WithValue(req1.Context, selectedPromptContextKey{}, selectedName)

		execReq2 := calque.NewRequest(ctx2, strings.NewReader("{}"))
		var execOutput2 strings.Builder
		execRes2 := calque.NewResponse(&execOutput2)

		err = executeHandler.ServeFlow(execReq2, execRes2)
		if err != nil {
			t.Fatalf("Second ExecutePrompt call failed: %v", err)
		}

		result2 := execOutput2.String()
		if result2 == "" {
			t.Error("Expected prompt content on cache hit (this was the bug!)")
		}

		// Verify cache hit returned same content
		if result1 != result2 {
			t.Error("Cache hit should return same content as cache miss")
		}

		t.Log("✅ ExecutePrompt caching works correctly - content retrieved on both cache miss and hit")
	})

	t.Run("ExecutePrompt caching with different args", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "./examples/mcp/cmd/server"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				PromptTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		// Setup pipeline: PromptRegistry → select prompt → ExecutePrompt
		promptRegistry := PromptRegistry(client)
		executeHandler := ExecutePrompt(client)

		// Setup context with prompt registry
		req1 := calque.NewRequest(context.Background(), strings.NewReader("test input"))
		var registryOutput1 strings.Builder
		res1 := calque.NewResponse(&registryOutput1)

		err = promptRegistry.ServeFlow(req1, res1)
		if err != nil {
			t.Skipf("Skipping test - MCP server connection failed: %v", err)
		}

		prompts := GetPrompts(req1.Context)
		if len(prompts) == 0 {
			t.Skip("No prompts available from MCP server")
		}

		// Find a prompt that accepts arguments
		var selectedPrompt *mcp.Prompt
		for _, p := range prompts {
			if len(p.Arguments) > 0 {
				selectedPrompt = p
				break
			}
		}

		if selectedPrompt == nil {
			t.Skip("No prompts with arguments available")
		}

		// Call with first set of args - cache miss
		args1JSON := `{"` + selectedPrompt.Arguments[0].Name + `":"value1"}`
		ctx1 := context.WithValue(req1.Context, selectedPromptContextKey{}, selectedPrompt.Name)
		execReq1 := calque.NewRequest(ctx1, strings.NewReader(args1JSON))
		var execOutput1 strings.Builder
		execRes1 := calque.NewResponse(&execOutput1)

		err = executeHandler.ServeFlow(execReq1, execRes1)
		if err != nil {
			t.Fatalf("First ExecutePrompt call failed: %v", err)
		}

		result1 := execOutput1.String()

		// Call with different args - should be cache miss (different key)
		args2JSON := `{"` + selectedPrompt.Arguments[0].Name + `":"value2"}`
		ctx2 := context.WithValue(req1.Context, selectedPromptContextKey{}, selectedPrompt.Name)
		execReq2 := calque.NewRequest(ctx2, strings.NewReader(args2JSON))
		var execOutput2 strings.Builder
		execRes2 := calque.NewResponse(&execOutput2)

		err = executeHandler.ServeFlow(execReq2, execRes2)
		if err != nil {
			t.Fatalf("Second ExecutePrompt call failed: %v", err)
		}

		result2 := execOutput2.String()

		// Results might differ based on args, but both should have content
		if result1 == "" {
			t.Error("Expected prompt content on first call")
		}
		if result2 == "" {
			t.Error("Expected prompt content on second call")
		}

		// Call with first args again - should be cache hit
		ctx3 := context.WithValue(req1.Context, selectedPromptContextKey{}, selectedPrompt.Name)
		execReq3 := calque.NewRequest(ctx3, strings.NewReader(args1JSON))
		var execOutput3 strings.Builder
		execRes3 := calque.NewResponse(&execOutput3)

		err = executeHandler.ServeFlow(execReq3, execRes3)
		if err != nil {
			t.Fatalf("Third ExecutePrompt call failed: %v", err)
		}

		result3 := execOutput3.String()

		// Third call should match first call (same args, cache hit)
		if result1 != result3 {
			t.Error("Cache with same args should return same content")
		}

		t.Log("✅ ExecutePrompt caching correctly differentiates by args in cache key")
	})
}
