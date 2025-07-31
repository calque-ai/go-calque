package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

func TestEvaluateExpression(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected float64
		hasError bool
	}{
		{
			name:     "addition",
			expr:     "10+5",
			expected: 15.0,
		},
		{
			name:     "subtraction",
			expr:     "10-5",
			expected: 5.0,
		},
		{
			name:     "multiplication",
			expr:     "10*5",
			expected: 50.0,
		},
		{
			name:     "division",
			expr:     "10/5",
			expected: 2.0,
		},
		{
			name:     "single number",
			expr:     "42",
			expected: 42.0,
		},
		{
			name:     "division by zero",
			expr:     "10/0",
			hasError: true,
		},
		{
			name:     "invalid expression",
			expr:     "10++5",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluateExpression(tt.expr)

			if tt.hasError {
				if err == nil {
					t.Errorf("evaluateExpression(%q) expected error, got nil", tt.expr)
				}
				return
			}

			if err != nil {
				t.Errorf("evaluateExpression(%q) unexpected error: %v", tt.expr, err)
				return
			}

			if result != tt.expected {
				t.Errorf("evaluateExpression(%q) = %f, want %f", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestCalculatorTool(t *testing.T) {
	calculator := tools.Simple("calculator", "Math Calculator", func(expr string) string {
		_, err := evaluateExpression(expr)
		if err != nil {
			return "Error: " + err.Error()
		}
		return "42.00"
	})

	// Test the tool interface
	if calculator.Name() != "calculator" {
		t.Errorf("calculator.Name() = %q, want %q", calculator.Name(), "calculator")
	}

	// Test tool execution
	ctx := context.Background()
	var result string
	err := core.New().Use(tools.Registry(calculator)).Use(
		core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
			tool := tools.GetTool(ctx, "calculator")
			if tool == nil {
				return fmt.Errorf("calculator tool not found")
			}
			return tool.ServeFlow(ctx, strings.NewReader("2+2"), w)
		}),
	).Run(ctx, "", &result)

	if err != nil {
		t.Errorf("Calculator tool execution error: %v", err)
		return
	}

	if result != "42.00" {
		t.Errorf("Calculator tool result = %q, want %q", result, "42.00")
	}
}

func TestTextProcessorHandler(t *testing.T) {
	handler := createTextProcessorHandler()

	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:  "simple text",
			input: "Hello world",
			contains: []string{
				"Text Analysis:",
				"Words: 2",
				"Characters: 11",
				"Sentences: 1",
				"Hello world",
			},
		},
		{
			name:  "text with punctuation",
			input: "Hello world! How are you? I'm fine.",
			contains: []string{
				"Text Analysis:",
				"Words: 7",
				"Characters: 35",
				"Sentences: 3",
			},
		},
		{
			name:     "empty text",
			input:    "",
			contains: []string{"Words: 0", "Characters: 0", "Sentences: 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			err := core.New().Use(handler).Run(context.Background(), tt.input, &result)
			if err != nil {
				t.Errorf("TextProcessor error: %v", err)
				return
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("TextProcessor result missing %q, got: %s", expected, result)
				}
			}
		})
	}
}

func TestToolIntegration(t *testing.T) {
	// Test that all our tools work together in a pipeline
	calculator := tools.Simple("calculator", "Math Calculator", func(expr string) string {
		result, err := evaluateExpression(expr)
		if err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("%.2f", result)
	})

	textProcessor := tools.New("text_processor", "Process text", createTextProcessorHandler())

	// Test registry and retrieval
	ctx := context.Background()
	var result string

	pipeline := core.New().
		Use(tools.Registry(calculator, textProcessor)).
		Use(core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
			// Verify tools are available
			if !tools.HasTool(ctx, "calculator") {
				return fmt.Errorf("calculator tool not found in context")
			}
			if !tools.HasTool(ctx, "text_processor") {
				return fmt.Errorf("text_processor tool not found in context")
			}

			// Test tool execution
			calc := tools.GetTool(ctx, "calculator")
			var calcResult strings.Builder
			err := calc.ServeFlow(ctx, strings.NewReader("10*5"), &calcResult)
			if err != nil {
				return err
			}

			return core.Write(w, "Calculator result: "+calcResult.String())
		}))

	err := pipeline.Run(ctx, "test", &result)
	if err != nil {
		t.Errorf("Tool integration error: %v", err)
		return
	}

	if !strings.Contains(result, "Calculator result: 50.00") {
		t.Errorf("Tool integration result = %q, expected to contain 'Calculator result: 50.00'", result)
	}
}
