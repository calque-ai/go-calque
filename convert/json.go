package convert

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
)

// JSON converter - handles JSON data
var JSON core.Converter = jsonConverter{}

type jsonConverter struct{}

func (jsonConverter) ToReader(input any) (io.Reader, error) {
	switch v := input.(type) {
	case map[string]any, []any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	case string: // a string that is valid JSON
		var temp any
		if err := json.Unmarshal([]byte(v), &temp); err != nil {
			return nil, core.ErrUnsupportedType
		}
		return strings.NewReader(v), nil
	case []byte: // a byte slice that is valid JSON
		// Validate it's valid JSON first
		var temp any
		if err := json.Unmarshal(v, &temp); err != nil {
			return nil, core.ErrUnsupportedType
		}
		return bytes.NewReader(v), nil
	default:
		return nil, core.ErrUnsupportedType
	}
}

func (jsonConverter) FromReader(reader io.Reader) (any, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		// Return as string if not valid JSON
		return string(data), nil
	}

	return result, nil
}
