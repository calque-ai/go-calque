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
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for tool registry: %w", err))
		}

		var mcpTools []*Tool
		cacheKey := makeRegistryCacheKey("tool", client)

		// Try to get from cache
		if getCachedRegistry(client, cacheKey, &mcpTools) {
			// Restore client reference (not serialized)
			for _, tool := range mcpTools {
				tool.Client = client
			}
			req.Context = context.WithValue(ctx, mcpToolsContextKey{}, mcpTools)
			_, err := io.Copy(res.Data, req.Data)
			return err
		}

		// Cache miss - fetch from MCP server
		listResult, err := client.session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			return client.handleError(fmt.Errorf("failed to list MCP tools: %w", err))
		}

		// Convert MCP tools to our Tool format
		mcpTools = make([]*Tool, len(listResult.Tools))
		for i, tool := range listResult.Tools {
			mcpTools[i] = &Tool{
				MCPTool:     tool,
				Client:      client,
				InputSchema: convertGoogleSchemaToInvopop(tool.InputSchema),
				Name:        tool.Name,
				Description: tool.Description,
			}
		}

		// Store in cache
		setCachedRegistry(client, cacheKey, mcpTools)

		// Store in context
		req.Context = context.WithValue(ctx, mcpToolsContextKey{}, mcpTools)
		_, err = io.Copy(res.Data, req.Data)
		return err
	})
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
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for resource registry: %w", err))
		}

		var resources []*mcp.Resource
		cacheKey := makeRegistryCacheKey("resource", client)

		// Try to get from cache
		if getCachedRegistry(client, cacheKey, &resources) {
			req.Context = context.WithValue(ctx, mcpResourcesContextKey{}, resources)
			_, err := io.Copy(res.Data, req.Data)
			return err
		}

		// Cache miss - fetch from MCP server
		listResult, err := client.session.ListResources(ctx, &mcp.ListResourcesParams{})
		if err != nil {
			return client.handleError(fmt.Errorf("failed to list MCP resources: %w", err))
		}

		resources = listResult.Resources

		// Store in cache
		setCachedRegistry(client, cacheKey, resources)

		// Store in context
		req.Context = context.WithValue(ctx, mcpResourcesContextKey{}, resources)
		_, err = io.Copy(res.Data, req.Data)
		return err
	})
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
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for prompt registry: %w", err))
		}

		var prompts []*mcp.Prompt
		cacheKey := makeRegistryCacheKey("prompt", client)

		// Try to get from cache
		if getCachedRegistry(client, cacheKey, &prompts) {
			req.Context = context.WithValue(ctx, mcpPromptsContextKey{}, prompts)
			_, err := io.Copy(res.Data, req.Data)
			return err
		}

		// Cache miss - fetch from MCP server
		listResult, err := client.session.ListPrompts(ctx, &mcp.ListPromptsParams{})
		if err != nil {
			return client.handleError(fmt.Errorf("failed to list MCP prompts: %w", err))
		}

		prompts = listResult.Prompts

		// Store in cache
		setCachedRegistry(client, cacheKey, prompts)

		// Store in context
		req.Context = context.WithValue(ctx, mcpPromptsContextKey{}, prompts)
		_, err = io.Copy(res.Data, req.Data)
		return err
	})
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

// getCachedRegistry attempts to retrieve cached registry data and unmarshal it.
// Returns true if cache hit and unmarshal succeeded, false otherwise.
func getCachedRegistry[T any](client *Client, cacheKey string, data *T) bool {
	if client.cache == nil || client.cacheConfig == nil || client.cacheConfig.RegistryTTL <= 0 {
		return false
	}

	cached, err := client.cache.Get(cacheKey)
	if err != nil || cached == nil {
		return false
	}

	return json.Unmarshal(cached, data) == nil
}

// setCachedRegistry stores registry data in cache if caching is enabled.
func setCachedRegistry[T any](client *Client, cacheKey string, data T) {
	if client.cache == nil || client.cacheConfig == nil || client.cacheConfig.RegistryTTL <= 0 {
		return
	}

	if jsonData, err := json.Marshal(data); err == nil {
		_ = client.cache.Set(cacheKey, jsonData, client.cacheConfig.RegistryTTL)
	}
}

// makeRegistryCacheKey creates a cache key for a registry type, including client pointer for uniqueness.
func makeRegistryCacheKey(registryType string, client *Client) string {
	return fmt.Sprintf("mcp:%s-registry:%p", registryType, client)
}
