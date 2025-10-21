package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProgressNotificationParams is a type alias for MCP progress notifications
type ProgressNotificationParams = mcp.ProgressNotificationParams

// ResourceUpdatedNotificationParams is a type alias for MCP resource update notifications
type ResourceUpdatedNotificationParams = mcp.ResourceUpdatedNotificationParams

// Context keys for MCP operations
type (
	// Resource-related context keys
	mcpResourcesContextKey     struct{}
	selectedResourceContextKey struct{}
	resourceContentContextKey  struct{}

	// Prompt-related context keys
	mcpPromptsContextKey     struct{}
	selectedPromptContextKey struct{}
	promptContentContextKey  struct{}
)

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
