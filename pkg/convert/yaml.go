package convert

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/goccy/go-yaml"
)

// Input converter for YAML data -> YAML bytes
type yamlInputConverter struct {
	data any
}

// Output converter for YAML bytes -> any type
type yamlOutputConverter struct {
	target any
}

// ToYaml creates an input converter for transforming structured data to YAML streams.
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
//	err := pipeline.Run(ctx, convert.ToYaml(config), &result)
func ToYaml(data any) calque.InputConverter {
	return &yamlInputConverter{data: data}
}

// FromYaml creates an output converter for parsing YAML streams to structured data.
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
//	err := pipeline.Run(ctx, input, convert.FromYaml(&config))
//	fmt.Printf("DB: %s:%d\n", config.Database.Host, config.Database.Port)
func FromYaml(target any) calque.OutputConverter {
	return &yamlOutputConverter{target: target}
}

// InputConverter interface
func (y *yamlInputConverter) ToReader() (io.Reader, error) {
	switch v := y.data.(type) {
	case map[string]any, map[any]any, []any:
		// Use yaml.Encoder for streaming marshal of structured data
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			encoder := yaml.NewEncoder(pw)
			if err := encoder.Encode(v); err != nil {
				pw.CloseWithError(fmt.Errorf("failed to encode YAML: %w", err))
			}
		}()
		return pr, nil
	case string:
		// Validate first, then stream if valid
		var temp any
		if err := yaml.Unmarshal([]byte(v), &temp); err != nil {
			return nil, fmt.Errorf("invalid YAML string: %w", err)
		}
		return strings.NewReader(v), nil
	case []byte:
		// Validate first, then stream if valid
		var temp any
		if err := yaml.Unmarshal(v, &temp); err != nil {
			return nil, fmt.Errorf("invalid YAML bytes: %w", err)
		}
		return bytes.NewReader(v), nil
	case io.Reader:
		// True streaming validation for io.Reader
		return y.createStreamingValidatingReader(v, "invalid YAML stream")
	default:
		// Use yaml.Encoder for streaming marshal of any other type
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			encoder := yaml.NewEncoder(pw)
			if err := encoder.Encode(y.data); err != nil {
				pw.CloseWithError(fmt.Errorf("failed to encode YAML for type %T: %w", y.data, err))
			}
		}()
		return pr, nil
	}
}

// createStreamingValidatingReader creates a streaming reader with chunked validation for io.Reader inputs
func (y *yamlInputConverter) createStreamingValidatingReader(reader io.Reader, errorPrefix string) (io.Reader, error) {
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
			}

			// Handle read errors first (except EOF)
			if err != nil && err != io.EOF {
				pw.CloseWithError(err)
				return
			}

			// Skip validation if not enough data and not EOF
			if totalRead < 2048 && err != io.EOF {
				continue
			}

			// Validate YAML
			var temp any
			validateErr := yaml.Unmarshal(validationBuf.Bytes(), &temp)

			// Handle validation success - flush and continue
			if validateErr == nil {
				if _, writeErr := io.Copy(bufWriter, &tempBuf); writeErr != nil {
					pw.CloseWithError(writeErr)
					return
				}
				bufWriter.Flush()

				// Continue streaming rest directly if not EOF
				if err != io.EOF {
					if _, copyErr := io.Copy(bufWriter, reader); copyErr != nil {
						pw.CloseWithError(copyErr)
						return
					}
				}
				bufWriter.Flush()
				return
			}

			// Handle validation failure
			if !isIncompleteYAMLError(validateErr) {
				pw.CloseWithError(fmt.Errorf("%s: %w", errorPrefix, validateErr))
				return
			}

			// If EOF and still invalid, it's a final error
			if err == io.EOF {
				pw.CloseWithError(fmt.Errorf("%s: %w", errorPrefix, validateErr))
				return
			}
		}
	}()
	return pr, nil
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

// OutputConverter interface
func (y *yamlOutputConverter) FromReader(reader io.Reader) error {
	// Use yaml.Decoder for streaming decode
	decoder := yaml.NewDecoder(reader)
	if err := decoder.Decode(y.target); err != nil {
		// Handle EOF as empty input (valid for YAML)
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("failed to decode YAML: %w", err)
	}
	return nil
}
