package convert

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
)

// Input converter for YAML data -> YAML bytes
type yamlInputConverter struct {
	data any
}

// Output converter for YAML bytes -> any type
type yamlOutputConverter struct {
	target any
}

// Yaml creates an input converter: converter.Yaml(data)
// Handles: map[string]any, map[any]any, []any, yaml string, yaml []byte -> YAML bytes
func Yaml(data any) *yamlInputConverter {
	return &yamlInputConverter{data: data}
}

// YamlOutput creates an output converter: converter.YamlOutput(&target)
// Handles: YAML bytes -> target ([]byte, string, or any unmarshallable type)
func YamlOutput(target any) *yamlOutputConverter {
	return &yamlOutputConverter{target: target}
}

// InputConverter interface
func (y *yamlInputConverter) ToReader() (io.Reader, error) {
	switch v := y.data.(type) {
	case map[string]any, map[any]any, []any:
		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal YAML: %w", err)
		}
		return bytes.NewReader(data), nil
	case string:
		// Validate it's valid YAML
		var temp any
		if err := yaml.Unmarshal([]byte(v), &temp); err != nil {
			return nil, fmt.Errorf("invalid YAML string: %w", err)
		}
		return strings.NewReader(v), nil
	case []byte:
		// Validate it's valid YAML
		var temp any
		if err := yaml.Unmarshal(v, &temp); err != nil {
			return nil, fmt.Errorf("invalid YAML bytes: %w", err)
		}
		return bytes.NewReader(v), nil
	default:
		return nil, fmt.Errorf("unsupported YAML input type: %T", v)
	}
}

// OutputConverter interface
func (y *yamlOutputConverter) FromReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read YAML data: %w", err)
	}

	// Unmarshal directly into the target
	if err := yaml.Unmarshal(data, y.target); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return nil
}
