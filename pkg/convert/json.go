// Package convert provides utilities for converting structured data to and from JSON streams.
// It includes converters for both input (structured data to JSON) and output (JSON streams to structured data).
package convert

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// JSONInputConverter for structured data -> JSON streams
type JSONInputConverter struct {
	data any
}

// JSONOutputConverter for JSON streams -> structured data
type JSONOutputConverter struct {
	target any
}

// ToJSON creates an input converter for transforming structured data to JSON streams.
//
// Input: any data type (structs, maps, slices, JSON strings, JSON bytes)
// Output: calque.InputConverter for pipeline input position
// Behavior: STREAMING - uses json.Encoder for automatic streaming optimization
//
// Converts various data types to valid JSON format for pipeline processing:
// - Structs/maps/slices: Marshaled using encoding/json
// - JSON strings: Validated and passed through
// - JSON bytes: Validated and passed through
// - Other types: Attempted JSON marshaling
//
// Example usage:
//
//	type User struct {
//		Name string `json:"name"`
//		Age  int    `json:"age"`
//	}
//
//	user := User{Name: "Alice", Age: 30}
//	err := pipeline.Run(ctx, convert.ToJson(user), &result)
func ToJSON(data any) calque.InputConverter {
	return &JSONInputConverter{data: data}
}

// FromJSON creates an output converter for parsing JSON streams to structured data.
//
// Input: pointer to target variable for unmarshaling
// Output: calque.OutputConverter for pipeline output position
// Behavior: STREAMING - uses json.Decoder for automatic streaming/buffering as needed
//
// Parses JSON data from pipeline output into the specified target type.
// Target must be a pointer to the desired output type. Uses encoding/json
// for unmarshaling, supporting all standard JSON types and struct tags.
//
// Example usage:
//
//	type User struct {
//		Name string `json:"name"`
//		Age  int    `json:"age"`
//	}
//
//	var user User
//	err := pipeline.Run(ctx, input, convert.FromJSON(&user))
//	fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
func FromJSON(target any) calque.OutputConverter {
	return &JSONOutputConverter{target: target}
}

// ToReader converts the input data to an io.Reader for streaming JSON processing.
func (j *JSONInputConverter) ToReader() (io.Reader, error) {
	switch v := j.data.(type) {
	case map[string]any, []any:
		// Use json.Encoder for streaming marshal of structured data
		pr, pw := io.Pipe()
		go func() {
			defer func() {
				if err := pw.Close(); err != nil {
					// Pipe writer close errors here are expected if we already called CloseWithError
					// for encoding failures, so we can safely ignore them
					_ = err
				}
			}()
			encoder := json.NewEncoder(pw)
			if err := encoder.Encode(v); err != nil {
				pw.CloseWithError(calque.WrapErr(context.Background(), err, "failed to encode JSON"))
			}
		}()
		return pr, nil
	case string:
		// Validate first, then stream if valid
		var temp any
		if err := json.Unmarshal([]byte(v), &temp); err != nil {
			return nil, calque.WrapErr(context.Background(), err, "invalid JSON string")
		}
		return strings.NewReader(v), nil
	case []byte:
		// Validate first, then stream if valid
		var temp any
		if err := json.Unmarshal(v, &temp); err != nil {
			return nil, calque.WrapErr(context.Background(), err, "invalid JSON bytes")
		}
		return bytes.NewReader(v), nil
	case io.Reader:
		// streaming validation for io.Reader
		return j.createStreamingValidatingReader(v, "invalid JSON stream")
	default:
		// Use json.Encoder for streaming marshal of any other type
		pr, pw := io.Pipe()
		go func() {
			defer func() {
				if err := pw.Close(); err != nil {
					// Pipe writer close errors here are expected if we already called CloseWithError
					// for encoding failures, so we can safely ignore them
					_ = err
				}
			}()
			encoder := json.NewEncoder(pw)
			if err := encoder.Encode(j.data); err != nil {
				pw.CloseWithError(calque.WrapErr(context.Background(), err, fmt.Sprintf("failed to encode JSON for type %T", j.data)))
			}
		}()
		return pr, nil
	}
}

// createStreamingValidatingReader creates a streaming reader with chunked validation for io.Reader inputs
func (j *JSONInputConverter) createStreamingValidatingReader(reader io.Reader, errorPrefix string) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		defer func() {
			if err := pw.Close(); err != nil {
				// Pipe writer close errors here are expected if we already called CloseWithError
				// for encoding failures, so we can safely ignore them
				_ = err
			}
		}()

		j.processStreamingValidation(reader, pw, errorPrefix)
	}()
	return pr, nil
}

// processStreamingValidation handles the complex streaming validation logic
func (j *JSONInputConverter) processStreamingValidation(reader io.Reader, pw *io.PipeWriter, errorPrefix string) {
	// Use buffered writer to control output flow
	bufWriter := bufio.NewWriterSize(pw, 4096) // 4KB buffer
	var validationBuf bytes.Buffer

	// TeeReader splits input: to validation buffer AND to a temp buffer for later output
	var tempBuf bytes.Buffer
	teeReader := io.TeeReader(reader, io.MultiWriter(&validationBuf, &tempBuf))

	// Read in small chunks to allow early validation
	buf := make([]byte, 1024) // 1KB chunks
	totalRead := 0
	validationPassed := false

	for {
		n, err := teeReader.Read(buf)
		if n > 0 {
			totalRead += n

			// Try validating what we have so far (every 2KB or so)
			if totalRead >= 2048 || err == io.EOF {
				if j.handleValidationCheck(&validationBuf, &tempBuf, bufWriter, pw, errorPrefix) {
					validationPassed = true
					break
				}
			}
		}

		if err == io.EOF {
			if j.handleFinalValidation(&validationBuf, &tempBuf, bufWriter, pw, errorPrefix) {
				return
			}
			break
		} else if err != nil {
			pw.CloseWithError(err)
			return
		}
	}

	// If validation passed early, continue reading the rest of the data
	if validationPassed {
		// Continue reading from the teeReader to get all remaining data
		if _, err := io.Copy(bufWriter, teeReader); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := bufWriter.Flush(); err != nil {
			pw.CloseWithError(calque.WrapErr(context.Background(), err, "failed to flush buffer"))
			return
		}
	}
}

// handleValidationCheck processes validation during streaming
func (j *JSONInputConverter) handleValidationCheck(validationBuf, tempBuf *bytes.Buffer, bufWriter *bufio.Writer, pw *io.PipeWriter, errorPrefix string) bool {
	decoder := json.NewDecoder(bytes.NewReader(validationBuf.Bytes()))
	var temp any
	validateErr := decoder.Decode(&temp)

	if validateErr == nil {
		// Valid complete JSON - flush everything and switch to direct streaming
		if j.flushBufferedData(tempBuf, bufWriter, pw) {
			return true
		}

		// Continue streaming rest directly (validation passed)
		// Note: We can't continue streaming from the original reader here
		// as it's already been consumed by the teeReader
		if err := bufWriter.Flush(); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to flush buffer: %w", err))
			return true
		}
		return true
	} else if validateErr != io.EOF && validateErr != io.ErrUnexpectedEOF {
		// Definite validation error (not incomplete JSON)
		pw.CloseWithError(calque.WrapErr(context.Background(), validateErr, errorPrefix))
		return true
	}
	// Otherwise continue reading (JSON might be incomplete)
	return false
}

// handleFinalValidation processes final validation at EOF
func (j *JSONInputConverter) handleFinalValidation(validationBuf, tempBuf *bytes.Buffer, bufWriter *bufio.Writer, pw *io.PipeWriter, errorPrefix string) bool {
	decoder := json.NewDecoder(validationBuf)
	var temp any
	if finalErr := decoder.Decode(&temp); finalErr != nil {
		pw.CloseWithError(calque.WrapErr(context.Background(), finalErr, errorPrefix))
		return true
	}

	// Stream final buffered data
	return j.flushBufferedData(tempBuf, bufWriter, pw)
}

// flushBufferedData flushes buffered data to the writer
func (j *JSONInputConverter) flushBufferedData(tempBuf *bytes.Buffer, bufWriter *bufio.Writer, pw *io.PipeWriter) bool {
	if _, writeErr := io.Copy(bufWriter, tempBuf); writeErr != nil {
		pw.CloseWithError(writeErr)
		return true
	}
	if err := bufWriter.Flush(); err != nil {
		pw.CloseWithError(fmt.Errorf("failed to flush buffer: %w", err))
		return true
	}
	return false
}

// FromReader implements the OutputConverter interface for JSON streams -> structured data.
func (j *JSONOutputConverter) FromReader(reader io.Reader) error {
	// Use json.Decoder for streaming decode
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(j.target); err != nil {
		return calque.WrapErr(context.Background(), err, "failed to decode JSON")
	}
	return nil
}
