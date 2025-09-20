package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDetectHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		llmResponse    string
		llmShouldError bool
		hasTools       bool
		expectedTool   string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "successful tool selection",
			input:          "search for golang tutorials",
			llmResponse:    `{"selected_tool": "search", "confidence": 0.9}`,
			hasTools:       true,
			expectedTool:   "search",
			expectedOutput: "search for golang tutorials",
		},
		{
			name:           "no tool selected",
			input:          "hello world",
			llmResponse:    `{"selected_tool": null, "confidence": 0.1}`,
			hasTools:       true,
			expectedTool:   "",
			expectedOutput: "hello world",
		},
		{
			name:           "invalid tool selected",
			input:          "do something",
			llmResponse:    `{"selected_tool": "nonexistent", "confidence": 0.8}`,
			hasTools:       true,
			expectedTool:   "",
			expectedOutput: "do something",
		},
		{
			name:           "LLM error - fallback",
			input:          "search for something",
			llmShouldError: true,
			hasTools:       true,
			expectedTool:   "",
			expectedOutput: "search for something",
		},
		{
			name:           "invalid JSON response - fallback",
			input:          "connect to server",
			llmResponse:    `invalid json response`,
			hasTools:       true,
			expectedTool:   "",
			expectedOutput: "connect to server",
		},
		{
			name:           "no tools available - pass through",
			input:          "do something",
			hasTools:       false,
			expectedTool:   "",
			expectedOutput: "do something",
		},
		{
			name:           "empty input - pass through",
			input:          "",
			hasTools:       true,
			expectedTool:   "",
			expectedOutput: "",
		},
		{
			name:           "whitespace only input - pass through",
			input:          "   \n\t   ",
			hasTools:       true,
			expectedTool:   "",
			expectedOutput: "   \n\t   ",
		},
		{
			name:           "case insensitive tool matching",
			input:          "search for data",
			llmResponse:    `{"selected_tool": "SEARCH", "confidence": 0.9}`,
			hasTools:       true,
			expectedTool:   "search",
			expectedOutput: "search for data",
		},
		{
			name:           "partial tool matching",
			input:          "analyze some data",
			llmResponse:    `{"selected_tool": "anal", "confidence": 0.8}`,
			hasTools:       true,
			expectedTool:   "analyze_data",
			expectedOutput: "analyze some data",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasTools {
				ctx = createTestContext()
			} else {
				ctx = context.Background()
			}

			// Setup mock LLM
			var mockLLM ai.Client
			if tt.llmShouldError {
				mockLLM = ai.NewMockClientWithError("mock LLM error")
			} else {
				// Use the exact response from the test case
				mockLLM = ai.NewMockClient(tt.llmResponse)
			}

			// Create handler
			handler := Detect(mockLLM)

			// Execute
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check output
			if output.String() != tt.expectedOutput {
				t.Errorf("Output = %q, expected %q", output.String(), tt.expectedOutput)
			}

			// Check selected tool in context
			selectedTool := GetSelectedTool(req.Context)
			if selectedTool != tt.expectedTool {
				t.Errorf("Selected tool = %q, expected %q", selectedTool, tt.expectedTool)
			}
		})
	}
}

func TestGetSelectedTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "no tool selected",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "tool selected",
			ctx:      context.WithValue(context.Background(), selectedToolContextKey{}, "search"),
			expected: "search",
		},
		{
			name:     "empty tool selected",
			ctx:      context.WithValue(context.Background(), selectedToolContextKey{}, ""),
			expected: "",
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), selectedToolContextKey{}, 123),
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetSelectedTool(tt.ctx)
			if result != tt.expected {
				t.Errorf("GetSelectedTool() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestHasSelectedTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "no tool selected",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name:     "tool selected",
			ctx:      context.WithValue(context.Background(), selectedToolContextKey{}, "search"),
			expected: true,
		},
		{
			name:     "empty tool selected",
			ctx:      context.WithValue(context.Background(), selectedToolContextKey{}, ""),
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := HasSelectedTool(tt.ctx)
			if result != tt.expected {
				t.Errorf("HasSelectedTool() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Helper function to create test context with mock tools
func createTestContext() context.Context {
	tools := []*Tool{
		{
			Name:        "search",
			Description: "Search for information",
			MCPTool: &mcp.Tool{
				Name:        "search",
				Description: "Search for information",
			},
		},
		{
			Name:        "connect",
			Description: "Connect to server",
			MCPTool: &mcp.Tool{
				Name:        "connect",
				Description: "Connect to server",
			},
		},
		{
			Name:        "analyze_data",
			Description: "Analyze data",
			MCPTool: &mcp.Tool{
				Name:        "analyze_data",
				Description: "Analyze data",
			},
		},
	}

	return context.WithValue(context.Background(), mcpToolsContextKey{}, tools)
}
