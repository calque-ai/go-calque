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

// func TestFormatInputWithTools(t *testing.T) {
// 	calc := tools.Simple("calculator", "Math Calculator", func(s string) string { return s })
// 	search := tools.Simple("search", "Search tool", func(s string) string { return s })

// 	ctx := context.WithValue(context.Background(), agentToolsKey{}, []tools.Tool{calc, search})

// 	tests := []struct {
// 		name     string
// 		input    string
// 		contains []string
// 	}{
// 		{
// 			name:  "simple format",
// 			input: "Test input",
// 			contains: []string{
// 				"Test input",
// 				"Available tools:",
// 				"1. calculator",
// 				"2. search",
// 				"Please use the tools if needed",
// 			},
// 		},
// 		{
// 			name:  "detailed format",
// 			input: "Help me",
// 			contains: []string{
// 				"Help me",
// 				"Available tools:",
// 				"Tool 1: calculator",
// 				"Description: Tool: calculator",
// 				"Usage: TOOL:calculator:",
// 				"Example: TOOL:calculator:sample input",
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := formatInputWithTools(ctx, []byte(tt.input))
// 			if err != nil {
// 				t.Errorf("formatInputWithTools() error = %v", err)
// 				return
// 			}

// 			for _, expected := range tt.contains {
// 				if !strings.Contains(string(result), expected) {
// 					t.Errorf("formatInputWithTools() missing expected string %q", expected)
// 				}
// 			}
// 		})
// 	}
// }

// func TestFormatInputWithNoTools(t *testing.T) {
// 	ctx := context.Background() // No tools in context
// 	input := "Test input"

// 	result, err := formatInputWithTools(ctx, []byte(input))
// 	if err != nil {
// 		t.Errorf("formatInputWithTools() with no tools error = %v", err)
// 		return
// 	}

// 	if string(result) != input {
// 		t.Errorf("formatInputWithTools() with no tools = %q, want %q", result, input)
// 	}
// }

// func TestCallLLM(t *testing.T) {
// 	provider := &mockLLMProvider{
// 		responses: []string{"Test response"},
// 	}

// 	result, err := callLLM(context.Background(), provider, "Test input")
// 	if err != nil {
// 		t.Errorf("callLLM() error = %v", err)
// 		return
// 	}

// 	if result != "Test response" {
// 		t.Errorf("callLLM() = %q, want %q", result, "Test response")
// 	}
// }

// func TestCallLLMWithError(t *testing.T) {
// 	provider := &mockLLMProvider{
// 		shouldErr: true,
// 	}

// 	_, err := callLLM(context.Background(), provider, "Test input")
// 	if err == nil {
// 		t.Error("callLLM() with error provider should return error")
// 	}
// }

// func TestExecutetoolsToolCallsForAgent(t *testing.T) {
// 	calc := tools.Simple("calculator", "Math Calculator", func(expr string) string { return "42" })
// 	errorTool := createErrorTool()

// 	ctx := context.WithValue(context.Background(), agentToolsKey{}, []tools.Tool{calc, errorTool})

// 	tests := []struct {
// 		name        string
// 		toolCalls   []tools.ToolCall
// 		config      tools.ExecuteConfig
// 		expectError bool
// 		checkResult func([]tools.ToolResult) bool
// 	}{
// 		{
// 			name: "successful execution",
// 			toolCalls: []tools.ToolCall{
// 				{Name: "calculator", Arguments: "2+2"},
// 			},
// 			config: tools.ExecuteConfig{PassThroughOnError: false},
// 			checkResult: func(results []tools.ToolResult) bool {
// 				return len(results) == 1 && string(results[0].Result) == "42" && results[0].Error == ""
// 			},
// 		},
// 		{
// 			name: "error with pass through enabled",
// 			toolCalls: []tools.ToolCall{
// 				{Name: "error_tool", Arguments: "test"},
// 			},
// 			config:      tools.ExecuteConfig{PassThroughOnError: true},
// 			expectError: false, // Should not error, but result will have error
// 			checkResult: func(results []tools.ToolResult) bool {
// 				return len(results) == 1 && results[0].Error != ""
// 			},
// 		},
// 		{
// 			name: "error with pass through disabled",
// 			toolCalls: []tools.ToolCall{
// 				{Name: "error_tool", Arguments: "test"},
// 			},
// 			config:      tools.ExecuteConfig{PassThroughOnError: false},
// 			expectError: true, // Should error immediately
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			results, err := executeToolCallsForAgent(ctx, tt.toolCalls, tt.config)

// 			if tt.expectError {
// 				if err == nil {
// 					t.Error("Expected error, got nil")
// 				}
// 				return
// 			}

// 			if err != nil {
// 				t.Errorf("executetools.ToolCallsForAgent() error = %v", err)
// 				return
// 			}

// 			if tt.checkResult != nil && !tt.checkResult(results) {
// 				t.Errorf("executetools.ToolCallsForAgent() results don't match expectations: %+v", results)
// 			}
// 		})
// 	}
// }

// func TestFormattoolsToolResultsForNextIteration(t *testing.T) {
// 	results := []tools.ToolResult{
// 		{
// 			ToolCall: tools.ToolCall{Name: "calculator", Arguments: "2+2"},
// 			Result:   []byte("4"),
// 		},
// 		{
// 			ToolCall: tools.ToolCall{Name: "search", Arguments: "golang"},
// 			Error:    "Search failed",
// 		},
// 	}

// 	originalResponse := "I'll help you with that."

// 	formatted := formatToolResultsForNextIteration(results, originalResponse)

// 	expected := []string{
// 		"Previous response: I'll help you with that.",
// 		"Tool execution results:",
// 		"Tool 1 (calculator):",
// 		"Result: 4",
// 		"Tool 2 (search):",
// 		"Error: Search failed",
// 		"Please continue your response",
// 	}

// 	for _, exp := range expected {
// 		if !strings.Contains(formatted, exp) {
// 			t.Errorf("formattools.ToolResultsForNextIteration() missing expected string %q", exp)
// 		}
// 	}
// }

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

	if !strings.Contains(err.Error(), "LLM call failed") {
		t.Errorf("Agent() error should mention LLM failure, got: %v", err)
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
