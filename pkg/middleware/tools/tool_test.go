package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// Mock handler for testing
type mockHandler struct {
	returnError bool
	transform   func(string) string
}

func (m *mockHandler) ServeFlow(req *calque.Request, res *calque.Response) error {
	if m.returnError {
		return errors.New("mock handler error")
	}

	var input string
	if err := calque.Read(req, &input); err != nil {
		return err
	}

	var output string
	if m.transform != nil {
		output = m.transform(input)
	} else {
		output = "processed: " + input
	}

	return calque.Write(res, output)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		description string
		input       string
		transform   func(string) string
		expected    string
	}{
		{
			name:        "basic tool creation",
			toolName:    "test_tool",
			description: "A test tool",
			input:       "hello",
			expected:    "processed: hello",
		},
		{
			name:        "tool with custom transformation",
			toolName:    "uppercase",
			description: "Converts to uppercase",
			input:       "hello world",
			transform:   strings.ToUpper,
			expected:    "HELLO WORLD",
		},
		{
			name:        "empty input",
			toolName:    "empty_test",
			description: "Test empty input",
			input:       "",
			expected:    "processed: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &mockHandler{transform: tt.transform}

			// Create a basic schema for testing
			properties := orderedmap.New[string, *jsonschema.Schema]()
			properties.Set("input", &jsonschema.Schema{
				Type:        "string",
				Description: "Test input parameter",
			})
			schema := &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"input"},
			}

			tool := New(tt.toolName, tt.description, schema, handler)

			// Test metadata
			if tool.Name() != tt.toolName {
				t.Errorf("Name() = %q, want %q", tool.Name(), tt.toolName)
			}

			if tool.Description() != tt.description {
				t.Errorf("Description() = %q, want %q", tool.Description(), tt.description)
			}

			// Test execution
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := tool.ServeFlow(req, res)
			if err != nil {
				t.Errorf("ServeFlow() error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("ServeFlow() output = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNewWithError(t *testing.T) {
	handler := &mockHandler{returnError: true}

	// Create a basic schema for testing
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("input", &jsonschema.Schema{
		Type:        "string",
		Description: "Test input parameter",
	})
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"input"},
	}

	tool := New("error_tool", "Tool that errors", schema, handler)

	var buf bytes.Buffer
	reader := strings.NewReader("test")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := tool.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error from tool execution, got nil")
	}

	expectedErr := "mock handler error"
	if err.Error() != expectedErr {
		t.Errorf("Error message = %q, want %q", err.Error(), expectedErr)
	}
}

func TestSimple(t *testing.T) {
	name := "test_tool"
	description := "Custom description for test tool"
	fn := func(s string) string { return "result: " + s }

	tool := Simple(name, description, fn)

	// Test metadata
	if tool.Name() != name {
		t.Errorf("Name() = %q, want %q", tool.Name(), name)
	}

	if tool.Description() != description {
		t.Errorf("Description() = %q, want %q", tool.Description(), description)
	}

	// Test execution
	var buf bytes.Buffer
	reader := strings.NewReader("input")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := tool.ServeFlow(req, res)
	if err != nil {
		t.Errorf("ServeFlow() error = %v", err)
		return
	}

	expected := "result: input"
	if got := buf.String(); got != expected {
		t.Errorf("ServeFlow() output = %q, want %q", got, expected)
	}
}

func TestHandlerFunc(t *testing.T) {
	name := "inline_tool"
	description := "Tool created with HandlerFunc"

	tool := HandlerFunc(name, description, func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		result := fmt.Sprintf("inline: %s", input)
		return calque.Write(res, result)
	})

	// Test metadata
	if tool.Name() != name {
		t.Errorf("Name() = %q, want %q", tool.Name(), name)
	}

	if tool.Description() != description {
		t.Errorf("Description() = %q, want %q", tool.Description(), description)
	}

	// Test execution
	var buf bytes.Buffer
	reader := strings.NewReader("test")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := tool.ServeFlow(req, res)
	if err != nil {
		t.Errorf("ServeFlow() error = %v", err)
		return
	}

	expected := "inline: test"
	if got := buf.String(); got != expected {
		t.Errorf("ServeFlow() output = %q, want %q", got, expected)
	}
}

func TestHandlerFuncWithError(t *testing.T) {
	tool := HandlerFunc("error_tool", "Tool that returns error", func(req *calque.Request, res *calque.Response) error {
		return errors.New("handler function error")
	})

	var buf bytes.Buffer
	reader := strings.NewReader("test")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := tool.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error from HandlerFunc, got nil")
	}

	expectedErr := "handler function error"
	if err.Error() != expectedErr {
		t.Errorf("Error message = %q, want %q", err.Error(), expectedErr)
	}
}

func TestToolInterfaceCompliance(t *testing.T) {
	// Test that all tool constructors return objects that implement Tool interface
	var tools []Tool

	// New
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("input", &jsonschema.Schema{
		Type:        "string",
		Description: "Test input parameter",
	})
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"input"},
	}
	tools = append(tools, New("test", "desc", schema, &mockHandler{}))

	// QuickWithDesc
	tools = append(tools, Simple("test", "desc", func(s string) string { return s }))

	// HandlerFunc
	tools = append(tools, HandlerFunc("test", "desc", func(req *calque.Request, res *calque.Response) error {
		return nil
	}))

	for i, tool := range tools {
		t.Run(fmt.Sprintf("tool_%d", i), func(t *testing.T) {
			// Test that each tool implements Tool interface
			if tool.Name() == "" {
				t.Error("Tool.Name() returned empty string")
			}

			if tool.Description() == "" {
				t.Error("Tool.Description() returned empty string")
			}

			// Test that ServeFlow method exists and can be called
			var buf bytes.Buffer
			reader := strings.NewReader("test")

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := tool.ServeFlow(req, res)
			// We don't check for specific errors here, just that the method exists
			_ = err
		})
	}
}

func TestToolWithIOReadError(t *testing.T) {
	tool := Simple("test", "a simple test func", func(s string) string { return s })

	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	req := calque.NewRequest(context.Background(), errorReader)
	res := calque.NewResponse(&buf)
	err := tool.ServeFlow(req, res)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Expected io.ErrUnexpectedEOF, got %v", err)
	}
}

// errorReader for testing IO errors
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func TestToolWithLargeInput(t *testing.T) {
	largeInput := strings.Repeat("abcdefghij", 1000) // 10KB

	tool := Simple("large_test", "a larger test func", func(s string) string {
		return fmt.Sprintf("length: %d", len(s))
	})

	var buf bytes.Buffer
	reader := strings.NewReader(largeInput)

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := tool.ServeFlow(req, res)
	if err != nil {
		t.Errorf("ServeFlow() with large input error = %v", err)
		return
	}

	expected := "length: 10000"
	if got := buf.String(); got != expected {
		t.Errorf("ServeFlow() output = %q, want %q", got, expected)
	}
}
