package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		selectedResourceURI := GetSelectedResource(req.Context)
		if selectedResourceURI == "" {
			return passThrough(req, res)
		}

		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := client.connect(ctx); err != nil {
			return client.handleError(fmt.Errorf("failed to connect for resource execution: %w", err))
		}

		var result *mcp.ReadResourceResult
		cacheKey := fmt.Sprintf("mcp:resource:%s", selectedResourceURI)

		// Try to get from cache
		if getCachedResource(client, cacheKey, &result) {
			return storeInContextAndPassThrough(ctx, req, res, resourceContentContextKey{}, result)
		}

		// Cache miss - fetch from MCP server
		params := &mcp.ReadResourceParams{URI: selectedResourceURI}
		var err error
		result, err = client.session.ReadResource(ctx, params)
		if err != nil {
			return client.handleError(fmt.Errorf("failed to read resource %s: %w", selectedResourceURI, err))
		}

		// Store in cache
		setCachedResource(client, cacheKey, result)

		// Store in context and pass through
		return storeInContextAndPassThrough(ctx, req, res, resourceContentContextKey{}, result)
	})
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
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		selectedPromptName := GetSelectedPrompt(req.Context)
		if selectedPromptName == "" {
			return passThrough(req, res)
		}

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
			_ = json.Unmarshal(input, &args) // Ignore errors if not JSON
		}

		cacheKey := makePromptCacheKey(selectedPromptName, args)
		var result *mcp.GetPromptResult

		// Try to get from cache
		if getCachedPrompt(client, cacheKey, &result) {
			req.Context = context.WithValue(ctx, promptContentContextKey{}, result)
			return calque.Write(res, input)
		}

		// Cache miss - fetch from MCP server
		params := &mcp.GetPromptParams{
			Name:      selectedPromptName,
			Arguments: args,
		}

		var err error
		result, err = client.session.GetPrompt(ctx, params)
		if err != nil {
			return client.handleError(fmt.Errorf("failed to get prompt %s: %w", selectedPromptName, err))
		}

		// Store in cache
		setCachedPrompt(client, cacheKey, result)

		// Store in context and pass through
		req.Context = context.WithValue(ctx, promptContentContextKey{}, result)
		return calque.Write(res, input)
	})
}

// makePromptCacheKey creates a cache key for a prompt, including args hash if present.
// This ensures different args produce different cache entries while supporting no-args prompts.
func makePromptCacheKey(name string, args map[string]string) string {
	if len(args) == 0 {
		return fmt.Sprintf("mcp:prompt:%s", name)
	}

	// Marshal args to JSON for consistent hashing
	argsJSON, err := json.Marshal(args)
	if err != nil {
		// Fallback to name only if marshaling fails
		return fmt.Sprintf("mcp:prompt:%s", name)
	}

	return fmt.Sprintf("mcp:prompt:%s:%s", name, string(argsJSON))
}

// passThrough reads input and writes it to output unchanged.
func passThrough(req *calque.Request, res *calque.Response) error {
	var input []byte
	if err := calque.Read(req, &input); err != nil {
		return err
	}
	return calque.Write(res, input)
}

// storeInContextAndPassThrough stores data in context and passes input through unchanged.
func storeInContextAndPassThrough(ctx context.Context, req *calque.Request, res *calque.Response, key any, value any) error {
	req.Context = context.WithValue(ctx, key, value)
	return passThrough(req, res)
}
