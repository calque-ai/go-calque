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
			handler := DetectTool(mockLLM)

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

func TestDetectResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		input            string
		llmResponse      string
		llmShouldError   bool
		hasResources     bool
		expectedResource string
		expectedOutput   string
		expectError      bool
	}{
		{
			name:             "successful resource selection",
			input:            "summarize the API documentation",
			llmResponse:      `{"selected_resource": "file:///docs/api.md", "confidence": 0.9}`,
			hasResources:     true,
			expectedResource: "file:///docs/api.md",
			expectedOutput:   "summarize the API documentation",
		},
		{
			name:             "no resource selected",
			input:            "hello world",
			llmResponse:      `{"selected_resource": null, "confidence": 0.1}`,
			hasResources:     true,
			expectedResource: "",
			expectedOutput:   "hello world",
		},
		{
			name:             "invalid resource selected",
			input:            "get some data",
			llmResponse:      `{"selected_resource": "file:///nonexistent.txt", "confidence": 0.8}`,
			hasResources:     true,
			expectedResource: "",
			expectedOutput:   "get some data",
		},
		{
			name:             "LLM error - fallback",
			input:            "fetch documentation",
			llmShouldError:   true,
			hasResources:     true,
			expectedResource: "",
			expectedOutput:   "fetch documentation",
		},
		{
			name:             "invalid JSON response - fallback",
			input:            "read config",
			llmResponse:      `invalid json response`,
			hasResources:     true,
			expectedResource: "",
			expectedOutput:   "read config",
		},
		{
			name:             "no resources available - pass through",
			input:            "do something",
			hasResources:     false,
			expectedResource: "",
			expectedOutput:   "do something",
		},
		{
			name:             "empty input - pass through",
			input:            "",
			hasResources:     true,
			expectedResource: "",
			expectedOutput:   "",
		},
		{
			name:             "whitespace only input - pass through",
			input:            "   \n\t   ",
			hasResources:     true,
			expectedResource: "",
			expectedOutput:   "   \n\t   ",
		},
		{
			name:             "case insensitive URI matching",
			input:            "get the docs",
			llmResponse:      `{"selected_resource": "FILE:///DOCS/API.MD", "confidence": 0.9}`,
			hasResources:     true,
			expectedResource: "file:///docs/api.md",
			expectedOutput:   "get the docs",
		},
		{
			name:             "name-based resource matching",
			input:            "show me the API guide",
			llmResponse:      `{"selected_resource": "API Documentation", "confidence": 0.85}`,
			hasResources:     true,
			expectedResource: "file:///docs/api.md",
			expectedOutput:   "show me the API guide",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasResources {
				ctx = createTestResourceContext()
			} else {
				ctx = context.Background()
			}

			// Setup mock LLM
			var mockLLM ai.Client
			if tt.llmShouldError {
				mockLLM = ai.NewMockClientWithError("mock LLM error")
			} else {
				mockLLM = ai.NewMockClient(tt.llmResponse)
			}

			// Create handler
			handler := DetectResource(mockLLM)

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

			// Check selected resource in context
			selectedResource := GetSelectedResource(req.Context)
			if selectedResource != tt.expectedResource {
				t.Errorf("Selected resource = %q, expected %q", selectedResource, tt.expectedResource)
			}
		})
	}
}

func TestDetectPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		llmResponse    string
		llmShouldError bool
		hasPrompts     bool
		expectedPrompt string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "successful prompt selection",
			input:          "help me write a blog post",
			llmResponse:    `{"selected_prompt": "blog_writer", "confidence": 0.9}`,
			hasPrompts:     true,
			expectedPrompt: "blog_writer",
			expectedOutput: "help me write a blog post",
		},
		{
			name:           "no prompt selected",
			input:          "hello there",
			llmResponse:    `{"selected_prompt": null, "confidence": 0.1}`,
			hasPrompts:     true,
			expectedPrompt: "",
			expectedOutput: "hello there",
		},
		{
			name:           "invalid prompt selected",
			input:          "do something",
			llmResponse:    `{"selected_prompt": "nonexistent", "confidence": 0.8}`,
			hasPrompts:     true,
			expectedPrompt: "",
			expectedOutput: "do something",
		},
		{
			name:           "LLM error - fallback",
			input:          "create content",
			llmShouldError: true,
			hasPrompts:     true,
			expectedPrompt: "",
			expectedOutput: "create content",
		},
		{
			name:           "invalid JSON response - fallback",
			input:          "write code",
			llmResponse:    `invalid json response`,
			hasPrompts:     true,
			expectedPrompt: "",
			expectedOutput: "write code",
		},
		{
			name:           "no prompts available - pass through",
			input:          "do something",
			hasPrompts:     false,
			expectedPrompt: "",
			expectedOutput: "do something",
		},
		{
			name:           "empty input - pass through",
			input:          "",
			hasPrompts:     true,
			expectedPrompt: "",
			expectedOutput: "",
		},
		{
			name:           "whitespace only input - pass through",
			input:          "   \n\t   ",
			hasPrompts:     true,
			expectedPrompt: "",
			expectedOutput: "   \n\t   ",
		},
		{
			name:           "case insensitive prompt matching",
			input:          "review my code",
			llmResponse:    `{"selected_prompt": "CODE_REVIEW", "confidence": 0.9}`,
			hasPrompts:     true,
			expectedPrompt: "code_review",
			expectedOutput: "review my code",
		},
		{
			name:           "partial prompt matching",
			input:          "write blog content",
			llmResponse:    `{"selected_prompt": "blog", "confidence": 0.85}`,
			hasPrompts:     true,
			expectedPrompt: "blog_writer",
			expectedOutput: "write blog content",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasPrompts {
				ctx = createTestPromptContext()
			} else {
				ctx = context.Background()
			}

			// Setup mock LLM
			var mockLLM ai.Client
			if tt.llmShouldError {
				mockLLM = ai.NewMockClientWithError("mock LLM error")
			} else {
				mockLLM = ai.NewMockClient(tt.llmResponse)
			}

			// Create handler
			handler := DetectPrompt(mockLLM)

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

			// Check selected prompt in context
			selectedPrompt := GetSelectedPrompt(req.Context)
			if selectedPrompt != tt.expectedPrompt {
				t.Errorf("Selected prompt = %q, expected %q", selectedPrompt, tt.expectedPrompt)
			}
		})
	}
}

// Helper function to create test context with mock resources
func createTestResourceContext() context.Context {
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
		{
			URI:         "file:///data/users.db",
			Name:        "User Database",
			Description: "User data storage",
		},
	}

	return context.WithValue(context.Background(), mcpResourcesContextKey{}, resources)
}

// Helper function to create test context with mock prompts
func createTestPromptContext() context.Context {
	prompts := []*mcp.Prompt{
		{
			Name:        "blog_writer",
			Description: "Help write blog posts",
		},
		{
			Name:        "code_review",
			Description: "Review code for best practices",
		},
		{
			Name:        "summarizer",
			Description: "Summarize text content",
		},
	}

	return context.WithValue(context.Background(), mcpPromptsContextKey{}, prompts)
}
