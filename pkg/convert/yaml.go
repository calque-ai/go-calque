package convert

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/goccy/go-yaml"
)

// YAMLInputConverter is an input converter for transforming structured data to YAML streams.
type YAMLInputConverter struct {
	data any
}

// YAMLOutputConverter is an output converter for parsing YAML streams to structured data.
type YAMLOutputConverter struct {
	target any
}

// ToYAML creates an input converter for transforming structured data to YAML streams.
//
// Input: any data type (structs, maps, slices, YAML strings, YAML bytes)
// Output: calque.InputConverter for pipeline input position
// Behavior: STREAMING - uses yaml.Encoder for automatic streaming optimization
//
// Converts various data types to valid YAML format for pipeline processing:
// - Structs/maps/slices: Marshaled using goccy/go-yaml
// - YAML strings: Validated and passed through
// - YAML bytes: Validated and passed through
// - Other types: Attempted YAML marshaling
//
// Example usage:
//
//	type Config struct {
//		Database struct {
//			Host string `yaml:"host"`
//			Port int    `yaml:"port"`
//		} `yaml:"database"`
//	}
//
//	config := Config{Database: {Host: "localhost", Port: 5432}}
//	err := pipeline.Run(ctx, convert.ToYAML(config), &result)
func ToYAML(data any) calque.InputConverter {
	return &YAMLInputConverter{data: data}
}

// FromYAML creates an output converter for parsing YAML streams to structured data.
//
// Input: pointer to target variable for unmarshaling
// Output: calque.OutputConverter for pipeline output position
// Behavior: STREAMING - uses yaml.Decoder for automatic streaming/buffering as needed
//
// Parses YAML data from pipeline output into the specified target type.
// Target must be a pointer to the desired output type. Uses goccy/go-yaml
// for unmarshaling, supporting standard YAML types and struct tags.
//
// Example usage:
//
//	type Config struct {
//		Database struct {
//			Host string `yaml:"host"`
//			Port int    `yaml:"port"`
//		} `yaml:"database"`
//	}
//
//	var config Config
//	err := pipeline.Run(ctx, input, convert.FromYAML(&config))
//	fmt.Printf("DB: %s:%d\n", config.Database.Host, config.Database.Port)
func FromYAML(target any) calque.OutputConverter {
	return &YAMLOutputConverter{target: target}
}

// ToReader converts structured data to an io.Reader for YAML processing.
func (y *YAMLInputConverter) ToReader() (io.Reader, error) {
	switch v := y.data.(type) {
	case map[string]any, map[any]any, []any:
		// Use yaml.Encoder for streaming marshal of structured data
		pr, pw := io.Pipe()
		go func() {
			defer func() {
				if err := pw.Close(); err != nil {
					// Pipe writer close errors here are expected if we already called CloseWithError
					// for encoding failures, so we can safely ignore them
					_ = err
				}
			}()
			encoder := yaml.NewEncoder(pw)
			if err := encoder.Encode(v); err != nil {
				pw.CloseWithError(calque.WrapErr(context.Background(), err, "failed to encode YAML"))
			}
		}()
		return pr, nil
	case string:
		// Validate first, then stream if valid
		var temp any
		if err := yaml.Unmarshal([]byte(v), &temp); err != nil {
			return nil, calque.WrapErr(context.Background(), err, "invalid YAML string")
		}
		return strings.NewReader(v), nil
	case []byte:
		// Validate first, then stream if valid
		var temp any
		if err := yaml.Unmarshal(v, &temp); err != nil {
			return nil, calque.WrapErr(context.Background(), err, "invalid YAML bytes")
		}
		return bytes.NewReader(v), nil
	case io.Reader:
		// True streaming validation for io.Reader
		return y.createStreamingValidatingReader(v, "invalid YAML stream")
	default:
		// Use yaml.Encoder for streaming marshal of any other type
		pr, pw := io.Pipe()
		go func() {
			defer func() {
				if err := pw.Close(); err != nil {
					// Pipe writer close errors here are expected if we already called CloseWithError
					// for encoding failures, so we can safely ignore them
					_ = err
				}
			}()
			encoder := yaml.NewEncoder(pw)
			if err := encoder.Encode(y.data); err != nil {
				pw.CloseWithError(calque.WrapErr(context.Background(), err, fmt.Sprintf("failed to encode YAML for type %T", y.data)))
			}
		}()
		return pr, nil
	}
}

// createStreamingValidatingReader creates a streaming reader with chunked validation for io.Reader inputs
func (y *YAMLInputConverter) createStreamingValidatingReader(reader io.Reader, errorPrefix string) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		defer func() {
			if err := pw.Close(); err != nil {
				// Pipe writer close errors here are expected if we already called CloseWithError
				// for encoding failures, so we can safely ignore them
				_ = err
			}
		}()
		y.processStreamingValidation(reader, pw, errorPrefix)
	}()
	return pr, nil
}

// processStreamingValidation handles the complex streaming validation logic
func (y *YAMLInputConverter) processStreamingValidation(reader io.Reader, pw *io.PipeWriter, errorPrefix string) {
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
				if y.handleValidationCheck(&validationBuf, &tempBuf, bufWriter, pw, errorPrefix) {
					validationPassed = true
					break
				}
			}
		}

		if err == io.EOF {
			if y.handleFinalValidation(&validationBuf, &tempBuf, bufWriter, pw, errorPrefix) {
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
func (y *YAMLInputConverter) handleValidationCheck(validationBuf, tempBuf *bytes.Buffer, bufWriter *bufio.Writer, pw *io.PipeWriter, errorPrefix string) bool {
	var temp any
	validateErr := yaml.Unmarshal(validationBuf.Bytes(), &temp)

	if validateErr == nil {
		// Valid complete YAML - flush everything and switch to direct streaming
		if y.flushBufferedData(tempBuf, bufWriter, pw) {
			return true
		}

		// Continue streaming rest directly (validation passed)
		// Note: We can't continue streaming from the original reader here
		// as it's already been consumed by the teeReader
		if err := bufWriter.Flush(); err != nil {
			pw.CloseWithError(calque.WrapErr(context.Background(), err, "failed to flush buffer"))
			return true
		}
		return true
	} else if !isIncompleteYAMLError(validateErr) {
		// Definite validation error (not incomplete YAML)
		pw.CloseWithError(calque.WrapErr(context.Background(), validateErr, errorPrefix))
		return true
	}
	// Otherwise continue reading (YAML might be incomplete)
	return false
}

// handleFinalValidation processes final validation at EOF
func (y *YAMLInputConverter) handleFinalValidation(validationBuf, tempBuf *bytes.Buffer, bufWriter *bufio.Writer, pw *io.PipeWriter, errorPrefix string) bool {
	var temp any
	if finalErr := yaml.Unmarshal(validationBuf.Bytes(), &temp); finalErr != nil {
		pw.CloseWithError(calque.WrapErr(context.Background(), finalErr, errorPrefix))
		return true
	}

	// Stream final buffered data
	return y.flushBufferedData(tempBuf, bufWriter, pw)
}

// flushBufferedData flushes buffered data to the writer
func (y *YAMLInputConverter) flushBufferedData(tempBuf *bytes.Buffer, bufWriter *bufio.Writer, pw *io.PipeWriter) bool {
	if _, writeErr := io.Copy(bufWriter, tempBuf); writeErr != nil {
		pw.CloseWithError(writeErr)
		return true
	}
	if err := bufWriter.Flush(); err != nil {
		pw.CloseWithError(calque.WrapErr(context.Background(), err, "failed to flush buffer"))
		return true
	}
	return false
}

// Helper function to determine if YAML error is due to incomplete data
func isIncompleteYAMLError(err error) bool {
	if err == nil {
		return false
	}
	// YAML parsing errors that indicate incomplete data
	errStr := err.Error()
	return strings.Contains(errStr, "unexpected end of file") ||
		strings.Contains(errStr, "found unexpected end of stream") ||
		strings.Contains(errStr, "unexpected EOF") ||
		strings.Contains(errStr, "non-map value is specified") ||
		strings.Contains(errStr, "mapping value is not allowed in this context")
}

// FromReader reads YAML data from an io.Reader into the target variable.
func (y *YAMLOutputConverter) FromReader(reader io.Reader) error {
	// Use yaml.Decoder for streaming decode
	decoder := yaml.NewDecoder(reader)
	err := decoder.Decode(y.target)

	if err != nil {
		// Drain the reader on error to prevent pipe deadlock
		// When decode fails, the writer may still be trying to write data
		io.Copy(io.Discard, reader)

		// Handle EOF as empty input (valid for YAML)
		if err == io.EOF {
			return nil
		}
		return calque.WrapErr(context.Background(), err, "failed to decode YAML")
	}
	return nil
}
