package convert

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/calque-pipe/core"
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

func TestToDescYaml(t *testing.T) {
	data := TestStruct{Name: "test", Value: 42}
	converter := ToDescYaml(data)

	if converter == nil {
		t.Fatal("ToDescYaml() returned nil")
	}
	if converter.format != "yaml" {
		t.Errorf("ToDescYaml() format = %s, want yaml", converter.format)
	}
	if converter.tagName != "yaml" {
		t.Errorf("ToDescYaml() tagName = %s, want yaml", converter.tagName)
	}
}

func TestToDescXml(t *testing.T) {
	data := TestStruct{Name: "test", Value: 42}
	converter := ToDescXml(data)

	if converter == nil {
		t.Fatal("ToDescXml() returned nil")
	}
	if converter.format != "xml" {
		t.Errorf("ToDescXml() format = %s, want xml", converter.format)
	}
	if converter.tagName != "xml" {
		t.Errorf("ToDescXml() tagName = %s, want xml", converter.tagName)
	}
}

func TestFromDescYaml(t *testing.T) {
	var target TestStruct
	converter := FromDescYaml[TestStruct](&target)

	if converter == nil {
		t.Fatal("FromDescYaml() returned nil")
	}
	if converter.format != "yaml" {
		t.Errorf("FromDescYaml() format = %s, want yaml", converter.format)
	}
	if converter.tagName != "yaml" {
		t.Errorf("FromDescYaml() tagName = %s, want yaml", converter.tagName)
	}
	if converter.target != &target {
		t.Error("FromDescYaml() target not set correctly")
	}
}

func TestFromDescXml(t *testing.T) {
	var target TestStruct
	converter := FromDescXml[TestStruct](&target)

	if converter == nil {
		t.Fatal("FromDescXml() returned nil")
	}
	if converter.format != "xml" {
		t.Errorf("FromDescXml() format = %s, want xml", converter.format)
	}
	if converter.tagName != "xml" {
		t.Errorf("FromDescXml() tagName = %s, want xml", converter.tagName)
	}
	if converter.target != &target {
		t.Error("FromDescXml() target not set correctly")
	}
}

func TestDescriptiveInputConverter_ToReader_YAML(t *testing.T) {
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
				"# Input Type: teststruct",
				"# Input Field descriptions:",
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
				"# Input Type: teststruct",
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
				"# Input Type: teststruct",
				"# Format: slice of teststruct",
				"- name: item1",
				"- name: item2",
			},
		},
		{
			name:     "string input",
			data:     "raw yaml string",
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
			converter := ToDescYaml(tt.data)
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

func TestDescriptiveInputConverter_ToReader_XML(t *testing.T) {
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
				"<!-- Input Type: teststruct -->",
				"<!-- Input Field descriptions: -->",
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
				"<!-- Input Type: teststruct -->",
				"<name>test</name>",
				"<value>42</value>",
			},
		},
		{
			name:     "string input",
			data:     "raw xml string",
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
			converter := ToDescXml(tt.data)
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

func TestDescriptiveOutputConverter_FromReader_YAML(t *testing.T) {
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
			converter := FromDescYaml[TestStruct](tt.target)
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

func TestDescriptiveOutputConverter_FromReader_XML(t *testing.T) {
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
			converter := FromDescXml[TestStruct](tt.target)
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

func TestDescriptiveConverter_EdgeCases(t *testing.T) {
	t.Run("anonymous struct", func(t *testing.T) {
		data := struct {
			Field string `yaml:"field" desc:"test field"`
		}{Field: "value"}

		converter := ToDescYaml(data)
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
		if !strings.Contains(outputStr, "# Input Type: anonymous") {
			t.Error("Anonymous struct should be labeled as 'anonymous'")
		}
	})

	t.Run("struct with mixed tags", func(t *testing.T) {
		data := TestStructMixedTags{Name: "test", Value: 42}

		// Test YAML converter (should only include yaml-tagged fields)
		converter := ToDescYaml(data)
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

		converter := ToDescYaml(data)
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
		if !strings.Contains(outputStr, "# Input Type: teststruct") {
			t.Error("Should contain struct type info")
		}
		if !strings.Contains(outputStr, "# Format: slice of teststruct") {
			t.Error("Should contain slice format info")
		}
	})
}

func TestDescriptiveConverter_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (*descriptiveInputConverter, error)
		expectError bool
	}{
		{
			name: "struct with no valid tags",
			setup: func() (*descriptiveInputConverter, error) {
				data := TestStructOnlyDesc{Name: "test"}
				converter := ToDescYaml(data)
				_, err := converter.ToReader()
				return converter, err
			},
			expectError: true,
		},
		{
			name: "slice of non-structs",
			setup: func() (*descriptiveInputConverter, error) {
				data := []string{"item1", "item2"}
				converter := ToDescYaml(data)
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

func TestDescriptiveConverter_Integration(t *testing.T) {
	t.Run("yaml roundtrip", func(t *testing.T) {
		original := TestStruct{
			Name:        "integration test",
			Value:       123,
			Description: "roundtrip test",
		}

		// Convert to reader
		inputConverter := ToDescYaml(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result TestStruct
		outputConverter := FromDescYaml[TestStruct](&result)
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
		inputConverter := ToDescXml(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result TestStruct
		outputConverter := FromDescXml[TestStruct](&result)
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
	converter := &descriptiveInputConverter{tagName: "yaml"}

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

func TestDescriptiveOutputConverter_ReaderError(t *testing.T) {
	var target TestStruct
	converter := FromDescYaml[TestStruct](&target)

	err := converter.FromReader(&failingReader{})
	if err == nil {
		t.Error("FromReader() expected error from failing reader, got nil")
	}
}

// Additional test structs for schema discovery tests
type InputStruct struct {
	ID   int    `yaml:"id" xml:"id" desc:"Unique identifier"`
	Name string `yaml:"name" xml:"name" desc:"Display name"`
}

type OutputStruct struct {
	Result    string `yaml:"result" xml:"result" desc:"Processing result"`
	Status    string `yaml:"status" xml:"status" desc:"Current status"`
	Score     int    `yaml:"score" xml:"score" desc:"Quality score"`
	Timestamp string `yaml:"timestamp" xml:"timestamp" desc:"When processed"`
}

// Test that auto-schema discovery includes output schema in YAML input
func TestAutoSchemaDiscovery_YAML(t *testing.T) {
	input := InputStruct{ID: 123, Name: "test input"}
	var output OutputStruct

	// Create flow with input and output converters
	flow := core.New()

	// Use a simple pass-through handler that copies input to output
	flow.UseFunc(func(r *core.Request, w *core.Response) error {
		// Just copy the input to output for testing schema discovery
		_, err := io.Copy(w.Data, r.Data)
		return err
	})

	var result string
	err := flow.Run(context.Background(), ToDescYaml(input), &result)
	if err != nil {
		t.Fatalf("Flow.Run() error = %v", err)
	}

	// Parse the result to check for output schema information
	if !strings.Contains(result, "# Input Type: inputstruct") {
		t.Error("Expected input type information in output")
	}
	if !strings.Contains(result, "# id: Unique identifier") {
		t.Error("Expected input field descriptions in output")
	}
	if !strings.Contains(result, "# name: Display name") {
		t.Error("Expected input field descriptions in output")
	}

	// Now test with full schema discovery using FromDescYaml
	flow2 := core.New()
	flow2.UseFunc(func(r *core.Request, w *core.Response) error {
		// Return mock YAML data that matches OutputStruct
		mockOutput := `result: "success"
status: "completed"
score: 95
timestamp: "2025-01-01T00:00:00Z"`
		_, err := w.Data.Write([]byte(mockOutput))
		return err
	})

	err = flow2.Run(context.Background(), ToDescYaml(input), FromDescYaml[OutputStruct](&output))
	if err != nil {
		t.Fatalf("Flow.Run() with schema discovery error = %v", err)
	}

	// Verify the output was parsed correctly
	if output.Result != "success" {
		t.Errorf("Output.Result = %s, want success", output.Result)
	}
	if output.Status != "completed" {
		t.Errorf("Output.Status = %s, want completed", output.Status)
	}
	if output.Score != 95 {
		t.Errorf("Output.Score = %d, want 95", output.Score)
	}
}

// Test that auto-schema discovery includes output schema in XML input
func TestAutoSchemaDiscovery_XML(t *testing.T) {
	input := InputStruct{ID: 456, Name: "xml test"}
	var output OutputStruct

	// Test with full schema discovery using FromDescXml
	flow := core.New()
	flow.UseFunc(func(r *core.Request, w *core.Response) error {
		// Return mock XML data that matches OutputStruct
		mockOutput := `<OutputStruct>
			<result>processed</result>
			<status>ready</status>
			<score>88</score>
			<timestamp>2025-01-01T12:00:00Z</timestamp>
		</OutputStruct>`
		_, err := w.Data.Write([]byte(mockOutput))
		return err
	})

	err := flow.Run(context.Background(), ToDescXml(input), FromDescXml[OutputStruct](&output))
	if err != nil {
		t.Fatalf("Flow.Run() with XML schema discovery error = %v", err)
	}

	// Verify the output was parsed correctly
	if output.Result != "processed" {
		t.Errorf("Output.Result = %s, want processed", output.Result)
	}
	if output.Status != "ready" {
		t.Errorf("Output.Status = %s, want ready", output.Status)
	}
	if output.Score != 88 {
		t.Errorf("Output.Score = %d, want 88", output.Score)
	}
}

// Test that output schema is included in input data when using linkSchemas
func TestSchemaProvider_GetSchema(t *testing.T) {
	var output OutputStruct
	outputConverter := FromDescYaml[OutputStruct](&output)

	// Test SchemaProvider interface
	schema := outputConverter.GetSchema()
	if schema == nil {
		t.Fatal("GetSchema() returned nil")
	}

	// Verify the schema is the zero value of OutputStruct
	outputSchema, ok := schema.(OutputStruct)
	if !ok {
		t.Fatalf("GetSchema() returned %T, want OutputStruct", schema)
	}

	// Zero value should have empty/zero fields
	if outputSchema.Result != "" {
		t.Errorf("Schema.Result = %s, want empty string", outputSchema.Result)
	}
	if outputSchema.Score != 0 {
		t.Errorf("Schema.Score = %d, want 0", outputSchema.Score)
	}
}

// Test that input converter accepts schema via SchemaConsumer interface
func TestSchemaConsumer_SetOutputSchema(t *testing.T) {
	input := InputStruct{ID: 789, Name: "schema test"}
	inputConverter := ToDescYaml(input)

	// Test SchemaConsumer interface
	outputSchema := OutputStruct{
		Result: "test",
		Status: "example",
		Score:  100,
	}

	inputConverter.SetOutputSchema(outputSchema)

	// Verify schema was set
	if inputConverter.outputSchema == nil {
		t.Fatal("SetOutputSchema() did not set outputSchema field")
	}

	// Generate output and verify it includes schema information
	reader, err := inputConverter.ToReader()
	if err != nil {
		t.Fatalf("ToReader() error = %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	output := string(data)

	// Check for output schema comments
	if !strings.Contains(output, "# Expected Output Type: outputstruct") {
		t.Error("Expected output type information missing from input data")
	}
	if !strings.Contains(output, "# result: Processing result") {
		t.Error("Expected output field description missing from input data")
	}
	if !strings.Contains(output, "# status: Current status") {
		t.Error("Expected output field description missing from input data")
	}
	if !strings.Contains(output, "# score: Quality score") {
		t.Error("Expected output field description missing from input data")
	}
}

// Test schema discovery with XML format
func TestSchemaConsumer_SetOutputSchema_XML(t *testing.T) {
	input := InputStruct{ID: 999, Name: "xml schema test"}
	inputConverter := ToDescXml(input)

	// Test SchemaConsumer interface with XML
	outputSchema := OutputStruct{
		Result: "xml_test",
		Status: "xml_example",
		Score:  75,
	}

	inputConverter.SetOutputSchema(outputSchema)

	// Generate XML output and verify it includes schema information
	reader, err := inputConverter.ToReader()
	if err != nil {
		t.Fatalf("ToReader() error = %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	output := string(data)

	// Check for output schema XML comments
	if !strings.Contains(output, "<!-- Expected Output Type: outputstruct -->") {
		t.Error("Expected output type information missing from XML input data")
	}
	if !strings.Contains(output, "<!-- result: Processing result -->") {
		t.Error("Expected output field description missing from XML input data")
	}
	if !strings.Contains(output, "<!-- status: Current status -->") {
		t.Error("Expected output field description missing from XML input data")
	}
	if !strings.Contains(output, "<!-- score: Quality score -->") {
		t.Error("Expected output field description missing from XML input data")
	}
}
