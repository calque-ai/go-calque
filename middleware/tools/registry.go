package tools

import (
	"context"
	"io"

	"github.com/calque-ai/calque-pipe/core"
)

// toolsContextKey is used to store tools in context
type toolsContextKey struct{}

// Registry stores available tools in the context for later use by Execute middleware.
// This middleware is streaming - it passes input through unchanged while making
// tools available to downstream middleware.
//
// Input: any data type (streaming - passes through unchanged)
// Output: same as input (pass-through)
// Behavior: STREAMING - adds tools to context, passes data through
//
// The tools are stored in the request context and can be retrieved by Execute()
// or other middleware that needs access to the available tools.
//
// Example:
//
//	registry := tools.Registry(calculatorTool, searchTool)
//	pipe.Use(registry) // Tools now available in context for Execute()
func Registry(tools ...Tool) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Store tools in context for later retrieval
		ctx = context.WithValue(ctx, toolsContextKey{}, tools)

		// Pass input through unchanged while preserving the enhanced context
		// Note: We can't directly pass the enhanced context to io.Copy since it doesn't accept context,
		// but the context enhancement will be available to any downstream handlers
		_, err := io.Copy(w, r)
		return err
	})
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
