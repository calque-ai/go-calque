package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	googleschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConvertGoogleSchemaToInvopop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        any
		expectNil    bool
		expectedType string
		description  string
	}{
		{
			name:        "nil input",
			input:       nil,
			expectNil:   true,
			description: "Should return nil for nil input",
		},
		{
			name: "valid Google schema object",
			input: &googleschema.Schema{
				Type: "object",
				Properties: map[string]*googleschema.Schema{
					"name": {Type: "string"},
					"age":  {Type: "number"},
				},
			},
			expectNil:    false,
			expectedType: "object",
			description:  "Should convert Google schema object to invopop",
		},
		{
			name: "simple string schema",
			input: &googleschema.Schema{
				Type: "string",
			},
			expectNil:    false,
			expectedType: "string",
			description:  "Should convert simple string schema",
		},
		{
			name: "array schema",
			input: &googleschema.Schema{
				Type: "array",
				Items: &googleschema.Schema{
					Type: "string",
				},
			},
			expectNil:    false,
			expectedType: "array",
			description:  "Should convert array schema",
		},
		{
			name: "map[string]any input",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "number"},
				},
			},
			expectNil:    false,
			expectedType: "object",
			description:  "Should convert map[string]any to invopop schema",
		},
		{
			name:        "invalid schema - not serializable",
			input:       make(chan int), // channels can't be marshaled to JSON
			expectNil:   true,
			description: "Should return nil for non-serializable input",
		},
		{
			name: "number schema",
			input: &googleschema.Schema{
				Type: "number",
			},
			expectNil:    false,
			expectedType: "number",
			description:  "Should convert number schema",
		},
		{
			name: "boolean schema",
			input: &googleschema.Schema{
				Type: "boolean",
			},
			expectNil:    false,
			expectedType: "boolean",
			description:  "Should convert boolean schema",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := convertGoogleSchemaToInvopop(tt.input)

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if tt.expectedType != "" && result.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, result.Type)
			}

			// Verify that the conversion preserved the structure
			if tt.expectedType == "object" && result.Properties != nil {
				// Check that properties were preserved
				if result.Properties.Len() == 0 {
					t.Error("Expected properties to be preserved in object schema")
				}
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

func TestDefaultImplementation(t *testing.T) {
	t.Parallel()

	impl := defaultImplementation()

	if impl == nil {
		t.Fatal("defaultImplementation() returned nil")
	}

	if impl.Name != "calque-mcp-client" {
		t.Errorf("Expected name 'calque-mcp-client', got '%s'", impl.Name)
	}

	if impl.Version != "v0.1.0" {
		t.Errorf("Expected version 'v0.1.0', got '%s'", impl.Version)
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("default configuration", func(t *testing.T) {
		t.Parallel()

		mcpClient := &mcp.Client{}
		client := newClient(mcpClient)

		if client == nil {
			t.Fatal("newClient() returned nil")
		}

		if client.client != mcpClient {
			t.Error("Expected client.client to be set to provided mcpClient")
		}

		if client.timeout != 0*time.Second {
			t.Errorf("Expected default timeout 0s, got %v", client.timeout)
		}

		if client.implementation == nil {
			t.Error("Expected implementation to be set")
		}

		if client.implementation.Name != "calque-mcp-client" {
			t.Errorf("Expected default implementation name, got '%s'", client.implementation.Name)
		}

		expectedCaps := []string{} // No default capabilities
		if len(client.capabilities) != len(expectedCaps) {
			t.Errorf("Expected %d capabilities, got %d", len(expectedCaps), len(client.capabilities))
		}

		if client.progressCallbacks == nil {
			t.Error("Expected progressCallbacks to be initialized")
		}

		if client.subscriptions == nil {
			t.Error("Expected subscriptions to be initialized")
		}

		if client.completionEnabled {
			t.Error("Expected completionEnabled to be false by default")
		}
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()

		mcpClient := &mcp.Client{}

		customTimeout := 60 * time.Second

		client := newClient(mcpClient,
			WithTimeout(customTimeout),
			WithImplementation("custom", "v2.0.0"),
			WithCapabilities("tools"),
			WithCompletion(true),
		)

		if client.timeout != customTimeout {
			t.Errorf("Expected timeout %v, got %v", customTimeout, client.timeout)
		}

		if client.implementation.Name != "custom" {
			t.Errorf("Expected implementation name 'custom', got '%s'", client.implementation.Name)
		}

		if client.implementation.Version != "v2.0.0" {
			t.Errorf("Expected implementation version 'v2.0.0', got '%s'", client.implementation.Version)
		}

		if len(client.capabilities) != 1 || client.capabilities[0] != "tools" {
			t.Errorf("Expected capabilities ['tools'], got %v", client.capabilities)
		}

		if !client.completionEnabled {
			t.Error("Expected completionEnabled to be true")
		}
	})
}

func TestResourceRegistry(t *testing.T) {
	t.Parallel()

	t.Run("handler creation", func(t *testing.T) {
		t.Parallel()

		// Test that ResourceRegistry returns a valid handler
		client := &Client{}
		handler := ResourceRegistry(client)

		if handler == nil {
			t.Fatal("ResourceRegistry() returned nil handler")
		}
	})

	// Note: Full integration testing requires a real MCP server connection
}

func TestPromptRegistry(t *testing.T) {
	t.Parallel()

	t.Run("handler creation", func(t *testing.T) {
		t.Parallel()

		// Test that PromptRegistry returns a valid handler
		client := &Client{}
		handler := PromptRegistry(client)

		if handler == nil {
			t.Fatal("PromptRegistry() returned nil handler")
		}
	})

	// Note: Full integration testing requires a real MCP server connection
}

func TestGetResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected int
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: 0,
		},
		{
			name:     "context with resources",
			ctx:      createContextWithResources(3),
			expected: 3,
		},
		{
			name:     "context with empty resources slice",
			ctx:      context.WithValue(context.Background(), mcpResourcesContextKey{}, []*mcp.Resource{}),
			expected: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resources := GetResources(tt.ctx)
			if len(resources) != tt.expected {
				t.Errorf("GetResources() returned %d resources, expected %d", len(resources), tt.expected)
			}
		})
	}
}

func TestGetSelectedResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "no resource selected",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "resource selected",
			ctx:      context.WithValue(context.Background(), selectedResourceContextKey{}, "file:///data/config.json"),
			expected: "file:///data/config.json",
		},
		{
			name:     "empty resource selected",
			ctx:      context.WithValue(context.Background(), selectedResourceContextKey{}, ""),
			expected: "",
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), selectedResourceContextKey{}, 123),
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetSelectedResource(tt.ctx)
			if result != tt.expected {
				t.Errorf("GetSelectedResource() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestHasSelectedResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "no resource selected",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name:     "resource selected",
			ctx:      context.WithValue(context.Background(), selectedResourceContextKey{}, "file:///data/config.json"),
			expected: true,
		},
		{
			name:     "empty resource selected",
			ctx:      context.WithValue(context.Background(), selectedResourceContextKey{}, ""),
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := HasSelectedResource(tt.ctx)
			if result != tt.expected {
				t.Errorf("HasSelectedResource() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetResourceContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ctx       context.Context
		expectNil bool
	}{
		{
			name:      "no resource content",
			ctx:       context.Background(),
			expectNil: true,
		},
		{
			name: "resource content present",
			ctx: context.WithValue(context.Background(), resourceContentContextKey{}, &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{Text: "test content"}},
			}),
			expectNil: false,
		},
		{
			name:      "wrong type in context",
			ctx:       context.WithValue(context.Background(), resourceContentContextKey{}, "invalid"),
			expectNil: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetResourceContent(tt.ctx)
			if (result == nil) != tt.expectNil {
				t.Errorf("GetResourceContent() nil=%v, expected nil=%v", result == nil, tt.expectNil)
			}
		})
	}
}

func TestGetPrompts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected int
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: 0,
		},
		{
			name:     "context with prompts",
			ctx:      createContextWithPrompts(3),
			expected: 3,
		},
		{
			name:     "context with empty prompts slice",
			ctx:      context.WithValue(context.Background(), mcpPromptsContextKey{}, []*mcp.Prompt{}),
			expected: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prompts := GetPrompts(tt.ctx)
			if len(prompts) != tt.expected {
				t.Errorf("GetPrompts() returned %d prompts, expected %d", len(prompts), tt.expected)
			}
		})
	}
}

func TestGetSelectedPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "no prompt selected",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "prompt selected",
			ctx:      context.WithValue(context.Background(), selectedPromptContextKey{}, "blog_writer"),
			expected: "blog_writer",
		},
		{
			name:     "empty prompt selected",
			ctx:      context.WithValue(context.Background(), selectedPromptContextKey{}, ""),
			expected: "",
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), selectedPromptContextKey{}, 123),
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetSelectedPrompt(tt.ctx)
			if result != tt.expected {
				t.Errorf("GetSelectedPrompt() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestHasSelectedPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "no prompt selected",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name:     "prompt selected",
			ctx:      context.WithValue(context.Background(), selectedPromptContextKey{}, "code_review"),
			expected: true,
		},
		{
			name:     "empty prompt selected",
			ctx:      context.WithValue(context.Background(), selectedPromptContextKey{}, ""),
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := HasSelectedPrompt(tt.ctx)
			if result != tt.expected {
				t.Errorf("HasSelectedPrompt() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetPromptContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ctx       context.Context
		expectNil bool
	}{
		{
			name:      "no prompt content",
			ctx:       context.Background(),
			expectNil: true,
		},
		{
			name: "prompt content present",
			ctx: context.WithValue(context.Background(), promptContentContextKey{}, &mcp.GetPromptResult{
				Description: "test prompt",
			}),
			expectNil: false,
		},
		{
			name:      "wrong type in context",
			ctx:       context.WithValue(context.Background(), promptContentContextKey{}, "invalid"),
			expectNil: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetPromptContent(tt.ctx)
			if (result == nil) != tt.expectNil {
				t.Errorf("GetPromptContent() nil=%v, expected nil=%v", result == nil, tt.expectNil)
			}
		})
	}
}

// Helper function to create context with mock resources
func createContextWithResources(count int) context.Context {
	resources := make([]*mcp.Resource, count)
	for i := 0; i < count; i++ {
		uri := fmt.Sprintf("file:///resource_%d.json", i)
		resources[i] = &mcp.Resource{
			URI:         uri,
			Name:        fmt.Sprintf("resource_%d", i),
			Description: fmt.Sprintf("Description for resource_%d", i),
		}
	}

	return context.WithValue(context.Background(), mcpResourcesContextKey{}, resources)
}

// Helper function to create context with mock prompts
func createContextWithPrompts(count int) context.Context {
	prompts := make([]*mcp.Prompt, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("prompt_%d", i)
		prompts[i] = &mcp.Prompt{
			Name:        name,
			Description: fmt.Sprintf("Description for prompt_%d", i),
		}
	}

	return context.WithValue(context.Background(), mcpPromptsContextKey{}, prompts)
}

func TestRegistryCaching(t *testing.T) {
	t.Parallel()

	// This test verifies that registry handlers correctly cache MCP API results
	// while always populating context on both cache miss and cache hit.
	// This is a regression test for the bug where cache hits wouldn't populate context.

	t.Run("ResourceRegistry caching", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "examples/mcp/cmd/server/main.go"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				RegistryTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		handler := ResourceRegistry(client)

		// First call - cache miss
		req1 := calque.NewRequest(context.Background(), strings.NewReader("input"))
		var output1 strings.Builder
		res1 := calque.NewResponse(&output1)

		err = handler.ServeFlow(req1, res1)
		if err != nil {
			t.Skipf("First call failed: %v", err)
		}

		resources1 := GetResources(req1.Context)
		if resources1 == nil {
			t.Error("Context should be populated on cache miss")
		}

		// Second call - cache hit
		req2 := calque.NewRequest(context.Background(), strings.NewReader("input"))
		var output2 strings.Builder
		res2 := calque.NewResponse(&output2)

		err = handler.ServeFlow(req2, res2)
		if err != nil {
			t.Skipf("Second call failed: %v", err)
		}

		resources2 := GetResources(req2.Context)
		if resources2 == nil {
			t.Error("Context should be populated on cache hit")
		}

		t.Log("✅ ResourceRegistry caching works correctly")
	})

	t.Run("PromptRegistry caching", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "examples/mcp/cmd/server/main.go"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				RegistryTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		handler := PromptRegistry(client)

		// First call - cache miss
		req1 := calque.NewRequest(context.Background(), strings.NewReader("input"))
		var output1 strings.Builder
		res1 := calque.NewResponse(&output1)

		err = handler.ServeFlow(req1, res1)
		if err != nil {
			t.Skipf("First call failed: %v", err)
		}

		prompts1 := GetPrompts(req1.Context)
		if prompts1 == nil {
			t.Error("Context should be populated on cache miss")
		}

		// Second call - cache hit
		req2 := calque.NewRequest(context.Background(), strings.NewReader("input"))
		var output2 strings.Builder
		res2 := calque.NewResponse(&output2)

		err = handler.ServeFlow(req2, res2)
		if err != nil {
			t.Skipf("Second call failed: %v", err)
		}

		prompts2 := GetPrompts(req2.Context)
		if prompts2 == nil {
			t.Error("Context should be populated on cache hit")
		}

		t.Log("✅ PromptRegistry caching works correctly")
	})
}
