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
				"I couldn't find that tool.",
			},
			contains: []string{"I couldn't find that tool."},
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
				"The tool failed, so I can't complete that.",
			},
			contains: []string{"The tool failed, so I can't complete that."},
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

// TestAgentWithSchemaAndTools pins that Schema is still passed to the LLM on
// every loop iteration when tools are used, not just on the rare path where
// MaxIterations is exhausted - the common case is the model calling a tool
// once and then answering directly within the iteration budget.
func TestAgentWithSchemaAndTools(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(_ string) string { return "4" })

	client := NewMockClientWithResponses([]string{
		`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
		`{"name": "answer", "value": 4}`,
	})

	schema := &ResponseFormat{Type: ResponseFormatJSONObject}
	agent := Agent(client, WithTools(calc), WithSchema(schema))

	var buf bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("What is 2+2?"))
	res := calque.NewResponse(&buf)
	if err := agent.ServeFlow(req, res); err != nil {
		t.Fatalf("agent error = %v", err)
	}

	if !strings.Contains(buf.String(), `"value": 4`) {
		t.Errorf("expected structured output to survive tool-calling loop, got: %s", buf.String())
	}

	// The normal exit (model stops requesting tools within budget) must
	// still pass Schema through to the client, not just on the forced
	// MaxIterations-exhaustion path.
	if client.CallCount() != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", client.CallCount())
	}
	for i := range 2 {
		opts := client.OptionsAt(i)
		if opts == nil || opts.Schema == nil {
			t.Errorf("call %d: expected Schema to be set, got nil", i)
		}
	}
}

// TestAgentWithMultimodalDataAndTools pins that MultimodalData survives into
// the tool-calling loop's first turn instead of being silently dropped the
// moment tools are configured.
func TestAgentWithMultimodalDataAndTools(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(_ string) string { return "4" })

	client := NewMockClientWithResponses([]string{
		`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
		"It's 4, and the image shows a cat.",
	})

	multimodal := Multimodal(Text("What is 2+2? Also, describe this image."), ImageData([]byte("fake-image-bytes"), "image/png"))
	agent := Agent(client, WithTools(calc), WithMultimodalData(&multimodal))

	var buf bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader(`{"parts":[]}`))
	res := calque.NewResponse(&buf)
	if err := agent.ServeFlow(req, res); err != nil {
		t.Fatalf("agent error = %v", err)
	}

	firstTurnHistory := client.HistoryAt(0)
	if len(firstTurnHistory) == 0 {
		t.Fatal("expected first turn to have history")
	}
	if firstTurnHistory[0].Multimodal == nil {
		t.Fatal("expected first user message to carry MultimodalData, got nil")
	}
	if len(firstTurnHistory[0].Multimodal.Parts) != 2 {
		t.Errorf("expected 2 multimodal parts preserved, got %d", len(firstTurnHistory[0].Multimodal.Parts))
	}
}

// TestAgentToolFormatterClientUsedThroughoutLoop pins that a caller-supplied
// ToolFormatterClient is actually used for every LLM call in the loop, not
// just ignored in favor of the original client.
func TestAgentToolFormatterClientUsedThroughoutLoop(t *testing.T) {
	calc := tools.Simple("calculator", "Math Calculator", func(_ string) string { return "4" })

	// The primary client should never be called once a formatter client is
	// configured - every turn should route through formatterClient instead.
	primaryClient := NewMockClientWithError("primary client should not be called")

	formatterClient := NewMockClientWithResponses([]string{
		`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
		"The answer is 4.",
	})

	agent := Agent(primaryClient, WithTools(calc), WithToolResultFormatter(
		func(_ Client, _ []byte) calque.Handler {
			return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
				var final []byte
				if err := calque.Read(r, &final); err != nil {
					return err
				}
				return calque.Write(w, final)
			})
		},
		formatterClient,
	))

	var buf bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("What is 2+2?"))
	res := calque.NewResponse(&buf)
	if err := agent.ServeFlow(req, res); err != nil {
		t.Fatalf("agent error = %v (primary client should never be invoked)", err)
	}

	if !strings.Contains(buf.String(), "The answer is 4.") {
		t.Errorf("expected answer from formatterClient, got: %s", buf.String())
	}
	if formatterClient.CallCount() != 2 {
		t.Errorf("expected formatterClient to be called twice, got %d", formatterClient.CallCount())
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
