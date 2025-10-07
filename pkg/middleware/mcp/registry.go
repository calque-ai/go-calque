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

// ToolRegistry creates a handler that discovers and makes MCP tools available in the execution context.
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
//	registry := mcp.ToolRegistry(client)
//	flow.Use(registry)
//	// Tools are now available via mcp.GetTools(ctx)
func ToolRegistry(client *Client) calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
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

	// Apply caching if enabled - cache key includes client pointer to separate different MCP servers
	if client.cache != nil && client.cacheConfig != nil && client.cacheConfig.RegistryTTL > 0 {
		return client.cache.CacheWithKey(handler, client.cacheConfig.RegistryTTL, func(_ *calque.Request) string {
			return fmt.Sprintf("mcp:tool-registry:%p", client)
		})
	}

	return handler
}

// ResourceRegistry creates a handler that discovers and makes MCP resources available in the execution context.
//
// Input: any data type (streaming - passes through unchanged)
// Output: same as input (pass-through)
// Behavior: STREAMING - discovers MCP resources and makes them available via GetResources()
//
// The registry connects to the MCP server, discovers all available resources via ListResources(),
// and stores them in the request context for use by downstream handlers like DetectResource() and ExecuteResource().
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	registry := mcp.ResourceRegistry(client)
//	flow.Use(registry)
//	// Resources are now available via mcp.GetResources(ctx)
func ResourceRegistry(client *Client) calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for resource registry: %w", err))
		}

		// Discover all available MCP resources
		listResult, err := client.session.ListResources(ctx, &mcp.ListResourcesParams{})
		if err != nil {
			return client.handleError(fmt.Errorf("failed to list MCP resources: %w", err))
		}

		// Store resources in context
		contextWithResources := context.WithValue(ctx, mcpResourcesContextKey{}, listResult.Resources)
		req.Context = contextWithResources

		// Pass input through unchanged
		_, err = io.Copy(res.Data, req.Data)
		return err
	})

	// Apply caching if enabled
	if client.cache != nil && client.cacheConfig != nil && client.cacheConfig.RegistryTTL > 0 {
		return client.cache.CacheWithKey(handler, client.cacheConfig.RegistryTTL, func(_ *calque.Request) string {
			return fmt.Sprintf("resource-registry:%p", client)
		})
	}

	return handler
}

// PromptRegistry creates a handler that discovers and makes MCP prompts available in the execution context.
//
// Input: any data type (streaming - passes through unchanged)
// Output: same as input (pass-through)
// Behavior: STREAMING - discovers MCP prompts and makes them available via GetPrompts()
//
// The registry connects to the MCP server, discovers all available prompts via ListPrompts(),
// and stores them in the request context for use by downstream handlers like DetectPrompt() and ExecutePrompt().
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	registry := mcp.PromptRegistry(client)
//	flow.Use(registry)
//	// Prompts are now available via mcp.GetPrompts(ctx)
func PromptRegistry(client *Client) calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for prompt registry: %w", err))
		}

		// Discover all available MCP prompts
		listResult, err := client.session.ListPrompts(ctx, &mcp.ListPromptsParams{})
		if err != nil {
			return client.handleError(fmt.Errorf("failed to list MCP prompts: %w", err))
		}

		// Store prompts in context
		contextWithPrompts := context.WithValue(ctx, mcpPromptsContextKey{}, listResult.Prompts)
		req.Context = contextWithPrompts

		// Pass input through unchanged
		_, err = io.Copy(res.Data, req.Data)
		return err
	})

	// Apply caching if enabled
	if client.cache != nil && client.cacheConfig != nil && client.cacheConfig.RegistryTTL > 0 {
		return client.cache.CacheWithKey(handler, client.cacheConfig.RegistryTTL, func(_ *calque.Request) string {
			return fmt.Sprintf("prompt-registry:%p", client)
		})
	}

	return handler
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
