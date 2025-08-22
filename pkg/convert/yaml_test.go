package convert

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestYaml(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{"map[string]any data", map[string]any{"key": "value"}},
		{"map[any]any data", map[any]any{"key": "value", 123: "number"}},
		{"slice data", []any{1, 2, 3}},
		{"string data", `key: value`},
		{"byte data", []byte(`key: value`)},
		{"io.Reader data", strings.NewReader(`reader: test`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := ToYaml(tt.data)
			if converter == nil {
				t.Fatal("Yaml() returned nil")
			}
			// Just verify the converter was created with some data
			if converter.data == nil && tt.data != nil {
				t.Error("Yaml() data is nil when input was not nil")
			}
		})
	}
}

func TestYamlOutput(t *testing.T) {
	var target map[string]any
	converter := FromYaml(&target)

	if converter == nil {
		t.Fatal("YamlOutput() returned nil")
	}
	if converter.target != &target {
		t.Error("YamlOutput() target not set correctly")
	}
}

func TestYamlInputConverter_ToReader(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    string
		wantErr bool
	}{
		{
			name: "map[string]any",
			data: map[string]any{"name": "test", "value": 42},
			want: "name: test\nvalue: 42\n",
		},
		{
			name: "map[any]any",
			data: map[any]any{"name": "test", 123: "number"},
			want: "\"123\": number\nname: test\n",
		},
		{
			name: "slice",
			data: []any{1, "two", 3.0},
			want: "- 1\n- two\n- 3.0\n",
		},
		{
			name: "valid YAML string",
			data: `name: test
value: 42`,
			want: `name: test
value: 42`,
		},
		{
			name: "valid YAML bytes",
			data: []byte(`name: test
value: 42`),
			want: `name: test
value: 42`,
		},
		{
			name:    "invalid YAML string",
			data:    `name: test\n  invalid: [unclosed`,
			wantErr: true,
		},
		{
			name:    "invalid YAML bytes",
			data:    []byte(`name: test\n  invalid: [unclosed`),
			wantErr: true,
		},
		{
			name:    "unsupported type",
			data:    complex(1, 2),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y := &yamlInputConverter{data: tt.data}
			reader, err := y.ToReader()

			if tt.wantErr {
				handleYamlExpectedError(t, err, reader)
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
				t.Errorf("ToReader() = %q, want %q", got, tt.want)
			}
		})
	}
}

// handleYamlExpectedError handles expected error cases in YAML tests
func handleYamlExpectedError(t *testing.T, err error, reader io.Reader) {
	if err == nil {
		// For streaming cases, error might occur during read
		if reader != nil {
			_, readErr := io.ReadAll(reader)
			if readErr == nil {
				t.Error("ToReader() expected error, got nil")
			}
		} else {
			t.Error("ToReader() expected error, got nil")
		}
	}
}

func TestYamlOutputConverter_FromReader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		target   any
		expected any
		wantErr  bool
	}{
		{
			name: "unmarshal to map[string]any",
			input: `name: test
value: 42`,
			target:   &map[string]any{},
			expected: map[string]any{"name": "test", "value": uint64(42)},
		},
		{
			name: "unmarshal to slice",
			input: `- 1
- 2
- 3`,
			target:   &[]any{},
			expected: []any{uint64(1), uint64(2), uint64(3)},
		},
		{
			name: "unmarshal to struct",
			input: `name: test
value: 42`,
			target: &struct {
				Name  string `yaml:"name"`
				Value int    `yaml:"value"`
			}{},
			expected: struct {
				Name  string `yaml:"name"`
				Value int    `yaml:"value"`
			}{"test", 42},
		},
		{
			name: "YAML with comments",
			input: `# This is a comment
name: test
value: 42`,
			target:   &map[string]any{},
			expected: map[string]any{"name": "test", "value": uint64(42)},
		},
		{
			name: "nested YAML",
			input: `person:
  name: test
  details:
    age: 30
    city: NYC`,
			target: &map[string]any{},
			expected: map[string]any{
				"person": map[string]any{
					"name": "test",
					"details": map[string]any{
						"age":  uint64(30),
						"city": "NYC",
					},
				},
			},
		},
		{
			name:    "invalid YAML",
			input:   `name: test\n  invalid: [unclosed`,
			target:  &map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y := &yamlOutputConverter{target: tt.target}
			reader := strings.NewReader(tt.input)

			err := y.FromReader(reader)

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
					actual := (*target)[k]
					if !deepEqual(actual, v) {
						t.Errorf("FromReader() result[%s] = %v (%T), want %v (%T)", k, actual, actual, v, v)
					}
				}
			case *[]any:
				expected := tt.expected.([]any)
				if len(*target) != len(expected) {
					t.Errorf("FromReader() result length = %d, want %d", len(*target), len(expected))
					return
				}
				for i, v := range expected {
					actual := (*target)[i]
					if !deepEqual(actual, v) {
						t.Errorf("FromReader() result[%d] = %v (%T), want %v (%T)", i, actual, actual, v, v)
					}
				}
			default:
				// For structs, compare directly
				targetValue := *target.(*struct {
					Name  string `yaml:"name"`
					Value int    `yaml:"value"`
				})
				expected := tt.expected.(struct {
					Name  string `yaml:"name"`
					Value int    `yaml:"value"`
				})
				if targetValue != expected {
					t.Errorf("FromReader() result = %+v, want %+v", targetValue, expected)
				}
			}
		})
	}
}

// deepEqual compares values recursively, handling nested structures
func deepEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	switch aVal := a.(type) {
	case map[string]any:
		bVal, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if len(aVal) != len(bVal) {
			return false
		}
		for k, v := range aVal {
			bValue, exists := bVal[k]
			if !exists || !deepEqual(v, bValue) {
				return false
			}
		}
		return true
	case []any:
		bVal, ok := b.([]any)
		if !ok {
			return false
		}
		if len(aVal) != len(bVal) {
			return false
		}
		for i, v := range aVal {
			if !deepEqual(v, bVal[i]) {
				return false
			}
		}
		return true
	default:
		// For primitive types, use direct comparison
		return a == b
	}
}

func TestYamlInputConverter_ToReader_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    string
		wantErr bool
	}{
		{
			name: "nil data",
			data: nil,
			want: "null\n",
		},
		{
			name: "empty map[string]any",
			data: map[string]any{},
			want: "{}\n",
		},
		{
			name: "empty map[any]any",
			data: map[any]any{},
			want: "{}\n",
		},
		{
			name: "empty slice",
			data: []any{},
			want: "[]\n",
		},
		{
			name: "nested structure",
			data: map[string]any{
				"nested": map[string]any{
					"value": []any{1, 2, 3},
				},
			},
			want: "nested:\n  value:\n  - 1\n  - 2\n  - 3\n",
		},
		{
			name: "YAML multiline string",
			data: `multiline: |
  This is a
  multiline string`,
			want: `multiline: |
  This is a
  multiline string`,
		},
		{
			name: "YAML with special characters",
			data: `special: "quotes and \"escapes\""
symbol: '@#$%'`,
			want: `special: "quotes and \"escapes\""
symbol: '@#$%'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y := &yamlInputConverter{data: tt.data}
			reader, err := y.ToReader()

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
				t.Errorf("ToReader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestYamlOutputConverter_FromReader_ErrorCases(t *testing.T) {
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
			y := &yamlOutputConverter{target: tt.target}
			err := y.FromReader(tt.reader)

			if err == nil {
				t.Error("FromReader() expected error, got nil")
			}
		})
	}
}

func TestYamlOutputConverter_FromReader_EmptyInput(t *testing.T) {
	var target map[string]any
	converter := &yamlOutputConverter{target: &target}
	reader := strings.NewReader("")

	err := converter.FromReader(reader)
	if err != nil {
		t.Errorf("FromReader() with empty input error = %v", err)
		return
	}

	// Empty YAML should result in nil/zero values
	if target != nil {
		t.Errorf("FromReader() with empty input result = %v, want nil", target)
	}
}

func TestYamlInputConverter_ToReader_IoReader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid YAML object",
			input: "name: test\nvalue: 42",
			want:  "name: test\nvalue: 42",
		},
		{
			name:  "valid YAML array",
			input: "- 1\n- 2\n- 3\n- test",
			want:  "- 1\n- 2\n- 3\n- test",
		},
		{
			name:  "valid YAML string",
			input: `simple string`,
			want:  `simple string`,
		},
		{
			name:  "valid YAML number",
			input: `42.5`,
			want:  `42.5`,
		},
		{
			name:  "valid YAML boolean",
			input: `true`,
			want:  `true`,
		},
		{
			name:  "valid YAML null",
			input: `null`,
			want:  `null`,
		},
		{
			name:  "valid nested YAML",
			input: "users:\n  - name: alice\n    age: 30\n  - name: bob\n    age: 25",
			want:  "users:\n  - name: alice\n    age: 30\n  - name: bob\n    age: 25",
		},
		{
			name:  "valid YAML with comments",
			input: "# Configuration\nname: test # inline comment\nvalue: 42",
			want:  "# Configuration\nname: test # inline comment\nvalue: 42",
		},
		{
			name:    "invalid YAML - bad indentation",
			input:   "name: test\n  bad: indentation",
			wantErr: true,
		},
		{
			name:    "invalid YAML - malformed",
			input:   "name: [unclosed array",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   ``,
			want:    ``,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			y := &yamlInputConverter{data: reader}

			result, err := y.ToReader()

			if tt.wantErr {
				if err == nil {
					// For streaming validation, error might come when reading
					data, readErr := io.ReadAll(result)
					if readErr == nil {
						t.Errorf("ToReader() expected error, but got valid result: %s", string(data))
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ToReader() error = %v", err)
				return
			}

			if result == nil {
				t.Fatal("ToReader() returned nil reader")
			}

			data, err := io.ReadAll(result)
			if err != nil {
				t.Errorf("Failed to read from result: %v", err)
				return
			}

			got := string(data)
			if got != tt.want {
				t.Errorf("ToReader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestYamlInputConverter_ToReader_IoReader_LargeData(t *testing.T) {
	// Test with data larger than buffer sizes to ensure chunked validation works
	largeObject := make(map[string]any)
	for i := 0; i < 1000; i++ {
		largeObject[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	// Convert to YAML bytes first
	yamlData, err := yaml.Marshal(largeObject)
	if err != nil {
		t.Fatalf("Failed to marshal large object: %v", err)
	}

	reader := bytes.NewReader(yamlData)
	y := &yamlInputConverter{data: reader}

	result, err := y.ToReader()
	if err != nil {
		t.Errorf("ToReader() error = %v", err)
		return
	}

	// Read result and verify it's valid YAML
	data, err := io.ReadAll(result)
	if err != nil {
		t.Errorf("Failed to read from result: %v", err)
		return
	}

	// Verify the result is valid YAML by unmarshaling
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Result is not valid YAML: %v", err)
		return
	}

	// Verify some content
	if len(parsed) != 1000 {
		t.Errorf("Parsed object length = %d, want 1000", len(parsed))
	}
}

func TestYamlInputConverter_ToReader_IoReader_ErrorCases(t *testing.T) {
	tests := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "reader error during streaming",
			reader: &failingReader{},
		},
		{
			name:   "slow reader with invalid YAML",
			reader: &slowReader{data: []byte("invalid: [yaml")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y := &yamlInputConverter{data: tt.reader}

			result, err := y.ToReader()
			if err != nil {
				// Error during setup is acceptable
				return
			}

			// Error should occur when reading from result
			_, err = io.ReadAll(result)
			if err == nil {
				t.Error("Expected error when reading from result, got nil")
			}
		})
	}
}

func TestYamlIntegration(t *testing.T) {
	t.Run("input to output roundtrip", func(t *testing.T) {
		original := map[string]any{
			"name":   "test",
			"value":  42,
			"nested": map[string]any{"key": "value"},
			"array":  []any{1, 2, 3},
		}

		// Convert to reader
		inputConverter := ToYaml(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result map[string]any
		outputConverter := FromYaml(&result)
		err = outputConverter.FromReader(reader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		// Verify roundtrip
		if result["name"] != "test" {
			t.Errorf("name = %v, want test", result["name"])
		}
		if result["value"] != uint64(42) {
			t.Errorf("value = %v, want 42", result["value"])
		}

		nested := result["nested"].(map[string]any)
		if nested["key"] != "value" {
			t.Errorf("nested.key = %v, want value", nested["key"])
		}

		array := result["array"].([]any)
		expected := []any{uint64(1), uint64(2), uint64(3)}
		if len(array) != len(expected) {
			t.Errorf("array length = %d, want %d", len(array), len(expected))
		}
		for i, v := range expected {
			if array[i] != v {
				t.Errorf("array[%d] = %v, want %v", i, array[i], v)
			}
		}
	})

	t.Run("struct roundtrip", func(t *testing.T) {
		type TestStruct struct {
			Name        string `yaml:"name"`
			Value       int    `yaml:"value"`
			Description string `yaml:"description,omitempty"`
		}

		original := TestStruct{
			Name:        "yaml test",
			Value:       123,
			Description: "test description",
		}

		// Convert to reader
		inputConverter := ToYaml(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result TestStruct
		outputConverter := FromYaml(&result)
		err = outputConverter.FromReader(reader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		// Verify roundtrip
		if result.Name != original.Name {
			t.Errorf("Name = %s, want %s", result.Name, original.Name)
		}
		if result.Value != original.Value {
			t.Errorf("Value = %d, want %d", result.Value, original.Value)
		}
		if result.Description != original.Description {
			t.Errorf("Description = %s, want %s", result.Description, original.Description)
		}
	})

	t.Run("string input roundtrip", func(t *testing.T) {
		originalYAML := `name: integration
config:
  enabled: true
  timeout: 30
items:
  - item1
  - item2`

		// Convert string to reader
		inputConverter := ToYaml(originalYAML)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result map[string]any
		outputConverter := FromYaml(&result)
		err = outputConverter.FromReader(reader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		// Verify structure
		if result["name"] != "integration" {
			t.Errorf("name = %v, want integration", result["name"])
		}

		config := result["config"].(map[string]any)
		if config["enabled"] != true {
			t.Errorf("config.enabled = %v, want true", config["enabled"])
		}
		if config["timeout"] != uint64(30) {
			t.Errorf("config.timeout = %v, want 30", config["timeout"])
		}

		items := result["items"].([]any)
		if len(items) != 2 {
			t.Errorf("items length = %d, want 2", len(items))
		}
		if items[0] != "item1" {
			t.Errorf("items[0] = %v, want item1", items[0])
		}
		if items[1] != "item2" {
			t.Errorf("items[1] = %v, want item2", items[1])
		}
	})

	t.Run("io.Reader to output roundtrip", func(t *testing.T) {
		yamlInput := `name: test
value: 42
array: [1, 2, 3]`
		reader := strings.NewReader(yamlInput)

		// Convert io.Reader to reader via ToYaml
		inputConverter := ToYaml(reader)
		pipeReader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result map[string]any
		outputConverter := FromYaml(&result)
		err = outputConverter.FromReader(pipeReader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		// Verify roundtrip
		if result["name"] != "test" {
			t.Errorf("name = %v, want test", result["name"])
		}
		if result["value"] != uint64(42) {
			t.Errorf("value = %v, want 42", result["value"])
		}

		array := result["array"].([]any)
		if len(array) != 3 {
			t.Errorf("array length = %d, want 3", len(array))
		}
		for i, expected := range []uint64{1, 2, 3} {
			if array[i] != expected {
				t.Errorf("array[%d] = %v, want %v", i, array[i], expected)
			}
		}
	})
}

func TestYamlSpecialFeatures(t *testing.T) {
	t.Run("yaml boolean values", func(t *testing.T) {
		input := `enabled: true
disabled: false
yes_value: yes
no_value: no`

		var result map[string]any
		converter := FromYaml(&result)
		reader := strings.NewReader(input)

		err := converter.FromReader(reader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		if result["enabled"] != true {
			t.Errorf("enabled = %v, want true", result["enabled"])
		}
		if result["disabled"] != false {
			t.Errorf("disabled = %v, want false", result["disabled"])
		}
		if result["yes_value"] != "yes" {
			t.Errorf("yes_value = %v, want yes", result["yes_value"])
		}
		if result["no_value"] != "no" {
			t.Errorf("no_value = %v, want no", result["no_value"])
		}
	})

	t.Run("yaml null values", func(t *testing.T) {
		input := `value1: null
value2: ~
value3:`

		var result map[string]any
		converter := FromYaml(&result)
		reader := strings.NewReader(input)

		err := converter.FromReader(reader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		if result["value1"] != nil {
			t.Errorf("value1 = %v, want nil", result["value1"])
		}
		if result["value2"] != nil {
			t.Errorf("value2 = %v, want nil", result["value2"])
		}
		if result["value3"] != nil {
			t.Errorf("value3 = %v, want nil", result["value3"])
		}
	})

	t.Run("yaml number formats", func(t *testing.T) {
		input := `decimal: 42
float: 3.14
scientific: 1.23e4
octal: 0o755
hex: 0xff`

		var result map[string]any
		converter := FromYaml(&result)
		reader := strings.NewReader(input)

		err := converter.FromReader(reader)
		if err != nil {
			t.Fatalf("FromReader() error = %v", err)
		}

		if result["decimal"] != uint64(42) {
			t.Errorf("decimal = %v, want 42", result["decimal"])
		}
		if result["float"] != 3.14 {
			t.Errorf("float = %v, want 3.14", result["float"])
		}
		if result["scientific"] != 12300.0 {
			t.Errorf("scientific = %v, want 12300", result["scientific"])
		}
		if result["octal"] != uint64(493) { // 0o755 = 493 in decimal
			t.Errorf("octal = %v, want 493", result["octal"])
		}
		if result["hex"] != uint64(255) { // 0xff = 255 in decimal
			t.Errorf("hex = %v, want 255", result["hex"])
		}
	})
}
