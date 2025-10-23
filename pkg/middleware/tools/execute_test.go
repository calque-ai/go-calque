package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Mock tools for testing
func createMockCalculator() Tool {
	return Simple("calculator", "Math calculator", func(expr string) string {
		if expr == "2+2" {
			return "4"
		}
		if expr == "10*5" {
			return "50"
		}
		return fmt.Sprintf("calculated: %s", expr)
	})
}

func createMockSearch() Tool {
	return Simple("search", "Search the web", func(query string) string {
		return fmt.Sprintf("search results for: %s", query)
	})
}

func createErrorTool() Tool {
	return HandlerFunc("error_tool", "Tool that always errors", func(_ *calque.Request, _ *calque.Response) error {
		return errors.New("tool execution failed")
	})
}

func TestExecute(t *testing.T) {
	calc := createMockCalculator()
	search := createMockSearch()

	tests := []struct {
		name     string
		tools    []Tool
		input    string
		contains []string // Expected strings in output
		isError  bool
	}{
		{
			name:    "no tool calls - pass through",
			tools:   []Tool{calc, search},
			input:   "This is just regular text with no tool calls.",
			isError: true, // Should error execute assumes tool calls were detected already.
		},
		{
			name:     "JSON tool call format",
			tools:    []Tool{calc, search},
			input:    `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "10*5"}}]}`,
			contains: []string{"Tool execution results", "calculator", "50"},
		},
		{
			name:     "multiple tool calls",
			tools:    []Tool{calc, search},
			input:    `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}, {"type": "function", "function": {"name": "search", "arguments": "golang tutorials"}}]}`,
			contains: []string{"Tool execution results", "calculator", "4", "search", "search results for: golang tutorials"},
		},
		{
			name:    "unknown tool",
			tools:   []Tool{calc},
			input:   `{"tool_calls": [{"type": "function", "function": {"name": "unknown_tool", "arguments": "some args"}}]}`,
			isError: true, // Should error because tool not found
		},
		{
			name:    "no tools in context",
			tools:   []Tool{},
			input:   `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
			isError: true, // Should error because if there are no tools available why are we running execute tools?
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up context with tools using Registry
			ctx := context.Background()
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			// Create a pipeline with Registry and Execute
			pipeline := NewPipelineForTest(tt.tools)

			req := calque.NewRequest(ctx, reader)
			res := calque.NewResponse(&buf)
			err := pipeline.ServeFlow(req, res)
			if tt.isError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() error = %v", err)
				return
			}

			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("Execute() output missing expected string %q, got %q", expected, output)
				}
			}
		})
	}
}

// NewPipelineForTest creates a test pipeline that combines Registry and Execute functionality
func NewPipelineForTest(tools []Tool) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Create context with tools (simulating what Registry + Execute should do)
		ctx := context.WithValue(req.Context, toolsContextKey{}, tools)
		req = req.WithContext(ctx)

		// Execute tools directly (this simulates the combined Registry + Execute flow)
		execute := Execute()
		return execute.ServeFlow(req, res)
	})
}

func TestExecuteWithOptions(t *testing.T) {
	calc := createMockCalculator()
	errorTool := createErrorTool()

	tests := []struct {
		name        string
		config      Config
		tools       []Tool
		input       string
		expectError bool
		contains    []string
	}{
		{
			name: "tool execution error",
			config: Config{
				MaxConcurrentTools: 1,
			},
			tools:       []Tool{errorTool},
			input:       `{"tool_calls": [{"type": "function", "function": {"name": "error_tool", "arguments": "test"}}]}`,
			expectError: true, // Should always error on tool failure now
		},
		{
			name: "include original output",
			config: Config{
				IncludeOriginalOutput: true,
			},
			tools:    []Tool{calc},
			input:    `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
			contains: []string{"Original LLM Output:", "Tool execution results", "4"},
		},
		{
			name: "raw output without original",
			config: Config{
				RawOutput: true,
			},
			tools:    []Tool{calc},
			input:    `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
			contains: []string{`"tool_call"`, `"result"`, `"name":"calculator"`, `"arguments":"2+2"`},
		},
		{
			name: "raw output with original",
			config: Config{
				RawOutput:             true,
				IncludeOriginalOutput: true,
			},
			tools:    []Tool{calc},
			input:    `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
			contains: []string{`"original_output"`, `"results"`, `"tool_call"`, `"name":"calculator"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			// Create pipeline with config
			pipeline := NewPipelineForTestWithConfig(tt.tools, tt.config)

			req := calque.NewRequest(ctx, reader)
			res := calque.NewResponse(&buf)
			err := pipeline.ServeFlow(req, res)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteWithOptions() error = %v", err)
				return
			}

			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("ExecuteWithOptions() output missing expected string %q, got %q", expected, output)
				}
			}
		})
	}
}

func NewPipelineForTestWithConfig(tools []Tool, config Config) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Create context with tools (simulating what Registry + Execute should do)
		ctx := context.WithValue(req.Context, toolsContextKey{}, tools)
		req = req.WithContext(ctx)

		// Execute tools with config
		execute := ExecuteWithOptions(config)
		return execute.ServeFlow(req, res)
	})
}

func TestParseToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ToolCall
	}{
		{
			name:  "single JSON tool call",
			input: `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "call_0"},
			},
		},
		{
			name:  "multiple JSON tool calls",
			input: `{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}, {"type": "function", "function": {"name": "search", "arguments": "golang"}}]}`,
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "call_0"},
				{Name: "search", Arguments: "golang", ID: "call_1"},
			},
		},
		{
			name:  "malformed JSON - invalid format",
			input: `{"tool_calls": [{"name": "missing_function_wrapper"}]}`,
			expected: []ToolCall{
				{
					Name:      "_parse_error",
					Arguments: `{"tool_calls": [{"name": "missing_function_wrapper"}]}`,
					Error:     "Failed to parse tool call JSON:",
				},
			},
		},
		{
			name:  "completely invalid JSON",
			input: `{"invalid_json": malformed`,
			expected: []ToolCall{
				{
					Name:      "_parse_error",
					Arguments: `{"invalid_json": malformed`,
					Error:     "Failed to parse tool call JSON:",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolCalls([]byte(tt.input))

			if len(result) != len(tt.expected) {
				t.Errorf("ParseToolCalls() returned %d calls, want %d", len(result), len(tt.expected))
				return
			}

			for i, call := range result {
				if i < len(tt.expected) {
					expected := tt.expected[i]
					if call.Name != expected.Name {
						t.Errorf("ParseToolCalls()[%d].Name = %q, want %q", i, call.Name, expected.Name)
					}
					if call.Arguments != expected.Arguments {
						t.Errorf("ParseToolCalls()[%d].Arguments = %q, want %q", i, call.Arguments, expected.Arguments)
					}
					// Note: ID is auto-generated, so we just check it exists if expected
					if expected.ID != "" && call.ID == "" {
						t.Errorf("ParseToolCalls()[%d].ID is empty, expected non-empty", i)
					}
				}
			}
		})
	}
}

func TestExecuteToolCall(t *testing.T) {
	calc := createMockCalculator()
	errorTool := createErrorTool()
	tools := []Tool{calc, errorTool}

	tests := []struct {
		name         string
		toolCall     ToolCall
		expectError  bool
		expectResult string
	}{
		{
			name:         "successful execution",
			toolCall:     ToolCall{Name: "calculator", Arguments: "2+2"},
			expectError:  false,
			expectResult: "4",
		},
		{
			name:        "tool not found",
			toolCall:    ToolCall{Name: "unknown", Arguments: "test"},
			expectError: true,
		},
		{
			name:        "tool execution error",
			toolCall:    ToolCall{Name: "error_tool", Arguments: "test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := executeToolCall(ctx, tools, tt.toolCall)

			if tt.expectError {
				if result.Error == "" {
					t.Error("Expected error in result, got none")
				}
			} else {
				if result.Error != "" {
					t.Errorf("Unexpected error in result: %s", result.Error)
				}
				if string(result.Result) != tt.expectResult {
					t.Errorf("executeToolCall() result = %q, want %q", string(result.Result), tt.expectResult)
				}
			}

			if result.ToolCall.Name != tt.toolCall.Name {
				t.Errorf("executeToolCall() tool name = %q, want %q", result.ToolCall.Name, tt.toolCall.Name)
			}
		})
	}
}

func TestExecuteWithIOError(t *testing.T) {
	// Create a pipeline with tools to test IO error
	calc := createMockCalculator()
	pipeline := NewPipelineForTest([]Tool{calc})
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	req := calque.NewRequest(context.Background(), errorReader)
	res := calque.NewResponse(&buf)
	err := pipeline.ServeFlow(req, res)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Execute() with IO error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func TestExecuteWithEmptyContext(t *testing.T) {
	execute := Execute()
	var buf bytes.Buffer
	reader := strings.NewReader(`{"tool_calls": [{"type": "function", "function": {"name": "test", "arguments": "args"}}]}`)

	// Empty context (no tools) - should error now
	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := execute.ServeFlow(req, res)
	if err == nil {
		t.Error("Execute() with empty context should error")
		return
	}

	// Should error with "no tools available"
	if !strings.Contains(err.Error(), "no tools available") {
		t.Errorf("Execute() with empty context error = %q, want error containing 'no tools available'", err.Error())
	}
}

func TestExecuteToolCallsConcurrency(t *testing.T) {
	// Create tools with varying execution times to test concurrency
	fastTool := Simple("fast", "Fast tool", func(_ string) string {
		return "fast_result"
	})

	slowTool := Simple("slow", "Slow tool", func(_ string) string {
		time.Sleep(50 * time.Millisecond)
		return "slow_result"
	})

	tools := []Tool{fastTool, slowTool}

	tests := []struct {
		name         string
		config       Config
		numToolCalls int
		expectError  bool
	}{
		{
			name: "no concurrency limit - all tools run concurrently",
			config: Config{
				MaxConcurrentTools: 0, // No limit
			},
			numToolCalls: 4,
			expectError:  false,
		},
		{
			name: "concurrency limited to 2 workers",
			config: Config{
				MaxConcurrentTools: 2,
			},
			numToolCalls: 5, // More tools than workers
			expectError:  false,
		},
		{
			name: "concurrency limited to 1 worker",
			config: Config{
				MaxConcurrentTools: 1,
			},
			numToolCalls: 3,
			expectError:  false,
		},
		{
			name: "single tool call optimization",
			config: Config{
				MaxConcurrentTools: 5,
			},
			numToolCalls: 1, // Should use single tool optimization
			expectError:  false,
		},
		{
			name: "more workers than tools",
			config: Config{
				MaxConcurrentTools: 10,
			},
			numToolCalls: 3, // Fewer tools than max workers
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tool calls alternating between fast and slow
			toolCalls := make([]ToolCall, tt.numToolCalls)
			for i := 0; i < tt.numToolCalls; i++ {
				toolName := "fast"
				if i%2 == 1 {
					toolName = "slow"
				}
				toolCalls[i] = ToolCall{
					Name:      toolName,
					Arguments: "test",
					ID:        fmt.Sprintf("call_%d", i),
				}
			}

			ctx := context.Background()
			start := time.Now()

			// Execute with timeout to catch any deadlocks/hangs
			done := make(chan []ToolResult, 1)
			errChan := make(chan error, 1)

			go func() {
				defer func() {
					if r := recover(); r != nil {
						errChan <- fmt.Errorf("panic: %v", r)
					}
				}()
				results := executeToolCallsWithConfig(ctx, tools, toolCalls, tt.config)
				done <- results
			}()

			// Set reasonable timeout based on test
			timeout := 500 * time.Millisecond
			if tt.config.MaxConcurrentTools == 1 && tt.numToolCalls > 2 {
				timeout = 1 * time.Second // Sequential execution needs more time
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			select {
			case results := <-done:
				elapsed := time.Since(start)

				if tt.expectError {
					t.Error("Expected error but function completed successfully")
					return
				}

				// Verify we got the expected number of results
				if len(results) != tt.numToolCalls {
					t.Errorf("Expected %d results, got %d", tt.numToolCalls, len(results))
				}

				// Verify all results are present and correct
				for i, result := range results {
					if result.Error != "" {
						t.Errorf("Tool call %d failed: %s", i, result.Error)
					}

					expectedResult := "fast_result"
					if toolCalls[i].Name == "slow" {
						expectedResult = "slow_result"
					}

					if string(result.Result) != expectedResult {
						t.Errorf("Tool call %d result = %q, want %q", i, string(result.Result), expectedResult)
					}
				}

				t.Logf("Completed %d tool calls in %v with max_concurrent=%d",
					tt.numToolCalls, elapsed, tt.config.MaxConcurrentTools)

			case err := <-errChan:
				if !tt.expectError {
					t.Errorf("Unexpected panic/error: %v", err)
				}

			case <-timeoutCtx.Done():
				t.Errorf("Function timed out after %v - possible deadlock or hang", timeout)
			}
		})
	}
}
