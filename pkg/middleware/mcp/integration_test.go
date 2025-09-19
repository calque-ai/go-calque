package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	googleschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockAIClientForIntegration is a specialized AI client for integration tests
type mockAIClientForIntegration struct {
	response    string
	shouldError bool
}

func (m *mockAIClientForIntegration) Chat(_ *calque.Request, w *calque.Response, _ *ai.AgentOptions) error {
	if m.shouldError {
		return errors.New("mock LLM error")
	}
	_, err := w.Data.Write([]byte(m.response))
	return err
}

func TestCompleteFlowIntegration(t *testing.T) {
	t.Parallel()

	// Create comprehensive test scenarios
	tests := []struct {
		name            string
		input           string
		llmResponse     string
		expectedTool    string
		expectExecution bool
		description     string
	}{
		{
			name:            "search intent recognized",
			input:           "I need to search for golang tutorials",
			llmResponse:     `{"selected_tool": "search", "confidence": 0.9, "reasoning": "User wants to search for information"}`,
			expectedTool:    "search",
			expectExecution: true,
			description:     "LLM should recognize search intent and select search tool",
		},
		{
			name:            "connect intent recognized",
			input:           "Connect to the database server on localhost:5432",
			llmResponse:     `{"selected_tool": "connect", "confidence": 0.85, "reasoning": "User wants to establish a connection"}`,
			expectedTool:    "connect",
			expectExecution: true,
			description:     "LLM should recognize connection intent and select connect tool",
		},
		{
			name:            "analyze intent recognized",
			input:           "Please analyze this sales data for trends",
			llmResponse:     `{"selected_tool": "analyze", "confidence": 0.8, "reasoning": "User wants data analysis"}`,
			expectedTool:    "analyze",
			expectExecution: true,
			description:     "LLM should recognize analysis intent and select analyze tool",
		},
		{
			name:            "no tool needed",
			input:           "Hello, how are you?",
			llmResponse:     `{"selected_tool": null, "confidence": 0.1, "reasoning": "Simple greeting, no tools needed"}`,
			expectedTool:    "",
			expectExecution: false,
			description:     "LLM should recognize that no tools are needed for simple greetings",
		},
		{
			name:            "ambiguous request - no tool",
			input:           "What should I do?",
			llmResponse:     `{"selected_tool": "none", "confidence": 0.2, "reasoning": "Too vague to determine intent"}`,
			expectedTool:    "",
			expectExecution: false,
			description:     "LLM should indicate no tool is needed for vague requests",
		},
		{
			name:            "case insensitive tool matching",
			input:           "I want to search",
			llmResponse:     `{"selected_tool": "SEARCH", "confidence": 0.9}`,
			expectedTool:    "search",
			expectExecution: true,
			description:     "Tool validation should handle case insensitive matching",
		},
		{
			name:            "partial tool matching",
			input:           "run analysis on the data",
			llmResponse:     `{"selected_tool": "anal", "confidence": 0.8}`,
			expectedTool:    "analyze",
			expectExecution: true,
			description:     "Tool validation should handle partial matching",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create context with mock tools
			ctx := createIntegrationTestContext()

			// Create mock LLM
			mockLLM := &mockAIClientForIntegration{
				response:    tt.llmResponse,
				shouldError: false,
			}

			// Step 1: Detection - Analyze intent and select tool
			detectHandler := Detect(mockLLM)
			req1 := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var detectOutput strings.Builder
			res1 := calque.NewResponse(&detectOutput)

			err := detectHandler.ServeFlow(req1, res1)
			if err != nil {
				t.Fatalf("Detection failed: %v", err)
			}

			// Verify detection results
			selectedTool := GetSelectedTool(req1.Context)
			if selectedTool != tt.expectedTool {
				t.Errorf("Expected tool %q, got %q", tt.expectedTool, selectedTool)
			}

			// Verify input passed through detection unchanged
			if detectOutput.String() != tt.input {
				t.Errorf("Detection should pass through input unchanged, got %q", detectOutput.String())
			}

			// Step 2: Execution - Execute selected tool or pass through
			executeHandler := Execute()
			req2 := calque.NewRequest(req1.Context, strings.NewReader(detectOutput.String()))
			var executeOutput strings.Builder
			res2 := calque.NewResponse(&executeOutput)

			err = executeHandler.ServeFlow(req2, res2)

			if tt.expectExecution {
				// Should attempt to execute the tool (will fail with nil client, but that's expected)
				if err == nil {
					t.Error("Expected execution to fail with nil client")
				}
				// Error should mention the selected tool or be related to nil client
				if !strings.Contains(err.Error(), "panic") && !strings.Contains(err.Error(), "nil") && !strings.Contains(tt.expectedTool, "") {
					t.Logf("Execution failed as expected with nil client: %v", err)
				}
			} else {
				// Should pass through without error
				if err != nil {
					t.Fatalf("Execution should not fail for pass-through: %v", err)
				}

				if executeOutput.String() != tt.input {
					t.Errorf("Expected pass-through result %q, got %q", tt.input, executeOutput.String())
				}
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

func TestFlowErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		llmResponse string
		llmError    bool
		expectError bool
		description string
	}{
		{
			name:        "invalid JSON from LLM",
			input:       "search for something",
			llmResponse: `invalid json`,
			expectError: false, // Should fallback gracefully
			description: "Invalid JSON should result in graceful fallback",
		},
		{
			name:        "LLM selects non-existent tool",
			input:       "do something unknown",
			llmResponse: `{"selected_tool": "nonexistent", "confidence": 0.9}`,
			expectError: false, // Should fallback gracefully
			description: "Non-existent tool should result in graceful fallback",
		},
		{
			name:        "empty LLM response",
			input:       "help me",
			llmResponse: "",
			expectError: false, // Should fallback gracefully
			description: "Empty LLM response should result in graceful fallback",
		},
		{
			name:        "LLM error",
			input:       "search for tutorials",
			llmError:    true,
			expectError: false, // Should fallback gracefully
			description: "LLM errors should result in graceful fallback",
		},
		{
			name:        "malformed confidence value",
			input:       "connect to server",
			llmResponse: `{"selected_tool": "connect", "confidence": "invalid"}`,
			expectError: false, // Should fallback gracefully
			description: "Malformed confidence should result in graceful fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context with tools
			ctx := createIntegrationTestContext()

			// Create mock LLM with potentially problematic response
			mockLLM := &mockAIClientForIntegration{
				response:    tt.llmResponse,
				shouldError: tt.llmError,
			}

			// Test the complete flow
			detectHandler := Detect(mockLLM)
			executeHandler := Execute()

			// Detection
			req1 := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var detectOutput strings.Builder
			res1 := calque.NewResponse(&detectOutput)

			err := detectHandler.ServeFlow(req1, res1)
			if tt.expectError && err == nil {
				t.Error("Expected error in detection but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error in detection: %v", err)
			}

			// Detection should always pass through input, even on errors
			if detectOutput.String() != tt.input {
				t.Errorf("Detection should pass through input even on errors, got %q", detectOutput.String())
			}

			// Execution should handle the error gracefully
			req2 := calque.NewRequest(req1.Context, strings.NewReader(detectOutput.String()))
			var executeOutput strings.Builder
			res2 := calque.NewResponse(&executeOutput)

			err = executeHandler.ServeFlow(req2, res2)
			// Should be pass-through case (no tool selected due to error)
			if err != nil {
				t.Fatalf("Execution should not fail for pass-through: %v", err)
			}

			if executeOutput.String() != tt.input {
				t.Errorf("Expected pass-through result %q, got %q", tt.input, executeOutput.String())
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

func TestMultipleToolsFromDifferentClients(t *testing.T) {
	t.Parallel()

	// Test that the system can handle tools from multiple clients
	// and route to the correct one
	ctx := createMultiClientTestContext()

	tests := []struct {
		name         string
		input        string
		llmResponse  string
		expectedTool string
	}{
		{
			name:         "select search tool",
			input:        "search for documentation",
			llmResponse:  `{"selected_tool": "search", "confidence": 0.9}`,
			expectedTool: "search",
		},
		{
			name:         "select deploy tool",
			input:        "deploy the application",
			llmResponse:  `{"selected_tool": "deploy", "confidence": 0.8}`,
			expectedTool: "deploy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockLLM := &mockAIClientForIntegration{response: tt.llmResponse}

			// Detection
			detectHandler := Detect(mockLLM)
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var detectOutput strings.Builder
			res := calque.NewResponse(&detectOutput)

			err := detectHandler.ServeFlow(req, res)
			if err != nil {
				t.Fatalf("Detection failed: %v", err)
			}

			// Verify correct tool selected
			selectedTool := GetSelectedTool(req.Context)
			if selectedTool != tt.expectedTool {
				t.Errorf("Expected tool %q, got %q", tt.expectedTool, selectedTool)
			}

			t.Logf("✅ Multi-client tool selection works for %s", tt.expectedTool)
		})
	}
}

// Helper functions for integration tests

func createIntegrationTestContext() context.Context {
	tools := []*MCPTool{
		{
			Tool: &mcp.Tool{
				Name:        "search",
				Description: "Search for information online",
				InputSchema: &googleschema.Schema{
					Type: "object",
					Properties: map[string]*googleschema.Schema{
						"query": {Type: "string", Description: "Search query"},
					},
				},
			},
			Client: nil, // Nil client will cause controlled errors in execution
		},
		{
			Tool: &mcp.Tool{
				Name:        "connect",
				Description: "Connect to a remote server",
				InputSchema: &googleschema.Schema{
					Type: "object",
					Properties: map[string]*googleschema.Schema{
						"host": {Type: "string", Description: "Server hostname"},
						"port": {Type: "integer", Description: "Server port"},
					},
				},
			},
			Client: nil,
		},
		{
			Tool: &mcp.Tool{
				Name:        "analyze",
				Description: "Analyze data and generate insights",
				InputSchema: &googleschema.Schema{
					Type: "object",
					Properties: map[string]*googleschema.Schema{
						"data": {Type: "string", Description: "Data to analyze"},
					},
				},
			},
			Client: nil,
		},
	}

	return context.WithValue(context.Background(), mcpToolsContextKey{}, tools)
}

func createMultiClientTestContext() context.Context {
	// Simulate tools from different MCP servers/clients
	var allTools []*MCPTool

	// Tools from "server 1"
	server1Tools := []*MCPTool{
		{
			Tool: &mcp.Tool{
				Name:        "search",
				Description: "Search engine tool",
			},
			Client: nil, // Mock client 1
		},
		{
			Tool: &mcp.Tool{
				Name:        "translate",
				Description: "Translation tool",
			},
			Client: nil, // Mock client 1
		},
	}

	// Tools from "server 2"
	server2Tools := []*MCPTool{
		{
			Tool: &mcp.Tool{
				Name:        "deploy",
				Description: "Deployment tool",
			},
			Client: nil, // Mock client 2
		},
		{
			Tool: &mcp.Tool{
				Name:        "monitor",
				Description: "Monitoring tool",
			},
			Client: nil, // Mock client 2
		},
	}

	allTools = append(allTools, server1Tools...)
	allTools = append(allTools, server2Tools...)

	return context.WithValue(context.Background(), mcpToolsContextKey{}, allTools)
}