package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	googleschema "github.com/google/jsonschema-go/jsonschema"
)

// ExtractParams creates a handler that extracts parameters from natural language
// input using the selected tool's input schema.
//
// This handler should be used after Detect() to convert natural language
// into structured parameters that match the tool's expected input schema.
//
// Input: Natural language user request
// Output: Extracted JSON parameters or request for more information
// Behavior: TRANSFORM - analyzes input against tool schema and extracts parameters
//
// Example:
//
//	flow.Use(mcp.Registry(client)).
//	     Use(mcp.Detect(aiClient)).
//	     Use(mcp.ExtractParams(aiClient)).
//	     Use(mcp.Execute())
//
//	// Input: "search for golang tutorials"
//	// Output: {"query": "golang tutorials"} (if search tool selected)
func ExtractParams(llmClient ai.Client) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Read user input
		var userInput string
		if err := calque.Read(req, &userInput); err != nil {
			return fmt.Errorf("failed to read user input for parameter extraction: %w", err)
		}

		// Check if a tool was selected by Detect
		selectedToolName := GetSelectedTool(req.Context)
		if selectedToolName == "" {
			// No tool selected - pass through original input
			return calque.Write(res, userInput)
		}

		// Get the selected tool and its schema
		selectedTool := GetTool(req.Context, selectedToolName)
		if selectedTool == nil {
			return fmt.Errorf("selected tool '%s' not found in registry", selectedToolName)
		}

		// Extract parameters using the tool's input schema
		extractedParams, needsMoreInfo, userPrompt, err := extractParametersFromInput(
			llmClient, userInput, selectedTool, req.Context)
		if err != nil {
			// If parameter extraction fails, pass through original input
			return calque.Write(res, userInput)
		}

		// If we need more information from the user
		if needsMoreInfo {
			return calque.Write(res, userPrompt)
		}

		// Store extracted parameters in context for Execute handler
		contextWithParams := context.WithValue(req.Context, extractedParamsContextKey{}, extractedParams)
		req.Context = contextWithParams

		// Return the extracted parameters as JSON for Execute handler
		paramsJSON, err := json.Marshal(extractedParams)
		if err != nil {
			return fmt.Errorf("failed to marshal extracted parameters: %w", err)
		}

		return calque.Write(res, paramsJSON)
	})
}

// extractParametersFromInput uses LLM to extract parameters from natural language
// based on the tool's input schema
func extractParametersFromInput(llmClient ai.Client, userInput string, tool *MCPTool, ctx context.Context) (
	map[string]any, bool, string, error) {

	inputSchema := tool.InputSchema()
	if inputSchema == nil {
		// Tool has no input schema - return empty parameters
		return map[string]any{}, false, "", nil
	}

	// Build structured prompt for parameter extraction
	prompt := buildParameterExtractionPrompt(userInput, tool.Name(), tool.Description(), inputSchema)

	// Use LLM with structured output to extract parameters
	promptReq := calque.NewRequest(ctx, strings.NewReader(prompt))
	var responseBuilder strings.Builder
	promptRes := calque.NewResponse(&responseBuilder)

	// Force structured JSON response using schema
	opts := &ai.AgentOptions{}
	ai.WithSchemaFor[ParameterExtractionResponse]().Apply(opts)

	if err := llmClient.Chat(promptReq, promptRes, opts); err != nil {
		return nil, false, "", fmt.Errorf("LLM parameter extraction failed: %w", err)
	}

	// Parse the LLM response
	llmResponse := responseBuilder.String()
	extractionResponse, err := parseParameterExtractionResponse(llmResponse)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to parse parameter extraction response: %w", err)
	}

	// Check if we need more information
	if extractionResponse.NeedsMoreInfo {
		userPrompt := extractionResponse.UserPrompt
		if userPrompt == "" {
			userPrompt = buildDefaultPromptForMissingParams(tool.Name(), extractionResponse.MissingParams)
		}
		return nil, true, userPrompt, nil
	}

	return extractionResponse.ExtractedParams, false, "", nil
}

// buildParameterExtractionPrompt creates a structured prompt for parameter extraction
func buildParameterExtractionPrompt(userInput, toolName, toolDescription string, inputSchema *googleschema.Schema) string {
	var prompt strings.Builder

	prompt.WriteString("You are a parameter extraction assistant. Extract parameters from natural language input.\n\n")

	prompt.WriteString(fmt.Sprintf("Tool: %s\n", toolName))
	prompt.WriteString(fmt.Sprintf("Description: %s\n\n", toolDescription))

	// Add schema information
	prompt.WriteString("Expected parameters schema:\n")
	prompt.WriteString(formatSchemaForPrompt(inputSchema))
	prompt.WriteString("\n\n")

	prompt.WriteString(fmt.Sprintf("User input: \"%s\"\n\n", userInput))

	prompt.WriteString("Analyze the user input and extract parameters that match the schema.\n")
	prompt.WriteString("Respond with a JSON object containing:\n")
	prompt.WriteString("- extracted_params: object with extracted parameter values\n")
	prompt.WriteString("- missing_params: array of required parameter names that couldn't be extracted\n")
	prompt.WriteString("- needs_more_info: boolean indicating if more information is needed\n")
	prompt.WriteString("- user_prompt: string asking user for missing information (if needed)\n")
	prompt.WriteString("- confidence: number 0-1 indicating confidence in extraction\n")
	prompt.WriteString("- reasoning: brief explanation of extraction choices\n\n")

	prompt.WriteString("Examples:\n")
	prompt.WriteString("If schema requires {\"query\": \"string\"} and input is \"search for golang\":\n")
	prompt.WriteString(`{"extracted_params": {"query": "golang"}, "missing_params": [], "needs_more_info": false, "confidence": 0.9}`)
	prompt.WriteString("\n\n")
	prompt.WriteString("If schema requires {\"query\": \"string\", \"limit\": \"number\"} and input is \"search something\":\n")
	prompt.WriteString(`{"extracted_params": {"query": "something"}, "missing_params": ["limit"], "needs_more_info": true, "user_prompt": "How many results would you like?", "confidence": 0.7}`)

	return prompt.String()
}

// formatSchemaForPrompt converts a JSON schema to a human-readable format for the prompt
func formatSchemaForPrompt(schema *googleschema.Schema) string {
	if schema == nil {
		return "No parameters required"
	}

	var output strings.Builder
	output.WriteString("{\n")

	// Handle object properties
	if schema.Properties != nil {
		for propName, propSchema := range schema.Properties {
			required := false
			if schema.Required != nil {
				for _, req := range schema.Required {
					if req == propName {
						required = true
						break
					}
				}
			}

			requiredText := ""
			if required {
				requiredText = " (required)"
			}

			propType := string(propSchema.Type)
			if propType == "" {
				propType = "any"
			}

			description := propSchema.Description
			if description == "" {
				description = "No description"
			}

			output.WriteString(fmt.Sprintf("  \"%s\": \"%s\"%s // %s\n",
				propName, propType, requiredText, description))
		}
	}

	output.WriteString("}")
	return output.String()
}

// parseParameterExtractionResponse parses the LLM response for parameter extraction
func parseParameterExtractionResponse(llmResponse string) (*ParameterExtractionResponse, error) {
	var response ParameterExtractionResponse

	// Clean up any potential JSON wrapper or extra text
	responseText := strings.TrimSpace(llmResponse)

	// Try to find JSON in the response
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}") + 1

	if jsonStart >= 0 && jsonEnd > jsonStart {
		responseText = responseText[jsonStart:jsonEnd]
	}

	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Initialize ExtractedParams if nil
	if response.ExtractedParams == nil {
		response.ExtractedParams = make(map[string]any)
	}

	return &response, nil
}

// buildDefaultPromptForMissingParams creates a default prompt when parameters are missing
func buildDefaultPromptForMissingParams(toolName string, missingParams []string) string {
	if len(missingParams) == 0 {
		return fmt.Sprintf("I need more information to use the %s tool. Please provide additional details.", toolName)
	}

	if len(missingParams) == 1 {
		return fmt.Sprintf("To use the %s tool, I need you to specify: %s", toolName, missingParams[0])
	}

	return fmt.Sprintf("To use the %s tool, I need you to specify: %s", toolName, strings.Join(missingParams, ", "))
}

// GetExtractedParams retrieves the extracted parameters from the context
func GetExtractedParams(ctx context.Context) map[string]any {
	if params, ok := ctx.Value(extractedParamsContextKey{}).(map[string]any); ok {
		return params
	}
	return nil
}

// HasExtractedParams checks if parameters were extracted and stored in the context
func HasExtractedParams(ctx context.Context) bool {
	return GetExtractedParams(ctx) != nil
}