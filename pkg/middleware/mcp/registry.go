package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
	"github.com/invopop/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tools fetches and returns all available MCP tools as native tools.Tool instances.
// Use this to register MCP tools with AI agents for automatic function calling.
//
// This connects to the MCP server, discovers available tools via ListTools(),
// converts them to tools.Tool format, and caches the result for performance.
// Each tool, when called by the AI, executes the corresponding MCP tool on the server.
//
// This is for AI-driven tool discovery and selection. For direct tool invocation
// when you know exactly which tool to call, use client.Tool("name") instead.
//
// Example - AI agent with MCP tools:
//
//	mcpTools, err := mcp.Tools(ctx, client)
//	if err != nil {
//	    return err
//	}
//	agent := ai.Agent(llmClient, ai.WithTools(mcpTools...))
func Tools(ctx context.Context, client *Client) ([]tools.Tool, error) {
	// Check cache first
	cacheKey := makeToolsRegistryCacheKey(client)
	if cached := getCachedToolsRegistry(client, cacheKey); cached != nil {
		return cached, nil
	}

	if err := client.connect(ctx); err != nil {
		return nil, client.handleError(calque.WrapErr(ctx, err, "failed to connect for tool registry"))
	}

	// Fetch MCP tools
	listResult, err := client.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, calque.WrapErr(ctx, err, "failed to list tools")
	}

	// Convert to tools.Tool slice
	nativeTools := make([]tools.Tool, len(listResult.Tools))
	for i, mcpTool := range listResult.Tools {
		nativeTools[i] = convertMCPToTool(client, mcpTool)
	}

	// Cache the converted tools
	setCachedToolsRegistry(client, cacheKey, nativeTools)

	return nativeTools, nil
}

// convertMCPToTool wraps an MCP tool as tools.Tool
func convertMCPToTool(client *Client, mcpTool *mcp.Tool) tools.Tool {
	return tools.New(
		mcpTool.Name,
		mcpTool.Description,
		convertGoogleSchemaToInvopop(mcpTool.InputSchema), // Convert schema
		createMCPToolHandler(client, mcpTool.Name),        // Handler that calls MCP
	)
}

// createMCPToolHandler creates a handler that calls the MCP tool
func createMCPToolHandler(client *Client, toolName string) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Read parameters as JSON
		var paramBytes []byte
		if err := calque.Read(r, &paramBytes); err != nil {
			return err
		}

		var params map[string]any
		if len(paramBytes) > 0 {
			if err := json.Unmarshal(paramBytes, &params); err != nil {
				return calque.WrapErr(r.Context, err, fmt.Sprintf("failed to unmarshal tool parameters for %s", toolName))
			}
		} else {
			params = make(map[string]any) // Empty params
		}

		// Call MCP tool
		result, err := client.session.CallTool(r.Context, &mcp.CallToolParams{
			Name:      toolName,
			Arguments: params,
		})
		if err != nil {
			return err
		}

		// Handle tool errors
		if result.IsError {
			var errorText strings.Builder
			for _, content := range result.Content {
				if textContent, ok := content.(*mcp.TextContent); ok {
					errorText.WriteString(textContent.Text)
				}
			}
			errorMessage := errorText.String()
			if errorMessage == "" {
				errorMessage = "unknown error (no text content in error response)"
			}
			return client.handleError(calque.NewErr(r.Context, fmt.Sprintf("tool %s returned error: %s", toolName, errorMessage)))
		}

		// Collect all content and write in one operation for efficiency
		var output strings.Builder

		// Prioritize structured content over text content to avoid duplication
		if result.StructuredContent != nil {
			structuredJSON, err := json.Marshal(result.StructuredContent)
			if err != nil {
				return client.handleError(calque.WrapErr(r.Context, err, "failed to marshal structured content"))
			}
			output.Write(structuredJSON)
		} else {
			// Only collect text content if no structured content is available
			for _, content := range result.Content {
				if textContent, ok := content.(*mcp.TextContent); ok {
					output.WriteString(textContent.Text)
				}
			}
		}

		if output.Len() > 0 {
			if err := calque.Write(w, output.String()); err != nil {
				return err
			}
		}

		return nil
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
			return client.handleError(calque.WrapErr(ctx, err, "failed to connect for resource registry"))
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
			return client.handleError(calque.WrapErr(ctx, err, "failed to list MCP resources"))
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
			return client.handleError(calque.WrapErr(ctx, err, "failed to connect for prompt registry"))
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
			return client.handleError(calque.WrapErr(ctx, err, "failed to list MCP prompts"))
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

// makeRegistryCacheKey creates a cache key for a registry type, including client pointer for uniqueness.
func makeRegistryCacheKey(registryType string, client *Client) string {
	return fmt.Sprintf("mcp:%s-registry:%p", registryType, client)
}

// makeToolsRegistryCacheKey creates a cache key for the tools.Tool registry
func makeToolsRegistryCacheKey(client *Client) string {
	return fmt.Sprintf("mcp:tools-native-registry:%p", client)
}

// getCachedToolsRegistry retrieves cached tools.Tool slice if available
func getCachedToolsRegistry(client *Client, cacheKey string) []tools.Tool {
	if client.cache == nil || client.cacheConfig == nil || client.cacheConfig.RegistryTTL <= 0 {
		return nil
	}

	cached, err := client.cache.Get(cacheKey)
	if err != nil || cached == nil {
		return nil
	}

	// Since tools.Tool is an interface, we need to store/retrieve as JSON
	// and reconstruct the tools
	var toolData []struct {
		Name   string          `json:"name"`
		Desc   string          `json:"description"`
		Schema json.RawMessage `json:"schema"`
	}

	if err := json.Unmarshal(cached, &toolData); err != nil {
		return nil
	}

	// Reconstruct tools
	result := make([]tools.Tool, len(toolData))
	for i, td := range toolData {
		var schema jsonschema.Schema
		if err := json.Unmarshal(td.Schema, &schema); err != nil {
			continue
		}
		result[i] = tools.New(td.Name, td.Desc, &schema, createMCPToolHandler(client, td.Name))
	}

	return result
}

// setCachedToolsRegistry stores tools.Tool slice in cache
func setCachedToolsRegistry(client *Client, cacheKey string, nativeTools []tools.Tool) {
	if client.cache == nil || client.cacheConfig == nil || client.cacheConfig.RegistryTTL <= 0 {
		return
	}

	// Extract serializable data from tools
	toolData := make([]struct {
		Name   string          `json:"name"`
		Desc   string          `json:"description"`
		Schema json.RawMessage `json:"schema"`
	}, len(nativeTools))

	for i, tool := range nativeTools {
		schemaBytes, err := json.Marshal(tool.ParametersSchema())
		if err != nil {
			continue
		}
		toolData[i] = struct {
			Name   string          `json:"name"`
			Desc   string          `json:"description"`
			Schema json.RawMessage `json:"schema"`
		}{
			Name:   tool.Name(),
			Desc:   tool.Description(),
			Schema: schemaBytes,
		}
	}

	if jsonData, err := json.Marshal(toolData); err == nil {
		_ = client.cache.Set(cacheKey, jsonData, client.cacheConfig.RegistryTTL)
	}
}
