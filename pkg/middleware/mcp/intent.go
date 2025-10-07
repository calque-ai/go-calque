package mcp

import (
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

// Resource selection

var resourceSelectionPromptTemplate = `You are a resource selection assistant. Analyze the user request and select the most appropriate resource.

User request: {{.Input}}

Available resources:
{{range .Resources}}- {{.URI}}: {{.Name}}{{if .Description}} - {{.Description}}{{end}}
{{end}}Return valid JSON only.`

// ResourceSelectionResponse represents the response from the LLM for resource selection
type ResourceSelectionResponse struct {
	SelectedResource string  `json:"selected_resource" jsonschema:"required,description=URI of the selected resource from available options, or empty string if none appropriate"`
	Confidence       float64 `json:"confidence" jsonschema:"required,minimum=0,maximum=1,description=Confidence level in the resource selection accuracy (0.0 to 1.0)"`
	Reasoning        string  `json:"reasoning,omitempty" jsonschema:"description=Brief explanation of why this resource was selected or why no resource is appropriate"`
}

// ResourceTemplateData represents resource data for the template
type ResourceTemplateData struct {
	URI         string
	Name        string
	Description string
}

// getResourceSelectionTemplateData creates the template data for resource selection
func getResourceSelectionTemplateData(userInput string, mcpResources []*mcp.Resource) map[string]any {
	resourceData := make([]ResourceTemplateData, len(mcpResources))
	for i, resource := range mcpResources {
		resourceData[i] = ResourceTemplateData{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
		}
	}

	return map[string]any{
		"Input":     userInput,
		"Resources": resourceData,
	}
}

// validateResourceSelection checks if the selected resource exists in the available resources
func validateResourceSelection(selectedResource string, mcpResources []*mcp.Resource) string {
	if selectedResource == "" {
		return ""
	}

	// Exact URI match first
	for _, resource := range mcpResources {
		if strings.EqualFold(resource.URI, selectedResource) {
			return resource.URI
		}
	}

	// Check name match
	for _, resource := range mcpResources {
		if strings.EqualFold(resource.Name, selectedResource) {
			return resource.URI
		}
	}

	// Prefix match on URI if no exact match
	for _, resource := range mcpResources {
		if strings.HasPrefix(strings.ToLower(resource.URI), strings.ToLower(selectedResource)) {
			return resource.URI
		}
	}

	// No match found
	return ""
}

// Prompt selection

var promptSelectionPromptTemplate = `You are a prompt selection assistant. Analyze the user request and select the most appropriate prompt template.

User request: {{.Input}}

Available prompts:
{{range .Prompts}}- {{.Name}}: {{.Description}}
{{end}}Return valid JSON only.`

// PromptSelectionResponse represents the response from the LLM for prompt selection
type PromptSelectionResponse struct {
	SelectedPrompt string  `json:"selected_prompt" jsonschema:"required,description=Name of the selected prompt from available options, or empty string if none appropriate"`
	Confidence     float64 `json:"confidence" jsonschema:"required,minimum=0,maximum=1,description=Confidence level in the prompt selection accuracy (0.0 to 1.0)"`
	Reasoning      string  `json:"reasoning,omitempty" jsonschema:"description=Brief explanation of why this prompt was selected or why no prompt is appropriate"`
}

// PromptTemplateData represents prompt data for the template
type PromptTemplateData struct {
	Name        string
	Description string
}

// getPromptSelectionTemplateData creates the template data for prompt selection
func getPromptSelectionTemplateData(userInput string, mcpPrompts []*mcp.Prompt) map[string]any {
	promptData := make([]PromptTemplateData, len(mcpPrompts))
	for i, promptItem := range mcpPrompts {
		promptData[i] = PromptTemplateData{
			Name:        promptItem.Name,
			Description: promptItem.Description,
		}
	}

	return map[string]any{
		"Input":   userInput,
		"Prompts": promptData,
	}
}

// validatePromptSelection checks if the selected prompt exists in the available prompts
func validatePromptSelection(selectedPrompt string, mcpPrompts []*mcp.Prompt) string {
	if selectedPrompt == "" {
		return ""
	}

	// Exact match first
	for _, promptItem := range mcpPrompts {
		if strings.EqualFold(promptItem.Name, selectedPrompt) {
			return promptItem.Name
		}
	}

	// Prefix match if no exact match
	for _, promptItem := range mcpPrompts {
		if strings.HasPrefix(strings.ToLower(promptItem.Name), strings.ToLower(selectedPrompt)) {
			return promptItem.Name
		}
	}

	// No match found
	return ""
}
