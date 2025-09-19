package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
)

// Detect creates a handler that uses LLM-based intent detection to select the appropriate MCP tool.
// This is the MCP equivalent of tools.Detect(), but uses semantic analysis instead of parsing tool calls.
//
// Input: Natural language user request
// Output: Same user input with optional tool selection stored in context
// Behavior: BUFFERED - reads full input for analysis, then passes through with optional tool selection
//
// The handler analyzes the user's natural language input using an LLM to determine which
// MCP tool would be most appropriate to handle the request. If a suitable tool is found,
// it's stored in the context for use by downstream handlers like Execute(). If no tool
// is appropriate, the input passes through unchanged.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	llmClient, _ := openai.New("gpt-4o-mini")
//	detector := mcp.Detect(client, llmClient)
//	flow.Use(mcp.Registry(client)).
//	     Use(detector).
//	     Use(mcp.Execute())  // Only executes if tool was selected
//
//	// Input: "search for golang tutorials" → selects "search" tool
//	// Input: "connect to localhost:8080" → selects "connect" tool
//	// Input: "hello world" → no tool selected, passes through
func Detect(llmClient ai.Client) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Read user input for analysis
		var userInput string
		if err := calque.Read(req, &userInput); err != nil {
			return fmt.Errorf("failed to read user input for tool detection: %w", err)
		}

		// Get available MCP tools from Registry context
		mcpTools := GetTools(req.Context)
		if len(mcpTools) == 0 {
			fmt.Println("DEBUG: No MCP tools available for detection")
			// No tools available - just pass through
			return calque.Write(res, userInput)
		}

		if strings.TrimSpace(userInput) == "" {
			fmt.Println("DEBUG: Empty user input received")
			// Empty input - just pass through
			return calque.Write(res, userInput)
		}

		// Build structured LLM prompt with complete tool information including schemas
		prompt := buildStructuredToolSelectionPrompt(userInput, mcpTools)

		// Use LLM with structured output to select the most appropriate tool
		promptReq := calque.NewRequest(req.Context, strings.NewReader(prompt))
		var responseBuilder strings.Builder
		promptRes := calque.NewResponse(&responseBuilder)

		// Force structured JSON response using schema
		opts := &ai.AgentOptions{}
		ai.WithSchemaFor[ToolSelectionResponse]().Apply(opts)

		if err := llmClient.Chat(promptReq, promptRes, opts); err != nil {
			// LLM error - just pass through without tool selection
			return calque.Write(res, userInput)
		}

		// Get the LLM response
		llmResponse := responseBuilder.String()

		// Parse structured JSON response
		selectionResponse, err := parseToolSelectionResponse(llmResponse)
		if err != nil {
			// JSON parsing error - just pass through without tool selection
			return calque.Write(res, userInput)
		}

		// Validate the tool selection
		validatedTool := validateToolSelection(selectionResponse.SelectedTool, mcpTools)

		if validatedTool != "" {
			// Debug: Log successful tool selection
			fmt.Printf("DEBUG: Selected tool '%s' for input: %s\n", validatedTool, userInput)

			// Store selected tool in context for Execute handler
			contextWithTool := context.WithValue(req.Context, selectedToolContextKey{}, validatedTool)
			req.Context = contextWithTool

			// Pass through the original user input for the Execute handler to process
			return calque.Write(res, userInput)
		}

		// No tool selected or validation failed - pass through original input
		return calque.Write(res, userInput)
	})
}

// GetSelectedTool retrieves the tool name selected by Detect from the context.
// Returns empty string if no tool was selected.
func GetSelectedTool(ctx context.Context) string {
	if tool, ok := ctx.Value(selectedToolContextKey{}).(string); ok {
		return tool
	}
	return ""
}

// HasSelectedTool checks if a tool was selected by Detect and stored in the context.
func HasSelectedTool(ctx context.Context) bool {
	return GetSelectedTool(ctx) != ""
}
