package mcp

import (
	"context"
	"fmt"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/invopop/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProgressNotificationParams is a type alias for MCP progress notifications
type ProgressNotificationParams = mcp.ProgressNotificationParams

// ResourceUpdatedNotificationParams is a type alias for MCP resource update notifications
type ResourceUpdatedNotificationParams = mcp.ResourceUpdatedNotificationParams

// Context keys for MCP operations
type (
	// Tool-related context keys
	mcpToolsContextKey        struct{}
	selectedToolContextKey    struct{}
	extractedParamsContextKey struct{}

	// Resource-related context keys
	mcpResourcesContextKey     struct{}
	selectedResourceContextKey struct{}
	resourceContentContextKey  struct{}

	// Prompt-related context keys
	mcpPromptsContextKey     struct{}
	selectedPromptContextKey struct{}
	promptContentContextKey  struct{}
)

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

// Tool helper functions

// GetTools retrieves MCP tools from the context.
// Returns nil if no tools are registered.
// This is the MCP equivalent of tools.GetTools().
func GetTools(ctx context.Context) []*Tool {
	if tools, ok := ctx.Value(mcpToolsContextKey{}).([]*Tool); ok {
		return tools
	}
	return nil
}

// GetSelectedTool retrieves the tool name selected by Detect from the context.
// Returns empty string if no tool was selected.
func GetSelectedTool(ctx context.Context) string {
	if tool, ok := ctx.Value(selectedToolContextKey{}).(string); ok {
		return tool
	}
	return ""
}

// HasSelectedTool checks if a tool was selected by Detect and stored in the context.
func HasSelectedTool(ctx context.Context) bool {
	return GetSelectedTool(ctx) != ""
}

// GetTool retrieves a specific MCP tool by name from the context.
// Returns nil if the tool is not found.
// This is the MCP equivalent of tools.GetTool().
func GetTool(ctx context.Context, name string) *Tool {
	tools := GetTools(ctx)
	if tools == nil {
		return nil
	}

	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	return nil
}

// HasTool checks if an MCP tool with the given name exists in the context.
// This is the MCP equivalent of tools.HasTool().
func HasTool(ctx context.Context, name string) bool {
	return GetTool(ctx, name) != nil
}

// ListToolNames returns a slice of all MCP tool names available in the context.
// This is the MCP equivalent of tools.ListToolNames().
func ListToolNames(ctx context.Context) []string {
	tools := GetTools(ctx)
	if tools == nil {
		return nil
	}

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

// Resource helper functions

// GetResources retrieves the list of available MCP resources from context
func GetResources(ctx context.Context) []*mcp.Resource {
	if resources, ok := ctx.Value(mcpResourcesContextKey{}).([]*mcp.Resource); ok {
		return resources
	}
	return nil
}

// GetSelectedResource retrieves the selected resource URI from context
func GetSelectedResource(ctx context.Context) string {
	if uri, ok := ctx.Value(selectedResourceContextKey{}).(string); ok {
		return uri
	}
	return ""
}

// HasSelectedResource checks if a resource was selected
func HasSelectedResource(ctx context.Context) bool {
	return GetSelectedResource(ctx) != ""
}

// GetResourceContent retrieves the fetched resource content from context
func GetResourceContent(ctx context.Context) *mcp.ReadResourceResult {
	if content, ok := ctx.Value(resourceContentContextKey{}).(*mcp.ReadResourceResult); ok {
		return content
	}
	return nil
}

// Prompt helper functions

// GetPrompts retrieves the list of available MCP prompts from context
func GetPrompts(ctx context.Context) []*mcp.Prompt {
	if prompts, ok := ctx.Value(mcpPromptsContextKey{}).([]*mcp.Prompt); ok {
		return prompts
	}
	return nil
}

// GetSelectedPrompt retrieves the selected prompt name from context
func GetSelectedPrompt(ctx context.Context) string {
	if name, ok := ctx.Value(selectedPromptContextKey{}).(string); ok {
		return name
	}
	return ""
}

// HasSelectedPrompt checks if a prompt was selected
func HasSelectedPrompt(ctx context.Context) bool {
	return GetSelectedPrompt(ctx) != ""
}

// GetPromptContent retrieves the fetched prompt content from context
func GetPromptContent(ctx context.Context) *mcp.GetPromptResult {
	if content, ok := ctx.Value(promptContentContextKey{}).(*mcp.GetPromptResult); ok {
		return content
	}
	return nil
}
