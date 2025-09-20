package mcp

import (
	"strings"

	"github.com/invopop/jsonschema"
)

var toolSelectionPromptTemplate = `You are a tool selection assistant. Analyze the user request and select the most appropriate tool.

User request: {{.Input}}

Available tools:
{{range .Tools}}- {{.Name}}: {{.Description}}
{{if .InputSchemaSummary}}  Input: {{.InputSchemaSummary}}{{end}}

{{end}}Return valid JSON only.`

// ToolTemplateData represents tool data for the template
type ToolTemplateData struct {
	Name               string
	Description        string
	InputSchemaSummary string
}

// getToolSelectionTemplateData creates the template data for tool selection
func getToolSelectionTemplateData(userInput string, mcpTools []*Tool) map[string]any {
	toolData := make([]ToolTemplateData, len(mcpTools))
	for i, tool := range mcpTools {
		toolData[i] = ToolTemplateData{
			Name:               tool.Name,
			Description:        tool.Description,
			InputSchemaSummary: summarizeSchema(tool.InputSchema),
		}
	}

	return map[string]any{
		"Input": userInput,
		"Tools": toolData,
	}
}

// summarizeSchema creates a human-readable summary of a JSON schema
func summarizeSchema(schema *jsonschema.Schema) string {
	if schema == nil {
		return "any"
	}

	// Try to extract a simple description from the schema
	if schema.Description != "" {
		return schema.Description
	}

	// Basic type information
	if schema.Type != "" {
		return string(schema.Type)
	}

	// Fallback to indicating it's structured data
	return "structured data"
}

// validateToolSelection checks if the selected tool exists in the available tools
func validateToolSelection(selectedTool string, mcpTools []*Tool) string {
	if selectedTool == "" {
		return ""
	}

	// Exact match first
	for _, tool := range mcpTools {
		if strings.EqualFold(tool.Name, selectedTool) {
			return tool.Name
		}
	}

	// Prefix match if no exact match
	for _, tool := range mcpTools {
		if strings.HasPrefix(strings.ToLower(tool.Name), strings.ToLower(selectedTool)) {
			return tool.Name
		}
	}

	// No match found
	return ""
}
