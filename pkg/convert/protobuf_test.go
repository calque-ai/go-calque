package convert

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	calquepb "github.com/calque-ai/go-calque/proto"
)

func TestProtobufConverters(t *testing.T) {
	tests := []struct {
		name        string
		input       proto.Message
		target      proto.Message
		expectError bool
		errorMsg    string
		testType    string // "input", "output", "stream_input", "stream_output", "roundtrip"
	}{
		{
			name: "FlowRequest - basic input conversion",
			input: &calquepb.FlowRequest{
				Version:  1,
				FlowName: "test-flow",
				Input:    "test input",
				Metadata: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			testType: "input",
		},
		{
			name: "FlowRequest - basic output conversion",
			input: &calquepb.FlowRequest{
				Version:  1,
				FlowName: "test-flow",
				Input:    "test input",
				Metadata: map[string]string{
					"key1": "value1",
				},
			},
			target:   &calquepb.FlowRequest{},
			testType: "output",
		},
		{
			name: "FlowRequest - roundtrip conversion",
			input: &calquepb.FlowRequest{
				Version:  1,
				FlowName: "test-flow",
				Input:    "test input",
				Metadata: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			target:   &calquepb.FlowRequest{},
			testType: "roundtrip",
		},
		{
			name: "FlowRequest - streaming input",
			input: &calquepb.FlowRequest{
				Version:  1,
				FlowName: "test-flow",
				Input:    "test input",
				Metadata: map[string]string{
					"key1": "value1",
				},
			},
			testType: "stream_input",
		},
		{
			name: "FlowRequest - streaming output",
			input: &calquepb.FlowRequest{
				Version:  1,
				FlowName: "test-flow",
				Input:    "test input",
				Metadata: map[string]string{
					"key1": "value1",
				},
			},
			target:   &calquepb.FlowRequest{},
			testType: "stream_output",
		},

		// FlowResponse tests
		{
			name: "FlowResponse - basic input conversion",
			input: &calquepb.FlowResponse{
				Version:      1,
				Output:       "test output",
				Success:      true,
				ErrorMessage: "",
				Metadata: map[string]string{
					"status": "success",
				},
			},
			testType: "input",
		},
		{
			name: "FlowResponse - roundtrip conversion",
			input: &calquepb.FlowResponse{
				Version:      1,
				Output:       "test output",
				Success:      false,
				ErrorMessage: "test error",
				Metadata: map[string]string{
					"status": "error",
				},
			},
			target:   &calquepb.FlowResponse{},
			testType: "roundtrip",
		},
		{
			name: "AIRequest - basic input conversion",
			input: &calquepb.AIRequest{
				Prompt: "test prompt",
				Parameters: map[string]string{
					"temperature": "0.7",
					"max_tokens":  "100",
				},
				Tools: []string{"tool1", "tool2"},
			},
			testType: "input",
		},
		{
			name: "AIRequest - roundtrip conversion",
			input: &calquepb.AIRequest{
				Prompt: "test prompt",
				Parameters: map[string]string{
					"temperature": "0.7",
				},
				Tools: []string{"tool1"},
			},
			target:   &calquepb.AIRequest{},
			testType: "roundtrip",
		},
		{
			name: "AIResponse - basic input conversion",
			input: &calquepb.AIResponse{
				Response: "test response",
				ToolCalls: []*calquepb.ToolCall{
					{
						Name:      "test_tool",
						Arguments: `{"arg1": "value1"}`,
						Id:        "call_123",
					},
				},
				Metadata: map[string]string{
					"model": "gpt-4",
				},
			},
			testType: "input",
		},
		{
			name: "AIResponse - roundtrip conversion",
			input: &calquepb.AIResponse{
				Response: "test response",
				ToolCalls: []*calquepb.ToolCall{
					{
						Name:      "test_tool",
						Arguments: `{"arg1": "value1"}`,
						Id:        "call_123",
					},
				},
				Metadata: map[string]string{
					"model": "gpt-4",
				},
			},
			target:   &calquepb.AIResponse{},
			testType: "roundtrip",
		},
		{
			name: "MemoryRequest - basic input conversion",
			input: &calquepb.MemoryRequest{
				Operation: "set",
				Key:       "test_key",
				Value:     "test_value",
				Metadata: map[string]string{
					"ttl": "3600",
				},
			},
			testType: "input",
		},
		{
			name: "MemoryRequest - roundtrip conversion",
			input: &calquepb.MemoryRequest{
				Operation: "get",
				Key:       "test_key",
				Value:     "",
				Metadata: map[string]string{
					"namespace": "default",
				},
			},
			target:   &calquepb.MemoryRequest{},
			testType: "roundtrip",
		},
		{
			name: "ToolRequest - basic input conversion",
			input: &calquepb.ToolRequest{
				Name:      "test_tool",
				Arguments: `{"param1": "value1", "param2": 42}`,
				Id:        "req_123",
				Metadata: map[string]string{
					"timeout": "30s",
				},
			},
			testType: "input",
		},
		{
			name: "ToolRequest - roundtrip conversion",
			input: &calquepb.ToolRequest{
				Name:      "test_tool",
				Arguments: `{"param1": "value1"}`,
				Id:        "req_123",
				Metadata: map[string]string{
					"timeout": "30s",
				},
			},
			target:   &calquepb.ToolRequest{},
			testType: "roundtrip",
		},
		{
			name:        "nil input data",
			input:       nil,
			expectError: true,
			errorMsg:    "protobuf data is nil",
			testType:    "input",
		},
		{
			name:        "nil target for output",
			target:      nil,
			expectError: true,
			errorMsg:    "protobuf target is nil",
			testType:    "output",
		},
		{
			name: "empty FlowRequest",
			input: &calquepb.FlowRequest{
				Version:  0,
				FlowName: "",
				Input:    "",
				Metadata: map[string]string{},
			},
			target:   &calquepb.FlowRequest{},
			testType: "roundtrip",
		},
		{
			name: "FlowRequest with large metadata",
			input: &calquepb.FlowRequest{
				Version:  1,
				FlowName: "test-flow",
				Input:    "test input",
				Metadata: generateLargeMetadata(1000), // 1000 key-value pairs
			},
			target:   &calquepb.FlowRequest{},
			testType: "roundtrip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			switch tt.testType {
			case "input":
				testInputConverter(t, tt.input, tt.expectError, tt.errorMsg)
			case "output":
				testOutputConverter(t, tt.input, tt.target, tt.expectError, tt.errorMsg)
			case "stream_input":
				testStreamInputConverter(t, tt.input, tt.expectError, tt.errorMsg)
			case "stream_output":
				testStreamOutputConverter(t, tt.input, tt.target, tt.expectError, tt.errorMsg)
			case "roundtrip":
				testRoundtripConversion(t, tt.input, tt.target)
			default:
				t.Fatalf("Unknown test type: %s", tt.testType)
			}
		})
	}
}

func testInputConverter(t *testing.T, input proto.Message, expectError bool, errorMsg string) {
	converter := ToProtobuf(input)
	reader, err := converter.ToReader()

	if expectError {
		if err == nil {
			t.Error("Expected error but got none")
			return
		}
		if errorMsg != "" && !strings.Contains(err.Error(), errorMsg) {
			t.Errorf("Expected error message to contain '%s', got '%s'", errorMsg, err.Error())
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Read the data
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-zero data length")
	}

	// Verify the data can be unmarshaled back to the original type
	unmarshaled := reflect.New(reflect.TypeOf(input).Elem()).Interface().(proto.Message)
	err = proto.Unmarshal(data, unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if !proto.Equal(input, unmarshaled) {
		t.Error("Converted data doesn't match original message")
	}
}

func testOutputConverter(t *testing.T, input proto.Message, target proto.Message, expectError bool, errorMsg string) {
	// Marshal the input data
	data, err := proto.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	converter := FromProtobuf(target)
	err = converter.FromReader(bytes.NewReader(data))

	if expectError {
		if err == nil {
			t.Error("Expected error but got none")
			return
		}
		if errorMsg != "" && !strings.Contains(err.Error(), errorMsg) {
			t.Errorf("Expected error message to contain '%s', got '%s'", errorMsg, err.Error())
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the target was populated correctly
	if !proto.Equal(input, target) {
		t.Errorf("Unmarshaled data doesn't match original. Expected: %+v, Got: %+v", input, target)
	}
}

func testStreamInputConverter(t *testing.T, input proto.Message, expectError bool, errorMsg string) {
	converter := ToProtobufStream(input)
	reader, err := converter.ToReader()

	if expectError {
		if err == nil {
			t.Error("Expected error but got none")
			return
		}
		if errorMsg != "" && !strings.Contains(err.Error(), errorMsg) {
			t.Errorf("Expected error message to contain '%s', got '%s'", errorMsg, err.Error())
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Read the data
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-zero data length")
	}

	// Verify the data can be unmarshaled back to the original type
	originalData, err := proto.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal original data: %v", err)
	}

	if !bytes.Equal(data, originalData) {
		t.Error("Stream converted data doesn't match original marshaled data")
	}
}

func testStreamOutputConverter(t *testing.T, input proto.Message, target proto.Message, expectError bool, errorMsg string) {
	// Marshal the input data
	data, err := proto.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	converter := FromProtobufStream(target)
	err = converter.FromReader(bytes.NewReader(data))

	if expectError {
		if err == nil {
			t.Error("Expected error but got none")
			return
		}
		if errorMsg != "" && !strings.Contains(err.Error(), errorMsg) {
			t.Errorf("Expected error message to contain '%s', got '%s'", errorMsg, err.Error())
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the target was populated correctly
	if !proto.Equal(input, target) {
		t.Errorf("Stream unmarshaled data doesn't match original. Expected: %+v, Got: %+v", input, target)
	}
}

func testRoundtripConversion(t *testing.T, input proto.Message, target proto.Message) {
	// Test regular converters
	converter := ToProtobuf(input)
	reader, err := converter.ToReader()
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	outputConverter := FromProtobuf(target)
	err = outputConverter.FromReader(reader)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !proto.Equal(input, target) {
		t.Errorf("Roundtrip conversion failed. Expected: %+v, Got: %+v", input, target)
	}

	// Test streaming converters
	streamTarget := reflect.New(reflect.TypeOf(target).Elem()).Interface().(proto.Message)
	streamConverter := ToProtobufStream(input)
	streamReader, err := streamConverter.ToReader()
	if err != nil {
		t.Fatalf("Failed to create stream reader: %v", err)
	}

	streamOutputConverter := FromProtobufStream(streamTarget)
	err = streamOutputConverter.FromReader(streamReader)
	if err != nil {
		t.Fatalf("Failed to unmarshal stream: %v", err)
	}

	if !proto.Equal(input, streamTarget) {
		t.Errorf("Stream roundtrip conversion failed. Expected: %+v, Got: %+v", input, streamTarget)
	}
}

func generateLargeMetadata(size int) map[string]string {
	metadata := make(map[string]string, size)
	for i := 0; i < size; i++ {
		metadata[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	return metadata
}

// Test chunked reading for large messages
func TestChunkedReading(t *testing.T) {
	// Create a large message that will trigger chunked reading
	largeMessage := &calquepb.FlowRequest{
		Version:  1,
		FlowName: "test-flow",
		Input:    strings.Repeat("large input data ", 10000), // ~160KB
		Metadata: generateLargeMetadata(1000),                // Additional metadata
	}

	converter := ToProtobufStream(largeMessage)
	reader, err := converter.ToReader()
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// Read in chunks to test chunked reading
	chunkSize := 1024
	totalRead := 0
	chunks := make([][]byte, 0)

	for {
		chunk := make([]byte, chunkSize)
		n, err := reader.Read(chunk)
		if err == io.EOF {
			if n > 0 {
				chunks = append(chunks, chunk[:n])
				totalRead += n
			}
			break
		}
		if err != nil {
			t.Fatalf("Error reading chunk: %v", err)
		}
		chunks = append(chunks, chunk[:n])
		totalRead += n
	}

	if totalRead == 0 {
		t.Error("Expected to read some data")
	}

	// Reconstruct the data and verify it can be unmarshaled
	reconstructed := make([]byte, 0, totalRead)
	for _, chunk := range chunks {
		reconstructed = append(reconstructed, chunk...)
	}

	target := &calquepb.FlowRequest{}
	err = proto.Unmarshal(reconstructed, target)
	if err != nil {
		t.Fatalf("Failed to unmarshal reconstructed data: %v", err)
	}

	if !proto.Equal(largeMessage, target) {
		t.Error("Chunked reading resulted in data corruption")
	}
}

// Test error handling with invalid data
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		target      proto.Message
		expectError bool
		errorMsg    string
	}{
		{
			name:        "invalid protobuf data",
			data:        []byte("invalid protobuf data"),
			target:      &calquepb.FlowRequest{},
			expectError: true,
			errorMsg:    "failed to unmarshal protobuf",
		},
		{
			name:        "empty data",
			data:        []byte{},
			target:      &calquepb.FlowRequest{},
			expectError: false, // Empty data is valid for protobuf (represents empty message)
			errorMsg:    "",
		},
		{
			name:        "nil target",
			data:        []byte("some data"),
			target:      nil,
			expectError: true,
			errorMsg:    "protobuf target is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := FromProtobuf(tt.target)
			err := converter.FromReader(bytes.NewReader(tt.data))

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestChunkedReader tests the chunked streaming functionality for large protobuf messages
func TestChunkedReader(t *testing.T) {
	// Create a large test message
	largeData := make([]byte, 2*1024*1024) // 2MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	chunkSize := 64 * 1024 // 64KB chunks
	reader := &chunkedReader{data: largeData, chunkSize: chunkSize}

	// Test reading the entire data
	readData := make([]byte, len(largeData))
	totalRead := 0

	for totalRead < len(largeData) {
		n, err := reader.Read(readData[totalRead:])
		if err != nil && err != io.EOF {
			t.Fatalf("Read failed at %d bytes: %v", totalRead, err)
		}
		totalRead += n
		if err == io.EOF {
			break
		}
	}

	if totalRead != len(largeData) {
		t.Errorf("Expected to read %d bytes, read %d", len(largeData), totalRead)
	}

	// Verify data integrity
	if !bytes.Equal(largeData, readData) {
		t.Error("Read data doesn't match original data")
	}
}

// TestChunkedReaderSmallData tests chunked reader with small data
func TestChunkedReaderSmallData(t *testing.T) {
	smallData := []byte("small test data")
	reader := &chunkedReader{data: smallData, chunkSize: 64 * 1024}

	readData := make([]byte, len(smallData))
	n, err := reader.Read(readData)
	if err != nil && err != io.EOF {
		t.Fatalf("Read failed: %v", err)
	}

	if n != len(smallData) {
		t.Errorf("Expected to read %d bytes, read %d", len(smallData), n)
	}

	if !bytes.Equal(smallData, readData) {
		t.Error("Read data doesn't match original data")
	}
}

// TestChunkedReaderMultipleReads tests multiple read operations
func TestChunkedReaderMultipleReads(t *testing.T) {
	largeData := make([]byte, 200*1024) // 200KB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	chunkSize := 32 * 1024 // 32KB chunks
	reader := &chunkedReader{data: largeData, chunkSize: chunkSize}

	// Read in multiple chunks
	buffer := make([]byte, chunkSize)
	totalRead := 0
	readCount := 0

	for totalRead < len(largeData) {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			t.Fatalf("Read %d failed: %v", readCount, err)
		}
		totalRead += n
		readCount++

		// Verify chunk size (except possibly the last chunk)
		// Note: The last chunk may be smaller than the expected chunk size
		if err != io.EOF && n != chunkSize && n != 0 {
			// Only error if we got a non-zero read that's not the expected size
			// and it's not the last chunk
			if totalRead < len(largeData) {
				t.Errorf("Read %d: expected chunk size %d, got %d", readCount, chunkSize, n)
			}
		}

		if err == io.EOF {
			break
		}
	}

	if totalRead != len(largeData) {
		t.Errorf("Expected to read %d bytes, read %d", len(largeData), totalRead)
	}

	// Should have read in multiple chunks
	expectedReads := (len(largeData) + chunkSize - 1) / chunkSize
	if readCount != expectedReads {
		t.Errorf("Expected %d reads, got %d", expectedReads, readCount)
	}
}
