package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/goccy/go-yaml"
)

// StructuredYAML creates a converter for structs with yaml tags and desc tags
func StructuredYAML[T any]() core.Converter {
	return &structuredConverter[T]{
		format:  "yaml",
		tagName: "yaml",
	}
}

// StructuredJSON creates a converter for structs with json tags and desc tags
func StructuredJSON[T any]() core.Converter {
	return &structuredConverter[T]{
		format:  "json",
		tagName: "json",
	}
}

type structuredConverter[T any] struct {
	format  string // "yaml" or "json"
	tagName string // "yaml" or "json"
}

// ToReader converts a struct to formatted bytes with descriptions
func (s *structuredConverter[T]) ToReader(input any) (io.Reader, error) {
	fmt.Printf("DEBUG ToReader: input type=%T, value=%+v\n", input, input)

	// Get the struct type and value
	val := reflect.ValueOf(input)
	typ := reflect.TypeOf(input)

	// Handle pointers
	if typ.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("input is nil pointer")
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		// If input is a string, pass it through as-is for parsing in FromReader
		if typ.Kind() == reflect.String {
			return strings.NewReader(input.(string)), nil
		}
		return nil, core.ErrUnsupportedType
	}

	// Extract struct information
	structInfo, err := s.extractStructInfo(typ, val)
	if err != nil {
		return nil, err
	}

	// Generate formatted output with descriptions
	var output []byte
	switch s.format {
	case "yaml":
		output, err = s.generateYAMLWithDesc(structInfo)
	case "json":
		output, err = s.generateJSONWithDesc(structInfo)
	default:
		return nil, fmt.Errorf("unsupported format: %s", s.format)
	}

	if err != nil {
		return nil, err
	}

	return bytes.NewReader(output), nil
}

// FromReader parses formatted data back into a struct
func (s *structuredConverter[T]) FromReader(reader io.Reader) (any, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// First, unmarshal into a map to handle the root wrapper key
	var wrapper map[string]any

	switch s.format {
	case "yaml":
		err = yaml.Unmarshal(data, &wrapper)
	case "json":
		err = json.Unmarshal(data, &wrapper)
	default:
		return nil, fmt.Errorf("unsupported format: %s", s.format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse %s wrapper: %w", s.format, err)
	}

	// Get the struct type name to find the correct wrapper key
	var zeroT T
	structName := strings.ToLower(reflect.TypeOf(zeroT).Name())

	// Extract the actual data from under the struct name key
	actualData, exists := wrapper[structName]
	if !exists {
		return nil, fmt.Errorf("expected wrapper key '%s' not found in %s", structName, s.format)
	}

	// Marshal the actual data back to bytes and unmarshal to the target struct
	var actualBytes []byte
	switch s.format {
	case "yaml":
		actualBytes, err = yaml.Marshal(actualData)
	case "json":
		actualBytes, err = json.Marshal(actualData)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal actual data: %w", err)
	}

	var result T
	switch s.format {
	case "yaml":
		err = yaml.Unmarshal(actualBytes, &result)
	case "json":
		err = json.Unmarshal(actualBytes, &result)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", s.format, err)
	}

	return result, nil
}

// structFieldInfo holds information about a struct field
type structFieldInfo struct {
	Name        string
	JSONName    string
	Description string
	Value       any
	Type        string
}

// structInfo holds information about the entire struct
type structInfo struct {
	Name   string
	Fields []structFieldInfo
}

// extractStructInfo uses reflection to extract struct field information
func (s *structuredConverter[T]) extractStructInfo(typ reflect.Type, val reflect.Value) (*structInfo, error) {
	info := &structInfo{
		Name:   strings.ToLower(typ.Name()),
		Fields: []structFieldInfo{},
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get tag value for the specified tag type
		tagValue := field.Tag.Get(s.tagName)
		if tagValue == "" || tagValue == "-" {
			continue // Skip fields without the required tag
		}

		// Parse tag (handle "name,omitempty" format)
		tagParts := strings.Split(tagValue, ",")
		jsonName := tagParts[0]
		if jsonName == "" {
			jsonName = strings.ToLower(field.Name)
		}

		// Get description from desc tag or generate it
		description := field.Tag.Get("desc")
		if description == "" {
			description = s.generateDescription(field.Name, jsonName)
		}

		// Get field value
		var value any
		if fieldVal.CanInterface() {
			value = fieldVal.Interface()
		}

		info.Fields = append(info.Fields, structFieldInfo{
			Name:        field.Name,
			JSONName:    jsonName,
			Description: description,
			Value:       value,
			Type:        field.Type.String(),
		})
	}

	if len(info.Fields) == 0 {
		return nil, fmt.Errorf("struct has no fields with %s tags", s.tagName)
	}

	return info, nil
}

// generateDescription creates a description when desc tag is missing
func (s *structuredConverter[T]) generateDescription(fieldName, jsonName string) string {
	// Use jsonName if it's different and more descriptive
	name := jsonName
	if jsonName == strings.ToLower(fieldName) {
		name = fieldName
	}

	// Convert camelCase/snake_case to human readable
	humanName := s.humanize(name)

	// Generate description
	return fmt.Sprintf("The %s", humanName)
}

// humanize converts various naming conventions to human-readable text
func (s *structuredConverter[T]) humanize(name string) string {
	// Convert snake_case to spaces
	name = strings.ReplaceAll(name, "_", " ")

	// Convert kebab-case to spaces
	name = strings.ReplaceAll(name, "-", " ")

	// Convert camelCase to spaces (simple approach)
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	name = re.ReplaceAllString(name, `$1 $2`)

	// Convert to lowercase and handle special cases
	name = strings.ToLower(name)

	// Handle common abbreviations
	name = strings.ReplaceAll(name, " id", " ID")
	name = strings.ReplaceAll(name, " url", " URL")
	name = strings.ReplaceAll(name, " api", " API")
	name = strings.ReplaceAll(name, " http", " HTTP")

	return name
}

// generateYAMLWithDesc creates YAML with description comments
func (s *structuredConverter[T]) generateYAMLWithDesc(info *structInfo) ([]byte, error) {
	// First, create the data structure for YAML marshaling
	data := make(map[string]any)
	fieldData := make(map[string]any)

	for _, field := range info.Fields {
		fieldData[field.JSONName] = field.Value
	}
	data[info.Name] = fieldData

	// Marshal to get proper YAML structure
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Now add comments by parsing the YAML and adding descriptions
	yamlStr := string(yamlBytes)
	lines := strings.Split(yamlStr, "\n")

	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check if this line contains a field we have a description for
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, ":") && !strings.HasSuffix(trimmed, ":") {
			// This is a field line, try to add description
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				fieldName := strings.TrimSpace(parts[0])

				// Find description for this field
				for _, field := range info.Fields {
					if field.JSONName == fieldName {
						line = line + "  # " + field.Description
						break
					}
				}
			}
		}

		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n")), nil
}

// generateJSONWithDesc creates JSON (descriptions would need to be in schema)
func (s *structuredConverter[T]) generateJSONWithDesc(info *structInfo) ([]byte, error) {
	// For JSON, we'll create a structure with data and schema
	data := make(map[string]any)
	schema := make(map[string]map[string]string)

	// Build data object
	fieldData := make(map[string]any)
	for _, field := range info.Fields {
		fieldData[field.JSONName] = field.Value

		// Build schema
		schema[field.JSONName] = map[string]string{
			"type":        s.getJSONType(field.Value),
			"description": field.Description,
		}
	}

	data[info.Name] = fieldData
	data["$schema"] = schema

	return json.MarshalIndent(data, "", "  ")
}

// formatYAMLValue formats a Go value for YAML output
func (s *structuredConverter[T]) formatYAMLValue(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v), nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case []string:
		// Handle string slices
		quoted := make([]string, len(v))
		for i, s := range v {
			quoted[i] = fmt.Sprintf("%q", s)
		}
		return fmt.Sprintf("[%s]", strings.Join(quoted, ", ")), nil
	case nil:
		return "null", nil
	default:
		// For complex types, use YAML marshaling
		yamlBytes, err := yaml.Marshal(v)
		if err != nil {
			return "", err
		}
		// Remove trailing newline and indent properly
		yamlStr := strings.TrimSpace(string(yamlBytes))
		return yamlStr, nil
	}
}

// getJSONType returns the JSON schema type for a Go value
func (s *structuredConverter[T]) getJSONType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []any, []string, []int:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return "string"
	}
}
