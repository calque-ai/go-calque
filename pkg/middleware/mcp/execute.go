package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Execute creates a handler that executes the MCP tool selected by Detect, or passes through if none selected.
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
//	flow.Use(mcp.Registry(client)).
//	     Use(mcp.Detect(client, llmClient)).
//	     Use(mcp.Execute())
//
//	// If Detect selected "search" tool: routes to client.Tool("search")
//	// If Detect selected no tool: passes input through unchanged
func Execute() calque.Handler {
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
