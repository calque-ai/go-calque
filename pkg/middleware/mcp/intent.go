package mcp

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var resourceSelectionPromptTemplate = `You are a resource selection assistant. Analyze the user request and select the most appropriate resource if needed.

User request: {{.Input}}

Available resources:
{{range .Resources}}- {{.URI}}: {{.Name}}{{if .Description}} - {{.Description}}{{end}}
{{end}}
If none of the available resources are appropriate or necessary for this request, return an empty string for selected_resource.
Return valid JSON only.`

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

var promptSelectionPromptTemplate = `You are a prompt selection assistant. Analyze the user request and select the most appropriate prompt template if needed.

User request: {{.Input}}

Available prompts:
{{range .Prompts}}- {{.Name}}: {{.Description}}
{{end}}
If none of the available prompts are appropriate or necessary for this request, return an empty string for selected_prompt.
Return valid JSON only.`

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
