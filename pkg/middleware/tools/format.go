package tools

import "encoding/json"

// ToolDefinition represents a function in OpenAI format for tool schema
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// FormatToolsAsOpenAI formats tools into OpenAI functions schema
func FormatToolsAsOpenAI(tools []Tool) string {
	if len(tools) == 0 {
		return ""
	}

	functions := make([]ToolDefinition, len(tools))

	for i, tool := range tools {
		// Convert jsonschema.Schema to map for JSON serialization
		var parameters map[string]interface{}
		if schema := tool.ParametersSchema(); schema != nil {
			// Marshal and unmarshal to convert to generic map
			if schemaBytes, err := json.Marshal(schema); err == nil {
				json.Unmarshal(schemaBytes, &parameters)
			}
		}

		functions[i] = ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  parameters,
		}
	}

	// Create the OpenAI functions format
	functionsData := map[string]interface{}{
		"functions": functions,
	}

	jsonBytes, err := json.MarshalIndent(functionsData, "", "  ")
	if err != nil {
		return ""
	}

	return "\n\nAvailable functions:\n" + string(jsonBytes) + "\n"
}
