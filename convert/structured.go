package convert

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
)

// Input converter for structured data -> formatted bytes with descriptions
type structuredInputConverter struct {
	data    any
	format  string // "yaml" or "xml"
	tagName string // "yaml" or "xml"
}

// Output converter for formatted bytes -> structured data
type structuredOutputConverter[T any] struct {
	target  any
	format  string // "yaml" or "xml"
	tagName string // "yaml" or "xml"
}

// StructuredYAML creates an input converter: StructuredYAML(data)
// Handles structs with yaml tags and desc tags -> YAML bytes with comments
func StructuredYAML(data any) *structuredInputConverter {
	return &structuredInputConverter{
		data:    data,
		format:  "yaml",
		tagName: "yaml",
	}
}

// StructuredXML creates an input converter: StructuredXML(data)
// Handles structs with xml tags and desc tags -> XML bytes with comments
func StructuredXML(data any) *structuredInputConverter {
	return &structuredInputConverter{
		data:    data,
		format:  "xml",
		tagName: "xml",
	}
}

// StructuredYAMLOutput creates an output converter: StructuredYAMLOutput[T](&target)
// Handles YAML bytes -> target struct
func StructuredYAMLOutput[T any](target any) *structuredOutputConverter[T] {
	return &structuredOutputConverter[T]{
		target:  target,
		format:  "yaml",
		tagName: "yaml",
	}
}

// StructuredXMLOutput creates an output converter: StructuredXMLOutput[T](&target)
// Handles XML bytes -> target struct
func StructuredXMLOutput[T any](target any) *structuredOutputConverter[T] {
	return &structuredOutputConverter[T]{
		target:  target,
		format:  "xml",
		tagName: "xml",
	}
}

// InputConverter interface
func (s *structuredInputConverter) ToReader() (io.Reader, error) {
	fmt.Printf("DEBUG ToReader: input type=%T, value=%+v\n", s.data, s.data)

	// Get the struct type and value
	val := reflect.ValueOf(s.data)
	typ := reflect.TypeOf(s.data)

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
			return strings.NewReader(s.data.(string)), nil
		}
		return nil, fmt.Errorf("unsupported structured input type: %T", s.data)
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
	case "xml":
		output, err = s.generateXMLWithDesc(structInfo)
	default:
		return nil, fmt.Errorf("unsupported format: %s", s.format)
	}

	if err != nil {
		return nil, err
	}

	return bytes.NewReader(output), nil
}

func (s *structuredOutputConverter[T]) FromReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read structured data: %w", err)
	}

	// First, unmarshal into a map to handle the root wrapper key
	var wrapper map[string]any

	switch s.format {
	case "yaml":
		err = yaml.Unmarshal(data, &wrapper)
	case "xml":
		err = xml.Unmarshal(data, &wrapper)
	default:
		return fmt.Errorf("unsupported format: %s", s.format)
	}

	if err != nil {
		return fmt.Errorf("failed to parse %s wrapper: %w", s.format, err)
	}

	// Get the struct type name to find the correct wrapper key
	var zeroT T
	structName := strings.ToLower(reflect.TypeOf(zeroT).Name())

	// Extract the actual data from under the struct name key
	actualData, exists := wrapper[structName]
	if !exists {
		return fmt.Errorf("expected wrapper key '%s' not found in %s", structName, s.format)
	}

	// Marshal the actual data back to bytes and unmarshal to the target struct
	var actualBytes []byte
	switch s.format {
	case "yaml":
		actualBytes, err = yaml.Marshal(actualData)
	case "xml":
		actualBytes, err = xml.Marshal(actualData)
	}

	if err != nil {
		return fmt.Errorf("failed to re-marshal actual data: %w", err)
	}

	// Unmarshal directly into the target
	switch s.format {
	case "yaml":
		err = yaml.Unmarshal(actualBytes, s.target)
	case "xml":
		err = xml.Unmarshal(actualBytes, s.target)
	}

	if err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", s.format, err)
	}

	return nil
}

// structFieldInfo holds information about a struct field
type structFieldInfo struct {
	Name        string
	FieldName   string // XML/YAML field name
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
func (s *structuredInputConverter) extractStructInfo(typ reflect.Type, val reflect.Value) (*structInfo, error) {
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
		fieldName := tagParts[0]
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		// Get description from desc tag (optional)
		description := field.Tag.Get("desc")

		// Get field value
		var value any
		if fieldVal.CanInterface() {
			value = fieldVal.Interface()
		}

		info.Fields = append(info.Fields, structFieldInfo{
			Name:        field.Name,
			FieldName:   fieldName,
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

// generateYAMLWithDesc creates YAML with description comments
func (s *structuredInputConverter) generateYAMLWithDesc(info *structInfo) ([]byte, error) {
	// First, create the data structure for YAML marshaling
	data := make(map[string]any)
	fieldData := make(map[string]any)

	for _, field := range info.Fields {
		fieldData[field.FieldName] = field.Value
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
					if field.FieldName == fieldName && field.Description != "" {
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

// generateXMLWithDesc creates XML with description comments
func (s *structuredInputConverter) generateXMLWithDesc(info *structInfo) ([]byte, error) {
	// Create XML structure with root element
	var xmlBuilder strings.Builder
	
	// Write XML declaration
	xmlBuilder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	
	// Write root element
	xmlBuilder.WriteString(fmt.Sprintf("<%s>\n", info.Name))
	
	// Write fields with descriptions as comments
	for _, field := range info.Fields {
		if field.Description != "" {
			xmlBuilder.WriteString(fmt.Sprintf("  <!-- %s -->\n", field.Description))
		}
		
		// Marshal field value to XML
		fieldXML, err := xml.Marshal(field.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal field %s: %w", field.Name, err)
		}
		
		// Write field element
		xmlBuilder.WriteString(fmt.Sprintf("  <%s>%s</%s>\n", field.FieldName, string(fieldXML), field.FieldName))
	}
	
	// Close root element
	xmlBuilder.WriteString(fmt.Sprintf("</%s>\n", info.Name))
	
	return []byte(xmlBuilder.String()), nil
}

