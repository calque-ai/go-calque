package tools

import (
	"reflect"
	"testing"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/calque-ai/go-calque/pkg/helpers"
)

func TestInternalToolSchema(t *testing.T) {
	tests := []struct {
		name        string
		tool        *InternalToolSchema
		expectError bool
	}{
		{
			name: "basic tool with parameters",
			tool: &InternalToolSchema{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: &InternalParameterSchema{
					Type: "object",
					Properties: map[string]*InternalProperty{
						"input": {
							Type:        "string",
							Description: "Input parameter",
						},
					},
					Required: []string{"input"},
				},
			},
			expectError: false,
		},
		{
			name: "tool without parameters",
			tool: &InternalToolSchema{
				Name:        "simple_tool",
				Description: "A simple tool",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool.Name == "" {
				t.Error("Tool name should not be empty")
			}

			if tt.tool.Description == "" {
				t.Error("Tool description should not be empty")
			}

			if tt.tool.Parameters != nil {
				if tt.tool.Parameters.Type != "object" {
					t.Errorf("Expected parameter type 'object', got %q", tt.tool.Parameters.Type)
				}

				if len(tt.tool.Parameters.Required) > 0 {
					for _, req := range tt.tool.Parameters.Required {
						if req == "" {
							t.Error("Required field name should not be empty")
						}
					}
				}
			}
		})
	}
}

func TestConvertToInternalSchema(t *testing.T) {
	tests := []struct {
		name          string
		schema        *jsonschema.Schema
		expectedType  string
		expectedProps int
		expectedReq   []string
		shouldBeNil   bool
	}{
		{
			name:        "nil schema",
			schema:      nil,
			shouldBeNil: true,
		},
		{
			name:          "simple object schema",
			schema:        &jsonschema.Schema{Type: "object"},
			expectedType:  "object",
			expectedProps: 0,
			expectedReq:   nil, // Function returns nil for empty required
			shouldBeNil:   false,
		},
		{
			name:          "schema with properties",
			schema:        createTestSchema(),
			expectedType:  "object",
			expectedProps: 1,
			expectedReq:   []string{"name"},
			shouldBeNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToInternalSchema(tt.schema)

			if tt.shouldBeNil {
				if result != nil {
					t.Error("Expected nil result for nil input")
				}
				return
			}

			if result == nil {
				t.Fatal("Expected internal schema to be created")
			}

			if result.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, result.Type)
			}

			// Handle nil vs empty slice comparison
			if tt.expectedReq == nil && result.Required != nil {
				t.Errorf("Expected nil required fields, got %v", result.Required)
			} else if tt.expectedReq != nil && !reflect.DeepEqual(result.Required, tt.expectedReq) {
				t.Errorf("Expected required fields %v, got %v", tt.expectedReq, result.Required)
			}

			if tt.expectedProps > 0 {
				if result.Properties == nil {
					t.Fatal("Expected properties to exist")
				}

				if len(result.Properties) != tt.expectedProps {
					t.Errorf("Expected %d properties, got %d", tt.expectedProps, len(result.Properties))
				}
			}
		})
	}
}

func TestFormatToolsAsInternal(t *testing.T) {
	tests := []struct {
		name          string
		tools         []Tool
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "empty tools list",
			tools:         []Tool{},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name: "single tool",
			tools: []Tool{
				Simple("calculator", "Perform calculations", func(s string) string { return s }),
			},
			expectedCount: 1,
			expectedNames: []string{"calculator"},
		},
		{
			name: "multiple tools",
			tools: []Tool{
				Simple("calculator", "Perform calculations", func(s string) string { return s }),
				Simple("search", "Search the web", func(s string) string { return s }),
			},
			expectedCount: 2,
			expectedNames: []string{"calculator", "search"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			internalTools := FormatToolsAsInternal(tt.tools)

			if len(internalTools) != tt.expectedCount {
				t.Errorf("Expected %d internal tools, got %d", tt.expectedCount, len(internalTools))
			}

			if tt.expectedCount == 0 {
				if internalTools != nil {
					t.Error("Expected nil result for empty tools")
				}
				return
			}

			for i, tool := range internalTools {
				if tool.Name != tt.expectedNames[i] {
					t.Errorf("Tool %d: expected name %q, got %q", i, tt.expectedNames[i], tool.Name)
				}

				if tool.Description == "" {
					t.Errorf("Tool %d: description should not be empty", i)
				}
			}
		})
	}
}

func TestFormatToolsAsOpenAIInternal(t *testing.T) {
	tests := []struct {
		name            string
		tools           []Tool
		expectedStrings []string
	}{
		{
			name:            "empty tools",
			tools:           []Tool{},
			expectedStrings: []string{},
		},
		{
			name:  "single tool",
			tools: []Tool{Simple("calculator", "Perform calculations", func(s string) string { return s })},
			expectedStrings: []string{
				"Available functions:",
				`"functions":`,
				`"name": "calculator"`,
				`"description": "Perform calculations"`,
			},
		},
		{
			name: "multiple tools",
			tools: []Tool{
				Simple("calculator", "Perform calculations", func(s string) string { return s }),
				Simple("search", "Search the web", func(s string) string { return s }),
			},
			expectedStrings: []string{
				"Available functions:",
				`"functions":`,
				`"name": "calculator"`,
				`"name": "search"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatToolsAsOpenAIInternal(tt.tools)

			if len(tt.tools) == 0 {
				if result != "" {
					t.Errorf("Expected empty string for empty tools, got %q", result)
				}
				return
			}

			if result == "" {
				t.Error("Expected non-empty result")
				return
			}

			for _, expected := range tt.expectedStrings {
				if !helpers.Contains(result, expected) {
					t.Errorf("Result missing expected string: %q", expected)
				}
			}
		})
	}
}

func TestProviderFormatConversions(t *testing.T) {
	tests := []struct {
		name           string
		tool           *InternalToolSchema
		expectedOpenAI map[string]any
		expectedGemini map[string]any
		expectedOllama map[string]any
	}{
		{
			name: "tool with parameters",
			tool: &InternalToolSchema{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: &InternalParameterSchema{
					Type: "object",
					Properties: map[string]*InternalProperty{
						"input": {
							Type:        "string",
							Description: "Input parameter",
						},
					},
					Required: []string{"input"},
				},
			},
			expectedOpenAI: map[string]any{
				"name":        "test_tool",
				"description": "A test tool",
			},
			expectedGemini: map[string]any{
				"name":        "test_tool",
				"description": "A test tool",
			},
			expectedOllama: map[string]any{
				"type": "function",
			},
		},
		{
			name: "tool without parameters",
			tool: &InternalToolSchema{
				Name:        "simple_tool",
				Description: "A simple tool",
			},
			expectedOpenAI: map[string]any{
				"name":        "simple_tool",
				"description": "A simple tool",
			},
			expectedGemini: map[string]any{
				"name":        "simple_tool",
				"description": "A simple tool",
			},
			expectedOllama: map[string]any{
				"type": "function",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test OpenAI format
			openaiFormat := tt.tool.ToOpenAIFormat()
			for key, expectedValue := range tt.expectedOpenAI {
				if actualValue, exists := openaiFormat[key]; !exists {
					t.Errorf("OpenAI format missing key: %s", key)
				} else if !reflect.DeepEqual(actualValue, expectedValue) {
					t.Errorf("OpenAI format %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}

			// Test Gemini format
			geminiFormat := tt.tool.ToGeminiFormat()
			for key, expectedValue := range tt.expectedGemini {
				if actualValue, exists := geminiFormat[key]; !exists {
					t.Errorf("Gemini format missing key: %s", key)
				} else if !reflect.DeepEqual(actualValue, expectedValue) {
					t.Errorf("Gemini format %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}

			// Test Ollama format
			ollamaFormat := tt.tool.ToOllamaFormat()
			for key, expectedValue := range tt.expectedOllama {
				if actualValue, exists := ollamaFormat[key]; !exists {
					t.Errorf("Ollama format missing key: %s", key)
				} else if !reflect.DeepEqual(actualValue, expectedValue) {
					t.Errorf("Ollama format %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// Helper function to create test schema
func createTestSchema() *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("name", &jsonschema.Schema{
		Type:        "string",
		Description: "Name field",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"name"},
	}
}
