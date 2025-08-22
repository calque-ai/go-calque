package text

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestTransform(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fn       func(string) string
		expected string
		wantErr  bool
	}{
		{
			name:     "uppercase transformation",
			input:    "hello world",
			fn:       strings.ToUpper,
			expected: "HELLO WORLD",
			wantErr:  false,
		},
		{
			name:     "lowercase transformation",
			input:    "HELLO WORLD",
			fn:       strings.ToLower,
			expected: "hello world",
			wantErr:  false,
		},
		{
			name:  "custom reverse transformation",
			input: "hello",
			fn: func(s string) string {
				runes := []rune(s)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return string(runes)
			},
			expected: "olleh",
			wantErr:  false,
		},
		{
			name:     "empty input",
			input:    "",
			fn:       strings.ToUpper,
			expected: "",
			wantErr:  false,
		},
		{
			name:  "prefix addition",
			input: "test",
			fn: func(s string) string {
				return "prefix: " + s
			},
			expected: "prefix: test",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Transform(tt.fn)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Transform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Transform() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBranch(t *testing.T) {
	// Mock handlers for testing
	mockIfHandler := calque.HandlerFunc(func(_ *calque.Request, res *calque.Response) error {
		return calque.Write(res, "if-handler")
	})

	mockElseHandler := calque.HandlerFunc(func(_ *calque.Request, res *calque.Response) error {
		return calque.Write(res, "else-handler")
	})

	errorHandler := calque.HandlerFunc(func(_ *calque.Request, res *calque.Response) error {
		return errors.New("handler error")
	})

	tests := []struct {
		name        string
		input       string
		condition   func(string) bool
		ifHandler   calque.Handler
		elseHandler calque.Handler
		expected    string
		wantErr     bool
	}{
		{
			name:        "condition true - uses if handler",
			input:       "json data",
			condition:   func(s string) bool { return strings.Contains(s, "json") },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "if-handler",
			wantErr:     false,
		},
		{
			name:        "condition false - uses else handler",
			input:       "text data",
			condition:   func(s string) bool { return strings.Contains(s, "json") },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "else-handler",
			wantErr:     false,
		},
		{
			name:        "empty input - condition false",
			input:       "",
			condition:   func(s string) bool { return len(s) > 0 },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "else-handler",
			wantErr:     false,
		},
		{
			name:        "prefix check - condition true",
			input:       "{\"key\": \"value\"}",
			condition:   func(s string) bool { return strings.HasPrefix(s, "{") },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "if-handler",
			wantErr:     false,
		},
		{
			name:        "if handler error",
			input:       "trigger condition",
			condition:   func(_ string) bool { return true },
			ifHandler:   errorHandler,
			elseHandler: mockElseHandler,
			expected:    "",
			wantErr:     true,
		},
		{
			name:        "else handler error",
			input:       "trigger condition",
			condition:   func(_ string) bool { return false },
			ifHandler:   mockIfHandler,
			elseHandler: errorHandler,
			expected:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Branch(tt.condition, tt.ifHandler, tt.elseHandler)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Branch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Branch() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	// Mock handler for testing
	mockHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		err := calque.Read(req, &input)
		if err != nil {
			return err
		}
		return calque.Write(res, "processed: "+input)
	})

	errorHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		return errors.New("processing error")
	})

	tests := []struct {
		name      string
		input     string
		condition func(string) bool
		handler   calque.Handler
		expected  string
		wantErr   bool
	}{
		{
			name:      "condition true - processes through handler",
			input:     "valid json",
			condition: func(s string) bool { return strings.Contains(s, "json") },
			handler:   mockHandler,
			expected:  "processed: valid json",
			wantErr:   false,
		},
		{
			name:      "condition false - passes through unchanged",
			input:     "plain text",
			condition: func(s string) bool { return strings.Contains(s, "json") },
			handler:   mockHandler,
			expected:  "plain text",
			wantErr:   false,
		},
		{
			name:      "empty input - condition false",
			input:     "",
			condition: func(s string) bool { return len(s) > 5 },
			handler:   mockHandler,
			expected:  "",
			wantErr:   false,
		},
		{
			name:      "empty input - condition true",
			input:     "",
			condition: func(s string) bool { return len(s) == 0 },
			handler:   mockHandler,
			expected:  "processed: ",
			wantErr:   false,
		},
		{
			name:      "length check - condition true",
			input:     "long enough string",
			condition: func(s string) bool { return len(s) > 10 },
			handler:   mockHandler,
			expected:  "processed: long enough string",
			wantErr:   false,
		},
		{
			name:      "handler error when condition true",
			input:     "trigger processing",
			condition: func(s string) bool { return true },
			handler:   errorHandler,
			expected:  "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Filter(tt.condition, tt.handler)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Filter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Filter() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLineProcessor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fn       func(string) string
		expected string
		wantErr  bool
	}{
		{
			name:     "single line uppercase",
			input:    "hello world",
			fn:       strings.ToUpper,
			expected: "HELLO WORLD\n",
			wantErr:  false,
		},
		{
			name:     "multiple lines uppercase",
			input:    "line one\nline two\nline three",
			fn:       strings.ToUpper,
			expected: "LINE ONE\nLINE TWO\nLINE THREE\n",
			wantErr:  false,
		},
		{
			name:     "empty input",
			input:    "",
			fn:       strings.ToUpper,
			expected: "",
			wantErr:  false,
		},
		{
			name:     "single empty line",
			input:    "\n",
			fn:       strings.ToUpper,
			expected: "\n",
			wantErr:  false,
		},
		{
			name:     "mixed empty and non-empty lines",
			input:    "first\n\nthird\n\nfifth",
			fn:       strings.ToUpper,
			expected: "FIRST\n\nTHIRD\n\nFIFTH\n",
			wantErr:  false,
		},
		{
			name:  "add line numbers",
			input: "alpha\nbeta\ngamma",
			fn: func(line string) string {
				return fmt.Sprintf("-> %s", line)
			},
			expected: "-> alpha\n-> beta\n-> gamma\n",
			wantErr:  false,
		},
		{
			name:  "prefix each line",
			input: "one\ntwo\nthree",
			fn: func(line string) string {
				return "prefix: " + line
			},
			expected: "prefix: one\nprefix: two\nprefix: three\n",
			wantErr:  false,
		},
		{
			name:  "trim and uppercase",
			input: "  spaced  \n\ttabbed\t\n  mixed \t ",
			fn: func(line string) string {
				return strings.ToUpper(strings.TrimSpace(line))
			},
			expected: "SPACED\nTABBED\nMIXED\n",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := LineProcessor(tt.fn)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("LineProcessor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("LineProcessor() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Benchmark data
var smallDataset = strings.Repeat("hello\nworld\ntest\ndata\n", 7)                               // 28 lines, similar to anagram example
var largeDataset = strings.Repeat("this is a longer line with more content to process\n", 10000) // 10k lines

// Benchmarks for LineProcessor with small datasets (like anagram example)
func BenchmarkLineProcessor_Small(b *testing.B) {
	processor := LineProcessor(strings.ToUpper)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		reader := strings.NewReader(smallDataset)

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)

		if err := processor.ServeFlow(req, res); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark LineProcessor with large datasets (where it should excel)
func BenchmarkLineProcessor_Large(b *testing.B) {
	processor := LineProcessor(strings.ToUpper)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		reader := strings.NewReader(largeDataset)

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)

		if err := processor.ServeFlow(req, res); err != nil {
			b.Fatal(err)
		}
	}
}

// Comparison: Transform vs LineProcessor for small data
func BenchmarkTransform_Small(b *testing.B) {
	transformer := Transform(strings.ToUpper)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		reader := strings.NewReader(smallDataset)

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)

		if err := transformer.ServeFlow(req, res); err != nil {
			b.Fatal(err)
		}
	}
}

// Comparison: Transform vs LineProcessor for large data
func BenchmarkTransform_Large(b *testing.B) {
	transformer := Transform(strings.ToUpper)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		reader := strings.NewReader(largeDataset)

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)

		if err := transformer.ServeFlow(req, res); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark LineProcessor with more complex processing
func BenchmarkLineProcessor_ComplexProcessing(b *testing.B) {
	processor := LineProcessor(func(line string) string {
		return fmt.Sprintf("[%d] %s", len(line), strings.ToUpper(strings.TrimSpace(line)))
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		reader := strings.NewReader(smallDataset)

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)

		if err := processor.ServeFlow(req, res); err != nil {
			b.Fatal(err)
		}
	}
}
