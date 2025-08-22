package ai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// Helper function to create a mock client for agent tests
func createMockClientForTest(responses []string, shouldErr bool) *MockClient {
	if shouldErr {
		return NewMockClientWithError("client error")
	}
	mockClient := NewMockClientWithResponses(responses)
	// For tool calling tests, we need to handle the first response differently
	// The first response should be the tool call JSON, subsequent responses are for synthesis
	return mockClient
}

// createErrorTool creates a tool that always returns an error for testing
func createErrorTool() tools.Tool {
	return tools.Simple("error_tool", "Always errors", func(_ string) string {
		// This will cause an error when the tool is executed
		panic("simulated tool error")
	})
}

func TestAgent(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(expr string) string {
		if expr == "2+2" {
			return "4"
		}
		return fmt.Sprintf("calculated: %s", expr)
	})

	search := tools.Simple("search", "Search the web", func(query string) string {
		return fmt.Sprintf("search results for: %s", query)
	})

	tests := []struct {
		name         string
		tools        []tools.Tool
		input        string
		llmResponses []string
		contains     []string
		expectError  bool
	}{
		{
			name:  "simple tool usage",
			tools: []tools.Tool{calc},
			input: "What is 2+2?",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
				"The answer is 4.",
			},
			contains: []string{"The answer is 4."},
		},
		{
			name:  "no tool usage",
			tools: []tools.Tool{calc, search},
			input: "Hello, how are you?",
			llmResponses: []string{
				"Hello! I'm doing well, thank you for asking.",
			},
			contains: []string{"Hello! I'm doing well, thank you for asking."},
		},
		{
			name:  "multiple tool usage",
			tools: []tools.Tool{calc, search},
			input: "Calculate 2+2 and search for golang",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}, {"type": "function", "function": {"name": "search", "arguments": "golang"}}]}`,
				"Here are the results: 4 and search results for golang.",
			},
			contains: []string{"Here are the results: 4 and search results for golang."},
		},
		{
			name:  "tool not found",
			tools: []tools.Tool{calc},
			input: "Search for something",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "unknown_tool", "arguments": "something"}}]}`,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockClientForTest(tt.llmResponses, tt.expectError && len(tt.llmResponses) == 0)
			agent := Agent(client, WithTools(tt.tools...))

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := agent.ServeFlow(req, res)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Agent() error = %v", err)
				return
			}

			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("Agent() output missing expected string %q, got %q", expected, output)
				}
			}
		})
	}
}

func TestAgentWithToolsConfig(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(_ string) string { return "result" })
	errorTool := createErrorTool()

	tests := []struct {
		name         string
		toolsConfig  *tools.Config
		tools        []tools.Tool
		input        string
		llmResponses []string
		expectError  bool
		contains     []string
	}{
		{
			name: "tool execution error",
			toolsConfig: &tools.Config{
				MaxConcurrentTools: 1,
			},
			tools: []tools.Tool{errorTool},
			input: "Use error tool",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "error_tool", "arguments": "test"}}]}`,
			},
			expectError: true, // Should always fail on tool error now
		},
		{
			name:         "basic tool execution",
			tools:        []tools.Tool{calc},
			input:        "Test input",
			llmResponses: []string{"Response"},
			contains:     []string{"Response"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockClientForTest(tt.llmResponses, tt.expectError && len(tt.llmResponses) == 0)

			// For error tool tests, we need to setup mock tool calls
			if len(tt.tools) > 0 && tt.tools[0] == errorTool {
				client.WithToolCalls(MockToolCall{Name: "error_tool", Arguments: "test"})
			}

			opts := []AgentOption{WithTools(tt.tools...)}
			if tt.toolsConfig != nil {
				opts = append(opts, WithToolsConfig(*tt.toolsConfig))
			}
			agent := Agent(client, opts...)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := agent.ServeFlow(req, res)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("AgentWithConfig() error = %v", err)
				return
			}

			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("AgentWithConfig() output missing expected string %q, got %q", expected, output)
				}
			}
		})
	}
}

func TestDefaultToolsConfig(t *testing.T) {
	// Test that default tools config is created correctly when none provided
	calc := tools.Simple("calculator", "Math Calculator", func(_ string) string { return "result" })
	client := createMockClientForTest([]string{"Response"}, false)

	// Agent with tools but no explicit config should use defaults
	agent := Agent(client, WithTools(calc))

	var buf bytes.Buffer
	reader := strings.NewReader("Test input")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Agent with default config error = %v", err)
	}
}

func TestAgentSimpleChat(t *testing.T) {
	// Test agent without any tools (simple chat mode)
	client := createMockClientForTest([]string{"Hello! How can I help you today?"}, false)

	// Create agent without tools - should use simple chat mode
	agent := Agent(client)

	var buf bytes.Buffer
	reader := strings.NewReader("Hello")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Simple chat agent error = %v", err)
		return
	}

	output := buf.String()
	if !strings.Contains(output, "Hello! How can I help you today?") {
		t.Errorf("Simple chat agent output = %q", output)
	}
}

func TestAgentWithSchema(t *testing.T) {
	// Test agent with schema (structured output)
	client := createMockClientForTest([]string{`{"name": "John", "age": 30}`}, false)

	// Create a simple response format
	schema := &ResponseFormat{
		Type: "json_object",
	}

	agent := Agent(client, WithSchema(schema))

	var buf bytes.Buffer
	reader := strings.NewReader("Generate a person")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Schema agent error = %v", err)
		return
	}

	output := buf.String()
	if !strings.Contains(output, `"name": "John"`) {
		t.Errorf("Schema agent output = %q", output)
	}
}

func TestAgentWithClientError(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(s string) string { return s })

	client := createMockClientForTest([]string{}, true)
	agent := Agent(client, WithTools(calc))

	var buf bytes.Buffer
	reader := strings.NewReader("Test input")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err == nil {
		t.Error("Agent() with client error should return error")
	}

	if !strings.Contains(err.Error(), "client error") {
		t.Errorf("Agent() error should mention client error, got: %v", err)
	}
}

func TestAgentWithIOError(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(s string) string { return s })
	client := createMockClientForTest([]string{"response"}, false)

	agent := Agent(client, WithTools(calc))
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	req := calque.NewRequest(context.Background(), errorReader)
	res := calque.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Agent() with IO error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

// errorReader for testing IO errors
type errorReader struct {
	err error
}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, e.err
}
