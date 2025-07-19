package core

import (
	"context"
	"io"
	"reflect"
	"strings"
	"sync"
)

// Flow is the core orchestration primitive
type Flow struct {
	handlers []Handler
}

func New() *Flow {
	return &Flow{}
}

// Use adds a handler to the flow
func (f *Flow) Use(handler Handler) *Flow {
	f.handlers = append(f.handlers, handler)
	return f
}

// UseFunc adds a function as a handler
func (f *Flow) UseFunc(fn HandlerFunc) *Flow {
	return f.Use(fn)
}

// Run executes the flow with the given input
func (f *Flow) Run(ctx context.Context, input any) (any, error) {
	if len(f.handlers) == 0 {
		return input, nil
	}

	// Create a chain of pipes between handlers
	pipes := make([]struct {
		r *PipeReader
		w *PipeWriter
	}, len(f.handlers))

	// Creates pipe pairs (r, w) for each handler - these connect handlers together
	for i := 0; i < len(f.handlers); i++ {
		pipes[i].r, pipes[i].w = Pipe()
	}

	// Convert input to reader
	reader, inputType, err := f.inputToReader(input)
	if err != nil {
		return nil, err
	}

	// Creates inputReader for the first handler's input
	inputR, inputW := io.Pipe()                    // Create a pipe for input
	inputReader := &PipeReader{PipeReader: inputR} // Wraps the input reader
	go func() {
		defer inputW.Close()
		io.Copy(inputW, reader) // Copy input reader to pipe writer
	}()

	// Sets finalReader to read the last handler's output
	var finalReader io.Reader
	if len(f.handlers) > 0 {
		finalReader = pipes[len(pipes)-1].r
	} else {
		finalReader = inputReader
	}

	//  Runs all handlers concurrently in goroutines
	var wg sync.WaitGroup
	errCh := make(chan error, len(f.handlers))

	for i, handler := range f.handlers {
		wg.Add(1)
		go func(idx int, h Handler) {
			defer wg.Done()
			defer pipes[idx].w.Close()

			var reader io.Reader
			if idx == 0 {
				reader = inputReader // Handler 0 reads from inputReader
			} else {
				reader = pipes[idx-1].r // Subsequent handlers read from the previous pipe's reader
			}

			// Each handler writes to its own pipe writer, which feeds the next handler
			if err := h.ServeFlow(ctx, reader, pipes[idx].w); err != nil {
				errCh <- err
			}
		}(i, handler)
	}

	// Consume final output in background
	type outputResult struct {
		data any
		err  error
	}
	outputDone := make(chan outputResult, 1)
	go func() {
		result, err := f.readerToOutput(finalReader, inputType)
		outputDone <- outputResult{result, err}
	}()

	// Waits for either: context cancellation, handler error, or all handlers complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case <-done:
		// Wait for output collection to complete
		result := <-outputDone
		return result.data, result.err
	}
}

type inputType int

const (
	typeString inputType = iota
	typeBytes
	typeReader
	typeStruct
)

// inputToReader converts any input to io.Reader and remembers the type
func (f *Flow) inputToReader(input any) (io.Reader, inputType, error) {
	switch v := input.(type) {
	case string:
		return strings.NewReader(v), typeString, nil
	case []byte:
		return strings.NewReader(string(v)), typeBytes, nil
	case io.Reader:
		return v, typeReader, nil
	default:
		// For structs/complex types, marshal to bytes first
		// Middleware can unmarshal back to their expected types
		data := f.marshalStruct(v)
		return strings.NewReader(string(data)), typeStruct, nil
	}
}

// readerToOutput converts io.Reader back to the expected output type
func (f *Flow) readerToOutput(reader io.Reader, originalType inputType) (any, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	switch originalType {
	case typeString:
		return string(data), nil
	case typeBytes:
		return data, nil
	case typeReader:
		return strings.NewReader(string(data)), nil
	case typeStruct:
		// Return as bytes for middleware to unmarshal
		return data, nil
	default:
		return string(data), nil
	}
}

// Simple struct marshaling (can be enhanced)
func (f *Flow) marshalStruct(v any) []byte {
	// Simple approach - convert to string representation
	// Real implementation might use JSON/YAML/etc
	return []byte(reflect.ValueOf(v).String())
}
