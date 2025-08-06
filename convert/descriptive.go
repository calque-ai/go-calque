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

// Input converter for descriptive data -> formatted bytes with descriptions
type descriptiveInputConverter struct {
	data         any
	outputSchema any
	format       string // "yaml" or "xml"
	tagName      string // "yaml" or "xml"
}

// Output converter for formatted bytes -> descriptive data
type descriptiveOutputConverter[T any] struct {
	target  any
	format  string // "yaml" or "xml"
	tagName string // "yaml" or "xml"
}

// ToDescYaml creates an input converter: ToDescYaml(data)
// Handles structs with yaml tags and desc tags -> YAML bytes with comments
// Output schema is automatically discovered from pipeline output converter
func ToDescYaml(data any) *descriptiveInputConverter {
	return &descriptiveInputConverter{
		data:    data,
		format:  "yaml",
		tagName: "yaml",
	}
}

// ToDescXml creates an input converter: ToDescXml(data)
// Handles structs with xml tags and desc tags -> XML bytes with comments
// Output schema is automatically discovered from pipeline output converter
func ToDescXml(data any) *descriptiveInputConverter {
	return &descriptiveInputConverter{
		data:    data,
		format:  "xml",
		tagName: "xml",
	}
}

// FromDescYaml creates an output converter: FromDescYaml[T](&target)
// Handles YAML bytes -> target struct
func FromDescYaml[T any](target any) *descriptiveOutputConverter[T] {
	return &descriptiveOutputConverter[T]{
		target:  target,
		format:  "yaml",
		tagName: "yaml",
	}
}

// FromDescXml creates an output converter: FromDescXml[T](&target)
// Handles XML bytes -> target struct
func FromDescXml[T any](target any) *descriptiveOutputConverter[T] {
	return &descriptiveOutputConverter[T]{
		target:  target,
		format:  "xml",
		tagName: "xml",
	}
}

// SchemaProvider interface - provides schema instance
func (s *descriptiveOutputConverter[T]) GetSchema() any {
	var zero T
	return zero // Return zero value directly, no reflection needed
}

// InputConverter interface
func (s *descriptiveInputConverter) ToReader() (io.Reader, error) {
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

// SchemaConsumer interface - accepts schema instance
func (s *descriptiveInputConverter) SetOutputSchema(schema any) {
	// Direct assignment of zero value instance
	s.outputSchema = schema
}

func (s *descriptiveOutputConverter[T]) FromReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read descriptive data: %w", err)
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
func (s *descriptiveInputConverter) handleSliceInput(typ reflect.Type) (io.Reader, error) {
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
func (s *descriptiveInputConverter) generateFormattedOutput(structInfo *structInfo, typ reflect.Type) ([]byte, error) {
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
func (s *descriptiveInputConverter) extractStructInfo(typ reflect.Type, val reflect.Value) (*structInfo, error) {
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
func (s *descriptiveInputConverter) processStructField(field reflect.StructField, fieldVal reflect.Value) (structFieldInfo, bool) {
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
func (s *descriptiveInputConverter) parseFieldNameFromTag(tagValue, defaultName string) string {
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
func (s *descriptiveInputConverter) generateYAMLWithDesc(info *structInfo, originalType reflect.Type) ([]byte, error) {
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
func (s *descriptiveInputConverter) generateYAMLComments(builder *strings.Builder, info *structInfo, originalType reflect.Type) {
	// Add type metadata as comments
	builder.WriteString("# Input Type: ")
	builder.WriteString(info.Name)
	builder.WriteByte('\n')

	if originalType.Kind() == reflect.Slice {
		builder.WriteString("# Format: slice of ")
		builder.WriteString(info.Name)
		builder.WriteByte('\n')
	}

	// Add field descriptions as comments
	if len(info.Fields) > 0 {
		builder.WriteString("# Input Field descriptions:\n")
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

	// Add output schema information if provided
	if s.outputSchema != nil {
		s.generateOutputSchemaComments(builder)
	}
}

// generateOutputSchemaComments generates comments for the expected output schema
func (s *descriptiveInputConverter) generateOutputSchemaComments(builder *strings.Builder) {
	val := reflect.ValueOf(s.outputSchema)
	typ := val.Type()

	// Handle pointers
	if typ.Kind() == reflect.Ptr {
		if !val.IsNil() {
			val = val.Elem()
		} else {
			val = reflect.New(typ.Elem()).Elem()
		}
		typ = typ.Elem()
	}

	// Handle non-struct types
	if typ.Kind() != reflect.Struct {
		builder.WriteString("# Expected Output Type: ")
		builder.WriteString(typ.String())
		builder.WriteString("\n\n")
		return
	}

	// Extract output schema info
	outputInfo, err := s.extractStructInfo(typ, val)
	if err != nil {
		builder.WriteString("# Expected Output: (unable to parse schema)\n\n")
		return
	}

	builder.WriteString("# REQUIRED OUTPUT FORMAT: YAML\n")
	builder.WriteString("# IMPORTANT: Do NOT wrap your response in ```yaml or ``` code blocks\n")
	builder.WriteString("# IMPORTANT: Return raw YAML only, no markdown formatting\n")
	builder.WriteString("# IMPORTANT: Arrays must use YAML list format with dashes (-), NOT comma-separated strings\n")
	builder.WriteString("# Expected Output Type: ")
	builder.WriteString(outputInfo.Name)
	builder.WriteByte('\n')
	builder.WriteString("# \n")
	builder.WriteString("# Please respond with YAML in exactly this structure:\n")

	if len(outputInfo.Fields) > 0 {
		// Generate example YAML structure
		builder.WriteString("# \n")
		for _, field := range outputInfo.Fields {
			builder.WriteString("# ")
			builder.WriteString(field.FieldName)
			builder.WriteString(": ")

			// Add type-specific example based on field type
			exampleValue := s.getExampleValue(field.Type)
			builder.WriteString(exampleValue)

			if field.Description != "" {
				builder.WriteString("  # ")
				builder.WriteString(field.Description)
			}
			builder.WriteByte('\n')
		}
		builder.WriteString("# \n")
		builder.WriteString("# For array fields, use this exact format:\n")
		builder.WriteString("# field_name:\n")
		builder.WriteString("#   - \"first item\"\n")
		builder.WriteString("#   - \"second item\"\n")
		builder.WriteString("# \n")
		builder.WriteString("# Field descriptions:\n")
		for _, field := range outputInfo.Fields {
			builder.WriteString("# - ")
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

// getExampleValue returns a type-appropriate example value for YAML structure demonstration
func (s *descriptiveInputConverter) getExampleValue(fieldType string) string {
	switch {
	case strings.Contains(fieldType, "string"):
		return "\"example_value\""
	case strings.Contains(fieldType, "int") || strings.Contains(fieldType, "float"):
		return "123"
	case strings.Contains(fieldType, "bool"):
		return "true"
	case strings.Contains(fieldType, "[]string"):
		return "\n#   - \"item1\"\n#   - \"item2\"  # MUST be array format, not comma-separated string"
	case strings.Contains(fieldType, "[]"):
		return "\n#   - item1\n#   - item2  # MUST be array format, not comma-separated string"
	default:
		return "\"value\""
	}
}

// getExampleValueXML returns a type-appropriate example value for XML structure demonstration
func (s *descriptiveInputConverter) getExampleValueXML(fieldType string) string {
	switch {
	case strings.Contains(fieldType, "string"):
		return "example_value"
	case strings.Contains(fieldType, "int") || strings.Contains(fieldType, "float"):
		return "123"
	case strings.Contains(fieldType, "bool"):
		return "true"
	case strings.Contains(fieldType, "[]"):
		return "item1, item2" // XML arrays are typically represented differently
	default:
		return "value"
	}
}

// prepareDataForMarshaling prepares data for YAML/XML marshaling
func (s *descriptiveInputConverter) prepareDataForMarshaling(info *structInfo, originalType reflect.Type) (any, error) {
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
func (s *descriptiveInputConverter) generateXMLWithDesc(info *structInfo, originalType reflect.Type) ([]byte, error) {
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
func (s *descriptiveInputConverter) generateXMLComments(builder *strings.Builder, info *structInfo, originalType reflect.Type) {
	// Write type metadata as comments
	builder.WriteString("<!-- Input Type: ")
	builder.WriteString(info.Name)
	builder.WriteString(" -->\n")

	if originalType.Kind() == reflect.Slice {
		builder.WriteString("<!-- Format: slice of ")
		builder.WriteString(info.Name)
		builder.WriteString(" -->\n")
	}

	// Write field descriptions as comments
	if len(info.Fields) > 0 {
		builder.WriteString("<!-- Input Field descriptions: -->\n")
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

	// Add output schema information if provided
	if s.outputSchema != nil {
		s.generateOutputSchemaXMLComments(builder)
	}
}

// generateOutputSchemaXMLComments generates XML comments for the expected output schema
func (s *descriptiveInputConverter) generateOutputSchemaXMLComments(builder *strings.Builder) {
	val := reflect.ValueOf(s.outputSchema)
	typ := val.Type()

	// Handle pointers
	if typ.Kind() == reflect.Ptr {
		if !val.IsNil() {
			val = val.Elem()
		} else {
			val = reflect.New(typ.Elem()).Elem()
		}
		typ = typ.Elem()
	}

	// Handle non-struct types
	if typ.Kind() != reflect.Struct {
		builder.WriteString("<!-- Expected Output Type: ")
		builder.WriteString(typ.String())
		builder.WriteString(" -->\n")
		return
	}

	// Extract output schema info
	outputInfo, err := s.extractStructInfo(typ, val)
	if err != nil {
		builder.WriteString("<!-- Expected Output: (unable to parse schema) -->\n")
		return
	}

	builder.WriteString("<!-- REQUIRED OUTPUT FORMAT: XML (not JSON) -->\n")
	builder.WriteString("<!-- IMPORTANT: Do NOT wrap your response in ```xml or ``` code blocks -->\n")
	builder.WriteString("<!-- IMPORTANT: Return raw XML only, no markdown formatting -->\n")
	builder.WriteString("<!-- Expected Output Type: ")
	builder.WriteString(outputInfo.Name)
	builder.WriteString(" -->\n")
	builder.WriteString("<!-- \n")
	builder.WriteString("Please respond with XML in exactly this structure:\n")

	if len(outputInfo.Fields) > 0 {
		// Generate example XML structure
		builder.WriteString("\n<")
		builder.WriteString(outputInfo.Name)
		builder.WriteString(">\n")

		for _, field := range outputInfo.Fields {
			builder.WriteString("  <")
			builder.WriteString(field.FieldName)
			builder.WriteString(">")

			// Add type-specific example based on field type
			exampleValue := s.getExampleValueXML(field.Type)
			builder.WriteString(exampleValue)

			builder.WriteString("</")
			builder.WriteString(field.FieldName)
			builder.WriteString(">")

			if field.Description != "" {
				builder.WriteString("  <!-- ")
				builder.WriteString(field.Description)
				builder.WriteString(" -->")
			}
			builder.WriteString("\n")
		}

		builder.WriteString("</")
		builder.WriteString(outputInfo.Name)
		builder.WriteString(">\n")
		builder.WriteString("\n")
		builder.WriteString("Field descriptions:\n")

		for _, field := range outputInfo.Fields {
			builder.WriteString("- ")
			builder.WriteString(field.FieldName)
			builder.WriteString(": ")
			if field.Description != "" {
				builder.WriteString(field.Description)
			}
			builder.WriteString("\n")
		}
		builder.WriteString(" -->\n")
	}
}

func (s *descriptiveOutputConverter[T]) unmarshal(data []byte, target any) error {
	switch s.format {
	case "yaml":
		return yaml.Unmarshal(data, target)
	case "xml":
		return xml.Unmarshal(data, target)
	default:
		return fmt.Errorf("unsupported format: %s", s.format)
	}
}
