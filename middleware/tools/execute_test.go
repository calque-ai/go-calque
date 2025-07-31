package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
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
	return HandlerFunc("error_tool", "Tool that always errors", func(ctx context.Context, r io.Reader, w io.Writer) error {
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
			name:     "no tool calls - pass through",
			tools:    []Tool{calc, search},
			input:    "This is just regular text with no tool calls.",
			contains: []string{"This is just regular text with no tool calls."},
		},
		{
			name:     "simple tool call format",
			tools:    []Tool{calc, search},
			input:    "I need to calculate: TOOL:calculator:2+2",
			contains: []string{"Tool execution results", "calculator", "4"},
		},
		{
			name:     "JSON tool call format",
			tools:    []Tool{calc, search},
			input:    `{"tool_calls": [{"name": "calculator", "arguments": "10*5"}]}`,
			contains: []string{"Tool execution results", "calculator", "50"},
		},
		{
			name:     "XML tool call format",
			tools:    []Tool{calc, search},
			input:    `Please calculate this: <tool name="calculator">2+2</tool>`,
			contains: []string{"Tool execution results", "calculator", "4"},
		},
		{
			name:     "multiple tool calls",
			tools:    []Tool{calc, search},
			input:    "TOOL:calculator:2+2 and TOOL:search:golang tutorials",
			contains: []string{"Tool execution results", "calculator", "4", "search", "search results for: golang tutorials"},
		},
		{
			name:     "unknown tool",
			tools:    []Tool{calc},
			input:    "TOOL:unknown_tool:some args",
			contains: []string{"Tool execution results", "unknown_tool", "Tool 'unknown_tool' not found"},
		},
		{
			name:     "no tools in context",
			tools:    []Tool{},
			input:    "TOOL:calculator:2+2",
			contains: []string{"TOOL:calculator:2+2"}, // Should pass through
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up context with tools
			ctx := context.WithValue(context.Background(), toolsContextKey{}, tt.tools)

			execute := Execute()
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			err := execute.ServeFlow(ctx, reader, &buf)

			if tt.isError {
				if err == nil {
					t.Error("Expected error, got nil")
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

func TestExecuteWithOptions(t *testing.T) {
	calc := createMockCalculator()
	errorTool := createErrorTool()

	tests := []struct {
		name     string
		tools    []Tool
		input    string
		config   ExecuteConfig
		contains []string
	}{
		{
			name:  "pass through on error - enabled",
			tools: []Tool{calc, errorTool},
			input: "TOOL:error_tool:some args",
			config: ExecuteConfig{
				PassThroughOnError: true,
			},
			contains: []string{"TOOL:error_tool:some args"}, // Should pass through original
		},
		{
			name:  "pass through on error - disabled",
			tools: []Tool{calc, errorTool},
			input: "TOOL:error_tool:some args",
			config: ExecuteConfig{
				PassThroughOnError: false,
			},
			contains: []string{"Tool execution results", "tool execution failed"},
		},
		{
			name:  "include original output",
			tools: []Tool{calc},
			input: "Please calculate: TOOL:calculator:2+2",
			config: ExecuteConfig{
				IncludeOriginalOutput: true,
			},
			contains: []string{"Original LLM Output:", "Please calculate: TOOL:calculator:2+2", "Tool execution results", "4"},
		},
		{
			name:  "max concurrent tools",
			tools: []Tool{calc},
			input: "TOOL:calculator:2+2",
			config: ExecuteConfig{
				MaxConcurrentTools: 1,
			},
			contains: []string{"Tool execution results", "4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), toolsContextKey{}, tt.tools)

			execute := ExecuteWithOptions(tt.config)
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			err := execute.ServeFlow(ctx, reader, &buf)
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

func TestParseToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ToolCall
	}{
		{
			name:     "no tool calls",
			input:    "Just regular text with no tool calls.",
			expected: []ToolCall{},
		},
		{
			name:  "simple format single tool",
			input: "TOOL:calculator:2+2",
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "call_0"},
			},
		},
		{
			name:  "simple format multiple tools",
			input: "TOOL:calculator:2+2 and then TOOL:search:golang",
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "call_0"},
				{Name: "search", Arguments: "golang", ID: "call_1"},
			},
		},
		{
			name:  "JSON format",
			input: `{"tool_calls": [{"name": "calculator", "arguments": "2+2", "id": "test_id"}]}`,
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "test_id"},
			},
		},
		{
			name:  "XML format",
			input: `<tool name="calculator">2+2</tool>`,
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "xml_call_0"},
			},
		},
		{
			name:  "mixed formats",
			input: `TOOL:search:test and <tool name="calculator">10*5</tool>`,
			expected: []ToolCall{
				{Name: "search", Arguments: "test", ID: "call_0"},
				{Name: "calculator", Arguments: "10*5", ID: "xml_call_0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolCalls(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("parseToolCalls() returned %d calls, want %d", len(result), len(tt.expected))
				return
			}

			for i, call := range result {
				if i < len(tt.expected) {
					expected := tt.expected[i]
					if call.Name != expected.Name {
						t.Errorf("parseToolCalls()[%d].Name = %q, want %q", i, call.Name, expected.Name)
					}
					if call.Arguments != expected.Arguments {
						t.Errorf("parseToolCalls()[%d].Arguments = %q, want %q", i, call.Arguments, expected.Arguments)
					}
					if expected.ID != "" && call.ID != expected.ID {
						t.Errorf("parseToolCalls()[%d].ID = %q, want %q", i, call.ID, expected.ID)
					}
				}
			}
		})
	}
}

func TestParseJSONToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ToolCall
	}{
		{
			name:     "invalid JSON",
			input:    "not json",
			expected: nil,
		},
		{
			name:     "valid JSON without tool_calls",
			input:    `{"message": "hello"}`,
			expected: nil,
		},
		{
			name:  "valid JSON with tool_calls",
			input: `{"tool_calls": [{"name": "calc", "arguments": "1+1"}]}`,
			expected: []ToolCall{
				{Name: "calc", Arguments: "1+1"},
			},
		},
		{
			name:  "multiple tool calls",
			input: `{"tool_calls": [{"name": "calc", "arguments": "1+1"}, {"name": "search", "arguments": "test"}]}`,
			expected: []ToolCall{
				{Name: "calc", Arguments: "1+1"},
				{Name: "search", Arguments: "test"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJSONToolCalls(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("parseJSONToolCalls() returned %d calls, want %d", len(result), len(tt.expected))
				return
			}

			for i, call := range result {
				if i < len(tt.expected) {
					expected := tt.expected[i]
					if call.Name != expected.Name || call.Arguments != expected.Arguments {
						t.Errorf("parseJSONToolCalls()[%d] = %+v, want %+v", i, call, expected)
					}
				}
			}
		})
	}
}

func TestParseSimpleToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ToolCall
	}{
		{
			name:     "no tool calls",
			input:    "regular text",
			expected: []ToolCall{},
		},
		{
			name:  "single tool call",
			input: "TOOL:calculator:2+2",
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "call_0"},
			},
		},
		{
			name:  "multiple tool calls",
			input: "First TOOL:calc:1+1 then TOOL:search:golang",
			expected: []ToolCall{
				{Name: "calc", Arguments: "1+1", ID: "call_0"},
				{Name: "search", Arguments: "golang", ID: "call_1"},
			},
		},
		{
			name:  "tool call with spaces",
			input: "TOOL: calculator : 2 + 2 ",
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2 + 2", ID: "call_0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSimpleToolCalls(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("parseSimpleToolCalls() returned %d calls, want %d", len(result), len(tt.expected))
				return
			}

			for i, call := range result {
				if i < len(tt.expected) {
					expected := tt.expected[i]
					if call.Name != expected.Name || call.Arguments != expected.Arguments {
						t.Errorf("parseSimpleToolCalls()[%d] = %+v, want %+v", i, call, expected)
					}
				}
			}
		})
	}
}

func TestParseXMLToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ToolCall
	}{
		{
			name:     "no tool calls",
			input:    "regular text",
			expected: []ToolCall{},
		},
		{
			name:  "single tool call",
			input: `<tool name="calculator">2+2</tool>`,
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "xml_call_0"},
			},
		},
		{
			name:  "multiple tool calls",
			input: `<tool name="calc">1+1</tool> and <tool name="search">golang</tool>`,
			expected: []ToolCall{
				{Name: "calc", Arguments: "1+1", ID: "xml_call_0"},
				{Name: "search", Arguments: "golang", ID: "xml_call_1"},
			},
		},
		{
			name:  "tool with extra attributes",
			input: `<tool name="calculator" type="math">2+2</tool>`,
			expected: []ToolCall{
				{Name: "calculator", Arguments: "2+2", ID: "xml_call_0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseXMLToolCalls(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("parseXMLToolCalls() returned %d calls, want %d", len(result), len(tt.expected))
				return
			}

			for i, call := range result {
				if i < len(tt.expected) {
					expected := tt.expected[i]
					if call.Name != expected.Name || call.Arguments != expected.Arguments {
						t.Errorf("parseXMLToolCalls()[%d] = %+v, want %+v", i, call, expected)
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
			name:         "tool not found",
			toolCall:     ToolCall{Name: "unknown", Arguments: "test"},
			expectError:  true,
			expectResult: "",
		},
		{
			name:         "tool execution error",
			toolCall:     ToolCall{Name: "error_tool", Arguments: "test"},
			expectError:  true,
			expectResult: "",
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

func TestFormatToolResults(t *testing.T) {
	results := []ToolResult{
		{
			ToolCall: ToolCall{Name: "calculator", Arguments: "2+2"},
			Result:   []byte("4"),
		},
		{
			ToolCall: ToolCall{Name: "search", Arguments: "golang"},
			Result:   []byte("search results for: golang"),
		},
		{
			ToolCall: ToolCall{Name: "error_tool", Arguments: "test"},
			Error:    "Tool execution failed",
		},
	}

	output := formatToolResults(results, "original")

	// Check that all expected elements are present
	expectedStrings := []string{
		"Tool execution results",
		"Tool 1: calculator",
		"Arguments: 2+2",
		"Result: 4",
		"Tool 2: search",
		"Arguments: golang",
		"Result: search results for: golang",
		"Tool 3: error_tool",
		"Error: Tool execution failed",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("formatToolResults() missing expected string %q", expected)
		}
	}
}

func TestFormatToolResultsWithOriginal(t *testing.T) {
	results := []ToolResult{
		{
			ToolCall: ToolCall{Name: "test", Arguments: "args"},
			Result:   []byte("result"),
		},
	}

	originalOutput := "This is the original LLM output"
	output := formatToolResultsWithOriginal(results, originalOutput)

	expectedStrings := []string{
		"Original LLM Output:",
		originalOutput,
		"Tool execution results",
		"Tool 1: test",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("formatToolResultsWithOriginal() missing expected string %q", expected)
		}
	}
}

func TestExecuteWithIOError(t *testing.T) {
	execute := Execute()
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	err := execute.ServeFlow(context.Background(), errorReader, &buf)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Execute() with IO error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func TestExecuteWithEmptyContext(t *testing.T) {
	execute := Execute()
	input := "TOOL:calculator:2+2"

	var buf bytes.Buffer
	reader := strings.NewReader(input)

	// Empty context (no tools)
	err := execute.ServeFlow(context.Background(), reader, &buf)
	if err != nil {
		t.Errorf("Execute() with empty context error = %v", err)
		return
	}

	// Should pass through original input
	if got := buf.String(); got != input {
		t.Errorf("Execute() with empty context = %q, want %q", got, input)
	}
}
