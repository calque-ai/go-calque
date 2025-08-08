package tools

import (
	"strings"
	"testing"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func TestFormatToolsAsOpenAI(t *testing.T) {
	// Create test tools with different schemas
	calc := Simple("calculator", "Perform mathematical calculations", func(s string) string { return s })
	search := Simple("search", "Search the web for information", func(s string) string { return s })

	tests := []struct {
		name     string
		tools    []Tool
		expected []string
	}{
		{
			name:  "empty tools",
			tools: []Tool{},
			expected: []string{
				"", // Should return empty string for no tools
			},
		},
		{
			name:  "single tool",
			tools: []Tool{calc},
			expected: []string{
				"Available functions:",
				`"functions":`,
				`"name": "calculator"`,
				`"description": "Perform mathematical calculations"`,
				`"parameters":`,
			},
		},
		{
			name:  "multiple tools",
			tools: []Tool{calc, search},
			expected: []string{
				"Available functions:",
				`"functions":`,
				`"name": "calculator"`,
				`"description": "Perform mathematical calculations"`,
				`"name": "search"`,
				`"description": "Search the web for information"`,
				`"parameters":`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatToolsAsOpenAI(tt.tools)

			if len(tt.tools) == 0 {
				if result != "" {
					t.Errorf("FormatToolsAsOpenAI() with empty tools = %q, want empty string", result)
				}
				return
			}

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("FormatToolsAsOpenAI() missing expected string %q in result: %q", expected, result)
				}
			}
		})
	}
}

func TestFormatToolsAsOpenAIWithCustomSchema(t *testing.T) {
	// Create a tool with a custom schema
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("query", &jsonschema.Schema{
		Type:        "string",
		Description: "The search query",
	})
	properties.Set("limit", &jsonschema.Schema{
		Type:        "integer",
		Description: "Maximum number of results",
		Default:     10,
	})

	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"query"},
	}

	customTool := New("web_search", "Search the web with advanced options", schema, Simple("test", "test", func(s string) string { return s }))

	result := FormatToolsAsOpenAI([]Tool{customTool})

	expected := []string{
		"Available functions:",
		`"name": "web_search"`,
		`"description": "Search the web with advanced options"`,
		`"parameters":`,
		`"query"`,
		`"limit"`,
		`"required":`,
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("FormatToolsAsOpenAI() with custom schema missing expected string %q in result: %q", exp, result)
		}
	}
}

func TestToolDefinitionStructure(t *testing.T) {
	// Test that ToolDefinition has the correct structure
	td := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input parameter",
				},
			},
		},
	}

	if td.Name != "test_tool" {
		t.Errorf("ToolDefinition.Name = %q, want %q", td.Name, "test_tool")
	}

	if td.Description != "A test tool" {
		t.Errorf("ToolDefinition.Description = %q, want %q", td.Description, "A test tool")
	}

	if td.Parameters == nil {
		t.Error("ToolDefinition.Parameters should not be nil")
	}
}

func TestFormatToolsAsOpenAIJsonStructure(t *testing.T) {
	// Test that the output is valid JSON structure
	calc := Simple("calculator", "Math tool", func(s string) string { return s })
	result := FormatToolsAsOpenAI([]Tool{calc})

	// Should contain valid JSON structure
	expectedStructure := []string{
		"{",     // JSON object start
		"}",     // JSON object end
		"[",     // Array start for functions
		"]",     // Array end for functions
		`"functions"`, // functions key
	}

	for _, expected := range expectedStructure {
		if !strings.Contains(result, expected) {
			t.Errorf("FormatToolsAsOpenAI() missing JSON structure element %q in result: %q", expected, result)
		}
	}
}