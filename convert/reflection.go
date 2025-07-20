package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
)

// Reflection converter
var Reflection core.Converter = reflectionConverter{}

type reflectionConverter struct{}

func (reflectionConverter) ToReader(input any) (io.Reader, error) {
	if input == nil {
		return strings.NewReader(""), nil
	}

	// Check if it's a basic type that can be converted to string easily
	value := reflect.ValueOf(input)
	kind := value.Kind()

	switch kind {
	case reflect.String:
		// Core should handle this, but just in case
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
		data, err := json.Marshal(input)
		if err != nil {
			// Ultimate fallback
			str := fmt.Sprintf("%+v", input)
			return strings.NewReader(str), nil
		}
		return bytes.NewReader(data), nil
	}
}

func (reflectionConverter) FromReader(reader io.Reader) (any, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Try JSON first
	var result any
	if err := json.Unmarshal(data, &result); err == nil {
		return result, nil
	}

	// Return as string
	return string(data), nil
}
