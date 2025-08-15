package calque

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRead_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "simple string",
			input:    "hello world",
			expected: "hello world",
			wantErr:  false,
		},
		{
			name:     "multiline string",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
			wantErr:  false,
		},
		{
			name:     "unicode string",
			input:    "🚀 Hello 世界",
			expected: "🚀 Hello 世界",
			wantErr:  false,
		},
		{
			name:     "large string",
			input:    strings.Repeat("abcdefghij", 1000),
			expected: strings.Repeat("abcdefghij", 1000),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(context.Background(), strings.NewReader(tt.input))
			var result string
			
			err := Read(req, &result)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if result != tt.expected {
				t.Errorf("Read() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRead_Bytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
		wantErr  bool
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: []byte{},
			wantErr:  false,
		},
		{
			name:     "simple bytes",
			input:    []byte("hello world"),
			expected: []byte("hello world"),
			wantErr:  false,
		},
		{
			name:     "binary data",
			input:    []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
			expected: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
			wantErr:  false,
		},
		{
			name:     "large bytes",
			input:    bytes.Repeat([]byte("test"), 2500),
			expected: bytes.Repeat([]byte("test"), 2500),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(context.Background(), bytes.NewReader(tt.input))
			var result []byte
			
			err := Read(req, &result)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Read() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWrite_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "simple string",
			input:    "hello world",
			expected: "hello world",
			wantErr:  false,
		},
		{
			name:     "multiline string",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
			wantErr:  false,
		},
		{
			name:     "unicode string",
			input:    "🚀 Hello 世界",
			expected: "🚀 Hello 世界",
			wantErr:  false,
		},
		{
			name:     "large string",
			input:    strings.Repeat("abcdefghij", 1000),
			expected: strings.Repeat("abcdefghij", 1000),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			res := NewResponse(&buf)
			
			err := Write(res, tt.input)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			result := buf.String()
			if result != tt.expected {
				t.Errorf("Write() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestWrite_Bytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
		wantErr  bool
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: []byte{},
			wantErr:  false,
		},
		{
			name:     "simple bytes",
			input:    []byte("hello world"),
			expected: []byte("hello world"),
			wantErr:  false,
		},
		{
			name:     "binary data",
			input:    []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
			expected: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
			wantErr:  false,
		},
		{
			name:     "large bytes",
			input:    bytes.Repeat([]byte("test"), 2500),
			expected: bytes.Repeat([]byte("test"), 2500),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			res := NewResponse(&buf)
			
			err := Write(res, tt.input)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			result := buf.Bytes()
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Write() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReadWrite_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "hello world"},
		{"unicode", "🚀 Hello 世界"},
		{"multiline", "line1\nline2\nline3"},
		{"large", strings.Repeat("abcdefghij", 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test string round trip
			req := NewRequest(context.Background(), strings.NewReader(tt.input))
			var readResult string
			if err := Read(req, &readResult); err != nil {
				t.Fatalf("Read() failed: %v", err)
			}

			var buf bytes.Buffer
			res := NewResponse(&buf)
			if err := Write(res, readResult); err != nil {
				t.Fatalf("Write() failed: %v", err)
			}

			if buf.String() != tt.input {
				t.Errorf("Round trip failed: got %q, want %q", buf.String(), tt.input)
			}
		})
	}
}

// Benchmark tests for small data (like anagram example)
func BenchmarkRead_String_Small(b *testing.B) {
	input := "hello world this is a small test string"
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		req := NewRequest(context.Background(), strings.NewReader(input))
		var result string
		if err := Read(req, &result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRead_Bytes_Small(b *testing.B) {
	input := []byte("hello world this is a small test string")
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		req := NewRequest(context.Background(), bytes.NewReader(input))
		var result []byte
		if err := Read(req, &result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWrite_String_Small(b *testing.B) {
	input := "hello world this is a small test string"
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		res := NewResponse(&buf)
		if err := Write(res, input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWrite_Bytes_Small(b *testing.B) {
	input := []byte("hello world this is a small test string")
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		res := NewResponse(&buf)
		if err := Write(res, input); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark tests for large data
func BenchmarkRead_String_Large(b *testing.B) {
	input := strings.Repeat("hello world this is a test string with some content ", 1000)
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		req := NewRequest(context.Background(), strings.NewReader(input))
		var result string
		if err := Read(req, &result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRead_Bytes_Large(b *testing.B) {
	input := bytes.Repeat([]byte("hello world this is a test string with some content "), 1000)
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		req := NewRequest(context.Background(), bytes.NewReader(input))
		var result []byte
		if err := Read(req, &result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWrite_String_Large(b *testing.B) {
	input := strings.Repeat("hello world this is a test string with some content ", 1000)
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		res := NewResponse(&buf)
		if err := Write(res, input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWrite_Bytes_Large(b *testing.B) {
	input := bytes.Repeat([]byte("hello world this is a test string with some content "), 1000)
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		res := NewResponse(&buf)
		if err := Write(res, input); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark for many small operations (like anagram processing)
func BenchmarkRead_ManySmall(b *testing.B) {
	inputs := []string{
		"cat", "dog", "bird", "fish", "elephant", "tiger", "lion", "bear",
		"apple", "banana", "orange", "grape", "strawberry", "blueberry",
		"car", "bike", "plane", "train", "boat", "ship", "truck", "bus",
	}
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		input := inputs[i%len(inputs)]
		req := NewRequest(context.Background(), strings.NewReader(input))
		var result string
		if err := Read(req, &result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWrite_ManySmall(b *testing.B) {
	inputs := []string{
		"cat", "dog", "bird", "fish", "elephant", "tiger", "lion", "bear",
		"apple", "banana", "orange", "grape", "strawberry", "blueberry",
		"car", "bike", "plane", "train", "boat", "ship", "truck", "bus",
	}
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		input := inputs[i%len(inputs)]
		var buf bytes.Buffer
		res := NewResponse(&buf)
		if err := Write(res, input); err != nil {
			b.Fatal(err)
		}
	}
}