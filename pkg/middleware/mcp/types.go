package mcp

import (
	"fmt"

	"github.com/calque-ai/go-calque/pkg/calque"
	googleschema "github.com/google/jsonschema-go/jsonschema"
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

// ToolInfo represents metadata about an MCP tool for discovery and selection
type ToolInfo struct {
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	InputSchema  *googleschema.Schema `json:"input_schema,omitempty"`
	OutputSchema *googleschema.Schema `json:"output_schema,omitempty"`
}

// ToolSelectionResponse represents the response from the LLM for tool selection
type ToolSelectionResponse struct {
	SelectedTool string  `json:"selected_tool"`
	Confidence   float64 `json:"confidence"`
	Reasoning    string  `json:"reasoning,omitempty"`
}

// ParameterExtractionResponse represents the response from the LLM for parameter extraction
type ParameterExtractionResponse struct {
	ExtractedParams  map[string]any `json:"extracted_params"`
	MissingParams    []string       `json:"missing_params,omitempty"`
	NeedsMoreInfo    bool           `json:"needs_more_info"`
	UserPrompt       string         `json:"user_prompt,omitempty"`
	Confidence       float64        `json:"confidence"`
	Reasoning        string         `json:"reasoning,omitempty"`
}

// MCPTool represents a discovered MCP tool with its metadata and execution capability
type MCPTool struct {
	Tool   *mcp.Tool
	Client *Client
}

// Convenience methods
func (t *MCPTool) Name() string                       { return t.Tool.Name }
func (t *MCPTool) Description() string                { return t.Tool.Description }
func (t *MCPTool) InputSchema() *googleschema.Schema  { return t.Tool.InputSchema }
func (t *MCPTool) OutputSchema() *googleschema.Schema { return t.Tool.OutputSchema }

// ServeFlow implements the calque.Handler interface for MCPTool
func (t *MCPTool) ServeFlow(req *calque.Request, res *calque.Response) error {
	if t.Client == nil {
		return fmt.Errorf("MCP client is nil for tool %s", t.Name())
	}

	// Delegate to the client's Tool handler
	handler := t.Client.Tool(t.Name())
	return handler.ServeFlow(req, res)
}
