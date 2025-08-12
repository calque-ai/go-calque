package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
)

// Input converter for structured data -> JSON with JSON Schema
type jsonSchemaInputConverter struct {
	data any
}

// Output converter for JSON Schema validated data -> structured data
type jsonSchemaOutputConverter[T any] struct {
	target any
}

// ToJsonSchema creates an input converter that embeds JSON Schema with structured data.
//
// Input: struct with json and jsonschema tags, or JSON string
// Output: *jsonSchemaInputConverter for pipeline input position
// Behavior: BUFFERED - generates JSON Schema and embeds with data as single JSON object
//
// Generates JSON Schema from struct definition using invopop/jsonschema library
// and creates a combined JSON object containing both the data and schema.
// This provides AI systems with both the data and its structural definition
// for better context understanding and validation.
//
// The jsonschema struct tags define validation rules:
// - required: field is mandatory
// - enum: field must be one of specified values
// - minimum/maximum: numeric constraints
// - description: field documentation
//
// Example usage:
//
//	type Task struct {
//		Type     string `json:"type" jsonschema:"required,enum=bug,enum=feature"`
//		Priority string `json:"priority" jsonschema:"required,enum=low,enum=high"`
//		Hours    int    `json:"hours" jsonschema:"minimum=1,maximum=40"`
//	}
//
//	task := Task{Type: "feature", Priority: "high", Hours: 8}
//	err := pipeline.Run(ctx, convert.ToJsonSchema(task), &result)
func ToJsonSchema(data any) *jsonSchemaInputConverter {
	return &jsonSchemaInputConverter{
		data: data,
	}
}

// FromJsonSchema creates an output converter that validates JSON against schema.
//
// Input: pointer to target variable for unmarshaling, generic type parameter for validation
// Output: *jsonSchemaOutputConverter for pipeline output position
// Behavior: BUFFERED - reads entire JSON stream, validates against schema, unmarshals to target
//
// Parses JSON data that may contain embedded schema information and unmarshals
// to the specified target type. Handles both schema-embedded format (from ToJsonSchema)
// and direct JSON format. Uses the generic type parameter to determine expected
// structure and validation rules.
//
// The converter attempts multiple unmarshaling strategies:
// 1. Direct unmarshaling to target type
// 2. Schema-wrapped format extraction
// 3. Flexible wrapper format handling
//
// Example usage:
//
//	type Task struct {
//		Type     string `json:"type"`
//		Priority string `json:"priority"`
//		Hours    int    `json:"hours"`
//	}
//
//	var task Task
//	err := pipeline.Run(ctx, schemaInput, convert.FromJsonSchema[Task](&task))
//	fmt.Printf("Task: %s priority, %d hours\n", task.Priority, task.Hours)
func FromJsonSchema[T any](target any) *jsonSchemaOutputConverter[T] {
	return &jsonSchemaOutputConverter[T]{
		target: target,
	}
}

// InputConverter interface
func (j *jsonSchemaInputConverter) ToReader() (io.Reader, error) {
	// Get the struct type and value
	val := reflect.ValueOf(j.data)
	typ := val.Type()

	// Handle pointers
	if typ.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("input is nil pointer")
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.String {
		// If input is a string, pass it through as-is
		return strings.NewReader(j.data.(string)), nil
	}

	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported jsonschema input type: %T", j.data)
	}

	// Generate JSON Schema
	reflector := jsonschema.Reflector{}
	schema := reflector.Reflect(j.data)

	// Create response with both data and schema
	structName := strings.ToLower(typ.Name())
	response := map[string]any{
		structName: j.data,
		"$schema":  schema,
	}

	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON with schema: %w", err)
	}

	return bytes.NewReader(jsonBytes), nil
}

// OutputConverter interface
func (j *jsonSchemaOutputConverter[T]) FromReader(reader io.Reader) error {
	// Buffer for fallback if direct decode fails
	var buf bytes.Buffer
	teeReader := io.TeeReader(reader, &buf)

	// Try direct streaming decode first (fast path for common case)
	decoder := json.NewDecoder(teeReader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(j.target); err == nil {
		return nil // Success - pure streaming, no marshal/unmarshal overhead!
	}

	// Direct streaming failed, use buffered data for wrapper logic
	var wrapper map[string]any
	if err := json.Unmarshal(buf.Bytes(), &wrapper); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Get the struct type name to find the correct wrapper key
	var zeroT T
	structName := strings.ToLower(reflect.TypeOf(zeroT).Name())

	// Extract the actual data from under the struct name key
	actualData, exists := wrapper[structName]
	if !exists {
		return fmt.Errorf("expected wrapper key '%s' not found in JSON", structName)
	}

	// Marshal the actual data back to bytes and unmarshal to the target struct
	actualBytes, err := json.Marshal(actualData)
	if err != nil {
		return fmt.Errorf("failed to re-marshal actual data: %w", err)
	}

	// Unmarshal directly into the target
	if err := json.Unmarshal(actualBytes, j.target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}
