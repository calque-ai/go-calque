package core

import (
	"context"
	"io"
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

// Run executes the flow with the given input and writes output to the provided pointer
func (f *Flow) Run(ctx context.Context, input any, output any) error {
	if len(f.handlers) == 0 {
		// No handlers, just copy input to output with conversion
		return f.copyInputToOutput(input, output)
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
	reader, err := f.inputToReader(input)
	if err != nil {
		return err
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

	//  Runs all handlers concurrently in goroutines for streaming
	//  Handler1: [========]
	//  Handler2:   [========]
	//  Handler3:     [========]
	var wg sync.WaitGroup
	errCh := make(chan error, len(f.handlers)+2) //create error chan with small extra buffer

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
	outputDone := make(chan error, 1)
	go func() {
		err := f.readerToOutput(finalReader, output)
		outputDone <- err
	}()

	// Waits for either: context cancellation, handler error, or all handlers complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-done:
		// Wait for output collection to complete
		return <-outputDone
	}
}
