package convert

import (
	"io"
	"strings"
	"testing"
)

func TestJson(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{"map data", map[string]any{"key": "value"}},
		{"slice data", []any{1, 2, 3}},
		{"string data", `{"test": "value"}`},
		{"byte data", []byte(`{"test": "value"}`)},
		{"struct data", struct{ Name string }{"test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := ToJson(tt.data)
			if converter == nil {
				t.Fatal("Json() returned nil")
			}
			// Just verify the converter was created with some data
			if converter.data == nil && tt.data != nil {
				t.Error("Json() data is nil when input was not nil")
			}
		})
	}
}

func TestJsonOutput(t *testing.T) {
	var target map[string]any
	converter := FromJson(&target)

	if converter == nil {
		t.Fatal("JsonOutput() returned nil")
	}
	if converter.target != &target {
		t.Error("JsonOutput() target not set correctly")
	}
}

func TestJsonInputConverter_ToReader(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    string
		wantErr bool
	}{
		{
			name: "map[string]any",
			data: map[string]any{"name": "test", "value": 42},
			want: `{"name":"test","value":42}`,
		},
		{
			name: "slice",
			data: []any{1, "two", 3.0},
			want: `[1,"two",3]`,
		},
		{
			name: "valid JSON string",
			data: `{"valid": "json"}`,
			want: `{"valid": "json"}`,
		},
		{
			name: "valid JSON bytes",
			data: []byte(`{"valid": "json"}`),
			want: `{"valid": "json"}`,
		},
		{
			name: "struct",
			data: struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{"test", 123},
			want: `{"name":"test","value":123}`,
		},
		{
			name:    "invalid JSON string",
			data:    `{"invalid": json}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON bytes",
			data:    []byte(`{"invalid": json}`),
			wantErr: true,
		},
		{
			name:    "unmarshalable data",
			data:    make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &jsonInputConverter{data: tt.data}
			reader, err := j.ToReader()

			if tt.wantErr {
				if err == nil {
					t.Error("ToReader() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ToReader() error = %v", err)
				return
			}

			if reader == nil {
				t.Fatal("ToReader() returned nil reader")
			}

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("Failed to read from reader: %v", err)
				return
			}

			got := string(data)
			if got != tt.want {
				t.Errorf("ToReader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJsonOutputConverter_FromReader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		target   any
		expected any
		wantErr  bool
	}{
		{
			name:     "unmarshal to map",
			input:    `{"name":"test","value":42}`,
			target:   &map[string]any{},
			expected: map[string]any{"name": "test", "value": float64(42)},
		},
		{
			name:     "unmarshal to slice",
			input:    `[1,2,3]`,
			target:   &[]any{},
			expected: []any{float64(1), float64(2), float64(3)},
		},
		{
			name:  "unmarshal to struct",
			input: `{"name":"test","value":42}`,
			target: &struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{},
			expected: struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{"test", 42},
		},
		{
			name:    "invalid JSON",
			input:   `{"invalid": json}`,
			target:  &map[string]any{},
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   ``,
			target:  &map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &jsonOutputConverter{target: tt.target}
			reader := strings.NewReader(tt.input)

			err := j.FromReader(reader)

			if tt.wantErr {
				if err == nil {
					t.Error("FromReader() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("FromReader() error = %v", err)
				return
			}

			// Check the result by comparing the dereferenced target
			switch target := tt.target.(type) {
			case *map[string]any:
				expected := tt.expected.(map[string]any)
				if len(*target) != len(expected) {
					t.Errorf("FromReader() result length = %d, want %d", len(*target), len(expected))
					return
				}
				for k, v := range expected {
					if (*target)[k] != v {
						t.Errorf("FromReader() result[%s] = %v, want %v", k, (*target)[k], v)
					}
				}
			case *[]any:
				expected := tt.expected.([]any)
				if len(*target) != len(expected) {
					t.Errorf("FromReader() result length = %d, want %d", len(*target), len(expected))
					return
				}
				for i, v := range expected {
					if (*target)[i] != v {
						t.Errorf("FromReader() result[%d] = %v, want %v", i, (*target)[i], v)
					}
				}
			default:
				// For structs, compare directly
				targetValue := *target.(*struct {
					Name  string `json:"name"`
					Value int    `json:"value"`
				})
				expected := tt.expected.(struct {
					Name  string `json:"name"`
					Value int    `json:"value"`
				})
				if targetValue != expected {
					t.Errorf("FromReader() result = %+v, want %+v", targetValue, expected)
				}
			}
		})
	}
}

func TestJsonInputConverter_ToReader_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    string
		wantErr bool
	}{
		{
			name: "nil data",
			data: nil,
			want: "null",
		},
		{
			name: "empty map",
			data: map[string]any{},
			want: "{}",
		},
		{
			name: "empty slice",
			data: []any{},
			want: "[]",
		},
		{
			name: "nested structure",
			data: map[string]any{
				"nested": map[string]any{
					"value": []any{1, 2, 3},
				},
			},
			want: `{"nested":{"value":[1,2,3]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &jsonInputConverter{data: tt.data}
			reader, err := j.ToReader()

			if tt.wantErr {
				if err == nil {
					t.Error("ToReader() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ToReader() error = %v", err)
				return
			}

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("Failed to read from reader: %v", err)
				return
			}

			got := string(data)
			if got != tt.want {
				t.Errorf("ToReader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJsonOutputConverter_FromReader_ErrorCases(t *testing.T) {
	tests := []struct {
		name   string
		reader io.Reader
		target any
	}{
		{
			name:   "reader error",
			reader: &failingReader{},
			target: &map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &jsonOutputConverter{target: tt.target}
			err := j.FromReader(tt.reader)

			if err == nil {
				t.Error("FromReader() expected error, got nil")
			}
		})
	}
}

type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func TestJsonIntegration(t *testing.T) {
	t.Run("input to output roundtrip", func(t *testing.T) {
		original := map[string]any{
			"name":   "test",
			"value":  42,
			"nested": map[string]any{"key": "value"},
			"array":  []any{1, 2, 3},
		}

		// Convert to reader
		inputConverter := ToJson(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result map[string]any
		outputConverter := FromJson(&result)
		err = outputConverter.FromReader(reader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		// Verify roundtrip (note: numbers become float64 in JSON)
		if result["name"] != "test" {
			t.Errorf("name = %v, want test", result["name"])
		}
		if result["value"] != float64(42) {
			t.Errorf("value = %v, want 42", result["value"])
		}

		nested := result["nested"].(map[string]any)
		if nested["key"] != "value" {
			t.Errorf("nested.key = %v, want value", nested["key"])
		}

		array := result["array"].([]any)
		expected := []any{float64(1), float64(2), float64(3)}
		if len(array) != len(expected) {
			t.Errorf("array length = %d, want %d", len(array), len(expected))
		}
		for i, v := range expected {
			if array[i] != v {
				t.Errorf("array[%d] = %v, want %v", i, array[i], v)
			}
		}
	})
}
