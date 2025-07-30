package convert

import (
	"io"
	"strings"
	"testing"
)

// Test structs for various scenarios
type TestStruct struct {
	Name        string `yaml:"name" xml:"name" desc:"The name field"`
	Value       int    `yaml:"value" xml:"value" desc:"The value field"`
	Description string `yaml:"description,omitempty" xml:"description" desc:"Optional description"`
}

type TestStructNoTags struct {
	Name  string
	Value int
}

type TestStructMixedTags struct {
	Name        string `yaml:"name" desc:"Has yaml tag"`
	Value       int    `xml:"value" desc:"Has xml tag"`
	Description string `desc:"No format tag"`
}

type TestStructOnlyDesc struct {
	Name string `desc:"Only description tag"`
}

func TestStructuredYAML(t *testing.T) {
	data := TestStruct{Name: "test", Value: 42}
	converter := StructuredYAML(data)
	
	if converter == nil {
		t.Fatal("StructuredYAML() returned nil")
	}
	if converter.format != "yaml" {
		t.Errorf("StructuredYAML() format = %s, want yaml", converter.format)
	}
	if converter.tagName != "yaml" {
		t.Errorf("StructuredYAML() tagName = %s, want yaml", converter.tagName)
	}
}

func TestStructuredXML(t *testing.T) {
	data := TestStruct{Name: "test", Value: 42}
	converter := StructuredXML(data)
	
	if converter == nil {
		t.Fatal("StructuredXML() returned nil")
	}
	if converter.format != "xml" {
		t.Errorf("StructuredXML() format = %s, want xml", converter.format)
	}
	if converter.tagName != "xml" {
		t.Errorf("StructuredXML() tagName = %s, want xml", converter.tagName)
	}
}

func TestStructuredYAMLOutput(t *testing.T) {
	var target TestStruct
	converter := StructuredYAMLOutput[TestStruct](&target)
	
	if converter == nil {
		t.Fatal("StructuredYAMLOutput() returned nil")
	}
	if converter.format != "yaml" {
		t.Errorf("StructuredYAMLOutput() format = %s, want yaml", converter.format)
	}
	if converter.tagName != "yaml" {
		t.Errorf("StructuredYAMLOutput() tagName = %s, want yaml", converter.tagName)
	}
	if converter.target != &target {
		t.Error("StructuredYAMLOutput() target not set correctly")
	}
}

func TestStructuredXMLOutput(t *testing.T) {
	var target TestStruct
	converter := StructuredXMLOutput[TestStruct](&target)
	
	if converter == nil {
		t.Fatal("StructuredXMLOutput() returned nil")
	}
	if converter.format != "xml" {
		t.Errorf("StructuredXMLOutput() format = %s, want xml", converter.format)
	}
	if converter.tagName != "xml" {
		t.Errorf("StructuredXMLOutput() tagName = %s, want xml", converter.tagName)
	}
	if converter.target != &target {
		t.Error("StructuredXMLOutput() target not set correctly")
	}
}

func TestStructuredInputConverter_ToReader_YAML(t *testing.T) {
	tests := []struct {
		name        string
		data        any
		expectError bool
		contains    []string
	}{
		{
			name: "struct with yaml tags",
			data: TestStruct{Name: "test", Value: 42, Description: "desc"},
			contains: []string{
				"# Type: teststruct",
				"# Field descriptions:",
				"# name: The name field",
				"# value: The value field",
				"name: test",
				"value: 42",
			},
		},
		{
			name: "struct pointer with yaml tags",
			data: &TestStruct{Name: "test", Value: 42},
			contains: []string{
				"# Type: teststruct",
				"name: test",
				"value: 42",
			},
		},
		{
			name: "slice of structs",
			data: []TestStruct{
				{Name: "item1", Value: 1},
				{Name: "item2", Value: 2},
			},
			contains: []string{
				"# Type: teststruct",
				"# Format: slice of teststruct",
				"- name: item1",
				"- name: item2",
			},
		},
		{
			name: "string input",
			data: "raw yaml string",
			contains: []string{"raw yaml string"},
		},
		{
			name:        "nil pointer",
			data:        (*TestStruct)(nil),
			expectError: true,
		},
		{
			name:        "struct without yaml tags",
			data:        TestStructNoTags{Name: "test", Value: 42},
			expectError: true,
		},
		{
			name:        "unsupported type",
			data:        make(chan int),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := StructuredYAML(tt.data)
			reader, err := converter.ToReader()
			
			if tt.expectError {
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
			
			output := string(data)
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("ToReader() output missing expected content: %s\nGot: %s", expected, output)
				}
			}
		})
	}
}

func TestStructuredInputConverter_ToReader_XML(t *testing.T) {
	tests := []struct {
		name        string
		data        any
		expectError bool
		contains    []string
	}{
		{
			name: "struct with xml tags",
			data: TestStruct{Name: "test", Value: 42, Description: "desc"},
			contains: []string{
				`<?xml version="1.0" encoding="UTF-8"?>`,
				"<!-- Type: teststruct -->",
				"<!-- Field descriptions: -->",
				"<!-- name: The name field -->",
				"<name>test</name>",
				"<value>42</value>",
			},
		},
		{
			name: "struct pointer with xml tags",
			data: &TestStruct{Name: "test", Value: 42},
			contains: []string{
				`<?xml version="1.0" encoding="UTF-8"?>`,
				"<!-- Type: teststruct -->",
				"<name>test</name>",
				"<value>42</value>",
			},
		},
		{
			name: "string input",
			data: "raw xml string",
			contains: []string{"raw xml string"},
		},
		{
			name:        "struct without xml tags",
			data:        TestStructNoTags{Name: "test", Value: 42},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := StructuredXML(tt.data)
			reader, err := converter.ToReader()
			
			if tt.expectError {
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
			
			output := string(data)
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("ToReader() output missing expected content: %s\nGot: %s", expected, output)
				}
			}
		})
	}
}

func TestStructuredOutputConverter_FromReader_YAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		target      any
		expectError bool
		validate    func(t *testing.T, target any)
	}{
		{
			name: "valid yaml to struct",
			input: `name: test
value: 42
description: test desc`,
			target: &TestStruct{},
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "test" {
					t.Errorf("Name = %s, want test", result.Name)
				}
				if result.Value != 42 {
					t.Errorf("Value = %d, want 42", result.Value)
				}
				if result.Description != "test desc" {
					t.Errorf("Description = %s, want test desc", result.Description)
				}
			},
		},
		{
			name: "yaml with comments",
			input: `# This is a comment
name: test
value: 42`,
			target: &TestStruct{},
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "test" {
					t.Errorf("Name = %s, want test", result.Name)
				}
				if result.Value != 42 {
					t.Errorf("Value = %d, want 42", result.Value)
				}
			},
		},
		{
			name:        "invalid yaml",
			input:       `name: test\nvalue: [invalid yaml`,
			target:      &TestStruct{},
			expectError: true,
		},
		{
			name:   "empty input",
			input:  "",
			target: &TestStruct{},
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				// Empty YAML should result in zero values
				if result.Name != "" {
					t.Errorf("Name = %s, want empty string", result.Name)
				}
				if result.Value != 0 {
					t.Errorf("Value = %d, want 0", result.Value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := StructuredYAMLOutput[TestStruct](tt.target)
			reader := strings.NewReader(tt.input)
			
			err := converter.FromReader(reader)
			
			if tt.expectError {
				if err == nil {
					t.Error("FromReader() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("FromReader() error = %v", err)
				return
			}
			
			if tt.validate != nil {
				tt.validate(t, tt.target)
			}
		})
	}
}

func TestStructuredOutputConverter_FromReader_XML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		target      any
		expectError bool
		validate    func(t *testing.T, target any)
	}{
		{
			name: "valid xml to struct",
			input: `<TestStruct>
				<name>test</name>
				<value>42</value>
				<description>test desc</description>
			</TestStruct>`,
			target: &TestStruct{},
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "test" {
					t.Errorf("Name = %s, want test", result.Name)
				}
				if result.Value != 42 {
					t.Errorf("Value = %d, want 42", result.Value)
				}
				if result.Description != "test desc" {
					t.Errorf("Description = %s, want test desc", result.Description)
				}
			},
		},
		{
			name: "xml with comments",
			input: `<!-- This is a comment -->
			<TestStruct>
				<name>test</name>
				<value>42</value>
			</TestStruct>`,
			target: &TestStruct{},
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "test" {
					t.Errorf("Name = %s, want test", result.Name)
				}
				if result.Value != 42 {
					t.Errorf("Value = %d, want 42", result.Value)
				}
			},
		},
		{
			name:        "invalid xml",
			input:       `<TestStruct><name>test</unclosed>`,
			target:      &TestStruct{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := StructuredXMLOutput[TestStruct](tt.target)
			reader := strings.NewReader(tt.input)
			
			err := converter.FromReader(reader)
			
			if tt.expectError {
				if err == nil {
					t.Error("FromReader() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("FromReader() error = %v", err)
				return
			}
			
			if tt.validate != nil {
				tt.validate(t, tt.target)
			}
		})
	}
}

func TestStructuredConverter_EdgeCases(t *testing.T) {
	t.Run("anonymous struct", func(t *testing.T) {
		data := struct {
			Field string `yaml:"field" desc:"test field"`
		}{Field: "value"}
		
		converter := StructuredYAML(data)
		reader, err := converter.ToReader()
		if err != nil {
			t.Errorf("ToReader() error = %v", err)
			return
		}
		
		output, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("Failed to read output: %v", err)
			return
		}
		
		outputStr := string(output)
		if !strings.Contains(outputStr, "# Type: anonymous") {
			t.Error("Anonymous struct should be labeled as 'anonymous'")
		}
	})
	
	t.Run("struct with mixed tags", func(t *testing.T) {
		data := TestStructMixedTags{Name: "test", Value: 42}
		
		// Test YAML converter (should only include yaml-tagged fields)
		converter := StructuredYAML(data)
		reader, err := converter.ToReader()
		if err != nil {
			t.Errorf("ToReader() error = %v", err)
			return
		}
		
		output, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("Failed to read output: %v", err)
			return
		}
		
		outputStr := string(output)
		if !strings.Contains(outputStr, "name: test") {
			t.Error("Should contain yaml-tagged field")
		}
		if strings.Contains(outputStr, "value: 42") {
			t.Error("Should not contain xml-only tagged field in YAML output")
		}
	})
	
	t.Run("slice with pointer elements", func(t *testing.T) {
		data := []*TestStruct{
			{Name: "item1", Value: 1},
			{Name: "item2", Value: 2},
		}
		
		converter := StructuredYAML(data)
		reader, err := converter.ToReader()
		if err != nil {
			t.Errorf("ToReader() error = %v", err)
			return
		}
		
		output, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("Failed to read output: %v", err)
			return
		}
		
		outputStr := string(output)
		if !strings.Contains(outputStr, "# Type: teststruct") {
			t.Error("Should contain struct type info")
		}
		if !strings.Contains(outputStr, "# Format: slice of teststruct") {
			t.Error("Should contain slice format info")
		}
	})
}

func TestStructuredConverter_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (*structuredInputConverter, error)
		expectError bool
	}{
		{
			name: "struct with no valid tags",
			setup: func() (*structuredInputConverter, error) {
				data := TestStructOnlyDesc{Name: "test"}
				converter := StructuredYAML(data)
				_, err := converter.ToReader()
				return converter, err
			},
			expectError: true,
		},
		{
			name: "slice of non-structs",
			setup: func() (*structuredInputConverter, error) {
				data := []string{"item1", "item2"}
				converter := StructuredYAML(data)
				_, err := converter.ToReader()
				return converter, err
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.setup()
			
			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestStructuredConverter_Integration(t *testing.T) {
	t.Run("yaml roundtrip", func(t *testing.T) {
		original := TestStruct{
			Name:        "integration test",
			Value:       123,
			Description: "roundtrip test",
		}
		
		// Convert to reader
		inputConverter := StructuredYAML(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}
		
		// Convert back from reader
		var result TestStruct
		outputConverter := StructuredYAMLOutput[TestStruct](&result)
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
	
	t.Run("xml roundtrip", func(t *testing.T) {
		original := TestStruct{
			Name:        "xml test",
			Value:       456,
			Description: "xml roundtrip",
		}
		
		// Convert to reader
		inputConverter := StructuredXML(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}
		
		// Convert back from reader
		var result TestStruct
		outputConverter := StructuredXMLOutput[TestStruct](&result)
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
}

func TestParseFieldNameFromTag(t *testing.T) {
	converter := &structuredInputConverter{tagName: "yaml"}
	
	tests := []struct {
		tagValue    string
		defaultName string
		want        string
	}{
		{"fieldname", "DefaultName", "fieldname"},
		{"fieldname,omitempty", "DefaultName", "fieldname"},
		{",omitempty", "DefaultName", "defaultname"},
		{"", "DefaultName", "defaultname"},
		{"field,required,omitempty", "DefaultName", "field"},
	}
	
	for _, tt := range tests {
		t.Run(tt.tagValue, func(t *testing.T) {
			got := converter.parseFieldNameFromTag(tt.tagValue, tt.defaultName)
			if got != tt.want {
				t.Errorf("parseFieldNameFromTag(%q, %q) = %q, want %q", 
					tt.tagValue, tt.defaultName, got, tt.want)
			}
		})
	}
}

func TestStructuredOutputConverter_ReaderError(t *testing.T) {
	var target TestStruct
	converter := StructuredYAMLOutput[TestStruct](&target)
	
	err := converter.FromReader(&failingReader{})
	if err == nil {
		t.Error("FromReader() expected error from failing reader, got nil")
	}
}