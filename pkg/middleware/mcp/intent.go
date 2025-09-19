package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	googleschema "github.com/google/jsonschema-go/jsonschema"
)

// buildStructuredToolSelectionPrompt creates a structured LLM prompt for tool selection with complete schema information
func buildStructuredToolSelectionPrompt(userInput string, mcpTools []*MCPTool) string {
	var prompt strings.Builder

	prompt.WriteString("You are a tool selection assistant. Analyze the user request and select the most appropriate tool.\n\n")

	// Build complete tool information with schemas
	toolOptions := make([]ToolInfo, len(mcpTools))
	for i, tool := range mcpTools {
		toolOptions[i] = ToolInfo{
			Name:         tool.Name(),
			Description:  tool.Description(),
			InputSchema:  tool.InputSchema(),
			OutputSchema: tool.OutputSchema(),
		}
	}

	// Include detailed tool information in the prompt
	prompt.WriteString("Available tools:\n")
	for _, toolOption := range toolOptions {
		prompt.WriteString(fmt.Sprintf("- %s: %s\n", toolOption.Name, toolOption.Description))

		if toolOption.InputSchema != nil {
			prompt.WriteString(fmt.Sprintf("  Input: %s\n", summarizeSchema(toolOption.InputSchema)))
		}

		if toolOption.OutputSchema != nil {
			prompt.WriteString(fmt.Sprintf("  Output: %s\n", summarizeSchema(toolOption.OutputSchema)))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString(fmt.Sprintf("User request: %s\n\n", userInput))

	prompt.WriteString("Respond with a JSON object containing:\n")
	prompt.WriteString("- selected_tool: exact tool name (string) or null if none appropriate\n")
	prompt.WriteString("- confidence: number 0-1 indicating confidence in selection\n")
	prompt.WriteString("- reasoning: brief explanation of choice (optional)\n\n")

	prompt.WriteString("Example: {\"selected_tool\": \"search\", \"confidence\": 0.9, \"reasoning\": \"User wants to find information\"}\n")

	return prompt.String()
}

// summarizeSchema creates a human-readable summary of a JSON schema
func summarizeSchema(schema *googleschema.Schema) string {
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

// parseToolSelectionResponse extracts the tool selection from structured LLM JSON response
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

// validateToolSelection checks if the selected tool exists in the available tools
func validateToolSelection(selectedTool string, mcpTools []*MCPTool) string {
	if selectedTool == "" {
		return ""
	}

	// Exact match first
	for _, tool := range mcpTools {
		if strings.EqualFold(tool.Name(), selectedTool) {
			return tool.Name()
		}
	}

	// Prefix match if no exact match
	for _, tool := range mcpTools {
		if strings.HasPrefix(strings.ToLower(tool.Name()), strings.ToLower(selectedTool)) {
			return tool.Name()
		}
	}

	// No match found
	return ""
}
