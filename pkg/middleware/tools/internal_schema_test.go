package tools

import (
	"encoding/json"
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
		{
			name: "tool with metadata",
			tool: &InternalToolSchema{
				Name:        "meta_tool",
				Description: "A tool with metadata",
				Metadata: map[string]any{
					"version": "1.0",
					"tags":    []string{"utility", "helper"},
				},
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

			// Test String method
			str := tt.tool.String()
			if str == "" {
				t.Error("String method should return non-empty result")
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
		{
			name:          "schema with additional properties",
			schema:        createSchemaWithAdditionalProperties(),
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

func TestConvertToInternalSchemaPropertyTypes(t *testing.T) {
	tests := []struct {
		name            string
		property        *jsonschema.Schema
		expectedType    string
		expectedDesc    string
		expectedMin     *float64
		expectedMax     *float64
		expectedMinLen  *int
		expectedMaxLen  *int
		expectedEnum    []any
		expectedDefault any
		hasItems        bool
		hasProperties   bool
	}{
		{
			name: "string property with constraints",
			property: &jsonschema.Schema{
				Type:        "string",
				Description: "Search query",
				MinLength:   helpers.PtrOf(uint64(1)),
				MaxLength:   helpers.PtrOf(uint64(100)),
				Pattern:     "^[a-zA-Z0-9\\s]+$",
			},
			expectedType:   "string",
			expectedDesc:   "Search query",
			expectedMinLen: helpers.PtrOf(1),
			expectedMaxLen: helpers.PtrOf(100),
		},
		{
			name: "integer property with constraints",
			property: &jsonschema.Schema{
				Type:        "integer",
				Description: "Maximum number of results",
				Default:     10,
				Minimum:     "1",
				Maximum:     "100",
			},
			expectedType:    "integer",
			expectedDesc:    "Maximum number of results",
			expectedMin:     helpers.PtrOf(1.0),
			expectedMax:     helpers.PtrOf(100.0),
			expectedDefault: 10,
		},
		{
			name: "array property with items",
			property: &jsonschema.Schema{
				Type: "array",
				Items: &jsonschema.Schema{
					Type: "string",
					Enum: []any{"active", "inactive", "pending"},
				},
			},
			expectedType: "array",
			hasItems:     true,
		},
		{
			name: "property with enum",
			property: &jsonschema.Schema{
				Type: "string",
				Enum: []any{"active", "inactive", "pending"},
			},
			expectedType: "string",
			expectedEnum: []any{"active", "inactive", "pending"},
		},
		{
			name: "property with nested properties",
			property: &jsonschema.Schema{
				Type: "object",
				Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
					props := orderedmap.New[string, *jsonschema.Schema]()
					props.Set("nested", &jsonschema.Schema{
						Type: "string",
					})
					return props
				}(),
			},
			expectedType:  "object",
			hasProperties: true,
		},
		{
			name: "property with additional properties",
			property: &jsonschema.Schema{
				Type: "object",
				AdditionalProperties: &jsonschema.Schema{
					Type: "string",
				},
			},
			expectedType: "object",
		},
		{
			name: "property with invalid minimum (non-numeric)",
			property: &jsonschema.Schema{
				Type:    "integer",
				Minimum: "invalid",
			},
			expectedType: "integer",
		},
		{
			name: "property with invalid maximum (non-numeric)",
			property: &jsonschema.Schema{
				Type:    "integer",
				Maximum: "invalid",
			},
			expectedType: "integer",
		},
		{
			name: "property with nil MinLength",
			property: &jsonschema.Schema{
				Type:      "string",
				MinLength: nil,
			},
			expectedType: "string",
		},
		{
			name: "property with nil MaxLength",
			property: &jsonschema.Schema{
				Type:      "string",
				MaxLength: nil,
			},
			expectedType: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			internal := convertPropertyToInternal(tt.property)

			if internal == nil {
				t.Fatal("Expected internal property to be created")
			}

			if internal.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, internal.Type)
			}

			if tt.expectedDesc != "" && internal.Description != tt.expectedDesc {
				t.Errorf("Expected description %q, got %q", tt.expectedDesc, internal.Description)
			}

			if tt.expectedMin != nil {
				if internal.Minimum == nil {
					t.Error("Expected Minimum to be set")
				} else if *internal.Minimum != *tt.expectedMin {
					t.Errorf("Expected Minimum %f, got %f", *tt.expectedMin, *internal.Minimum)
				}
			}

			if tt.expectedMax != nil {
				if internal.Maximum == nil {
					t.Error("Expected Maximum to be set")
				} else if *internal.Maximum != *tt.expectedMax {
					t.Errorf("Expected Maximum %f, got %f", *tt.expectedMax, *internal.Maximum)
				}
			}

			if tt.expectedMinLen != nil {
				if internal.MinLength == nil {
					t.Error("Expected MinLength to be set")
				} else if *internal.MinLength != *tt.expectedMinLen {
					t.Errorf("Expected MinLength %d, got %d", *tt.expectedMinLen, *internal.MinLength)
				}
			}

			if tt.expectedMaxLen != nil {
				if internal.MaxLength == nil {
					t.Error("Expected MaxLength to be set")
				} else if *internal.MaxLength != *tt.expectedMaxLen {
					t.Errorf("Expected MaxLength %d, got %d", *tt.expectedMaxLen, *internal.MaxLength)
				}
			}

			if tt.expectedEnum != nil {
				if !reflect.DeepEqual(internal.Enum, tt.expectedEnum) {
					t.Errorf("Expected enum %v, got %v", tt.expectedEnum, internal.Enum)
				}
			}

			if tt.expectedDefault != nil {
				if !reflect.DeepEqual(internal.Default, tt.expectedDefault) {
					t.Errorf("Expected default %v, got %v", tt.expectedDefault, internal.Default)
				}
			}

			if tt.hasItems && internal.Items == nil {
				t.Error("Expected Items to be set")
			}

			if tt.hasProperties && internal.Properties == nil {
				t.Error("Expected Properties to be set")
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
		})
	}
}

func TestInternalToolSchemaString(t *testing.T) {
	tests := []struct {
		name          string
		tool          *InternalToolSchema
		expectedName  string
		expectedValid bool
	}{
		{
			name: "basic tool",
			tool: &InternalToolSchema{
				Name:        "test_tool",
				Description: "A test tool",
			},
			expectedName:  "test_tool",
			expectedValid: true,
		},
		{
			name: "tool with metadata",
			tool: &InternalToolSchema{
				Name:        "meta_tool",
				Description: "A tool with metadata",
				Metadata: map[string]any{
					"version": "1.0",
					"tags":    []string{"utility", "helper"},
				},
			},
			expectedName:  "meta_tool",
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tool.String()

			if result == "" {
				t.Error("Expected non-empty string result")
				return
			}

			if tt.expectedValid {
				// Should be valid JSON
				var parsed map[string]any
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					t.Errorf("Expected valid JSON, got error: %v", err)
				}

				if parsed["name"] != tt.expectedName {
					t.Errorf("Expected parsed name %q, got %v", tt.expectedName, parsed["name"])
				}
			}
		})
	}
}

func TestConvertToInternalSchemaEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		schema        *jsonschema.Schema
		expectedNil   bool
		expectedType  string
		expectedProps int
	}{
		{
			name:        "nil schema",
			schema:      nil,
			expectedNil: true,
		},
		{
			name:          "empty schema",
			schema:        &jsonschema.Schema{},
			expectedNil:   false,
			expectedType:  "",
			expectedProps: 0,
		},
		{
			name: "schema with only type",
			schema: &jsonschema.Schema{
				Type: "string",
			},
			expectedNil:   false,
			expectedType:  "string",
			expectedProps: 0,
		},
		{
			name: "schema with empty properties",
			schema: &jsonschema.Schema{
				Type:       "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](),
			},
			expectedNil:   false,
			expectedType:  "object",
			expectedProps: 0,
		},
		{
			name: "schema with nil properties",
			schema: &jsonschema.Schema{
				Type:       "object",
				Properties: nil,
			},
			expectedNil:   false,
			expectedType:  "object",
			expectedProps: 0,
		},
		{
			name: "schema with nil additional properties",
			schema: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: nil,
			},
			expectedNil:   false,
			expectedType:  "object",
			expectedProps: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			internal := ConvertToInternalSchema(tt.schema)

			if tt.expectedNil {
				if internal != nil {
					t.Error("Expected nil result")
				}
				return
			}

			if internal == nil {
				t.Fatal("Expected internal schema to be created")
			}

			if internal.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, internal.Type)
			}

			if tt.expectedProps == 0 {
				if len(internal.Properties) > 0 {
					t.Errorf("Expected no properties, got %d", len(internal.Properties))
				}
			}
		})
	}
}

func TestConvertAdditionalPropertiesToInternal(t *testing.T) {
	tests := []struct {
		name          string
		additional    *jsonschema.Schema
		expectedType  string
		expectedProps int
		expectedNil   bool
	}{
		{
			name:        "nil additional properties",
			additional:  nil,
			expectedNil: true,
		},
		{
			name: "simple additional properties",
			additional: &jsonschema.Schema{
				Type: "string",
			},
			expectedType:  "string",
			expectedProps: 0,
		},
		{
			name: "additional properties with properties",
			additional: &jsonschema.Schema{
				Type: "object",
				Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
					props := orderedmap.New[string, *jsonschema.Schema]()
					props.Set("key", &jsonschema.Schema{
						Type: "string",
					})
					return props
				}(),
			},
			expectedType:  "object",
			expectedProps: 1,
		},
		{
			name: "additional properties with nested additional properties",
			additional: &jsonschema.Schema{
				Type: "object",
				AdditionalProperties: &jsonschema.Schema{
					Type: "string",
				},
			},
			expectedType:  "object",
			expectedProps: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			internal := convertAdditionalPropertiesToInternal(tt.additional)

			if tt.expectedNil {
				if internal != nil {
					t.Error("Expected nil result")
				}
				return
			}

			if internal == nil {
				t.Fatal("Expected internal additional properties to be created")
			}

			if internal.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, internal.Type)
			}

			if tt.expectedProps > 0 {
				if internal.Properties == nil {
					t.Fatal("Expected properties to exist")
				}
				if len(internal.Properties) != tt.expectedProps {
					t.Errorf("Expected %d properties, got %d", tt.expectedProps, len(internal.Properties))
				}
			}
		})
	}
}

func TestFormatToolsAsOpenAIInternalErrorCases(t *testing.T) {
	tests := []struct {
		name            string
		tools           []Tool
		expectedStrings []string
		expectedEmpty   bool
	}{
		{
			name:            "empty tools",
			tools:           []Tool{},
			expectedStrings: []string{},
			expectedEmpty:   true,
		},
		{
			name:  "single tool with parameters",
			tools: []Tool{Simple("calculator", "Perform calculations", func(s string) string { return s })},
			expectedStrings: []string{
				"Available functions:",
				`"functions":`,
				`"name": "calculator"`,
				`"description": "Perform calculations"`,
			},
			expectedEmpty: false,
		},
		{
			name: "multiple tools with complex parameters",
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
			expectedEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatToolsAsOpenAIInternal(tt.tools)

			if tt.expectedEmpty {
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

func TestInternalToolSchemaStringErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		tool          *InternalToolSchema
		expectedName  string
		expectedValid bool
	}{
		{
			name: "basic tool",
			tool: &InternalToolSchema{
				Name:        "test_tool",
				Description: "A test tool",
			},
			expectedName:  "test_tool",
			expectedValid: true,
		},
		{
			name: "tool with metadata",
			tool: &InternalToolSchema{
				Name:        "meta_tool",
				Description: "A tool with metadata",
				Metadata: map[string]any{
					"version": "1.0",
					"tags":    []string{"utility", "helper"},
				},
			},
			expectedName:  "meta_tool",
			expectedValid: true,
		},
		{
			name: "tool with complex nested structure",
			tool: &InternalToolSchema{
				Name:        "complex_tool",
				Description: "A tool with complex structure",
				Parameters: &InternalParameterSchema{
					Type: "object",
					Properties: map[string]*InternalProperty{
						"nested": {
							Type: "object",
							Properties: map[string]*InternalProperty{
								"deep": {
									Type: "string",
								},
							},
						},
					},
				},
			},
			expectedName:  "complex_tool",
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tool.String()

			if result == "" {
				t.Error("Expected non-empty string result")
				return
			}

			if tt.expectedValid {
				// Should be valid JSON
				var parsed map[string]any
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					t.Errorf("Expected valid JSON, got error: %v", err)
				}

				if parsed["name"] != tt.expectedName {
					t.Errorf("Expected parsed name %q, got %v", tt.expectedName, parsed["name"])
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

// Helper function to create schema with additional properties
func createSchemaWithAdditionalProperties() *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("name", &jsonschema.Schema{
		Type:        "string",
		Description: "Name field",
	})

	additionalProps := &jsonschema.Schema{
		Type: "string",
	}

	return &jsonschema.Schema{
		Type:                 "object",
		Properties:           properties,
		Required:             []string{"name"},
		AdditionalProperties: additionalProps,
	}
}
