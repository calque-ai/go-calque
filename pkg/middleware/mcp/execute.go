package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExecuteTool creates a handler that executes the MCP tool selected by Detect, or passes through if none selected.
// This is the MCP equivalent of tools.Execute(), but for single pre-selected MCP tools.
//
// Input: User input/arguments for the selected MCP tool (if any)
// Output: Tool result if tool selected, or original input if no tool selected
// Behavior: CONDITIONAL - executes tool if selected, otherwise passes through
//
// The handler looks for a tool selection made by Detect() in the request context.
// If no tool was selected, it passes through the input unchanged.
//
// Example:
//
//	flow.Use(mcp.ToolRegistry(client)).
//	     Use(mcp.DetectTool(client, llmClient)).
//	     Use(mcp.ExecuteTool())
//
//	// If DetectTool selected "search" tool: routes to client.Tool("search")
//	// If DetectTool selected no tool: passes input through unchanged
func ExecuteTool() calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Check if a tool was selected by Detect
		selectedToolName := GetSelectedTool(req.Context)
		if selectedToolName == "" {
			// No tool selected - pass through the input
			var input []byte
			if err := calque.Read(req, &input); err != nil {
				return err
			}
			return calque.Write(res, input)
		}

		// Find the specific tool by name
		selectedTool := GetTool(req.Context, selectedToolName)
		if selectedTool == nil {
			return fmt.Errorf("MCP tool '%s' not found in registry", selectedToolName)
		}

		// Check if we have parameter extraction response from ExtractParams handler
		paramResponse := GetParameterExtractionResponse(req.Context)
		if paramResponse != nil {
			// Check if more information is needed from the user
			if paramResponse.NeedsMoreInfo {
				// Return the user prompt asking for missing information
				return calque.Write(res, paramResponse.UserPrompt)
			}

			// Use extracted parameters - convert to JSON and create new request
			paramsJSON, err := json.Marshal(paramResponse.ExtractedParams)
			if err != nil {
				return fmt.Errorf("failed to marshal extracted parameters: %w", err)
			}

			// Create new request with extracted parameters
			paramReq := calque.NewRequest(req.Context, strings.NewReader(string(paramsJSON)))
			return selectedTool.ServeFlow(paramReq, res)
		}

		// Use the tool directly with original input
		return selectedTool.ServeFlow(req, res)
	})
}

// ExecuteResource creates a handler that fetches the MCP resource selected by DetectResource, or passes through if none selected.
//
// Input: User input (passes through unchanged)
// Output: Same as input (pass-through)
// Behavior: CONDITIONAL - fetches resource if selected and stores in context, otherwise passes through
//
// The handler looks for a resource selection made by DetectResource() in the request context.
// If a resource was selected, it fetches the resource content and stores it in context for downstream handlers.
// The original input passes through unchanged.
//
// Example:
//
//	flow.Use(mcp.ResourceRegistry(client)).
//	     Use(mcp.DetectResource(llmClient)).
//	     Use(mcp.ExecuteResource())
//	     Use(llm.Chat("gpt-4")) // Can access resource content from context
func ExecuteResource(client *Client) calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Check if a resource was selected by DetectResource
		selectedResourceURI := GetSelectedResource(req.Context)
		if selectedResourceURI == "" {
			// No resource selected - pass through the input
			var input []byte
			if err := calque.Read(req, &input); err != nil {
				return err
			}
			return calque.Write(res, input)
		}

		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for resource execution: %w", err))
		}

		// Fetch the resource
		params := &mcp.ReadResourceParams{
			URI: selectedResourceURI,
		}

		result, err := client.session.ReadResource(ctx, params)
		if err != nil {
			return client.handleError(fmt.Errorf("failed to read resource %s: %w", selectedResourceURI, err))
		}

		// Store resource content in context for downstream handlers
		contextWithResource := context.WithValue(ctx, resourceContentContextKey{}, result)
		req.Context = contextWithResource

		// Pass through original input
		var input []byte
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		return calque.Write(res, input)
	})

	// Apply caching if enabled - cache key based on resource URI
	if client.cache != nil && client.cacheConfig != nil && client.cacheConfig.ResourceTTL > 0 {
		return client.cache.CacheWithKey(handler, client.cacheConfig.ResourceTTL, func(req *calque.Request) string {
			uri := GetSelectedResource(req.Context)
			return fmt.Sprintf("mcp:resource:%s", uri)
		})
	}

	return handler
}

// ExecutePrompt creates a handler that fetches the MCP prompt selected by DetectPrompt, or passes through if none selected.
//
// Input: User input (passes through unchanged)
// Output: Same as input (pass-through)
// Behavior: CONDITIONAL - fetches prompt if selected and stores in context, otherwise passes through
//
// The handler looks for a prompt selection made by DetectPrompt() in the request context.
// If a prompt was selected, it fetches the prompt content and stores it in context for downstream handlers.
// The original input passes through unchanged.
//
// Example:
//
//	flow.Use(mcp.PromptRegistry(client)).
//	     Use(mcp.DetectPrompt(llmClient)).
//	     Use(mcp.ExecutePrompt())
//	     Use(llm.Chat("gpt-4")) // Can access prompt content from context
func ExecutePrompt(client *Client) calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Check if a prompt was selected by DetectPrompt
		selectedPromptName := GetSelectedPrompt(req.Context)
		if selectedPromptName == "" {
			// No prompt selected - pass through the input
			var input []byte
			if err := calque.Read(req, &input); err != nil {
				return err
			}
			return calque.Write(res, input)
		}

		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for prompt execution: %w", err))
		}

		// Read input to extract any prompt arguments
		var input []byte
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		// Parse arguments if input is JSON
		var args map[string]string
		if len(input) > 0 {
			// Try to parse as JSON, ignore errors if not JSON
			_ = json.Unmarshal(input, &args)
		}

		// Fetch the prompt
		params := &mcp.GetPromptParams{
			Name:      selectedPromptName,
			Arguments: args,
		}

		result, err := client.session.GetPrompt(ctx, params)
		if err != nil {
			return client.handleError(fmt.Errorf("failed to get prompt %s: %w", selectedPromptName, err))
		}

		// Store prompt content in context for downstream handlers
		contextWithPrompt := context.WithValue(ctx, promptContentContextKey{}, result)
		req.Context = contextWithPrompt

		// Pass through original input
		return calque.Write(res, input)
	})

	// Apply caching if enabled - cache key based on prompt name + args hash
	if client.cache != nil && client.cacheConfig != nil && client.cacheConfig.PromptTTL > 0 {
		return client.cache.CacheWithKey(handler, client.cacheConfig.PromptTTL, func(req *calque.Request) string {
			name := GetSelectedPrompt(req.Context)
			// Note: For simplicity, we cache by name only. Could enhance to include args hash
			return fmt.Sprintf("mcp:prompt:%s", name)
		})
	}

	return handler
}
