package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/invopop/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Registry creates a handler that discovers and makes MCP tools available in the execution context.
// This is the MCP equivalent of tools.Registry().
//
// Input: any data type (streaming - passes through unchanged)
// Output: same as input (pass-through)
// Behavior: STREAMING - discovers MCP tools and makes them available via GetTools() within handler execution
//
// The registry connects to the MCP server, discovers all available tools via ListTools(),
// and stores them in the request context for use by downstream handlers like Detect() and Execute().
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	registry := mcp.Registry(client)
//	flow.Use(registry)
//	// Tools are now available via mcp.GetTools(ctx)
func Registry(client *Client) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for tool registry: %w", err))
		}

		// Discover all available MCP tools
		listResult, err := client.session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			return client.handleError(fmt.Errorf("failed to list MCP tools: %w", err))
		}

		// Convert MCP tools to our Tool format with schema conversion
		mcpTools := make([]*Tool, len(listResult.Tools))
		for i, tool := range listResult.Tools {
			mcpTools[i] = &Tool{
				MCPTool:     tool,
				Client:      client,
				InputSchema: convertGoogleSchemaToInvopop(tool.InputSchema),
				Name:        tool.Name,
				Description: tool.Description,
			}
		}

		// Store tools in context (similar to tools.Registry)
		contextWithTools := context.WithValue(ctx, mcpToolsContextKey{}, mcpTools)
		req.Context = contextWithTools

		// Pass input through unchanged
		_, err = io.Copy(res.Data, req.Data)
		return err
	})
}

// GetTools retrieves MCP tools from the context.
// Returns nil if no tools are registered.
// This is the MCP equivalent of tools.GetTools().
func GetTools(ctx context.Context) []*Tool {
	if tools, ok := ctx.Value(mcpToolsContextKey{}).([]*Tool); ok {
		return tools
	}
	return nil
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

// convertGoogleSchemaToInvopop converts a Google JSON Schema to invopop format
// Both libraries implement JSON Schema spec, so we can marshal/unmarshal between them
func convertGoogleSchemaToInvopop(googleSchema any) *jsonschema.Schema {
	if googleSchema == nil {
		return nil
	}

	// Marshal the Google schema to JSON bytes
	schemaBytes, err := json.Marshal(googleSchema)
	if err != nil {
		return nil
	}

	// Unmarshal into invopop schema
	var invopopSchema jsonschema.Schema
	err = json.Unmarshal(schemaBytes, &invopopSchema)
	if err != nil {
		return nil
	}

	return &invopopSchema
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
