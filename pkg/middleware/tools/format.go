package tools

// ToolDefinition represents a function in OpenAI format for tool schema
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// FormatToolsAsOpenAI formats tools into OpenAI functions schema
// This function is kept for backward compatibility but now uses the internal schema system
func FormatToolsAsOpenAI(tools []Tool) string {
	return FormatToolsAsOpenAIInternal(tools)
}
