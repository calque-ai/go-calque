package ai

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

func TestIsMultimodalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid multimodal JSON",
			input:    `{"parts": [{"type": "text", "text": "Hello"}]}`,
			expected: true,
		},
		{
			name:     "valid multimodal JSON with image",
			input:    `{"parts": [{"type": "image", "data": "base64data", "mime_type": "image/jpeg"}]}`,
			expected: true,
		},
		{
			name:     "valid multimodal JSON with mixed content",
			input:    `{"parts": [{"type": "text", "text": "What's in this image?"}, {"type": "image", "data": "base64data", "mime_type": "image/jpeg"}]}`,
			expected: true,
		},
		{
			name:     "JSON with parts but no type",
			input:    `{"parts": [{"text": "Hello"}]}`,
			expected: false,
		},
		{
			name:     "JSON with type but no parts",
			input:    `{"type": "text", "text": "Hello"}`,
			expected: false,
		},
		{
			name:     "regular text",
			input:    "Hello, how are you?",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "invalid JSON",
			input:    `{"invalid": json}`,
			expected: false,
		},
		{
			name:     "valid JSON but not multimodal",
			input:    `{"message": "Hello", "user": "Alice"}`,
			expected: false,
		},
		{
			name:     "user JSON with parts field (false positive test)",
			input:    `{"parts": ["engine", "transmission"], "type": "car"}`,
			expected: true, // This is expected - it has both parts and type, so it passes the fast check
		},
		{
			name:     "user JSON with parts but no type (should be filtered out)",
			input:    `{"parts": ["engine", "transmission"], "model": "Tesla"}`,
			expected: false,
		},
		{
			name:     "nested JSON with parts and type",
			input:    `{"data": {"parts": [{"type": "text"}]}}`,
			expected: true,
		},
		{
			name:     "malformed multimodal JSON",
			input:    `{"parts": [{"type": "text", "text": "Hello"`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMultimodalJSON([]byte(tt.input))
			if result != tt.expected {
				t.Errorf("isMultimodalJSON() = %v, want %v for input: %s", result, tt.expected, tt.input)
			}
		})
	}
}

func TestClassifyInput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		opts         *AgentOptions
		expectedType InputType
		expectError  bool
	}{
		{
			name:         "simple text input",
			input:        "Hello, how are you?",
			opts:         nil,
			expectedType: TextInput,
			expectError:  false,
		},
		{
			name:         "empty text input",
			input:        "",
			opts:         nil,
			expectedType: TextInput,
			expectError:  false,
		},
		{
			name:         "JSON text input (not multimodal)",
			input:        `{"message": "Hello", "user": "Alice"}`,
			opts:         nil,
			expectedType: TextInput,
			expectError:  false,
		},
		{
			name:  "streaming multimodal input",
			input: "Hello",
			opts: &AgentOptions{
				MultimodalData: &MultimodalInput{
					Parts: []ContentPart{
						{Type: "text", Text: "Hello"},
						{Type: "image", Data: []byte("fake-image-data"), MimeType: "image/jpeg"},
					},
				},
			},
			expectedType: MultimodalStreamingInput,
			expectError:  false,
		},
		{
			name:         "valid multimodal JSON with text only",
			input:        `{"parts": [{"type": "text", "text": "Hello, world!"}]}`,
			opts:         nil,
			expectedType: TextInput, // No embedded data, should classify as text
			expectError:  false,
		},
		{
			name:         "valid multimodal JSON with embedded data",
			input:        `{"parts": [{"type": "text", "text": "What's in this image?"}, {"type": "image", "data": "aGVsbG8=", "mime_type": "image/jpeg"}]}`,
			opts:         nil,
			expectedType: MultimodalJSONInput,
			expectError:  false,
		},
		{
			name:         "valid multimodal JSON with audio data",
			input:        `{"parts": [{"type": "audio", "data": "aGVsbG8=", "mime_type": "audio/wav"}]}`,
			opts:         nil,
			expectedType: MultimodalJSONInput,
			expectError:  false,
		},
		{
			name:         "valid multimodal JSON with video data",
			input:        `{"parts": [{"type": "video", "data": "aGVsbG8=", "mime_type": "video/mp4"}]}`,
			opts:         nil,
			expectedType: MultimodalJSONInput,
			expectError:  false,
		},
		{
			name:         "malformed multimodal JSON",
			input:        `{"parts": [{"type": "text", "text": "Hello"`,
			opts:         nil,
			expectedType: TextInput, // Should fallback to text on parse error
			expectError:  false,
		},
		{
			name:         "user JSON with parts and type (false positive)",
			input:        `{"parts": ["engine", "transmission"], "type": "car"}`,
			opts:         nil,
			expectedType: TextInput, // Should fallback to text after JSON parsing fails
			expectError:  false,
		},
		{
			name:         "multimodal JSON with empty parts",
			input:        `{"parts": []}`,
			opts:         nil,
			expectedType: TextInput, // Empty parts should classify as text
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			req := calque.NewRequest(context.Background(), reader)

			result, err := ClassifyInput(req, tt.opts)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ClassifyInput() error = %v", err)
				return
			}

			if result.Type != tt.expectedType {
				t.Errorf("ClassifyInput() type = %v, want %v", result.Type, tt.expectedType)
			}

			// Verify raw bytes are preserved
			if string(result.RawBytes) != tt.input {
				t.Errorf("ClassifyInput() rawBytes = %q, want %q", string(result.RawBytes), tt.input)
			}

			// Check specific fields based on type
			switch result.Type {
			case TextInput:
				if result.Text != tt.input {
					t.Errorf("ClassifyInput() text = %q, want %q", result.Text, tt.input)
				}
				if result.Multimodal != nil {
					t.Error("ClassifyInput() multimodal should be nil for text input")
				}
			case MultimodalJSONInput, MultimodalStreamingInput:
				if result.Multimodal == nil {
					t.Error("ClassifyInput() multimodal should not be nil for multimodal input")
				}
				if result.Text != "" {
					t.Error("ClassifyInput() text should be empty for multimodal input")
				}
			}
		})
	}
}

func TestClassifyInputWithIOError(t *testing.T) {
	// Test IO error handling
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	req := calque.NewRequest(context.Background(), errorReader)

	_, err := ClassifyInput(req, nil)
	if err == nil {
		t.Error("Expected IO error, got none")
	}
	// The error will be wrapped, so check if it contains the original error
	if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Errorf("ClassifyInput() error = %v, want error containing 'unexpected EOF'", err)
	}
}

func TestClassifyInputStreamingMultimodalPriority(t *testing.T) {
	// Test that streaming multimodal takes priority over JSON multimodal
	input := `{"parts": [{"type": "text", "text": "This is JSON multimodal"}]}`
	reader := strings.NewReader(input)
	req := calque.NewRequest(context.Background(), reader)

	opts := &AgentOptions{
		MultimodalData: &MultimodalInput{
			Parts: []ContentPart{
				{Type: "text", Text: "This is streaming multimodal"},
			},
		},
	}

	result, err := ClassifyInput(req, opts)
	if err != nil {
		t.Errorf("ClassifyInput() error = %v", err)
		return
	}

	if result.Type != MultimodalStreamingInput {
		t.Errorf("ClassifyInput() type = %v, want %v", result.Type, MultimodalStreamingInput)
	}

	// Should use streaming multimodal data, not parse JSON
	if result.Multimodal != opts.MultimodalData {
		t.Error("ClassifyInput() should use streaming multimodal data when both are present")
	}
}

func TestGetSchema(t *testing.T) {
	tests := []struct {
		name           string
		opts           *AgentOptions
		expectedSchema *ResponseFormat
		expectNil      bool
	}{
		{
			name:      "nil options",
			opts:      nil,
			expectNil: true,
		},
		{
			name:      "options without schema",
			opts:      &AgentOptions{},
			expectNil: true,
		},
		{
			name: "options with json_object schema",
			opts: &AgentOptions{
				Schema: &ResponseFormat{
					Type: "json_object",
				},
			},
			expectedSchema: &ResponseFormat{
				Type: "json_object",
			},
			expectNil: false,
		},
		{
			name: "options with json_schema",
			opts: &AgentOptions{
				Schema: &ResponseFormat{
					Type:   "json_schema",
					Schema: nil,
				},
			},
			expectedSchema: &ResponseFormat{
				Type:   "json_schema",
				Schema: nil,
			},
			expectNil: false,
		},
		{
			name: "options with other fields but no schema",
			opts: &AgentOptions{
				Tools: []tools.Tool{
					tools.Simple("test", "test tool", func(s string) string { return s }),
				},
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSchema(tt.opts)

			if tt.expectNil {
				if result != nil {
					t.Errorf("GetSchema() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Error("GetSchema() should not be nil")
				return
			}

			if result.Type != tt.expectedSchema.Type {
				t.Errorf("GetSchema() Type = %v, want %v", result.Type, tt.expectedSchema.Type)
			}

			// For pointer equality check when we expect the same object
			if result != tt.opts.Schema {
				t.Error("GetSchema() should return the exact schema object from opts")
			}
		})
	}
}

func TestGetTools(t *testing.T) {
	tool1 := tools.Simple("tool1", "First tool", func(s string) string { return s })
	tool2 := tools.Simple("tool2", "Second tool", func(s string) string { return s })

	tests := []struct {
		name        string
		opts        *AgentOptions
		expectedLen int
		expectNil   bool
	}{
		{
			name:      "nil options",
			opts:      nil,
			expectNil: true,
		},
		{
			name:      "options without tools",
			opts:      &AgentOptions{},
			expectNil: true,
		},
		{
			name: "options with single tool",
			opts: &AgentOptions{
				Tools: []tools.Tool{tool1},
			},
			expectedLen: 1,
			expectNil:   false,
		},
		{
			name: "options with multiple tools",
			opts: &AgentOptions{
				Tools: []tools.Tool{tool1, tool2},
			},
			expectedLen: 2,
			expectNil:   false,
		},
		{
			name: "options with empty tools slice",
			opts: &AgentOptions{
				Tools: []tools.Tool{},
			},
			expectedLen: 0,
			expectNil:   false,
		},
		{
			name: "options with other fields but no tools",
			opts: &AgentOptions{
				Schema: &ResponseFormat{Type: "json_object"},
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTools(tt.opts)

			if tt.expectNil {
				if result != nil {
					t.Errorf("GetTools() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Error("GetTools() should not be nil")
				return
			}

			if len(result) != tt.expectedLen {
				t.Errorf("GetTools() length = %v, want %v", len(result), tt.expectedLen)
			}

			// Verify it's the exact same slice (pointer equality)
			if tt.opts != nil && len(tt.opts.Tools) > 0 {
				if &result[0] != &tt.opts.Tools[0] {
					t.Error("GetTools() should return the exact tools slice from opts")
				}
			}
		})
	}
}

func TestGetSchemaAndGetToolsTogether(t *testing.T) {
	// Test that both functions work correctly on the same options object
	tool := tools.Simple("test", "test", func(s string) string { return s })
	schema := &ResponseFormat{Type: "json_object"}

	opts := &AgentOptions{
		Tools:  []tools.Tool{tool},
		Schema: schema,
	}

	resultTools := GetTools(opts)
	resultSchema := GetSchema(opts)

	if resultTools == nil {
		t.Error("GetTools() should return tools")
	} else if len(resultTools) != 1 {
		t.Errorf("GetTools() length = %v, want 1", len(resultTools))
	}

	if resultSchema == nil {
		t.Error("GetSchema() should return schema")
	} else if resultSchema.Type != "json_object" {
		t.Errorf("GetSchema() Type = %v, want json_object", resultSchema.Type)
	}
}
