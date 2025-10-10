package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/invopop/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func TestExtractParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		input             string
		selectedTool      string
		tool              *Tool
		llmResponse       string
		llmError          bool
		expectPassThrough bool
		expectedParams    map[string]any
		expectError       bool
		description       string
	}{
		{
			name:              "no tool selected - pass through",
			input:             "Hello world",
			selectedTool:      "",
			expectPassThrough: true,
			description:       "Should pass through input when no tool is selected",
		},
		{
			name:         "selected tool not found in registry",
			input:        "Calculate 5 + 3",
			selectedTool: "nonexistent",
			expectError:  true,
			description:  "Should return error when selected tool is not found in registry",
		},
		{
			name:         "successful parameter extraction",
			input:        "Calculate 5 + 3",
			selectedTool: "calculator",
			tool: &Tool{
				Name:        "calculator",
				Description: "Performs mathematical calculations",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
						props := jsonschema.NewProperties()
						props.Set("a", &jsonschema.Schema{Type: "number"})
						props.Set("b", &jsonschema.Schema{Type: "number"})
						props.Set("operation", &jsonschema.Schema{Type: "string"})
						return props
					}(),
					Required: []string{"a", "b", "operation"},
				},
				MCPTool: &mcp.Tool{
					Name:        "calculator",
					Description: "Performs mathematical calculations",
				},
			},
			llmResponse: `{"extracted_params": {"a": 5, "b": 3, "operation": "add"}, "needs_more_info": false, "confidence": 0.9, "reasoning": "Clear addition operation"}`,
			expectedParams: map[string]any{
				"a":         float64(5),
				"b":         float64(3),
				"operation": "add",
			},
			description: "Should successfully extract parameters for valid tool input",
		},
		{
			name:         "tool with no input schema",
			input:        "Run health check",
			selectedTool: "health_check",
			tool: &Tool{
				Name:        "health_check",
				Description: "Runs system health check",
				InputSchema: nil,
				MCPTool: &mcp.Tool{
					Name:        "health_check",
					Description: "Runs system health check",
				},
			},
			llmResponse:    "", // Won't be used
			expectedParams: map[string]any{},
			description:    "Should handle tools with no input schema",
		},
		{
			name:         "parameter extraction with missing info",
			input:        "Calculate something",
			selectedTool: "calculator",
			tool: &Tool{
				Name:        "calculator",
				Description: "Performs mathematical calculations",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
						props := jsonschema.NewProperties()
						props.Set("a", &jsonschema.Schema{Type: "number"})
						props.Set("b", &jsonschema.Schema{Type: "number"})
						return props
					}(),
					Required: []string{"a", "b"},
				},
				MCPTool: &mcp.Tool{
					Name:        "calculator",
					Description: "Performs mathematical calculations",
				},
			},
			llmResponse:    `{"extracted_params": {}, "needs_more_info": true, "confidence": 0.2, "reasoning": "Missing required parameters", "user_prompt": "Please provide two numbers to calculate"}`,
			expectedParams: map[string]any{},
			description:    "Should handle cases where more information is needed",
		},
		{
			name:         "LLM error - fallback to pass through",
			input:        "Calculate 5 + 3",
			selectedTool: "calculator",
			tool: &Tool{
				Name:        "calculator",
				Description: "Performs mathematical calculations",
				InputSchema: &jsonschema.Schema{Type: "object"},
				MCPTool: &mcp.Tool{
					Name:        "calculator",
					Description: "Performs mathematical calculations",
				},
			},
			llmError:          true,
			expectPassThrough: true,
			description:       "Should fallback to pass through when LLM fails",
		},
		{
			name:         "invalid JSON from LLM - fallback to pass through",
			input:        "Calculate 5 + 3",
			selectedTool: "calculator",
			tool: &Tool{
				Name:        "calculator",
				Description: "Performs mathematical calculations",
				InputSchema: &jsonschema.Schema{Type: "object"},
				MCPTool: &mcp.Tool{
					Name:        "calculator",
					Description: "Performs mathematical calculations",
				},
			},
			llmResponse:       "invalid json response",
			expectPassThrough: true,
			description:       "Should fallback to pass through when LLM returns invalid JSON",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			ctx := context.Background()
			if tt.selectedTool != "" {
				ctx = context.WithValue(ctx, selectedToolContextKey{}, tt.selectedTool)
			}
			if tt.tool != nil {
				tools := []*Tool{tt.tool}
				ctx = context.WithValue(ctx, mcpToolsContextKey{}, tools)
			}

			// Setup mock LLM
			var mockLLM ai.Client
			if tt.llmError {
				mockLLM = ai.NewMockClientWithError("LLM error")
			} else {
				mockLLM = ai.NewMockClient(tt.llmResponse)
			}

			// Create handler
			handler := ExtractToolParams(mockLLM)

			// Create request and response
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			// Execute handler
			err := handler.ServeFlow(req, res)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check pass-through behavior
			if tt.expectPassThrough {
				if output.String() != tt.input {
					t.Errorf("Expected pass-through output %q, got %q", tt.input, output.String())
				}
				return
			}

			// Parse output as ParameterExtractionResponse
			var response ParameterExtractionResponse
			if err := json.Unmarshal([]byte(output.String()), &response); err != nil {
				t.Fatalf("Failed to parse output as JSON: %v", err)
			}

			// Verify extracted parameters
			if tt.expectedParams != nil {
				if len(response.ExtractedParams) != len(tt.expectedParams) {
					t.Errorf("Expected %d parameters, got %d", len(tt.expectedParams), len(response.ExtractedParams))
				}

				for key, expectedValue := range tt.expectedParams {
					if actualValue, exists := response.ExtractedParams[key]; !exists {
						t.Errorf("Expected parameter %q not found", key)
					} else if actualValue != expectedValue {
						t.Errorf("Parameter %q: expected %v, got %v", key, expectedValue, actualValue)
					}
				}
			}

			// Verify context was updated
			extractedResponse := GetParameterExtractionResponse(req.Context)
			if extractedResponse == nil {
				t.Error("Expected parameter extraction response to be stored in context")
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

func TestGetParameterExtractionResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected *ParameterExtractionResponse
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name: "context with response",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				ExtractedParams: map[string]any{"test": "value"},
				Confidence:      0.8,
			}),
			expected: &ParameterExtractionResponse{
				ExtractedParams: map[string]any{"test": "value"},
				Confidence:      0.8,
			},
		},
		{
			name:     "context with wrong type",
			ctx:      context.WithValue(context.Background(), extractedParamsContextKey{}, "invalid"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetParameterExtractionResponse(tt.ctx)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected response, got nil")
			}

			if result.Confidence != tt.expected.Confidence {
				t.Errorf("Expected confidence %f, got %f", tt.expected.Confidence, result.Confidence)
			}

			if len(result.ExtractedParams) != len(tt.expected.ExtractedParams) {
				t.Errorf("Expected %d params, got %d", len(tt.expected.ExtractedParams), len(result.ExtractedParams))
			}
		})
	}
}

func TestGetExtractedParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected map[string]any
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name: "context with params",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				ExtractedParams: map[string]any{"a": 5, "b": 3},
			}),
			expected: map[string]any{"a": 5, "b": 3},
		},
		{
			name: "context with empty params",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				ExtractedParams: map[string]any{},
			}),
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetExtractedParams(tt.ctx)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d params, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("Expected parameter %q not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Parameter %q: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestHasExtractedParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name: "context with response",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				ExtractedParams: map[string]any{"test": "value"},
			}),
			expected: true,
		},
		{
			name:     "context with wrong type",
			ctx:      context.WithValue(context.Background(), extractedParamsContextKey{}, "invalid"),
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := HasExtractedParams(tt.ctx)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsMoreInfoNeeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name: "context with needs_more_info true",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				NeedsMoreInfo: true,
			}),
			expected: true,
		},
		{
			name: "context with needs_more_info false",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				NeedsMoreInfo: false,
			}),
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsMoreInfoNeeded(tt.ctx)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetExtractionConfidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected float64
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: 0.0,
		},
		{
			name: "context with confidence",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				Confidence: 0.85,
			}),
			expected: 0.85,
		},
		{
			name: "context with zero confidence",
			ctx: context.WithValue(context.Background(), extractedParamsContextKey{}, &ParameterExtractionResponse{
				Confidence: 0.0,
			}),
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetExtractionConfidence(tt.ctx)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestCreateResponseSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tool        *Tool
		expectNil   bool
		description string
	}{
		{
			name: "tool with input schema",
			tool: &Tool{
				Name:        "calculator",
				Description: "Math calculator",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
						props := jsonschema.NewProperties()
						props.Set("a", &jsonschema.Schema{Type: "number"})
						props.Set("b", &jsonschema.Schema{Type: "number"})
						return props
					}(),
				},
			},
			expectNil:   false,
			description: "Should create response schema with tool's input schema",
		},
		{
			name: "tool with nil input schema",
			tool: &Tool{
				Name:        "simple_tool",
				Description: "Simple tool",
				InputSchema: nil,
			},
			expectNil:   false,
			description: "Should create response schema even with nil input schema",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := createResponseSchema(tt.tool)

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil response format, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected response format, got nil")
			}

			if result.Type != "json_schema" {
				t.Errorf("Expected type 'json_schema', got %q", result.Type)
			}

			if result.Schema == nil {
				t.Error("Expected schema to be set")
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

func TestExtractToolParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		userInput   string
		tool        *Tool
		llmResponse string
		llmError    bool
		expectError bool
		description string
	}{
		{
			name:      "tool with no input schema",
			userInput: "run simple command",
			tool: &Tool{
				Name:        "simple",
				Description: "Simple tool",
				InputSchema: nil,
			},
			expectError: false,
			description: "Should handle tools with no input schema gracefully",
		},
		{
			name:      "successful extraction",
			userInput: "calculate 5 + 3",
			tool: &Tool{
				Name:        "calculator",
				Description: "Math calculator",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
						props := jsonschema.NewProperties()
						props.Set("a", &jsonschema.Schema{Type: "number"})
						props.Set("b", &jsonschema.Schema{Type: "number"})
						return props
					}(),
				},
			},
			llmResponse: `{"extracted_params": {"a": 5, "b": 3}, "needs_more_info": false, "confidence": 0.9}`,
			expectError: false,
			description: "Should successfully extract parameters",
		},
		{
			name:      "LLM error",
			userInput: "calculate something",
			tool: &Tool{
				Name:        "calculator",
				Description: "Math calculator",
				InputSchema: &jsonschema.Schema{Type: "object"},
			},
			llmError:    true,
			expectError: true,
			description: "Should return error when LLM fails",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			var mockLLM ai.Client
			if tt.llmError {
				mockLLM = ai.NewMockClientWithError("LLM error")
			} else {
				mockLLM = ai.NewMockClient(tt.llmResponse)
			}

			result, err := extractToolParameters(ctx, mockLLM, tt.userInput, tt.tool)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			// For tools with no input schema
			if tt.tool.InputSchema == nil {
				if result.Confidence != 1.0 {
					t.Errorf("Expected confidence 1.0 for no-schema tool, got %f", result.Confidence)
				}
				if result.Reasoning != "Tool has no input schema" {
					t.Errorf("Expected specific reasoning for no-schema tool, got %q", result.Reasoning)
				}
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}
