package mcp

import (
	"strings"
	"testing"

	googleschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuildStructuredToolSelectionPrompt(t *testing.T) {
	t.Parallel()

	// Create test tools
	tools := []*MCPTool{
		{
			Tool: &mcp.Tool{
				Name:        "search",
				Description: "Search for information",
				InputSchema: &googleschema.Schema{
					Type:        "object",
					Description: "Search parameters",
				},
			},
		},
		{
			Tool: &mcp.Tool{
				Name:        "connect",
				Description: "Connect to server",
				InputSchema: &googleschema.Schema{
					Type:        "object",
					Description: "Connection parameters",
				},
			},
		},
	}

	tests := []struct {
		name        string
		userInput   string
		tools       []*MCPTool
		shouldContain []string
	}{
		{
			name:      "basic prompt with tools",
			userInput: "I want to search for golang",
			tools:     tools,
			shouldContain: []string{
				"tool selection assistant",
				"search",
				"connect",
				"Search for information",
				"Connect to server",
				"I want to search for golang",
				"JSON object",
				"selected_tool",
				"confidence",
			},
		},
		{
			name:      "empty tools",
			userInput: "test input",
			tools:     []*MCPTool{},
			shouldContain: []string{
				"tool selection assistant",
				"test input",
				"JSON object",
			},
		},
		{
			name:      "special characters in input",
			userInput: "Search for \"Go programming\" & tutorials!",
			tools:     tools[:1],
			shouldContain: []string{
				"Search for \"Go programming\" & tutorials!",
				"search",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prompt := buildStructuredToolSelectionPrompt(tt.userInput, tt.tools)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(prompt, expected) {
					t.Errorf("Prompt should contain '%s'\nPrompt: %s", expected, prompt)
				}
			}

			// Ensure prompt is not empty
			if strings.TrimSpace(prompt) == "" {
				t.Error("Prompt should not be empty")
			}
		})
	}
}

func TestSummarizeSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   *googleschema.Schema
		expected string
	}{
		{
			name:     "nil schema",
			schema:   nil,
			expected: "any",
		},
		{
			name: "schema with description",
			schema: &googleschema.Schema{
				Description: "Custom description",
				Type:        "object",
			},
			expected: "Custom description",
		},
		{
			name: "schema with type only",
			schema: &googleschema.Schema{
				Type: "string",
			},
			expected: "string",
		},
		{
			name: "schema with object type",
			schema: &googleschema.Schema{
				Type: "object",
			},
			expected: "object",
		},
		{
			name:     "empty schema",
			schema:   &googleschema.Schema{},
			expected: "structured data",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := summarizeSchema(tt.schema)
			if result != tt.expected {
				t.Errorf("summarizeSchema() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestParseToolSelectionResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		response    string
		expectError bool
		expected    *ToolSelectionResponse
	}{
		{
			name:     "valid complete response",
			response: `{"selected_tool": "search", "confidence": 0.9, "reasoning": "User wants to find information"}`,
			expected: &ToolSelectionResponse{
				SelectedTool: "search",
				Confidence:   0.9,
				Reasoning:    "User wants to find information",
			},
		},
		{
			name:     "valid minimal response",
			response: `{"selected_tool": "connect", "confidence": 0.8}`,
			expected: &ToolSelectionResponse{
				SelectedTool: "connect",
				Confidence:   0.8,
				Reasoning:    "",
			},
		},
		{
			name:     "null selected_tool",
			response: `{"selected_tool": null, "confidence": 0.1}`,
			expected: &ToolSelectionResponse{
				SelectedTool: "",
				Confidence:   0.1,
			},
		},
		{
			name:     "none selected_tool",
			response: `{"selected_tool": "none", "confidence": 0.0}`,
			expected: &ToolSelectionResponse{
				SelectedTool: "",
				Confidence:   0.0,
			},
		},
		{
			name:     "response with extra text",
			response: `Here's my selection: {"selected_tool": "analyze", "confidence": 0.95} Hope this helps!`,
			expected: &ToolSelectionResponse{
				SelectedTool: "analyze",
				Confidence:   0.95,
			},
		},
		{
			name:     "response with whitespace",
			response: `   {"selected_tool": "search", "confidence": 0.7}   `,
			expected: &ToolSelectionResponse{
				SelectedTool: "search",
				Confidence:   0.7,
			},
		},
		{
			name:        "invalid JSON",
			response:    `invalid json`,
			expectError: true,
		},
		{
			name:        "incomplete JSON",
			response:    `{"selected_tool": "test"`,
			expectError: true,
		},
		{
			name:        "missing required fields",
			response:    `{"reasoning": "test"}`,
			expectError: false, // Should parse with default values
			expected: &ToolSelectionResponse{
				SelectedTool: "",
				Confidence:   0,
				Reasoning:    "test",
			},
		},
		{
			name:        "empty response",
			response:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseToolSelectionResponse(tt.response)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.SelectedTool != tt.expected.SelectedTool {
				t.Errorf("SelectedTool = %s, expected %s", result.SelectedTool, tt.expected.SelectedTool)
			}

			if result.Confidence != tt.expected.Confidence {
				t.Errorf("Confidence = %f, expected %f", result.Confidence, tt.expected.Confidence)
			}

			if result.Reasoning != tt.expected.Reasoning {
				t.Errorf("Reasoning = %s, expected %s", result.Reasoning, tt.expected.Reasoning)
			}
		})
	}
}

func TestValidateToolSelection(t *testing.T) {
	t.Parallel()

	// Create test tools
	tools := []*MCPTool{
		{
			Tool: &mcp.Tool{Name: "search"},
		},
		{
			Tool: &mcp.Tool{Name: "connect"},
		},
		{
			Tool: &mcp.Tool{Name: "analyze_data"},
		},
	}

	tests := []struct {
		name         string
		selectedTool string
		tools        []*MCPTool
		expected     string
	}{
		{
			name:         "exact match",
			selectedTool: "search",
			tools:        tools,
			expected:     "search",
		},
		{
			name:         "case insensitive match",
			selectedTool: "SEARCH",
			tools:        tools,
			expected:     "search",
		},
		{
			name:         "case insensitive match mixed case",
			selectedTool: "Connect",
			tools:        tools,
			expected:     "connect",
		},
		{
			name:         "prefix match",
			selectedTool: "anal",
			tools:        tools,
			expected:     "analyze_data",
		},
		{
			name:         "prefix match with underscore",
			selectedTool: "analyze",
			tools:        tools,
			expected:     "analyze_data",
		},
		{
			name:         "no match",
			selectedTool: "nonexistent",
			tools:        tools,
			expected:     "",
		},
		{
			name:         "empty input",
			selectedTool: "",
			tools:        tools,
			expected:     "",
		},
		{
			name:         "empty tools",
			selectedTool: "search",
			tools:        []*MCPTool{},
			expected:     "",
		},
		{
			name:         "prefix match - returns first",
			selectedTool: "a", // matches "analyze_data" (starts with 'a')
			tools:        tools,
			expected:     "analyze_data", // First prefix match
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := validateToolSelection(tt.selectedTool, tt.tools)
			if result != tt.expected {
				t.Errorf("validateToolSelection(%s) = %s, expected %s", tt.selectedTool, result, tt.expected)
			}
		})
	}
}