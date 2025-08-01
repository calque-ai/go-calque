package tools

import (
	"context"
	"io"

	"github.com/calque-ai/calque-pipe/core"
)

// toolsContextKey is used to store tools in context
type toolsContextKey struct{}

// Registry creates a handler that makes tools available to its own execution context.
// This is useful for advanced pipeline composition where you want manual control.
//
// Note: Due to the streaming architecture, tools are only available within the same
// handler execution context, not to downstream handlers in a pipeline. For most use cases,
// prefer llm.Agent() which handles tool registration and execution automatically.
//
// Input: any data type (streaming - passes through unchanged)
// Output: same as input (pass-through)
// Behavior: STREAMING - makes tools available via GetTools() within handler execution
//
// Example:
//
//	registry := tools.Registry(calculatorTool, searchTool)
//	// Tools are available within the registry handler's execution context
func Registry(tools ...Tool) core.Handler {
	return &registryHandler{tools: tools}
}

// registryHandler implements the registry with tools stored as instance data
type registryHandler struct {
	tools []Tool
}

func (rh *registryHandler) ServeFlow(ctx context.Context, r io.Reader, w io.Writer) error {
	// Create a context with tools for this handler's execution
	ctx = context.WithValue(ctx, toolsContextKey{}, rh.tools)
	
	// Pass input through unchanged 
	_, err := io.Copy(w, r)
	return err
}

// GetTools retrieves tools from the context.
// Returns nil if no tools are registered.
func GetTools(ctx context.Context) []Tool {
	if tools, ok := ctx.Value(toolsContextKey{}).([]Tool); ok {
		return tools
	}
	return nil
}

// GetTool retrieves a specific tool by name from the context.
// Returns nil if the tool is not found.
func GetTool(ctx context.Context, name string) Tool {
	tools := GetTools(ctx)
	if tools == nil {
		return nil
	}

	for _, tool := range tools {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
}

// HasTool checks if a tool with the given name exists in the context.
func HasTool(ctx context.Context, name string) bool {
	return GetTool(ctx, name) != nil
}

// ListToolNames returns a slice of all tool names available in the context.
func ListToolNames(ctx context.Context) []string {
	tools := GetTools(ctx)
	if tools == nil {
		return nil
	}

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name()
	}
	return names
}

// ToolInfo represents metadata about a tool for external use
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListTools returns metadata about all tools available in the context.
// Useful for generating tool descriptions for LLMs or documentation.
func ListTools(ctx context.Context) []ToolInfo {
	tools := GetTools(ctx)
	if tools == nil {
		return nil
	}

	infos := make([]ToolInfo, len(tools))
	for i, tool := range tools {
		infos[i] = ToolInfo{
			Name:        tool.Name(),
			Description: tool.Description(),
		}
	}
	return infos
}
