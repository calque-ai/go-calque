package mcp

import (
	"context"
	"testing"
	"time"

	googleschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetTools(t *testing.T) {
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
			name:     "context with tools",
			ctx:      createContextWithTools(3),
			expected: 3,
		},
		{
			name:     "context with empty tools slice",
			ctx:      context.WithValue(context.Background(), mcpToolsContextKey{}, []*MCPTool{}),
			expected: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tools := GetTools(tt.ctx)
			if len(tools) != tt.expected {
				t.Errorf("GetTools() returned %d tools, expected %d", len(tools), tt.expected)
			}
		})
	}
}

func TestGetTool(t *testing.T) {
	t.Parallel()

	ctx := createContextWithTools(3)

	tests := []struct {
		name     string
		ctx      context.Context
		toolName string
		found    bool
	}{
		{
			name:     "existing tool",
			ctx:      ctx,
			toolName: "tool_0",
			found:    true,
		},
		{
			name:     "non-existing tool",
			ctx:      ctx,
			toolName: "nonexistent",
			found:    false,
		},
		{
			name:     "empty context",
			ctx:      context.Background(),
			toolName: "tool_0",
			found:    false,
		},
		{
			name:     "empty tool name",
			ctx:      ctx,
			toolName: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := GetTool(tt.ctx, tt.toolName)
			if (tool != nil) != tt.found {
				t.Errorf("GetTool() found=%v, expected=%v", tool != nil, tt.found)
			}

			if tt.found && tool.Name() != tt.toolName {
				t.Errorf("GetTool() returned tool with name=%s, expected=%s", tool.Name(), tt.toolName)
			}
		})
	}
}

func TestHasTool(t *testing.T) {
	t.Parallel()

	ctx := createContextWithTools(2)

	tests := []struct {
		name     string
		ctx      context.Context
		toolName string
		expected bool
	}{
		{
			name:     "existing tool",
			ctx:      ctx,
			toolName: "tool_0",
			expected: true,
		},
		{
			name:     "non-existing tool",
			ctx:      ctx,
			toolName: "nonexistent",
			expected: false,
		},
		{
			name:     "empty context",
			ctx:      context.Background(),
			toolName: "tool_0",
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := HasTool(tt.ctx, tt.toolName)
			if result != tt.expected {
				t.Errorf("HasTool() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestListToolNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected []string
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name:     "context with tools",
			ctx:      createContextWithTools(3),
			expected: []string{"tool_0", "tool_1", "tool_2"},
		},
		{
			name:     "context with empty tools",
			ctx:      context.WithValue(context.Background(), mcpToolsContextKey{}, []*MCPTool{}),
			expected: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names := ListToolNames(tt.ctx)
			if len(names) != len(tt.expected) {
				t.Fatalf("ListToolNames() returned %d names, expected %d", len(names), len(tt.expected))
			}

			for i, name := range names {
				if name != tt.expected[i] {
					t.Errorf("ListToolNames()[%d] = %s, expected %s", i, name, tt.expected[i])
				}
			}
		})
	}
}

func TestListTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		ctx           context.Context
		expectedCount int
		checkSchemas  bool
	}{
		{
			name:          "empty context",
			ctx:           context.Background(),
			expectedCount: 0,
			checkSchemas:  false,
		},
		{
			name:          "context with tools",
			ctx:           createContextWithTools(2),
			expectedCount: 2,
			checkSchemas:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			infos := ListTools(tt.ctx)
			if len(infos) != tt.expectedCount {
				t.Fatalf("ListTools() returned %d infos, expected %d", len(infos), tt.expectedCount)
			}

			if tt.checkSchemas {
				for i, info := range infos {
					expectedName := "tool_" + string(rune('0'+i))
					if info.Name != expectedName {
						t.Errorf("ListTools()[%d].Name = %s, expected %s", i, info.Name, expectedName)
					}

					if info.Description == "" {
						t.Errorf("ListTools()[%d].Description should not be empty", i)
					}

					if info.InputSchema == nil {
						t.Errorf("ListTools()[%d].InputSchema should not be nil", i)
					}
				}
			}
		})
	}
}

// Helper function to create context with mock tools
func createContextWithTools(count int) context.Context {
	tools := make([]*MCPTool, count)
	for i := 0; i < count; i++ {
		toolName := "tool_" + string(rune('0'+i))
		tools[i] = &MCPTool{
			Tool: &mcp.Tool{
				Name:        toolName,
				Description: "Description for " + toolName,
				InputSchema: &googleschema.Schema{
					Type: "object",
					Properties: map[string]*googleschema.Schema{
						"param": {Type: "string"},
					},
				},
			},
			Client: nil, // Mock client not needed for registry tests
		}
	}

	return context.WithValue(context.Background(), mcpToolsContextKey{}, tools)
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("handler creation", func(t *testing.T) {
		t.Parallel()

		// Test that Registry returns a valid handler
		client := &Client{}
		handler := Registry(client)

		if handler == nil {
			t.Fatal("Registry() returned nil handler")
		}

		// Verify it returns a HandlerFunc type
		// We can't easily test the full flow without a real MCP server,
		// but we can verify the handler was created properly
	})

	// Note: Full integration testing of Registry() requires a real MCP server
	// connection, which is beyond the scope of unit tests. The Registry function
	// is tested indirectly through integration tests.
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

		if client.timeout != 30*time.Second {
			t.Errorf("Expected default timeout 30s, got %v", client.timeout)
		}

		if client.implementation == nil {
			t.Error("Expected implementation to be set")
		}

		if client.implementation.Name != "calque-mcp-client" {
			t.Errorf("Expected default implementation name, got '%s'", client.implementation.Name)
		}

		expectedCaps := []string{"tools", "resources", "prompts"}
		if len(client.capabilities) != len(expectedCaps) {
			t.Errorf("Expected %d capabilities, got %d", len(expectedCaps), len(client.capabilities))
		}

		for i, cap := range expectedCaps {
			if client.capabilities[i] != cap {
				t.Errorf("Expected capability[%d] = '%s', got '%s'", i, cap, client.capabilities[i])
			}
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
