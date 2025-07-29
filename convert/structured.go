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
	// Get the struct type and value
	val := reflect.ValueOf(s.data)
	typ := val.Type()

	// Handle pointers
	if typ.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("invalid input: nil pointer")
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	// Handle different input types
	switch typ.Kind() {
	case reflect.String:
		dataStr, ok := s.data.(string)
		if !ok {
			return nil, fmt.Errorf("invalid input: expected string, got %T", s.data)
		}
		return strings.NewReader(dataStr), nil
	case reflect.Slice:
		return s.handleSliceInput(typ)
	case reflect.Struct:
		// Continue to struct processing below
	default:
		return nil, fmt.Errorf("invalid input: unsupported type %T", s.data)
	}

	// Extract struct information
	structInfo, err := s.extractStructInfo(typ, val)
	if err != nil {
		return nil, err
	}

	// Generate formatted output
	output, err := s.generateFormattedOutput(structInfo, typ)
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

	// comments are ignored by parsers
	err = s.unmarshal(data, s.target)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", s.format, err)
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

// handleSliceInput processes slice types using description-aware generation
func (s *structuredInputConverter) handleSliceInput(typ reflect.Type) (io.Reader, error) {
	if typ.Kind() != reflect.Slice {
		return nil, fmt.Errorf("invalid input: expected slice, got %s", typ.Kind())
	}

	elemType := typ.Elem()

	// Handle pointer elements
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	if elemType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("invalid slice: elements must be structs, got %s", elemType.Kind())
	}

	// Extract struct info from element type (use zero value for schema)
	sampleElem := reflect.New(elemType).Elem()
	structInfo, err := s.extractStructInfo(elemType, sampleElem)
	if err != nil {
		return nil, err
	}

	// Generate formatted output
	output, err := s.generateFormattedOutput(structInfo, typ)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(output), nil
}

// generateFormattedOutput generates formatted output with descriptions
func (s *structuredInputConverter) generateFormattedOutput(structInfo *structInfo, typ reflect.Type) ([]byte, error) {
	switch s.format {
	case "yaml":
		return s.generateYAMLWithDesc(structInfo, typ)
	case "xml":
		return s.generateXMLWithDesc(structInfo, typ)
	default:
		return nil, fmt.Errorf("invalid format: %s", s.format)
	}
}

// extractStructInfo uses reflection to extract struct field information
func (s *structuredInputConverter) extractStructInfo(typ reflect.Type, val reflect.Value) (*structInfo, error) {
	// Validate input types
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("invalid input: expected struct, got %s", typ.Kind())
	}
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("invalid input: value type mismatch")
	}

	// Handle anonymous structs gracefully
	structName := typ.Name()
	if structName == "" {
		structName = "anonymous"
	}

	info := &structInfo{
		Name:   strings.ToLower(structName),
		Fields: []structFieldInfo{},
	}

	// Process each field using helper method
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		fieldInfo, shouldInclude := s.processStructField(field, fieldVal)
		if shouldInclude {
			info.Fields = append(info.Fields, fieldInfo)
		}
	}

	if len(info.Fields) == 0 {
		return nil, fmt.Errorf("invalid struct: no fields with %s tags", s.tagName)
	}

	return info, nil
}

// processStructField processes a single struct field and returns field info and whether to include it
func (s *structuredInputConverter) processStructField(field reflect.StructField, fieldVal reflect.Value) (structFieldInfo, bool) {
	// Skip unexported fields
	if !field.IsExported() {
		return structFieldInfo{}, false
	}

	// Get tag value for the specified tag type
	tagValue := field.Tag.Get(s.tagName)
	if tagValue == "" || tagValue == "-" {
		return structFieldInfo{}, false
	}

	// Parse field name from tag
	fieldName := s.parseFieldNameFromTag(tagValue, field.Name)

	// Get description from desc tag (optional)
	description := field.Tag.Get("desc")

	// Get field value safely
	var value any
	if fieldVal.CanInterface() {
		value = fieldVal.Interface()
	}

	return structFieldInfo{
		Name:        field.Name,
		FieldName:   fieldName,
		Description: description,
		Value:       value,
		Type:        field.Type.String(),
	}, true
}

// parseFieldNameFromTag extracts field name from tag value (handles "name,omitempty" format)
func (s *structuredInputConverter) parseFieldNameFromTag(tagValue, defaultName string) string {
	if tagValue == "" {
		return strings.ToLower(defaultName)
	}

	// Find first comma
	for i, c := range tagValue {
		if c == ',' {
			if i == 0 {
				return strings.ToLower(defaultName)
			}
			return tagValue[:i]
		}
	}

	return tagValue
}

// generateYAMLWithDesc creates YAML with description comments
func (s *structuredInputConverter) generateYAMLWithDesc(info *structInfo, originalType reflect.Type) ([]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("invalid input: structInfo is nil")
	}
	if originalType == nil {
		return nil, fmt.Errorf("invalid input: originalType is nil")
	}

	var builder strings.Builder
	s.generateYAMLComments(&builder, info, originalType)

	// Prepare data for marshaling
	dataToMarshal, err := s.prepareDataForMarshaling(info, originalType)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare data: %w", err)
	}

	// Marshal the data
	yamlBytes, err := yaml.Marshal(dataToMarshal)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Add the marshaled data
	builder.WriteString(string(yamlBytes))

	return []byte(builder.String()), nil
}

// generateYAMLComments generates YAML comments
func (s *structuredInputConverter) generateYAMLComments(builder *strings.Builder, info *structInfo, originalType reflect.Type) {
	// Add type metadata as comments
	builder.WriteString("# Type: ")
	builder.WriteString(info.Name)
	builder.WriteByte('\n')

	if originalType.Kind() == reflect.Slice {
		builder.WriteString("# Format: slice of ")
		builder.WriteString(info.Name)
		builder.WriteByte('\n')
	}

	// Add field descriptions as comments
	if len(info.Fields) > 0 {
		builder.WriteString("# Field descriptions:\n")
		for _, field := range info.Fields {
			builder.WriteString("# ")
			builder.WriteString(field.FieldName)
			builder.WriteString(": ")
			if field.Description != "" {
				builder.WriteString(field.Description)
			}
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	}
}

// prepareDataForMarshaling prepares data for YAML/XML marshaling
func (s *structuredInputConverter) prepareDataForMarshaling(info *structInfo, originalType reflect.Type) (any, error) {
	if originalType.Kind() == reflect.Slice {
		// For slices, use data directly
		return s.data, nil
	}

	// For structs, build field data with pre-sized map
	fieldData := make(map[string]any, len(info.Fields))
	for _, field := range info.Fields {
		fieldData[field.FieldName] = field.Value
	}
	return fieldData, nil
}

// generateXMLWithDesc creates XML with description comments
func (s *structuredInputConverter) generateXMLWithDesc(info *structInfo, originalType reflect.Type) ([]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("invalid input: structInfo is nil")
	}
	if originalType == nil {
		return nil, fmt.Errorf("invalid input: originalType is nil")
	}

	var xmlBuilder strings.Builder

	// Write XML declaration
	xmlBuilder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	xmlBuilder.WriteByte('\n')

	// Generate comments
	s.generateXMLComments(&xmlBuilder, info, originalType)

	var xmlBytes []byte
	var err error

	if originalType.Kind() == reflect.Slice {
		// For slices, create a simple wrapper struct for XML root element
		wrapper := struct {
			Items any `xml:"items"`
		}{Items: s.data}
		xmlBytes, err = xml.Marshal(wrapper)
	} else {
		// For structs, marshal the original data directly
		xmlBytes, err = xml.Marshal(s.data)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal XML: %w", err)
	}

	xmlBuilder.Write(xmlBytes)
	return []byte(xmlBuilder.String()), nil
}

// generateXMLComments generates XML comments
func (s *structuredInputConverter) generateXMLComments(builder *strings.Builder, info *structInfo, originalType reflect.Type) {
	// Write type metadata as comments
	builder.WriteString("<!-- Type: ")
	builder.WriteString(info.Name)
	builder.WriteString(" -->\n")

	if originalType.Kind() == reflect.Slice {
		builder.WriteString("<!-- Format: slice of ")
		builder.WriteString(info.Name)
		builder.WriteString(" -->\n")
	}

	// Write field descriptions as comments
	if len(info.Fields) > 0 {
		builder.WriteString("<!-- Field descriptions: -->\n")
		for _, field := range info.Fields {
			builder.WriteString("<!-- ")
			builder.WriteString(field.FieldName)
			builder.WriteString(": ")
			if field.Description != "" {
				builder.WriteString(field.Description)
			}
			builder.WriteString(" -->\n")
		}
	}
}

func (s *structuredOutputConverter[T]) unmarshal(data []byte, target any) error {
	switch s.format {
	case "yaml":
		return yaml.Unmarshal(data, target)
	case "xml":
		return xml.Unmarshal(data, target)
	default:
		return fmt.Errorf("unsupported format: %s", s.format)
	}
}
