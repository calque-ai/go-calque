package calque

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

const (
	testData     = "test data"
	testDataLong = "this is a longer test string with more content to verify proper handling"
)

// Mock converters for testing
type mockInputConverter struct {
	data string
	err  error
}

func (m *mockInputConverter) ToReader() (io.Reader, error) {
	if m.err != nil {
		return nil, m.err
	}
	return strings.NewReader(m.data), nil
}

type mockOutputConverter struct {
	received string
	err      error
}

func (m *mockOutputConverter) FromReader(reader io.Reader) error {
	if m.err != nil {
		return m.err
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.received = string(data)
	return nil
}

type mockBidirectionalConverter struct {
	*mockInputConverter
	*mockOutputConverter
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, e.err
}

func TestFlow_inputToReader(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expectedOut string
		wantErr     bool
		description string
	}{
		{
			name:        "string_input",
			input:       testData,
			expectedOut: testData,
			wantErr:     false,
			description: "String input should be converted to strings.Reader",
		},
		{
			name:        "empty_string",
			input:       "",
			expectedOut: "",
			wantErr:     false,
			description: "Empty string should work",
		},
		{
			name:        "long_string",
			input:       testDataLong,
			expectedOut: testDataLong,
			wantErr:     false,
			description: "Long string should be handled properly",
		},
		{
			name:        "byte_slice",
			input:       []byte(testData),
			expectedOut: testData,
			wantErr:     false,
			description: "Byte slice should be converted to bytes.Reader",
		},
		{
			name:        "empty_byte_slice",
			input:       []byte{},
			expectedOut: "",
			wantErr:     false,
			description: "Empty byte slice should work",
		},
		{
			name:        "io_reader",
			input:       strings.NewReader(testData),
			expectedOut: testData,
			wantErr:     false,
			description: "io.Reader should be passed through unchanged",
		},
		{
			name:        "input_converter_success",
			input:       &mockInputConverter{data: testData},
			expectedOut: testData,
			wantErr:     false,
			description: "InputConverter should be called and data extracted",
		},
		{
			name:        "input_converter_error",
			input:       &mockInputConverter{err: errors.New("converter error")},
			expectedOut: "",
			wantErr:     true,
			description: "InputConverter error should be propagated",
		},
		{
			name:        "input_converter_empty",
			input:       &mockInputConverter{data: ""},
			expectedOut: "",
			wantErr:     false,
			description: "InputConverter with empty data should work",
		},
		{
			name:        "unsupported_type_int",
			input:       42,
			expectedOut: "",
			wantErr:     true,
			description: "Unsupported type should return error",
		},
		{
			name:        "unsupported_type_struct",
			input:       struct{ Name string }{Name: "test"},
			expectedOut: "",
			wantErr:     true,
			description: "Unsupported struct type should return error",
		},
		{
			name:        "nil_input",
			input:       nil,
			expectedOut: "",
			wantErr:     true,
			description: "Nil input should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewFlow()
			reader, err := flow.inputToReader(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: expected error but got none", tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
				return
			}

			if reader == nil {
				t.Errorf("%s: expected reader but got nil", tt.description)
				return
			}

			// Read the data from the reader
			data, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("%s: error reading from reader: %v", tt.description, err)
				return
			}

			got := string(data)
			if got != tt.expectedOut {
				t.Errorf("%s: expected %q, got %q", tt.description, tt.expectedOut, got)
			}
		})
	}
}

func TestFlow_readerToOutput(t *testing.T) {
	tests := []struct {
		name        string
		readerData  string
		output      any
		expectedOut any
		wantErr     bool
		description string
	}{
		{
			name:        "output_to_string_ptr",
			readerData:  testData,
			output:      new(string),
			expectedOut: testData,
			wantErr:     false,
			description: "Reader data should be written to string pointer",
		},
		{
			name:        "output_to_string_ptr_empty",
			readerData:  "",
			output:      new(string),
			expectedOut: "",
			wantErr:     false,
			description: "Empty reader should write empty string",
		},
		{
			name:        "output_to_string_ptr_long",
			readerData:  testDataLong,
			output:      new(string),
			expectedOut: testDataLong,
			wantErr:     false,
			description: "Long data should be handled properly",
		},
		{
			name:        "output_to_byte_slice_ptr",
			readerData:  testData,
			output:      new([]byte),
			expectedOut: []byte(testData),
			wantErr:     false,
			description: "Reader data should be written to byte slice pointer",
		},
		{
			name:        "output_to_byte_slice_ptr_empty",
			readerData:  "",
			output:      new([]byte),
			expectedOut: []byte{},
			wantErr:     false,
			description: "Empty reader should write empty byte slice",
		},
		{
			name:        "output_to_reader_ptr",
			readerData:  testData,
			output:      new(io.Reader),
			expectedOut: testData, // We'll verify by reading from the assigned reader
			wantErr:     false,
			description: "Reader should be assigned to io.Reader pointer",
		},
		{
			name:        "output_converter_success",
			readerData:  testData,
			output:      &mockOutputConverter{},
			expectedOut: testData,
			wantErr:     false,
			description: "OutputConverter should receive reader data",
		},
		{
			name:        "output_converter_error",
			readerData:  testData,
			output:      &mockOutputConverter{err: errors.New("converter error")},
			expectedOut: "",
			wantErr:     true,
			description: "OutputConverter error should be propagated",
		},
		{
			name:        "output_converter_empty",
			readerData:  "",
			output:      &mockOutputConverter{},
			expectedOut: "",
			wantErr:     false,
			description: "OutputConverter should handle empty data",
		},
		{
			name:        "unsupported_output_type",
			readerData:  testData,
			output:      42, // Not a pointer
			expectedOut: nil,
			wantErr:     true,
			description: "Unsupported output type should return error",
		},
		{
			name:        "nil_output",
			readerData:  testData,
			output:      nil,
			expectedOut: nil,
			wantErr:     false,
			description: "Nil output should discard data without error (prevents memory leak)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewFlow()
			reader := strings.NewReader(tt.readerData)

			err := flow.readerToOutput(reader, tt.output)

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: expected error but got none", tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
				return
			}

			// Verify the output based on type
			switch out := tt.output.(type) {
			case *string:
				if *out != tt.expectedOut {
					t.Errorf("%s: expected %q, got %q", tt.description, tt.expectedOut, *out)
				}
			case *[]byte:
				expected := tt.expectedOut.([]byte)
				if !bytes.Equal(*out, expected) {
					t.Errorf("%s: expected %v, got %v", tt.description, expected, *out)
				}
			case *io.Reader:
				// Read from the assigned reader
				data, err := io.ReadAll(*out)
				if err != nil {
					t.Errorf("%s: error reading from assigned reader: %v", tt.description, err)
					return
				}
				got := string(data)
				if got != tt.expectedOut {
					t.Errorf("%s: expected %q, got %q", tt.description, tt.expectedOut, got)
				}
			case *mockOutputConverter:
				if out.received != tt.expectedOut {
					t.Errorf("%s: expected %q, got %q", tt.description, tt.expectedOut, out.received)
				}
			}
		})
	}
}

func TestFlow_readerToOutput_ErrorReading(t *testing.T) {
	flow := NewFlow()
	errorReader := &errorReader{err: errors.New("read error")}

	tests := []struct {
		name   string
		output any
	}{
		{"string_output", new(string)},
		{"byte_output", new([]byte)},
		{"converter_output", &mockOutputConverter{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := flow.readerToOutput(errorReader, tt.output)
			if err == nil {
				t.Error("Expected error from error reader")
			}
		})
	}
}

func TestFlow_copyInputToOutput(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		output      any
		expectedOut any
		wantErr     bool
		description string
	}{
		// Direct assignment cases
		{
			name:        "string_to_string_direct",
			input:       testData,
			output:      new(string),
			expectedOut: testData,
			wantErr:     false,
			description: "String to string should use direct assignment",
		},
		{
			name:        "string_to_string_empty",
			input:       "",
			output:      new(string),
			expectedOut: "",
			wantErr:     false,
			description: "Empty string to string should work",
		},
		{
			name:        "bytes_to_bytes_direct",
			input:       []byte(testData),
			output:      new([]byte),
			expectedOut: []byte(testData),
			wantErr:     false,
			description: "Bytes to bytes should use direct assignment with copy",
		},
		{
			name:        "bytes_to_bytes_empty",
			input:       []byte{},
			output:      new([]byte),
			expectedOut: []byte{},
			wantErr:     false,
			description: "Empty bytes to bytes should work",
		},
		{
			name:        "reader_to_reader_direct",
			input:       strings.NewReader(testData),
			output:      new(io.Reader),
			expectedOut: testData, // We'll verify by reading
			wantErr:     false,
			description: "Reader to reader should use direct assignment",
		},
		// Streaming conversion cases
		{
			name:        "string_to_bytes_streaming",
			input:       testData,
			output:      new([]byte),
			expectedOut: []byte(testData),
			wantErr:     false,
			description: "String to bytes should use streaming conversion",
		},
		{
			name:        "bytes_to_string_streaming",
			input:       []byte(testData),
			output:      new(string),
			expectedOut: testData,
			wantErr:     false,
			description: "Bytes to string should use streaming conversion",
		},
		{
			name:        "reader_to_string_streaming",
			input:       strings.NewReader(testData),
			output:      new(string),
			expectedOut: testData,
			wantErr:     false,
			description: "Reader to string should use streaming conversion",
		},
		{
			name:        "reader_to_bytes_streaming",
			input:       strings.NewReader(testData),
			output:      new([]byte),
			expectedOut: []byte(testData),
			wantErr:     false,
			description: "Reader to bytes should use streaming conversion",
		},
		{
			name:        "string_to_reader_streaming",
			input:       testData,
			output:      new(io.Reader),
			expectedOut: testData, // We'll verify by reading
			wantErr:     false,
			description: "String to reader should use streaming conversion",
		},
		{
			name:        "bytes_to_reader_streaming",
			input:       []byte(testData),
			output:      new(io.Reader),
			expectedOut: testData, // We'll verify by reading
			wantErr:     false,
			description: "Bytes to reader should use streaming conversion",
		},
		// Converter cases
		{
			name:        "input_converter_to_string",
			input:       &mockInputConverter{data: testData},
			output:      new(string),
			expectedOut: testData,
			wantErr:     false,
			description: "InputConverter to string should work",
		},
		{
			name:        "string_to_output_converter",
			input:       testData,
			output:      &mockOutputConverter{},
			expectedOut: testData,
			wantErr:     false,
			description: "String to OutputConverter should work",
		},
		{
			name:        "input_converter_to_output_converter",
			input:       &mockInputConverter{data: testData},
			output:      &mockOutputConverter{},
			expectedOut: testData,
			wantErr:     false,
			description: "InputConverter to OutputConverter should work",
		},
		// Error cases
		{
			name:        "input_converter_error",
			input:       &mockInputConverter{err: errors.New("input error")},
			output:      new(string),
			expectedOut: "",
			wantErr:     true,
			description: "InputConverter error should be propagated",
		},
		{
			name:        "output_converter_error",
			input:       testData,
			output:      &mockOutputConverter{err: errors.New("output error")},
			expectedOut: "",
			wantErr:     true,
			description: "OutputConverter error should be propagated",
		},
		{
			name:        "unsupported_input",
			input:       42,
			output:      new(string),
			expectedOut: "",
			wantErr:     true,
			description: "Unsupported input type should return error",
		},
		{
			name:        "unsupported_output",
			input:       testData,
			output:      42,
			expectedOut: "",
			wantErr:     true,
			description: "Unsupported output type should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewFlow()
			err := flow.copyInputToOutput(tt.input, tt.output)

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: expected error but got none", tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
				return
			}

			// Verify the output based on type
			switch out := tt.output.(type) {
			case *string:
				if *out != tt.expectedOut {
					t.Errorf("%s: expected %q, got %q", tt.description, tt.expectedOut, *out)
				}
			case *[]byte:
				expected := tt.expectedOut.([]byte)
				if !bytes.Equal(*out, expected) {
					t.Errorf("%s: expected %v, got %v", tt.description, expected, *out)
				}
			case *io.Reader:
				// Read from the assigned reader
				data, err := io.ReadAll(*out)
				if err != nil {
					t.Errorf("%s: error reading from assigned reader: %v", tt.description, err)
					return
				}
				got := string(data)
				if got != tt.expectedOut {
					t.Errorf("%s: expected %q, got %q", tt.description, tt.expectedOut, got)
				}
			case *mockOutputConverter:
				if out.received != tt.expectedOut {
					t.Errorf("%s: expected %q, got %q", tt.description, tt.expectedOut, out.received)
				}
			}
		})
	}
}

func TestFlow_copyInputToOutput_ByteSliceIsolation(t *testing.T) {
	// Test that byte slice copying creates isolation (prevents mutation)
	flow := NewFlow()
	originalBytes := []byte(testData)
	output := new([]byte)

	err := flow.copyInputToOutput(originalBytes, output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Modify the original bytes
	originalBytes[0] = 'X'

	// Output should not be affected
	if (*output)[0] == 'X' {
		t.Error("Byte slice copy should prevent mutation of original data")
	}

	// Verify output still has original data
	if string(*output) != testData {
		t.Errorf("Expected %q, got %q", testData, string(*output))
	}
}

func TestConverter_Interfaces(t *testing.T) {
	// Test that our mock implements the interfaces correctly
	var _ InputConverter = &mockInputConverter{}
	var _ OutputConverter = &mockOutputConverter{}
	var _ Converter = &mockBidirectionalConverter{}

	// Test bidirectional converter
	inputConv := &mockInputConverter{data: testData}
	outputConv := &mockOutputConverter{}
	biConv := &mockBidirectionalConverter{
		mockInputConverter:  inputConv,
		mockOutputConverter: outputConv,
	}

	// Test input side
	reader, err := biConv.ToReader()
	if err != nil {
		t.Fatalf("ToReader() error: %v", err)
	}

	// Test output side
	err = biConv.FromReader(reader)
	if err != nil {
		t.Fatalf("FromReader() error: %v", err)
	}

	if outputConv.received != testData {
		t.Errorf("Expected %q, got %q", testData, outputConv.received)
	}
}

func TestConverter_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		testFunc    func(*testing.T)
		description string
	}{
		{
			name: "large_data_handling",
			testFunc: func(t *testing.T) {
				// Test with large data (1MB)
				largeData := strings.Repeat("a", 1024*1024)
				flow := NewFlow()

				var output string
				err := flow.copyInputToOutput(largeData, &output)
				if err != nil {
					t.Errorf("Large data handling failed: %v", err)
				}
				if output != largeData {
					t.Error("Large data was not copied correctly")
				}
			},
			description: "Should handle large data correctly",
		},
		{
			name: "binary_data_handling",
			testFunc: func(t *testing.T) {
				// Test with binary data
				binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
				flow := NewFlow()

				var output []byte
				err := flow.copyInputToOutput(binaryData, &output)
				if err != nil {
					t.Errorf("Binary data handling failed: %v", err)
				}
				if !bytes.Equal(output, binaryData) {
					t.Error("Binary data was not copied correctly")
				}
			},
			description: "Should handle binary data correctly",
		},
		{
			name: "unicode_data_handling",
			testFunc: func(t *testing.T) {
				// Test with Unicode data
				unicodeData := "Hello 世界 🌍 emoji test ñoño"
				flow := NewFlow()

				var output string
				err := flow.copyInputToOutput(unicodeData, &output)
				if err != nil {
					t.Errorf("Unicode data handling failed: %v", err)
				}
				if output != unicodeData {
					t.Error("Unicode data was not handled correctly")
				}
			},
			description: "Should handle Unicode data correctly",
		},
		{
			name: "reader_reuse_isolation",
			testFunc: func(t *testing.T) {
				// Test that readers are not accidentally reused
				originalReader := strings.NewReader(testData)
				flow := NewFlow()

				var output1, output2 string
				err := flow.copyInputToOutput(originalReader, &output1)
				if err != nil {
					t.Errorf("First copy failed: %v", err)
				}

				// Second copy should fail or be empty since reader is exhausted
				// reader is exhausted, ignore error
				_ = flow.copyInputToOutput(originalReader, &output2)

				if output1 != testData {
					t.Errorf("First output: expected %q, got %q", testData, output1)
				}
				if output2 == testData {
					t.Error("Second output should be empty (reader exhausted)")
				}
			},
			description: "Should handle reader exhaustion correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestConverter_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (any, any)
		expectError bool
		description string
	}{
		{
			name: "reader_with_read_error",
			setupFunc: func() (any, any) {
				return &errorReader{err: errors.New("read failed")}, new(string)
			},
			expectError: true,
			description: "Reader with read error should propagate error",
		},
		{
			name: "input_converter_panic_recovery",
			setupFunc: func() (any, any) {
				// This would test panic recovery, but we don't have panics in current implementation
				return &mockInputConverter{data: testData}, new(string)
			},
			expectError: false,
			description: "Normal operation should not panic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, output := tt.setupFunc()
			flow := NewFlow()

			err := flow.copyInputToOutput(input, output)

			if tt.expectError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkFlow_inputToReader_String(b *testing.B) {
	flow := NewFlow()
	data := testDataLong

	for b.Loop() {
		_, err := flow.inputToReader(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFlow_inputToReader_Bytes(b *testing.B) {
	flow := NewFlow()
	data := []byte(testDataLong)

	for b.Loop() {
		_, err := flow.inputToReader(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFlow_readerToOutput_String(b *testing.B) {
	flow := NewFlow()
	data := testDataLong

	for b.Loop() {
		reader := strings.NewReader(data)
		var output string
		err := flow.readerToOutput(reader, &output)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFlow_copyInputToOutput_DirectString(b *testing.B) {
	flow := NewFlow()
	data := testDataLong

	for b.Loop() {
		var output string
		err := flow.copyInputToOutput(data, &output)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFlow_copyInputToOutput_DirectBytes(b *testing.B) {
	flow := NewFlow()
	data := []byte(testDataLong)

	for b.Loop() {
		var output []byte
		err := flow.copyInputToOutput(data, &output)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFlow_copyInputToOutput_StreamingConversion(b *testing.B) {
	flow := NewFlow()
	data := testDataLong

	for b.Loop() {
		var output []byte
		err := flow.copyInputToOutput(data, &output) // string to bytes requires streaming
		if err != nil {
			b.Fatal(err)
		}
	}
}
