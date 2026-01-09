package convert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/invopop/jsonschema"
)

// SchemaInputConverter for structured data -> JSON Schema validated data
type SchemaInputConverter struct {
	data any
}

// SchemaOutputConverter for JSON Schema validated data -> structured data
type SchemaOutputConverter[T any] struct {
	target any
}

// ToJSONSchema creates an input converter for transforming structured data to JSON streams.
//
// Input: any data type (structs, maps, slices, JSON strings, JSON bytes)
// Output: calque.InputConverter for pipeline input position
// Behavior: STREAMING - uses json.Encoder for automatic streaming optimization
//
// Converts various data types to valid JSON format for pipeline processing:
// - Structs/maps/slices: Marshaled using encoding/json
// - JSON strings: Validated and passed through
// - JSON bytes: Validated and passed through
// - Other types: Attempted JSON marshaling
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
//	err := pipeline.Run(ctx, convert.ToJSONSchema(task), &result)
func ToJSONSchema(data any) calque.InputConverter {
	return &SchemaInputConverter{
		data: data,
	}
}

// FromJSONSchema creates an output converter that validates JSON against schema.
//
// Input: pointer to target variable for unmarshaling, generic type parameter for validation
// Output: calque.OutputConverter for pipeline output position
// Behavior: BUFFERED - reads entire JSON stream, validates against schema, unmarshals to target
//
// Parses JSON data that may contain embedded schema information and unmarshals
// to the specified target type. Handles both schema-embedded format (from ToJSONSchema)
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
//	err := pipeline.Run(ctx, schemaInput, convert.FromJSONSchema[Task](&task))
//	fmt.Printf("Task: %s priority, %d hours\n", task.Priority, task.Hours)
func FromJSONSchema[T any](target any) calque.OutputConverter {
	return &SchemaOutputConverter[T]{
		target: target,
	}
}

// ToReader implements inputConverter interface
func (j *SchemaInputConverter) ToReader() (io.Reader, error) {
	// Get the struct type and value
	val := reflect.ValueOf(j.data)
	typ := val.Type()

	// Handle pointers
	if typ.Kind() == reflect.Pointer {
		if val.IsNil() {
			return nil, calque.NewErr(context.Background(), "input is nil pointer")
		}
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.String {
		// If input is a string, pass it through as-is
		return strings.NewReader(j.data.(string)), nil
	}

	if typ.Kind() != reflect.Struct {
		return nil, calque.NewErr(context.Background(), fmt.Sprintf("unsupported jsonschema input type: %T", j.data))
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
		return nil, calque.WrapErr(context.Background(), err, "failed to marshal JSON with schema")
	}

	return bytes.NewReader(jsonBytes), nil
}

// FromReader implements outputConverter interface
func (j *SchemaOutputConverter[T]) FromReader(reader io.Reader) error {
	// Buffer for fallback if direct decode fails
	var buf bytes.Buffer
	teeReader := io.TeeReader(reader, &buf)

	// Try direct streaming decode first (fast path for common case)
	decoder := json.NewDecoder(teeReader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(j.target); err == nil {
		// Drain any remaining data in the reader to prevent pipe deadlock
		io.Copy(io.Discard, reader)
		return nil // Success - pure streaming, no marshal/unmarshal overhead!
	}

	// Direct streaming failed, drain the teeReader to get all data into buffer
	io.Copy(io.Discard, teeReader)

	// Use buffered data for wrapper logic
	var wrapper map[string]any
	if err := json.Unmarshal(buf.Bytes(), &wrapper); err != nil {
		return calque.WrapErr(context.Background(), err, "failed to parse JSON")
	}

	// Get the struct type name to find the correct wrapper key
	var zeroT T
	structName := strings.ToLower(reflect.TypeOf(zeroT).Name())

	// Extract the actual data from under the struct name key
	actualData, exists := wrapper[structName]
	if !exists {
		return calque.NewErr(context.Background(), fmt.Sprintf("expected wrapper key '%s' not found in JSON", structName))
	}

	// Marshal the actual data back to bytes and unmarshal to the target struct
	actualBytes, err := json.Marshal(actualData)
	if err != nil {
		return calque.WrapErr(context.Background(), err, "failed to re-marshal actual data")
	}

	// Unmarshal directly into the target
	if err := json.Unmarshal(actualBytes, j.target); err != nil {
		return calque.WrapErr(context.Background(), err, "failed to unmarshal JSON")
	}

	return nil
}
