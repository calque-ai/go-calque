package convert

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

const jsonSchemaTestConstant = "test"

// Test structs for JSON Schema scenarios
type JSONSchemaTestStruct struct {
	Name        string `json:"name" jsonschema:"title=Name,description=The name field"`
	Value       int    `json:"value" jsonschema:"title=Value,description=The value field,minimum=0"`
	Description string `json:"description,omitempty" jsonschema:"title=Description,description=Optional description"`
	Active      bool   `json:"active" jsonschema:"title=Active,description=Whether this is active"`
}

type SimpleStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type NestedStruct struct {
	Person JSONSchemaTestStruct `json:"person" jsonschema:"title=Person,description=Person details"`
	Count  int                  `json:"count" jsonschema:"minimum=1"`
}

func TestToJSONSchema(t *testing.T) {
	data := JSONSchemaTestStruct{Name: "test", Value: 42, Active: true}
	converter := ToJSONSchema(data)

	if converter == nil {
		t.Fatal("ToJSONSchema() returned nil")
	}
	if converter.data == nil {
		t.Error("ToJSONSchema() did not set data")
	}
}

func TestFromJSONSchema(t *testing.T) {
	var target JSONSchemaTestStruct
	converter := FromJSONSchema[JSONSchemaTestStruct](&target)

	if converter == nil {
		t.Fatal("FromJSONSchema() returned nil")
	}
	if converter.target != &target {
		t.Error("FromJSONSchema() target not set correctly")
	}
}

func TestSchemaInputConverter_ToReader(t *testing.T) {
	tests := []struct {
		name        string
		data        any
		expectError bool
		contains    []string
		notContains []string
	}{
		{
			name: "struct with jsonschema tags",
			data: JSONSchemaTestStruct{
				Name:        "test",
				Value:       42,
				Description: "test description",
				Active:      true,
			},
			contains: []string{
				`"jsonschemateststruct":`,
				`"name": "test"`,
				`"value": 42`,
				`"active": true`,
				`"$schema":`,
				`"type": "object"`,
				`"properties":`,
				`"title": "Name"`,
				`"description": "The name field"`,
			},
		},
		{
			name: "simple struct",
			data: SimpleStruct{ID: 1, Name: "simple"},
			contains: []string{
				`"simplestruct":`,
				`"id": 1`,
				`"name": "simple"`,
				`"$schema":`,
			},
		},
		{
			name: "nested struct",
			data: NestedStruct{
				Person: JSONSchemaTestStruct{Name: "nested", Value: 10, Active: false},
				Count:  5,
			},
			contains: []string{
				`"nestedstruct":`,
				`"person":`,
				`"count": 5`,
				`"$schema":`,
			},
		},
		{
			name: "pointer to struct",
			data: &SimpleStruct{ID: 2, Name: "pointer"},
			contains: []string{
				`"simplestruct":`,
				`"id": 2`,
				`"name": "pointer"`,
			},
		},
		{
			name:     "string input",
			data:     `{"test": "raw json"}`,
			contains: []string{`{"test": "raw json"}`},
		},
		{
			name:        "nil pointer",
			data:        (*SimpleStruct)(nil),
			expectError: true,
		},
		{
			name:        "non-struct type",
			data:        []string{"not", "a", "struct"},
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
			converter := &SchemaInputConverter{data: tt.data}
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

			// Check for required content
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("ToReader() output missing expected content: %s\nGot: %s", expected, output)
				}
			}

			// Check for content that should not be present
			for _, notExpected := range tt.notContains {
				if strings.Contains(output, notExpected) {
					t.Errorf("ToReader() output contains unexpected content: %s\nGot: %s", notExpected, output)
				}
			}

			// Verify it's valid JSON
			var jsonCheck map[string]any
			if err := json.Unmarshal(data, &jsonCheck); err != nil {
				t.Errorf("ToReader() output is not valid JSON: %v\nGot: %s", err, output)
			}
		})
	}
}

func TestJSONSchemaOutputConverter_FromReader(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		target      any
		expectError bool
		validate    func(t *testing.T, target any)
	}{
		{
			name: "valid JSON with schema wrapper",
			input: `{
				"jsonschemateststruct": {
					"name": "test",
					"value": 42,
					"active": true
				},
				"$schema": {
					"type": "object"
				}
			}`,
			target: &JSONSchemaTestStruct{},
			validate: func(t *testing.T, target any) {
				result := target.(*JSONSchemaTestStruct)
				if result.Name != jsonSchemaTestConstant {
					t.Errorf("Name = %s, want test", result.Name)
				}
				if result.Value != 42 {
					t.Errorf("Value = %d, want 42", result.Value)
				}
				if result.Active != true {
					t.Errorf("Active = %t, want true", result.Active)
				}
			},
		},
		{
			name: "simple struct with wrapper",
			input: `{
				"simplestruct": {
					"id": 123,
					"name": "simple test"
				},
				"$schema": {
					"type": "object"
				}
			}`,
			target: &SimpleStruct{},
			validate: func(t *testing.T, target any) {
				result := target.(*SimpleStruct)
				if result.ID != 123 {
					t.Errorf("ID = %d, want 123", result.ID)
				}
				if result.Name != "simple test" {
					t.Errorf("Name = %s, want simple test", result.Name)
				}
			},
		},
		{
			name:        "missing wrapper key",
			input:       `{"wrongkey": {"id": 1}, "$schema": {}}`,
			target:      &SimpleStruct{},
			expectError: true,
		},
		{
			name:        "invalid JSON",
			input:       `{"invalid": json}`,
			target:      &SimpleStruct{},
			expectError: true,
		},
		{
			name:        "empty input",
			input:       ``,
			target:      &SimpleStruct{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch target := tt.target.(type) {
			case *JSONSchemaTestStruct:
				converter := FromJSONSchema[JSONSchemaTestStruct](target)
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

			case *SimpleStruct:
				converter := FromJSONSchema[SimpleStruct](target)
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
			}
		})
	}
}

func TestJSONSchemaConverter_EdgeCases(t *testing.T) {
	t.Run("anonymous struct", func(t *testing.T) {
		data := struct {
			Field string `json:"field" jsonschema:"title=Field"`
		}{Field: "value"}

		converter := &SchemaInputConverter{data: data}
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
		// Anonymous structs should use empty string as struct name, resulting in empty key
		if !strings.Contains(outputStr, `"":`) {
			t.Error("Anonymous struct should have empty string key")
		}
	})

	t.Run("struct with complex jsonschema tags", func(t *testing.T) {
		type ComplexStruct struct {
			Email string   `json:"email" jsonschema:"format=email,title=Email Address"`
			Age   int      `json:"age" jsonschema:"minimum=0,maximum=150,title=Age"`
			Score float64  `json:"score" jsonschema:"minimum=0.0,maximum=100.0,multipleOf=0.1"`
			Tags  []string `json:"tags" jsonschema:"minItems=1,maxItems=10,title=Tags"`
		}

		data := ComplexStruct{
			Email: "test@example.com",
			Age:   30,
			Score: 85.5,
			Tags:  []string{"tag1", "tag2"},
		}

		converter := &SchemaInputConverter{data: data}
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
		expectedConstraints := []string{
			`"format": "email"`,
			`"minimum": 0`,
			`"maximum": 150`,
			`"multipleOf": 0.1`,
			`"minItems": 1`,
			`"maxItems": 10`,
		}

		for _, constraint := range expectedConstraints {
			if !strings.Contains(outputStr, constraint) {
				t.Errorf("Output missing expected constraint: %s", constraint)
			}
		}
	})
}

func TestJSONSchemaConverter_SchemaGeneration(t *testing.T) {
	t.Run("schema contains required fields", func(t *testing.T) {
		data := JSONSchemaTestStruct{Name: "test", Value: 42, Active: true}
		converter := &SchemaInputConverter{data: data}

		reader, err := converter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		output, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read output: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("Failed to unmarshal output: %v", err)
		}

		// Check that schema is present
		schema, exists := result["$schema"]
		if !exists {
			t.Fatal("Schema not found in output")
		}

		schemaMap, ok := schema.(map[string]any)
		if !ok {
			t.Fatal("Schema is not a map")
		}

		// Verify basic schema structure (using $ref and $defs pattern)
		if schemaMap["$schema"] == nil {
			t.Error("Schema missing $schema field")
		}

		if schemaMap["$ref"] == nil {
			t.Error("Schema missing $ref field")
		}

		defs, exists := schemaMap["$defs"]
		if !exists {
			t.Fatal("Schema missing $defs")
		}

		defsMap, ok := defs.(map[string]any)
		if !ok {
			t.Fatal("Schema $defs is not a map")
		}

		// Find the struct definition in $defs
		var structDef map[string]any
		for _, def := range defsMap {
			if defMap, ok := def.(map[string]any); ok {
				if defMap["type"] == "object" {
					structDef = defMap
					break
				}
			}
		}

		if structDef == nil {
			t.Fatal("No object type definition found in $defs")
		}

		// Verify the struct definition has the expected structure
		if structDef["type"] != "object" {
			t.Errorf("Struct definition type = %v, want object", structDef["type"])
		}

		properties, exists := structDef["properties"]
		if !exists {
			t.Fatal("Struct definition missing properties")
		}

		propertiesMap, ok := properties.(map[string]any)
		if !ok {
			t.Fatal("Struct definition properties is not a map")
		}

		// Check that all struct fields are in schema properties
		expectedFields := []string{"name", "value", "description", "active"}
		for _, field := range expectedFields {
			if _, exists := propertiesMap[field]; !exists {
				t.Errorf("Schema missing property: %s", field)
			}
		}
	})
}

func TestJSONSchemaConverter_ErrorHandling(t *testing.T) {
	tests := []struct {
		name   string
		reader io.Reader
		target any
	}{
		{
			name:   "reader error",
			reader: &failingReader{},
			target: &SimpleStruct{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := FromJSONSchema[SimpleStruct](tt.target)
			err := converter.FromReader(tt.reader)

			if err == nil {
				t.Error("FromReader() expected error, got nil")
			}
		})
	}
}

func TestJSONSchemaConverter_Integration(t *testing.T) {
	t.Run("roundtrip conversion", func(t *testing.T) {
		original := JSONSchemaTestStruct{
			Name:        "integration test",
			Value:       123,
			Description: "roundtrip test",
			Active:      true,
		}

		// Convert to reader
		inputConverter := ToJSONSchema(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Convert back from reader
		var result JSONSchemaTestStruct
		outputConverter := FromJSONSchema[JSONSchemaTestStruct](&result)
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
		if result.Active != original.Active {
			t.Errorf("Active = %t, want %t", result.Active, original.Active)
		}
	})

	t.Run("schema validation preserves structure", func(t *testing.T) {
		// Create a nested structure
		original := NestedStruct{
			Person: JSONSchemaTestStruct{
				Name:        "nested person",
				Value:       456,
				Description: "nested description",
				Active:      false,
			},
			Count: 10,
		}

		// Convert to JSON with schema
		inputConverter := ToJSONSchema(original)
		reader, err := inputConverter.ToReader()
		if err != nil {
			t.Fatalf("ToReader() error = %v", err)
		}

		// Read the output and verify it contains both data and schema
		output, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read output: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("Failed to unmarshal output: %v", err)
		}

		// Verify both data and schema are present
		if _, exists := result["nestedstruct"]; !exists {
			t.Error("Data section missing from output")
		}

		if _, exists := result["$schema"]; !exists {
			t.Error("Schema section missing from output")
		}

		// Verify the data structure is preserved
		dataSection := result["nestedstruct"].(map[string]any)
		if dataSection["count"] != float64(10) { // JSON unmarshals numbers as float64
			t.Errorf("Count = %v, want 10", dataSection["count"])
		}

		person := dataSection["person"].(map[string]any)
		if person["name"] != "nested person" {
			t.Errorf("Person name = %v, want nested person", person["name"])
		}
	})
}

func TestJSONSchemaConverter_StringInput(t *testing.T) {
	t.Run("string input passthrough", func(t *testing.T) {
		rawJSON := `{"direct": "json", "number": 42}`

		converter := &SchemaInputConverter{data: rawJSON}
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
		if outputStr != rawJSON {
			t.Errorf("String input not passed through correctly. Got: %s, Want: %s", outputStr, rawJSON)
		}
	})
}
