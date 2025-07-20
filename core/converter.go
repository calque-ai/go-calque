package core

import (
	"bytes"
	"errors"
	"io"
	"strings"
)

// Converter interfaces
type InputConverter interface {
	ToReader(input any) (io.Reader, error)
}

type OutputConverter interface {
	FromReader(reader io.Reader) (any, error)
}

type Converter interface {
	InputConverter
	OutputConverter
}

// Common errors
var (
	ErrUnsupportedType  = errors.New("unsupported input type")
	ErrConversionFailed = errors.New("conversion failed")
)

type builtInType int

const (
	typeString builtInType = iota
	typeBytes
	typeReader
	typeConverter // Used converter
)

// inputToReaderBuiltIn handles common types in core
func (f *Flow) inputToReaderBuiltIn(input any, converters ...Converter) (io.Reader, builtInType, error) {
	// Try converter first if provided
	if len(converters) > 0 {
		reader, err := converters[0].ToReader(input)
		if err == nil {
			return reader, typeConverter, nil
		}
		// If converter fails, continue to built-in types
	}

	// Fall back to built-in types
	switch v := input.(type) {
	case string:
		return strings.NewReader(v), typeString, nil
	case []byte:
		return bytes.NewReader(v), typeBytes, nil
	case io.Reader:
		return v, typeReader, nil
	default:
		// FALLBACK: Convert unknown types to string representation
		return strings.NewReader(f.fallbackToString(input)), typeString, nil
	}
}

// readerToOutputBuiltIn converts back using built-in logic or converter
func (f *Flow) readerToOutputBuiltIn(reader io.Reader, inputType builtInType, converters ...Converter) (any, error) {
	// If converter was used for input, use it for output too
	if inputType == typeConverter && len(converters) > 0 {
		return converters[0].FromReader(reader)
	}

	// Handle common output types
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	switch inputType {
	case typeString:
		return string(data), nil
	case typeBytes:
		return data, nil
	case typeReader:
		return bytes.NewReader(data), nil
	default:
		return string(data), nil
	}
}

// fallbackToString provides a basic string representation for unknown types
func (f *Flow) fallbackToString(input any) string {
	// Simple fallback without reflection
	return "unknown_type"
}
