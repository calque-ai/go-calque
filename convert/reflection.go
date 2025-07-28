package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Input converter for any data type -> string/JSON representation
type reflectInputConverter struct {
	data any
}

// Output converter for string/JSON -> any type via reflection
type reflectOutputConverter struct {
	target any
}

// Reflect creates an input converter: converter.Reflect(data)
// Handles: any type -> string representation (primitives) or JSON (complex types)
func Reflect(data any) *reflectInputConverter {
	return &reflectInputConverter{data: data}
}

// ReflectOutput creates an output converter: converter.ReflectOutput(&target)
// Handles: string/JSON -> target type via JSON unmarshaling
func ReflectOutput(target any) *reflectOutputConverter {
	return &reflectOutputConverter{target: target}
}

// InputConverter interface
func (r *reflectInputConverter) ToReader() (io.Reader, error) {
	if r.data == nil {
		return strings.NewReader(""), nil
	}

	// Check if it's a basic type that can be converted to string easily
	value := reflect.ValueOf(r.data)
	kind := value.Kind()

	switch kind {
	case reflect.String:
		return strings.NewReader(value.String()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strings.NewReader(fmt.Sprintf("%d", value.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strings.NewReader(fmt.Sprintf("%d", value.Uint())), nil
	case reflect.Float32, reflect.Float64:
		return strings.NewReader(fmt.Sprintf("%g", value.Float())), nil
	case reflect.Bool:
		return strings.NewReader(fmt.Sprintf("%t", value.Bool())), nil
	default:
		// For complex types, use JSON
		data, err := json.Marshal(r.data)
		if err != nil {
			// Ultimate fallback - use Go's string representation
			str := fmt.Sprintf("%+v", r.data)
			return strings.NewReader(str), nil
		}
		return bytes.NewReader(data), nil
	}
}

// OutputConverter interface
func (r *reflectOutputConverter) FromReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read reflection data: %w", err)
	}

	// Try JSON unmarshaling first
	if err := json.Unmarshal(data, r.target); err != nil {
		// If JSON fails, try to convert string to primitive types via reflection
		str := string(data)
		if err := r.stringToPrimitive(str); err != nil {
			return fmt.Errorf("failed to unmarshal reflection data: %w", err)
		}
	}

	return nil
}

// stringToPrimitive attempts to convert a string to primitive types using reflection
func (r *reflectOutputConverter) stringToPrimitive(str string) error {
	if r.target == nil {
		return fmt.Errorf("target is nil")
	}

	value := reflect.ValueOf(r.target)
	if value.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	elem := value.Elem()
	if !elem.CanSet() {
		return fmt.Errorf("target cannot be set")
	}

	switch elem.Kind() {
	case reflect.String:
		elem.SetString(str)
		return nil
	default:
		return fmt.Errorf("unsupported primitive type conversion for: %T", r.target)
	}
}
