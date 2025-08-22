package convert

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Input converter for JSON data -> JSON bytes
type jsonInputConverter struct {
	data any
}

// Output converter for JSON bytes -> any type
type jsonOutputConverter struct {
	target any
}

// ToJson creates an input converter for transforming structured data to JSON streams.
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
func ToJson(data any) calque.InputConverter {
	return &jsonInputConverter{data: data}
}

// FromJson creates an output converter for parsing JSON streams to structured data.
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
//	err := pipeline.Run(ctx, input, convert.FromJson(&user))
//	fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
func FromJson(target any) calque.OutputConverter {
	return &jsonOutputConverter{target: target}
}

// InputConverter interface
func (j *jsonInputConverter) ToReader() (io.Reader, error) {
	switch v := j.data.(type) {
	case map[string]any, []any:
		// Use json.Encoder for streaming marshal of structured data
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			encoder := json.NewEncoder(pw)
			if err := encoder.Encode(v); err != nil {
				pw.CloseWithError(fmt.Errorf("failed to encode JSON: %w", err))
			}
		}()
		return pr, nil
	case string:
		// Validate first, then stream if valid
		var temp any
		if err := json.Unmarshal([]byte(v), &temp); err != nil {
			return nil, fmt.Errorf("invalid JSON string: %w", err)
		}
		return strings.NewReader(v), nil
	case []byte:
		// Validate first, then stream if valid
		var temp any
		if err := json.Unmarshal(v, &temp); err != nil {
			return nil, fmt.Errorf("invalid JSON bytes: %w", err)
		}
		return bytes.NewReader(v), nil
	case io.Reader:
		// streaming validation for io.Reader
		return j.createStreamingValidatingReader(v, "invalid JSON stream")
	default:
		// Use json.Encoder for streaming marshal of any other type
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			encoder := json.NewEncoder(pw)
			if err := encoder.Encode(j.data); err != nil {
				pw.CloseWithError(fmt.Errorf("failed to encode JSON for type %T: %w", j.data, err))
			}
		}()
		return pr, nil
	}
}

// createStreamingValidatingReader creates a streaming reader with chunked validation for io.Reader inputs
func (j *jsonInputConverter) createStreamingValidatingReader(reader io.Reader, errorPrefix string) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()

		// Use buffered writer to control output flow
		bufWriter := bufio.NewWriterSize(pw, 4096) // 4KB buffer
		var validationBuf bytes.Buffer

		// TeeReader splits input: to validation buffer AND to a temp buffer for later output
		var tempBuf bytes.Buffer
		teeReader := io.TeeReader(reader, io.MultiWriter(&validationBuf, &tempBuf))

		// Read in small chunks to allow early validation
		buf := make([]byte, 1024) // 1KB chunks
		totalRead := 0

		for {
			n, err := teeReader.Read(buf)
			if n > 0 {
				totalRead += n

				// Try validating what we have so far (every 2KB or so)
				if totalRead >= 2048 || err == io.EOF {
					decoder := json.NewDecoder(bytes.NewReader(validationBuf.Bytes()))
					var temp any
					validateErr := decoder.Decode(&temp)

					if validateErr == nil {
						// Valid complete JSON - flush everything and switch to direct streaming
						if _, writeErr := io.Copy(bufWriter, &tempBuf); writeErr != nil {
							pw.CloseWithError(writeErr)
							return
						}
						bufWriter.Flush()

						// Continue streaming rest directly (validation passed)
						if err != io.EOF {
							if _, copyErr := io.Copy(bufWriter, reader); copyErr != nil {
								pw.CloseWithError(copyErr)
								return
							}
						}
						bufWriter.Flush()
						return
					} else if validateErr != io.EOF && validateErr != io.ErrUnexpectedEOF {
						// Definite validation error (not incomplete JSON)
						pw.CloseWithError(fmt.Errorf("%s: %w", errorPrefix, validateErr))
						return
					}
					// Otherwise continue reading (JSON might be incomplete)
				}
			}

			if err == io.EOF {
				// Final validation check
				decoder := json.NewDecoder(&validationBuf)
				var temp any
				if finalErr := decoder.Decode(&temp); finalErr != nil {
					pw.CloseWithError(fmt.Errorf("%s: %w", errorPrefix, finalErr))
					return
				}

				// Stream final buffered data
				if _, writeErr := io.Copy(bufWriter, &tempBuf); writeErr != nil {
					pw.CloseWithError(writeErr)
					return
				}
				bufWriter.Flush()
				break
			} else if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()
	return pr, nil
}

func (j *jsonOutputConverter) FromReader(reader io.Reader) error {
	// Use json.Decoder for streaming decode
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(j.target); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	return nil
}
