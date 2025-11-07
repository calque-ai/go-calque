package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResourcesConvenienceIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		input            string
		llmResponse      string
		expectedResource string
		expectExecution  bool
		description      string
	}{
		{
			name:             "complete resources flow - API docs",
			input:            "show me the API documentation",
			llmResponse:      `{"selected_resource": "file:///docs/api.md", "confidence": 0.9}`,
			expectedResource: "file:///docs/api.md",
			expectExecution:  true,
			description:      "Resources() convenience should handle full detect+execute flow",
		},
		{
			name:             "complete resources flow - no resource",
			input:            "hello there",
			llmResponse:      `{"selected_resource": null, "confidence": 0.1}`,
			expectedResource: "",
			expectExecution:  false,
			description:      "Resources() should pass through when no resource needed",
		},
		{
			name:             "name-based resource selection",
			input:            "get the settings",
			llmResponse:      `{"selected_resource": "Settings", "confidence": 0.85}`,
			expectedResource: "file:///config/settings.json",
			expectExecution:  true,
			description:      "Resources() should handle name-based selection",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create context with resources
			ctx := createResourceIntegrationTestContext()

			// Mock LLM
			mockLLM := ai.NewMockClient(tt.llmResponse)

			// Simulate the Resources() chain: ResourceRegistry → DetectResource → ExecuteResource
			detectHandler := DetectResource(mockLLM)
			executeHandler := ExecuteResource(&Client{}) // Nil client will cause controlled errors

			// Step 1: Detection
			req1 := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var detectOutput strings.Builder
			res1 := calque.NewResponse(&detectOutput)

			err := detectHandler.ServeFlow(req1, res1)
			if err != nil {
				t.Fatalf("Detection failed: %v", err)
			}

			selectedResource := GetSelectedResource(req1.Context)
			if selectedResource != tt.expectedResource {
				t.Errorf("Expected resource %q, got %q", tt.expectedResource, selectedResource)
			}

			// Verify input passed through
			if detectOutput.String() != tt.input {
				t.Errorf("Detection should pass through input unchanged")
			}

			// Step 2: Execution
			req2 := calque.NewRequest(req1.Context, strings.NewReader(detectOutput.String()))
			var executeOutput strings.Builder
			res2 := calque.NewResponse(&executeOutput)

			err = executeHandler.ServeFlow(req2, res2)

			if tt.expectExecution {
				// Should attempt execution and fail with nil client
				if err == nil {
					t.Error("Expected execution to fail with nil client")
				}
			} else {
				// Should pass through
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

func TestPromptsConvenienceIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		llmResponse     string
		expectedPrompt  string
		expectExecution bool
		description     string
	}{
		{
			name:            "complete prompts flow - blog writer",
			input:           "help me write a blog post",
			llmResponse:     `{"selected_prompt": "blog_writer", "confidence": 0.9}`,
			expectedPrompt:  "blog_writer",
			expectExecution: true,
			description:     "Prompts() convenience should handle full detect+execute flow",
		},
		{
			name:            "complete prompts flow - no prompt",
			input:           "just chatting",
			llmResponse:     `{"selected_prompt": null, "confidence": 0.1}`,
			expectedPrompt:  "",
			expectExecution: false,
			description:     "Prompts() should pass through when no prompt needed",
		},
		{
			name:            "partial prompt matching",
			input:           "review my code",
			llmResponse:     `{"selected_prompt": "code", "confidence": 0.85}`,
			expectedPrompt:  "code_review",
			expectExecution: true,
			description:     "Prompts() should handle partial matching",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create context with prompts
			ctx := createPromptIntegrationTestContext()

			// Mock LLM
			mockLLM := ai.NewMockClient(tt.llmResponse)

			// Simulate the Prompts() chain: PromptRegistry → DetectPrompt → ExecutePrompt
			detectHandler := DetectPrompt(mockLLM)
			executeHandler := ExecutePrompt(&Client{}) // Nil client will cause controlled errors

			// Step 1: Detection
			req1 := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var detectOutput strings.Builder
			res1 := calque.NewResponse(&detectOutput)

			err := detectHandler.ServeFlow(req1, res1)
			if err != nil {
				t.Fatalf("Detection failed: %v", err)
			}

			selectedPrompt := GetSelectedPrompt(req1.Context)
			if selectedPrompt != tt.expectedPrompt {
				t.Errorf("Expected prompt %q, got %q", tt.expectedPrompt, selectedPrompt)
			}

			// Verify input passed through
			if detectOutput.String() != tt.input {
				t.Errorf("Detection should pass through input unchanged")
			}

			// Step 2: Execution
			req2 := calque.NewRequest(req1.Context, strings.NewReader(detectOutput.String()))
			var executeOutput strings.Builder
			res2 := calque.NewResponse(&executeOutput)

			err = executeHandler.ServeFlow(req2, res2)

			if tt.expectExecution {
				// Should attempt execution and fail with nil client
				if err == nil {
					t.Error("Expected execution to fail with nil client")
				}
			} else {
				// Should pass through
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

// Helper to create context with resources for integration tests
func createResourceIntegrationTestContext() context.Context {
	resources := []*mcp.Resource{
		{
			URI:         "file:///docs/api.md",
			Name:        "API Documentation",
			Description: "Complete API documentation",
		},
		{
			URI:         "file:///config/settings.json",
			Name:        "Settings",
			Description: "Application settings",
		},
		{
			URI:         "file:///data/users.db",
			Name:        "User Database",
			Description: "User data storage",
		},
	}

	return context.WithValue(context.Background(), mcpResourcesContextKey{}, resources)
}

// Helper to create context with prompts for integration tests
func createPromptIntegrationTestContext() context.Context {
	prompts := []*mcp.Prompt{
		{
			Name:        "blog_writer",
			Description: "Help write blog posts",
		},
		{
			Name:        "code_review",
			Description: "Review code for best practices",
		},
		{
			Name:        "summarizer",
			Description: "Summarize text content",
		},
	}

	return context.WithValue(context.Background(), mcpPromptsContextKey{}, prompts)
}
