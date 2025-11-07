package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecuteResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		input            string
		hasResources     bool
		selectedResource string
		expectedOutput   string
		expectError      bool
		errorContains    string
	}{
		{
			name:             "no resource selected - pass through",
			input:            "hello world",
			hasResources:     true,
			selectedResource: "",
			expectedOutput:   "hello world",
			expectError:      false,
		},
		{
			name:             "resource selected - execution attempted",
			input:            "summarize this",
			hasResources:     true,
			selectedResource: "file:///docs/api.md",
			expectError:      true, // Will fail because we have no real client
			errorContains:    "failed to connect",
		},
		{
			name:           "empty input with no resource selected",
			input:          "",
			hasResources:   true,
			expectedOutput: "",
			expectError:    false,
		},
		{
			name:           "unicode input - pass through",
			input:          "🔍 search for 中文 content",
			hasResources:   false,
			expectedOutput: "🔍 search for 中文 content",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasResources {
				ctx = createTestResourceContextForExecute()
			} else {
				ctx = context.Background()
			}

			// Add selected resource to context if specified
			if tt.selectedResource != "" {
				ctx = context.WithValue(ctx, selectedResourceContextKey{}, tt.selectedResource)
			}

			// Create handler with nil client (will error on execution)
			client := &Client{} // Nil session will cause errors
			handler := ExecuteResource(client)

			// Execute
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check output for non-error cases
			if output.String() != tt.expectedOutput {
				t.Errorf("Output = %q, expected %q", output.String(), tt.expectedOutput)
			}
		})
	}
}

func TestExecutePrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		hasPrompts     bool
		selectedPrompt string
		expectedOutput string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "no prompt selected - pass through",
			input:          "hello world",
			hasPrompts:     true,
			selectedPrompt: "",
			expectedOutput: "hello world",
			expectError:    false,
		},
		{
			name:           "prompt selected - execution attempted",
			input:          "{}",
			hasPrompts:     true,
			selectedPrompt: "blog_writer",
			expectError:    true, // Will fail because we have no real client
			errorContains:  "failed to connect",
		},
		{
			name:           "empty input with no prompt selected",
			input:          "",
			hasPrompts:     true,
			expectedOutput: "",
			expectError:    false,
		},
		{
			name:           "large input - pass through",
			input:          strings.Repeat("a", 10000),
			hasPrompts:     false,
			expectedOutput: strings.Repeat("a", 10000),
			expectError:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup context
			var ctx context.Context
			if tt.hasPrompts {
				ctx = createTestPromptContextForExecute()
			} else {
				ctx = context.Background()
			}

			// Add selected prompt to context if specified
			if tt.selectedPrompt != "" {
				ctx = context.WithValue(ctx, selectedPromptContextKey{}, tt.selectedPrompt)
			}

			// Create handler with nil client (will error on execution)
			client := &Client{} // Nil session will cause errors
			handler := ExecutePrompt(client)

			// Execute
			req := calque.NewRequest(ctx, strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := handler.ServeFlow(req, res)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check output for non-error cases
			if output.String() != tt.expectedOutput {
				t.Errorf("Output = %q, expected %q", output.String(), tt.expectedOutput)
			}
		})
	}
}

// Helper function to create test context with mock resources for execute tests
func createTestResourceContextForExecute() context.Context {
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
	}

	return context.WithValue(context.Background(), mcpResourcesContextKey{}, resources)
}

// Helper function to create test context with mock prompts for execute tests
func createTestPromptContextForExecute() context.Context {
	prompts := []*mcp.Prompt{
		{
			Name:        "blog_writer",
			Description: "Help write blog posts",
		},
		{
			Name:        "code_review",
			Description: "Review code for best practices",
		},
	}

	return context.WithValue(context.Background(), mcpPromptsContextKey{}, prompts)
}
