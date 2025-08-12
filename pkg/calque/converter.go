package calque

import (
	"bytes"
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

type Converter interface {
	InputConverter
	OutputConverter
}

// inputToReader converts input to io.Reader
func (f *Pipeline) inputToReader(input any) (io.Reader, error) {
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
		return nil, fmt.Errorf("unsupported input type: %T", input)
	}
}

// readerToOutput writes the final reader data to the output pointer
func (f *Pipeline) readerToOutput(reader io.Reader, output any) error {
	// Check if output is a converter - it handles its own target
	if conv, ok := output.(OutputConverter); ok {
		return conv.FromReader(reader)
	}

	// Read data for built-in types
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Handle built-in types
	switch outPtr := output.(type) {
	case *string:
		*outPtr = string(data)
	case *[]byte:
		*outPtr = data
	case *io.Reader:
		*outPtr = bytes.NewReader(data)
	default:
		return fmt.Errorf("unsupported output type: %T (use a converter for complex types)", output)
	}

	return nil
}

// copyInputToOutput handles the case when there are no handlers
func (f *Pipeline) copyInputToOutput(input any, output any) error {
	// convert input to reader
	reader, err := f.inputToReader(input)
	if err != nil {
		return err
	}

	// Then write reader to output
	return f.readerToOutput(reader, output)
}
