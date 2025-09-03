package tools

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// InternalToolSchema represents a tool in our provider-agnostic internal format
// This schema is designed to be easily convertible to any LLM provider's tool format
type InternalToolSchema struct {
	// Core tool information
	Name        string `json:"name"`
	Description string `json:"description"`

	// Parameter schema in a simplified, provider-agnostic format
	Parameters *InternalParameterSchema `json:"parameters,omitempty"`

	// Optional metadata for advanced use cases
	Metadata map[string]any `json:"metadata,omitempty"`
}

// InternalParameterSchema represents parameters in a simplified, provider-agnostic format
// This avoids the complexity of jsonschema.Schema while maintaining all necessary information
type InternalParameterSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]*InternalProperty  `json:"properties,omitempty"`
	Required   []string                      `json:"required,omitempty"`
	Additional *InternalAdditionalProperties `json:"additionalProperties,omitempty"`
}

// InternalProperty represents a single parameter property
type InternalProperty struct {
	Type                 string                        `json:"type"`
	Description          string                        `json:"description,omitempty"`
	Enum                 []any                         `json:"enum,omitempty"`
	Default              any                           `json:"default,omitempty"`
	Format               string                        `json:"format,omitempty"`
	Pattern              string                        `json:"pattern,omitempty"`
	Minimum              *float64                      `json:"minimum,omitempty"`
	Maximum              *float64                      `json:"maximum,omitempty"`
	MinLength            *int                          `json:"minLength,omitempty"`
	MaxLength            *int                          `json:"maxLength,omitempty"`
	Items                *InternalProperty             `json:"items,omitempty"`
	Properties           map[string]*InternalProperty  `json:"properties,omitempty"`
	AdditionalProperties *InternalAdditionalProperties `json:"additionalProperties,omitempty"`
}

// InternalAdditionalProperties represents additional properties configuration
type InternalAdditionalProperties struct {
	Type                 string                        `json:"type,omitempty"`
	Properties           map[string]*InternalProperty  `json:"properties,omitempty"`
	AdditionalProperties *InternalAdditionalProperties `json:"additionalProperties,omitempty"`
}

// ConvertToInternalSchema converts a jsonschema.Schema to our internal format
// This eliminates the need for runtime JSON marshaling/unmarshaling
func ConvertToInternalSchema(schema *jsonschema.Schema) *InternalParameterSchema {
	if schema == nil {
		return nil
	}

	internal := &InternalParameterSchema{
		Type:     schema.Type,
		Required: schema.Required,
	}

	// Convert properties
	if schema.Properties != nil {
		internal.Properties = make(map[string]*InternalProperty)
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			internal.Properties[pair.Key] = convertPropertyToInternal(pair.Value)
		}
	}

	// Convert additional properties
	if schema.AdditionalProperties != nil {
		internal.Additional = convertAdditionalPropertiesToInternal(schema.AdditionalProperties)
	}

	return internal
}

// convertPropertyToInternal converts a jsonschema.Property to InternalProperty
func convertPropertyToInternal(prop *jsonschema.Schema) *InternalProperty {
	if prop == nil {
		return nil
	}

	internal := &InternalProperty{
		Type:        prop.Type,
		Description: prop.Description,
		Enum:        prop.Enum,
		Default:     prop.Default,
		Format:      prop.Format,
		Pattern:     prop.Pattern,
	}

	// Convert numeric constraints
	if prop.Minimum != "" {
		if min, err := prop.Minimum.Float64(); err == nil {
			internal.Minimum = &min
		}
	}
	if prop.Maximum != "" {
		if max, err := prop.Maximum.Float64(); err == nil {
			internal.Maximum = &max
		}
	}
	if prop.MinLength != nil {
		minLen := int(*prop.MinLength)
		internal.MinLength = &minLen
	}
	if prop.MaxLength != nil {
		maxLen := int(*prop.MaxLength)
		internal.MaxLength = &maxLen
	}

	// Convert nested structures
	if prop.Items != nil {
		internal.Items = convertPropertyToInternal(prop.Items)
	}
	if prop.Properties != nil {
		internal.Properties = make(map[string]*InternalProperty)
		for pair := prop.Properties.Oldest(); pair != nil; pair = pair.Next() {
			internal.Properties[pair.Key] = convertPropertyToInternal(pair.Value)
		}
	}
	if prop.AdditionalProperties != nil {
		internal.AdditionalProperties = convertAdditionalPropertiesToInternal(prop.AdditionalProperties)
	}

	return internal
}

// convertAdditionalPropertiesToInternal converts additional properties
func convertAdditionalPropertiesToInternal(additional *jsonschema.Schema) *InternalAdditionalProperties {
	if additional == nil {
		return nil
	}

	internal := &InternalAdditionalProperties{
		Type: additional.Type,
	}

	if additional.Properties != nil {
		internal.Properties = make(map[string]*InternalProperty)
		for pair := additional.Properties.Oldest(); pair != nil; pair = pair.Next() {
			internal.Properties[pair.Key] = convertPropertyToInternal(pair.Value)
		}
	}

	if additional.AdditionalProperties != nil {
		internal.AdditionalProperties = convertAdditionalPropertiesToInternal(additional.AdditionalProperties)
	}

	return internal
}

// FormatToolsAsInternal converts tools to our internal schema format
// This is the main entry point for provider-agnostic tool formatting
func FormatToolsAsInternal(tools []Tool) []*InternalToolSchema {
	if len(tools) == 0 {
		return nil
	}

	internalTools := make([]*InternalToolSchema, len(tools))
	for i, tool := range tools {
		internalTools[i] = &InternalToolSchema{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  ConvertToInternalSchema(tool.ParametersSchema()),
		}
	}

	return internalTools
}

// FormatToolsAsOpenAIInternal converts tools to OpenAI function calling format
// This is kept in tools package since OpenAI format is commonly used
func FormatToolsAsOpenAIInternal(tools []Tool) string {
	if len(tools) == 0 {
		return ""
	}

	internalTools := FormatToolsAsInternal(tools)
	functions := make([]map[string]any, len(internalTools))

	for i, tool := range internalTools {
		function := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
		}

		if tool.Parameters != nil {
			// Convert internal schema back to map for JSON serialization
			parametersBytes, err := json.Marshal(tool.Parameters)
			if err == nil {
				var parameters map[string]any
				if err := json.Unmarshal(parametersBytes, &parameters); err == nil {
					function["parameters"] = parameters
				}
			}
		}

		functions[i] = function
	}

	// Create the OpenAI functions format
	functionsData := map[string]any{
		"functions": functions,
	}

	jsonBytes, err := json.MarshalIndent(functionsData, "", "  ")
	if err != nil {
		return ""
	}

	return "\n\nAvailable functions:\n" + string(jsonBytes) + "\n"
}

// ToOpenAIFormat converts internal schema to OpenAI function calling format
// This is useful for providers that need OpenAI-compatible output
func (t *InternalToolSchema) ToOpenAIFormat() map[string]any {
	result := map[string]any{
		"name":        t.Name,
		"description": t.Description,
	}

	if t.Parameters != nil {
		// Convert internal schema to map
		parametersBytes, err := json.Marshal(t.Parameters)
		if err == nil {
			var parameters map[string]any
			if err := json.Unmarshal(parametersBytes, &parameters); err == nil {
				result["parameters"] = parameters
			}
		}
	}

	return result
}

// ToGeminiFormat converts internal schema to Gemini function declaration format
func (t *InternalToolSchema) ToGeminiFormat() map[string]any {
	result := map[string]any{
		"name":        t.Name,
		"description": t.Description,
	}

	if t.Parameters != nil {
		// Gemini expects the schema directly
		parametersBytes, err := json.Marshal(t.Parameters)
		if err == nil {
			var parameters map[string]any
			if err := json.Unmarshal(parametersBytes, &parameters); err == nil {
				result["parameters"] = parameters
			}
		}
	}

	return result
}

// ToOllamaFormat converts internal schema to Ollama tool format
func (t *InternalToolSchema) ToOllamaFormat() map[string]any {
	result := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name,
			"description": t.Description,
		},
	}

	if t.Parameters != nil {
		// Ollama expects a specific format structure
		ollamaParams := map[string]any{
			"type": "object",
		}

		if t.Parameters.Properties != nil {
			properties := make(map[string]any)
			for name, prop := range t.Parameters.Properties {
				properties[name] = map[string]any{
					"type":        prop.Type,
					"description": prop.Description,
				}
			}
			ollamaParams["properties"] = properties
		}

		if t.Parameters.Required != nil {
			ollamaParams["required"] = t.Parameters.Required
		}

		result["function"].(map[string]any)["parameters"] = ollamaParams
	}

	return result
}

// String returns a string representation of the internal tool schema
func (t *InternalToolSchema) String() string {
	bytes, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Sprintf("InternalToolSchema{Name: %s, Description: %s}", t.Name, t.Description)
	}
	return string(bytes)
}
