package observability

import (
	"context"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestTracingHandler(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryTracerProvider()

	// Create a simple handler
	innerHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		return calque.Write(res, strings.ToUpper(input))
	})

	handler := TracingHandler(provider, "test-operation", innerHandler)

	// Execute the handler
	req := calque.NewRequest(context.Background(), strings.NewReader("hello"))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if buf.String() != "HELLO" {
		t.Errorf("Expected HELLO, got %s", buf.String())
	}

	// Check span was recorded
	spans := provider.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "test-operation" {
		t.Errorf("Expected span name 'test-operation', got '%s'", span.Name)
	}

	if span.Status != SpanStatusOK {
		t.Errorf("Expected SpanStatusOK, got %v", span.Status)
	}
}

func TestTracingHandlerWithError(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryTracerProvider()

	// Create a handler that returns an error
	innerHandler := calque.HandlerFunc(func(req *calque.Request, _ *calque.Response) error {
		return calque.NewErr(req.Context, "test error")
	})

	handler := TracingHandler(provider, "error-operation", innerHandler)

	req := calque.NewRequest(context.Background(), strings.NewReader("input"))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check span was recorded with error status
	spans := provider.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status != SpanStatusError {
		t.Errorf("Expected SpanStatusError, got %v", span.Status)
	}

	if span.Error == nil {
		t.Error("Expected error to be recorded in span")
	}
}

func TestTracingHandlerWithRecordInput(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryTracerProvider()

	innerHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		return calque.Write(res, input)
	})

	handler := TracingHandler(provider, "input-recording", innerHandler, WithRecordInput())

	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	spans := provider.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if input, ok := span.Attributes["input"]; !ok || input != "test input" {
		t.Errorf("Expected input attribute 'test input', got '%v'", input)
	}
}

func TestTracingHandlerWithRecordOutput(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryTracerProvider()

	innerHandler := calque.HandlerFunc(func(_ *calque.Request, res *calque.Response) error {
		return calque.Write(res, "output data")
	})

	handler := TracingHandler(provider, "output-recording", innerHandler, WithRecordOutput())

	req := calque.NewRequest(context.Background(), strings.NewReader("input"))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	spans := provider.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if output, ok := span.Attributes["output"]; !ok || output != "output data" {
		t.Errorf("Expected output attribute 'output data', got '%v'", output)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"hello", 0, "hello"}, // 0 means no limit
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestTracingConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultTracingConfig()

	if cfg.RecordInput {
		t.Error("Expected RecordInput to be false by default")
	}

	if cfg.RecordOutput {
		t.Error("Expected RecordOutput to be false by default")
	}

	if cfg.MaxAttributeLength != 1024 {
		t.Errorf("Expected MaxAttributeLength 1024, got %d", cfg.MaxAttributeLength)
	}
}
