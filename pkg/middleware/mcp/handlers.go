package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool creates a handler that calls an MCP tool using input as arguments.
//
// Input: JSON tool arguments from AI (e.g., {"query": "golang", "limit": 10})
// Output: Tool execution result content
// Behavior: TRANSFORM - reads JSON args, calls MCP tool, returns result
//
// Follows function calling pattern: AI generates structured tool arguments,
// MCP tool executes with those arguments, result goes back to AI for processing.
// Input should be valid JSON that matches the tool's parameter schema.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	handler := client.Tool("search")
//	flow.Use(handler) // Input: {"query": "golang"} → Output: search results
func (c *Client) Tool(name string, progressCallbacks ...func(*ProgressNotificationParams)) calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := c.connect(ctx); err != nil {
			return c.handleError(fmt.Errorf("failed to connect for tool %s: %w", name, err))
		}

		// Read input as tool arguments
		var argsJSON []byte
		if err := calque.Read(req, &argsJSON); err != nil {
			return c.handleError(fmt.Errorf("failed to read tool arguments: %w", err))
		}

		// Parse arguments
		var args map[string]any
		if len(argsJSON) > 0 {
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return c.handleError(fmt.Errorf("invalid tool arguments JSON: %w", err))
			}
		}

		// Call the tool
		params := &mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		}

		result, err := c.session.CallTool(ctx, params)
		if err != nil {
			return c.handleError(fmt.Errorf("tool %s failed: %w", name, err))
		}

		// Register progress callbacks if provided
		if len(progressCallbacks) > 0 && result.Meta != nil {
			if progressToken, ok := result.Meta["progressToken"].(string); ok {
				c.mu.Lock()
				c.progressCallbacks[progressToken] = progressCallbacks
				c.mu.Unlock()
			}
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
			return c.handleError(fmt.Errorf("tool %s returned error: %s", name, errorMessage))
		}

		// Collect all content and write in one operation for efficiency
		var output strings.Builder

		// Prioritize structured content over text content to avoid duplication
		if result.StructuredContent != nil {
			structuredJSON, err := json.Marshal(result.StructuredContent)
			if err != nil {
				return c.handleError(fmt.Errorf("failed to marshal structured content: %w", err))
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
			if err := calque.Write(res, output.String()); err != nil {
				return err
			}
		}

		return nil
	})

	// Apply caching if enabled and TTL > 0 (tools are usually dynamic)
	if c.cache != nil && c.cacheConfig != nil && c.cacheConfig.ToolTTL > 0 {
		return c.cache.Cache(handler, c.cacheConfig.ToolTTL)
	}

	return handler
}

// Resource creates a handler that augments input with MCP resource content.
//
// Input: User query/content (any text)
// Output: Input content + Resource content (RAG pattern)
// Behavior: AUGMENT - fetches MCP resource(s) and combines with input
//
// Follows RAG pattern: fetches resource content and prepends it to user input
// to provide additional context for AI processing. Perfect for adding file
// contents, documentation, or contextual data to user queries.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	handler := client.Resource("file:///docs/api.md", "file:///docs/guide.md")
//	flow.Use(handler) // Input: "How do I use the API?" → Output: [API docs] + [guide] + user query
func (c *Client) Resource(uris ...string) calque.Handler {
	handler := c.multiResourceHandler(func(_ []byte) ([]string, error) {
		return uris, nil
	}, "resources")

	// Apply caching if enabled
	if c.cache != nil && c.cacheConfig != nil && c.cacheConfig.ResourceTTL > 0 {
		return c.cache.Cache(handler, c.cacheConfig.ResourceTTL)
	}

	return handler
}

// ResourceTemplate creates a handler that resolves MCP resource templates dynamically.
//
// Input: JSON template variables (e.g., {"path": "config.json", "env": "prod"})
// Output: Input content + Resolved resource content (RAG pattern)
// Behavior: AUGMENT - resolves template URI(s) and fetches resource content
//
// Resolves resource templates like "file:///{path}" using input variables,
// then fetches the resolved resource(s) and augments input with their content.
// Perfect for dynamic resource access based on runtime parameters.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	handler := client.ResourceTemplate("file:///{path}", "file:///{env}/config.json")
//	flow.Use(handler) // Input: {"path": "config.json", "env": "prod"} → Output: [config content] + [env config] + input
func (c *Client) ResourceTemplate(uriTemplates ...string) calque.Handler {
	handler := c.multiResourceHandler(func(input []byte) ([]string, error) {
		var resolvedURIs []string

		// Parse template variables from input
		var templateVars map[string]string
		if len(input) > 0 {
			if err := json.Unmarshal(input, &templateVars); err != nil {
				return nil, fmt.Errorf("invalid template variables JSON: %w", err)
			}
		}

		// Resolve each template
		for _, uriTemplate := range uriTemplates {
			// Resolve template URI with security validation
			resolvedURI := uriTemplate
			for key, value := range templateVars {
				// Security: Clean path component to prevent traversal
				cleanValue := filepath.Clean(value)
				if cleanValue != value || strings.Contains(cleanValue, "..") {
					return nil, fmt.Errorf("invalid template variable %s: path traversal not allowed", key)
				}

				// Security: Basic URI component validation
				if strings.ContainsAny(value, "\n\r\t") {
					return nil, fmt.Errorf("invalid template variable %s: control characters not allowed", key)
				}

				resolvedURI = strings.ReplaceAll(resolvedURI, "{"+key+"}", value)
			}

			// Security: Validate final resolved URI
			if parsedURI, err := url.Parse(resolvedURI); err != nil {
				return nil, fmt.Errorf("invalid resolved URI: %w", err)
			} else if parsedURI.Scheme == "" {
				return nil, fmt.Errorf("resolved URI missing scheme: %s", resolvedURI)
			}

			resolvedURIs = append(resolvedURIs, resolvedURI)
		}

		return resolvedURIs, nil
	}, "resource templates")

	// Apply caching if enabled
	if c.cache != nil && c.cacheConfig != nil && c.cacheConfig.ResourceTTL > 0 {
		return c.cache.Cache(handler, c.cacheConfig.ResourceTTL)
	}

	return handler
}

// Prompt creates a handler that executes MCP prompt templates using input as arguments.
//
// Input: JSON template arguments (e.g., {"topic": "golang", "style": "beginner"})
// Output: Formatted prompt messages
// Behavior: TRANSFORM - reads JSON args, expands MCP prompt template
//
// Follows template pattern: reads JSON arguments from input and uses them to
// expand the specified MCP prompt template. Returns formatted prompt messages
// ready for AI processing.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	handler := client.Prompt("code_review")
//	flow.Use(handler) // Input: {"code": "func main() {}"} → Output: formatted review prompt
func (c *Client) Prompt(name string) calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := c.connect(ctx); err != nil {
			return c.handleError(fmt.Errorf("failed to connect for prompt %s: %w", name, err))
		}

		// Read input as template arguments
		var argsJSON []byte
		if err := calque.Read(req, &argsJSON); err != nil {
			return c.handleError(fmt.Errorf("failed to read prompt arguments: %w", err))
		}

		// Parse arguments
		var args map[string]string
		if len(argsJSON) > 0 {
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return c.handleError(fmt.Errorf("invalid prompt arguments JSON: %w", err))
			}
		}

		// Get the prompt
		params := &mcp.GetPromptParams{
			Name:      name,
			Arguments: args,
		}

		result, err := c.session.GetPrompt(ctx, params)
		if err != nil {
			return c.handleError(fmt.Errorf("prompt %s failed: %w", name, err))
		}

		// Write prompt messages
		var output strings.Builder
		for i, message := range result.Messages {
			if i > 0 {
				output.WriteString("\n")
			}

			if textContent, ok := message.Content.(*mcp.TextContent); ok {
				output.WriteString(fmt.Sprintf("%s: %s", message.Role, textContent.Text))
			}
		}

		return calque.Write(res, output.String())
	})

	// Apply caching if enabled
	if c.cache != nil && c.cacheConfig != nil && c.cacheConfig.PromptTTL > 0 {
		return c.cache.Cache(handler, c.cacheConfig.PromptTTL)
	}

	return handler
}

// SubscribeToResource creates a handler that subscribes to MCP resource changes.
//
// Input: Initial data (any content)
// Output: Resource change notifications
// Behavior: SUBSCRIBE - establishes subscription and forwards change notifications
//
// Subscribes to resource updates and calls the provided callback when changes occur.
// The handler passes through the initial input and then monitors for resource updates.
// Perfect for reactive flows that need to respond to external resource changes.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	handler := client.SubscribeToResource("file:///config.json", func(update *ResourceUpdatedNotificationParams) {
//		log.Printf("Resource %s updated", update.URI)
//	})
//	flow.Use(handler)
func (c *Client) SubscribeToResource(uri string, onChange func(*ResourceUpdatedNotificationParams)) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := c.connect(ctx); err != nil {
			return c.handleError(fmt.Errorf("failed to connect for resource subscription %s: %w", uri, err))
		}

		// Register the subscription callback
		c.mu.Lock()
		c.subscriptions[uri] = onChange
		c.mu.Unlock()

		// Subscribe to the resource
		params := &mcp.SubscribeParams{
			URI: uri,
		}

		err := c.session.Subscribe(ctx, params)
		if err != nil {
			c.mu.Lock()
			delete(c.subscriptions, uri)
			c.mu.Unlock()
			return c.handleError(fmt.Errorf("failed to subscribe to resource %s: %w", uri, err))
		}

		// Pass through the input
		var input []byte
		if err := calque.Read(req, &input); err != nil {
			return c.handleError(fmt.Errorf("failed to read input: %w", err))
		}

		return calque.Write(res, input)
	})
}

// Complete creates a handler that provides auto-completion for MCP prompt/resource arguments.
//
// Input: JSON completion request (e.g., {"ref": {"type": "ref/prompt", "name": "code_review"}, "argument": {"name": "language", "value": "go"}})
// Output: Completion suggestions
// Behavior: TRANSFORM - reads completion request, returns available options
//
// Provides auto-completion suggestions for prompt arguments and resource URIs.
// Helps users discover valid parameter values and reduces input errors.
// Requires completion capability to be enabled on the client.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"}, mcp.WithCompletion(true))
//	handler := client.Complete()
//	flow.Use(handler) // Input: completion request → Output: suggestion list
func (c *Client) Complete() calque.Handler {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		if !c.completionEnabled {
			return c.handleError(fmt.Errorf("completion not enabled on client"))
		}

		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := c.connect(ctx); err != nil {
			return c.handleError(fmt.Errorf("failed to connect for completion: %w", err))
		}

		// Read completion request
		var requestJSON []byte
		if err := calque.Read(req, &requestJSON); err != nil {
			return c.handleError(fmt.Errorf("failed to read completion request: %w", err))
		}

		// Parse completion parameters
		var params mcp.CompleteParams
		if err := json.Unmarshal(requestJSON, &params); err != nil {
			return c.handleError(fmt.Errorf("invalid completion request JSON: %w", err))
		}

		// Call completion
		result, err := c.session.Complete(ctx, &params)
		if err != nil {
			return c.handleError(fmt.Errorf("completion failed: %w", err))
		}

		// Write completion results
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return c.handleError(fmt.Errorf("failed to marshal completion result: %w", err))
		}

		return calque.Write(res, resultJSON)
	})

	// Apply caching if enabled
	if c.cache != nil && c.cacheConfig != nil && c.cacheConfig.CompletionTTL > 0 {
		return c.cache.Cache(handler, c.cacheConfig.CompletionTTL)
	}

	return handler
}

// multiResourceHandler is the shared implementation for both Resource and ResourceTemplate
func (c *Client) multiResourceHandler(getURIs func([]byte) ([]string, error), description string) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Establish connection if needed
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}

		if err := c.connect(ctx); err != nil {
			return c.handleError(fmt.Errorf("failed to connect for %s: %w", description, err))
		}

		// Read original input
		var input []byte
		if err := calque.Read(req, &input); err != nil {
			return c.handleError(fmt.Errorf("failed to read input: %w", err))
		}

		// Get the URIs (either static or resolved from templates)
		uris, err := getURIs(input)
		if err != nil {
			return c.handleError(err)
		}

		// Build augmented output: input + resource contents (RAG pattern)
		var output strings.Builder

		// Add user input first
		output.WriteString("=== User Query ===\n")
		output.Write(input)
		output.WriteString("\n\n")

		// Fetch each resource
		resourceNum := 1
		for _, uri := range uris {
			params := &mcp.ReadResourceParams{
				URI: uri,
			}

			result, err := c.session.ReadResource(ctx, params)
			if err != nil {
				// Continue with error note rather than failing completely
				output.WriteString(fmt.Sprintf("=== Resource %d (Error) ===\n", resourceNum))
				output.WriteString(fmt.Sprintf("Failed to read %s: %v\n\n", uri, err))
				resourceNum++
				continue
			}

			// Add all contents from this resource
			for i, content := range result.Contents {
				if len(result.Contents) == 1 {
					output.WriteString(fmt.Sprintf("=== Resource %d ===\n", resourceNum))
				} else {
					output.WriteString(fmt.Sprintf("=== Resource %d.%d ===\n", resourceNum, i+1))
				}
				if content.Text != "" {
					output.WriteString(content.Text)
				} else if len(content.Blob) > 0 {
					output.WriteString(fmt.Sprintf("[Binary content: %d bytes, type: %s]", len(content.Blob), content.MIMEType))
				}
				output.WriteString("\n\n")
			}
			resourceNum++
		}

		return calque.Write(res, output.String())
	})
}
