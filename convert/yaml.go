package convert

import (
	"bytes"
	"io"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/goccy/go-yaml"
)

// YAML converter
var YAML core.Converter = yamlConverter{}

type yamlConverter struct{}

func (yamlConverter) ToReader(input any) (io.Reader, error) {
	switch v := input.(type) {
	case map[any]any:
		// YAML-specific map type
		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	default:
		return nil, core.ErrUnsupportedType
	}
}

func (yamlConverter) FromReader(reader io.Reader) (any, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var result any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return string(data), nil
	}

	return result, nil
}
