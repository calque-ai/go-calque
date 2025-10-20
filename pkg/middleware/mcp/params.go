package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/convert"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
	"github.com/invopop/jsonschema"
)

var extractPromptTemplate = `Extract parameters for the {{.toolName}} tool from this user request:

User request: {{.Input}}

Tool: {{.toolDesc}}

Return valid JSON only.`

// ExtractToolParams creates a handler that extracts parameters from natural language
// input using the selected tool's exact input schema for precise parameter extraction.
//
// Input: Natural language user request
// Output: JSON parameters that exactly match the selected tool's input schema
// Behavior: TRANSFORM - converts natural language to tool-compatible JSON parameters
//
// The handler uses the MCP tool's InputSchema directly with AI schema enforcement
// to ensure extracted parameters match exactly what the tool expects.
//
// Example:
//
//	flow.Use(mcp.ToolRegistry(client)).
//	     Use(mcp.DetectTool(aiClient)).
//	     Use(mcp.ExtractToolParams(aiClient)).
//	     Use(mcp.ExecuteTool())
//
//	// Input: "What is 6 times 7?" → Output: {"a": 6, "b": 7}
func ExtractToolParams(llmClient ai.Client, opts ...DetectOption) calque.Handler {
	options := &DetectOptions{}
	for _, opt := range opts {
		opt.Apply(options)
	}

	template := extractPromptTemplate
	if options.PromptTemplate != "" {
		template = options.PromptTemplate
	}

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
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

		// Extract parameters using tool's input schema
		response, err := extractToolParameters(req.Context, llmClient, userInput, selectedTool, template)
		if err != nil {
			// If parameter extraction fails, pass through original input
			return calque.Write(res, userInput)
		}

		// Store the full response in context for downstream handlers
		contextWithResponse := context.WithValue(req.Context, extractedParamsContextKey{}, response)
		req.Context = contextWithResponse

		// Return the full ParameterExtractionResponse as JSON
		responseJSON, err := json.Marshal(response)
		if err != nil {
			return fmt.Errorf("failed to marshal parameter extraction response: %w", err)
		}

		return calque.Write(res, responseJSON)
	})
}

// extractToolParameters uses the tool's exact input schema to extract parameters from natural language
func extractToolParameters(ctx context.Context, llmClient ai.Client, userInput string, tool *Tool, template string) (*ParameterExtractionResponse, error) {
	if tool.InputSchema == nil {
		// Tool has no input schema - return empty response
		return &ParameterExtractionResponse{
			ExtractedParams: map[string]any{},
			NeedsMoreInfo:   false,
			Confidence:      1.0,
			Reasoning:       "Tool has no input schema",
		}, nil
	}

	// Create response format embedding the tool's input schema
	schemaFormat := createResponseSchema(tool)

	pipe := calque.NewFlow()
	pipe.Use(prompt.Template(template, map[string]any{
		"toolName": tool.Name,
		"toolDesc": tool.Description,
	})).
		Use(ai.Agent(llmClient, ai.WithSchema(schemaFormat)))

	// Extract parameters with structured response
	var response ParameterExtractionResponse
	err := pipe.Run(ctx, userInput, convert.FromJSON(&response))
	if err != nil {
		return nil, fmt.Errorf("failed to extract tool parameters: %w", err)
	}

	return &response, nil
}

func createResponseSchema(tool *Tool) *ai.ResponseFormat {
	// Create a response format that embeds the tool's input schema
	reflector := jsonschema.Reflector{}
	responseSchema := reflector.Reflect(ParameterExtractionResponse{})

	// The reflector creates a schema with definitions, we need to get the actual schema
	var actualSchema *jsonschema.Schema
	if def, exists := responseSchema.Definitions["ParameterExtractionResponse"]; exists {
		actualSchema = def
	} else {
		actualSchema = responseSchema
	}

	// Replace the extracted_params property with the tool's actual input schema
	if tool.InputSchema != nil && actualSchema.Properties != nil {
		actualSchema.Properties.Set("extracted_params", tool.InputSchema)
	}

	return &ai.ResponseFormat{
		Type:   "json_schema",
		Schema: actualSchema,
	}
}

// GetParameterExtractionResponse retrieves the full parameter extraction response from the context
func GetParameterExtractionResponse(ctx context.Context) *ParameterExtractionResponse {
	if response, ok := ctx.Value(extractedParamsContextKey{}).(*ParameterExtractionResponse); ok {
		return response
	}
	return nil
}

// GetExtractedParams retrieves just the extracted parameters from the context (for backward compatibility)
func GetExtractedParams(ctx context.Context) map[string]any {
	if response := GetParameterExtractionResponse(ctx); response != nil {
		return response.ExtractedParams
	}
	return nil
}

// HasExtractedParams checks if parameters were extracted and stored in the context
func HasExtractedParams(ctx context.Context) bool {
	return GetParameterExtractionResponse(ctx) != nil
}

// IsMoreInfoNeeded checks if the parameter extraction indicates more information is needed
func IsMoreInfoNeeded(ctx context.Context) bool {
	if response := GetParameterExtractionResponse(ctx); response != nil {
		return response.NeedsMoreInfo
	}
	return false
}

// GetExtractionConfidence returns the confidence level of the parameter extraction
func GetExtractionConfidence(ctx context.Context) float64 {
	if response := GetParameterExtractionResponse(ctx); response != nil {
		return response.Confidence
	}
	return 0.0
}
