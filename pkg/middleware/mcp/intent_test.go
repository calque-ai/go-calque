package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/invopop/jsonschema"
)

// parseToolSelectionResponse extracts the tool selection from structured LLM JSON response (test helper)
func parseToolSelectionResponse(llmResponse string) (*ToolSelectionResponse, error) {
	var response ToolSelectionResponse

	// Clean up any potential JSON wrapper or extra text
	responseText := strings.TrimSpace(llmResponse)

	// Try to find JSON in the response (in case LLM adds extra text)
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}") + 1

	if jsonStart >= 0 && jsonEnd > jsonStart {
		responseText = responseText[jsonStart:jsonEnd]
	}

	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Handle null/empty tool selection
	if response.SelectedTool == "null" || response.SelectedTool == "none" {
		response.SelectedTool = ""
	}

	return &response, nil
}

func TestBuildStructuredToolSelectionPrompt(t *testing.T) {
	t.Parallel()

	// Create test tools
	tools := []*Tool{
		{
			Name:        "search",
			Description: "Search for information",
			InputSchema: &jsonschema.Schema{
				Type:        "object",
				Description: "Search parameters",
			},
		},
		{
			Name:        "connect",
			Description: "Connect to server",
			InputSchema: &jsonschema.Schema{
				Type:        "object",
				Description: "Connection parameters",
			},
		},
	}

	tests := []struct {
		name          string
		userInput     string
		tools         []*Tool
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
				"Return valid JSON only",
			},
		},
		{
			name:      "empty tools",
			userInput: "test input",
			tools:     []*Tool{},
			shouldContain: []string{
				"tool selection assistant",
				"test input",
				"Return valid JSON only",
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

			// Build prompt using template
			tmpl, err := template.New("test").Parse(toolSelectionPromptTemplate)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			templateData := getToolSelectionTemplateData(tt.userInput, tt.tools)
			var promptBuilder strings.Builder
			err = tmpl.Execute(&promptBuilder, templateData)
			if err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}
			prompt := promptBuilder.String()

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
		schema   *jsonschema.Schema
		expected string
	}{
		{
			name:     "nil schema",
			schema:   nil,
			expected: "any",
		},
		{
			name: "schema with description",
			schema: &jsonschema.Schema{
				Description: "Custom description",
				Type:        "object",
			},
			expected: "Custom description",
		},
		{
			name: "schema with type only",
			schema: &jsonschema.Schema{
				Type: "string",
			},
			expected: "string",
		},
		{
			name: "schema with object type",
			schema: &jsonschema.Schema{
				Type: "object",
			},
			expected: "object",
		},
		{
			name:     "empty schema",
			schema:   &jsonschema.Schema{},
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
	tools := []*Tool{
		{
			Name: "search",
		},
		{
			Name: "connect",
		},
		{
			Name: "analyze_data",
		},
	}

	tests := []struct {
		name         string
		selectedTool string
		tools        []*Tool
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
			tools:        []*Tool{},
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
