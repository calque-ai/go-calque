package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

// Helper function to create a mock provider for agent tests
func createMockProviderForTest(responses []string, shouldErr bool) *MockProvider {
	if shouldErr {
		return NewMockProviderWithError("LLM provider error")
	}
	mock := NewMockProviderWithResponses(responses)
	// For tool calling tests, we need to enable tool call simulation
	if len(responses) > 0 && strings.Contains(responses[0], "tool_calls") {
		// Parse the first response to create mock tool calls
		if strings.Contains(responses[0], "error_tool") {
			mock.WithToolCalls(MockToolCall{Name: "error_tool", Arguments: "test"})
		} else if strings.Contains(responses[0], "calculator") {
			mock.WithToolCalls(MockToolCall{Name: "calculator", Arguments: "2+2"})
		}
	}
	return mock
}

// createErrorTool creates a tool that always returns an error for testing
func createErrorTool() tools.Tool {
	return tools.Simple("error_tool", "Always errors", func(input string) string {
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
			provider := createMockProviderForTest(tt.llmResponses, tt.expectError && len(tt.llmResponses) == 0)
			agent := Agent(provider, tt.tools...)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
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

func TestAgentWithConfig(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(expr string) string { return "result" })
	errorTool := createErrorTool()

	tests := []struct {
		name         string
		config       AgentConfig
		tools        []tools.Tool
		input        string
		llmResponses []string
		expectError  bool
		contains     []string
	}{
		{
			name: "pass through on error enabled",
			config: AgentConfig{
				MaxIterations: 3,
				ExecuteConfig: tools.ExecuteConfig{
					PassThroughOnError: true,
				},
			},
			tools: []tools.Tool{errorTool},
			input: "Use error tool",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "error_tool", "arguments": "test"}}]}`,
			},
			contains: []string{`{"tool_calls": [{"type": "function", "function": {"name": "error_tool", "arguments": "test"}}]}`}, // Should pass through
		},
		{
			name: "pass through on error disabled",
			config: AgentConfig{
				MaxIterations: 3,
				ExecuteConfig: tools.ExecuteConfig{
					PassThroughOnError: false,
				},
			},
			tools: []tools.Tool{errorTool},
			input: "Use error tool",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "error_tool", "arguments": "test"}}]}`,
			},
			expectError: true, // Should fail due to tool error
		},
		{
			name: "timeout configuration",
			config: AgentConfig{
				MaxIterations: 10,
				Timeout:       100 * time.Millisecond,
			},
			tools:        []tools.Tool{calc},
			input:        "Test timeout",
			llmResponses: []string{"Response"},
			contains:     []string{"Response"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := createMockProviderForTest(tt.llmResponses, tt.expectError && len(tt.llmResponses) == 0)
			agent := AgentWithConfig(provider, tt.config, tt.tools...)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
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
					t.Errorf("AgentWithConfig() output missing expected string %q", expected)
				}
			}
		})
	}
}

func TestDefaultAgentConfig(t *testing.T) {
	config := DefaultAgentConfig()

	if config.MaxIterations != 5 {
		t.Errorf("DefaultAgentConfig().MaxIterations = %d, want 5", config.MaxIterations)
	}

	if config.ExecuteConfig.PassThroughOnError {
		t.Error("DefaultAgentConfig().tools.ExecuteConfig.PassThroughOnError should be false")
	}

	if config.Timeout != 0 {
		t.Errorf("DefaultAgentConfig().Timeout = %v, want 0", config.Timeout)
	}
}

func TestQuickAgent(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(expr string) string { return "quick result" })

	// QuickAgent should be more forgiving (PassThroughOnError = true)
	provider := createMockProviderForTest([]string{"Result without tools"}, false)

	agent := Agent(provider, calc)
	var buf bytes.Buffer
	reader := strings.NewReader("Test input")

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err != nil {
		t.Errorf("QuickAgent() error = %v", err)
		return
	}

	output := buf.String()
	if !strings.Contains(output, "Result without tools") {
		t.Errorf("QuickAgent() output = %q", output)
	}
}

func TestAgentWithLLMError(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(s string) string { return s })

	provider := createMockProviderForTest([]string{}, true)
	agent := Agent(provider, calc)

	var buf bytes.Buffer
	reader := strings.NewReader("Test input")

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err == nil {
		t.Error("Agent() with LLM error should return error")
	}

	if !strings.Contains(err.Error(), "LLM provider error") {
		t.Errorf("Agent() error should mention LLM provider error, got: %v", err)
	}
}

func TestAgentWithIOError(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(s string) string { return s })
	provider := createMockProviderForTest([]string{"response"}, false)

	agent := Agent(provider, calc)
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	req := core.NewRequest(context.Background(), errorReader)
	res := core.NewResponse(&buf)
	err := agent.ServeFlow(req, res)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Agent() with IO error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

// errorReader for testing IO errors
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
