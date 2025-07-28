package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Input converter for JSON data -> JSON bytes
type jsonInputConverter struct {
	data any
}

// Output converter for JSON bytes -> any type
type jsonOutputConverter struct {
	target any
}

// Json creates an input converter: converter.Json(data)
// Handles: map[string]any, []any, json string, json []byte -> JSON bytes
func Json(data any) *jsonInputConverter {
	return &jsonInputConverter{data: data}
}

// JsonOutput creates an output converter: converter.JsonOutput(&target)
// Handles: JSON bytes -> target ([]byte, string, or any unmarshallable type)
func JsonOutput(target any) *jsonOutputConverter {
	return &jsonOutputConverter{target: target}
}

// InputConverter interface
func (j *jsonInputConverter) ToReader() (io.Reader, error) {
	switch v := j.data.(type) {
	case map[string]any, []any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %w", err)
		}
		return bytes.NewReader(data), nil
	case string: // a string that is valid JSON
		var temp any
		if err := json.Unmarshal([]byte(v), &temp); err != nil {
			return nil, fmt.Errorf("invalid JSON string: %w", err)
		}
		return strings.NewReader(v), nil
	case []byte: // a byte slice that is valid JSON
		// Validate it's valid JSON first
		var temp any
		if err := json.Unmarshal(v, &temp); err != nil {
			return nil, fmt.Errorf("invalid JSON bytes: %w", err)
		}
		return bytes.NewReader(v), nil
	default:
		return nil, fmt.Errorf("unsupported JSON input type: %T", v)
	}
}

func (j *jsonOutputConverter) FromReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read JSON data: %w", err)
	}

	// Unmarshal directly into the target
	if err := json.Unmarshal(data, j.target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}
