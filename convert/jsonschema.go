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

// ToJsonSchema creates an input converter: ToJsonSchema(data)
// Handles structs with json and jsonschema tags -> JSON with embedded schema
func ToJsonSchema(data any) *jsonSchemaInputConverter {
	return &jsonSchemaInputConverter{
		data: data,
	}
}

// FromJsonSchema creates an output converter: FromJsonSchema[T](&target)
// Handles JSON with schema -> target struct with validation
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
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read JSON schema data: %w", err)
	}

	// First, unmarshal into a map to handle the root wrapper key
	var wrapper map[string]any
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("failed to parse JSON wrapper: %w", err)
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
