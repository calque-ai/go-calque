package mcp

import (
	"fmt"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/invopop/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProgressNotificationParams is a type alias for MCP progress notifications
type ProgressNotificationParams = mcp.ProgressNotificationParams

// ResourceUpdatedNotificationParams is a type alias for MCP resource update notifications
type ResourceUpdatedNotificationParams = mcp.ResourceUpdatedNotificationParams

// mcpToolsContextKey is used to store discovered MCP tools in context
type mcpToolsContextKey struct{}

// selectedToolContextKey is used to store the selected tool name in context
type selectedToolContextKey struct{}

// extractedParamsContextKey is used to store extracted parameters in context
type extractedParamsContextKey struct{}

// ToolSelectionResponse represents the response from the LLM for tool selection
type ToolSelectionResponse struct {
	SelectedTool string  `json:"selected_tool" jsonschema:"required,description=Exact name of the selected tool from available options, or empty string if none appropriate"`
	Confidence   float64 `json:"confidence" jsonschema:"required,minimum=0,maximum=1,description=Confidence level in the tool selection accuracy (0.0 to 1.0)"`
	Reasoning    string  `json:"reasoning,omitempty" jsonschema:"description=Brief explanation of why this tool was selected or why no tool is appropriate"`
}

// ParameterExtractionResponse represents the response from the LLM for parameter extraction
type ParameterExtractionResponse struct {
	ExtractedParams map[string]any `json:"extracted_params" jsonschema:"required,description=Extracted parameter values that match the tool's input schema"`
	MissingParams   []string       `json:"missing_params,omitempty" jsonschema:"description=Array of required parameter names that could not be extracted from user input"`
	NeedsMoreInfo   bool           `json:"needs_more_info" jsonschema:"required,description=Boolean indicating if more information is needed from the user"`
	UserPrompt      string         `json:"user_prompt,omitempty" jsonschema:"description=Human-readable prompt asking user for missing information (only if needs_more_info is true)"`
	Confidence      float64        `json:"confidence" jsonschema:"required,minimum=0,maximum=1,description=Confidence level in the parameter extraction accuracy (0.0 to 1.0)"`
	Reasoning       string         `json:"reasoning,omitempty" jsonschema:"description=Brief explanation of extraction decisions and logic"`
}

// Tool represents a discovered MCP tool with its metadata and execution capability
type Tool struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	InputSchema *jsonschema.Schema `json:"input_schema,omitempty"`
	MCPTool     *mcp.Tool
	Client      *Client
}

// ServeFlow implements the calque.Handler interface for Tool
func (t *Tool) ServeFlow(req *calque.Request, res *calque.Response) error {
	if t.Client == nil {
		return fmt.Errorf("MCP client is nil for tool %s", t.Name)
	}

	// Delegate to the client's Tool handler
	handler := t.Client.Tool(t.Name)
	return handler.ServeFlow(req, res)
}
