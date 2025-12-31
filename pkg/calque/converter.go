// Package calque provides a flexible data processing framework with flow-based operations.
package calque

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// InputConverter converts data to an io.Reader for processing
type InputConverter interface {
	ToReader() (io.Reader, error)
}

// OutputConverter processes an io.Reader into a target
type OutputConverter interface {
	FromReader(reader io.Reader) error
}

// Converter combines InputConverter and OutputConverter interfaces for bidirectional data conversion.
type Converter interface {
	InputConverter
	OutputConverter
}

// inputToReader converts input to io.Reader
func (f *Flow) inputToReader(input any) (io.Reader, error) {
	// Check if input is a converter with data
	if conv, ok := input.(InputConverter); ok {
		return conv.ToReader()
	}

	// Handle built-in types
	switch v := input.(type) {
	case string:
		return strings.NewReader(v), nil
	case []byte:
		return bytes.NewReader(v), nil
	case io.Reader:
		return v, nil
	default:
		return nil, NewErr(context.Background(), fmt.Sprintf("unsupported input type: %T", input))
	}
}

// readerToOutput writes the final reader data to the output pointer
// Only buffers when necessary
func (f *Flow) readerToOutput(reader io.Reader, output any) error {
	// Check if output is a converter
	if conv, ok := output.(OutputConverter); ok {
		return conv.FromReader(reader)
	}

	// Handle built-in types
	switch outPtr := output.(type) {
	case *io.Reader:
		*outPtr = reader
		return nil

	case *[]byte:
		var buf bytes.Buffer
		_, err := io.Copy(&buf, reader)
		if err != nil {
			return err
		}
		*outPtr = buf.Bytes()
		return nil

	case *string:
		var builder strings.Builder
		_, err := io.Copy(&builder, reader)
		if err != nil {
			return err
		}
		*outPtr = builder.String()
		return nil

	default:
		return NewErr(context.Background(), fmt.Sprintf("unsupported output type: %T (use a converter for complex types)", output))
	}
}

// copyInputToOutput handles the case when there are no handlers
func (f *Flow) copyInputToOutput(input any, output any) error {
	// Check if input and output are the same type
	switch in := input.(type) {
	case string:
		if outPtr, ok := output.(*string); ok {
			*outPtr = in // Direct assignment, no conversion needed
			return nil
		}
	case []byte:
		if outPtr, ok := output.(*[]byte); ok {
			// Make a copy to prevent mutation issues
			*outPtr = make([]byte, len(in))
			copy(*outPtr, in)
			return nil
		}
	case io.Reader:
		if outPtr, ok := output.(*io.Reader); ok {
			*outPtr = in // Direct assignment
			return nil
		}
	}

	// Fall back to streaming conversion
	reader, err := f.inputToReader(input)
	if err != nil {
		return err
	}

	return f.readerToOutput(reader, output)
}
