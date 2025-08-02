package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

// Mock LLM provider for testing
type mockLLMProvider struct {
	responses []string
	callCount int
	shouldErr bool
}

func (m *mockLLMProvider) Chat(req *core.Request, res *core.Response) error {
	return m.ChatWithTools(req, res)
}

func (m *mockLLMProvider) ChatWithTools(req *core.Request, res *core.Response, toolList ...tools.Tool) error {
	if m.shouldErr {
		return errors.New("LLM provider error")
	}

	if m.callCount >= len(m.responses) {
		return errors.New("no more responses available")
	}

	response := m.responses[m.callCount]
	m.callCount++

	return core.Write(res, response)
}

func (m *mockLLMProvider) reset() {
	m.callCount = 0
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
			provider := &mockLLMProvider{responses: tt.llmResponses}
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
			name: "max iterations limit",
			config: AgentConfig{
				MaxIterations: 2,
			},
			tools: []tools.Tool{calc},
			input: "Keep using tools",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "test1"}}]}`,
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "test2"}}]}`,
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "test3"}}]}`, // This should not be reached
			},
			expectError: true, // Should fail due to max iterations
		},
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
			provider := &mockLLMProvider{responses: tt.llmResponses}
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
	provider := &mockLLMProvider{
		responses: []string{"Result without tools"},
	}

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

	provider := &mockLLMProvider{shouldErr: true}
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
	provider := &mockLLMProvider{responses: []string{"response"}}

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
