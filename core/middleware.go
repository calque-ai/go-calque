package core

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// Retry wraps a handler with retry logic
func Retry(handler Handler, maxAttempts int) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		var lastErr error
		for attempt := range maxAttempts {
			var output strings.Builder
			err := handler.ServeFlow(ctx, strings.NewReader(string(input)), &output)
			if err == nil {
				_, writeErr := w.Write([]byte(output.String()))
				return writeErr
			}
			lastErr = err

			// Exponential backoff
			if attempt < maxAttempts-1 {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			}
		}

		return fmt.Errorf("retry exhausted: %w", lastErr)
	})
}

// Branch creates conditional routing like an if-else
func Branch(condition func(string) bool, ifHandler Handler, elseHandler Handler) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		if condition(string(input)) {
			return ifHandler.ServeFlow(ctx, strings.NewReader(string(input)), w)
		}
		return elseHandler.ServeFlow(ctx, strings.NewReader(string(input)), w)
	})
}

// Parallel runs multiple handlers concurrently and combines results
func Parallel(handlers ...Handler) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		type result struct {
			output string
			err    error
		}

		results := make(chan result, len(handlers))

		for _, handler := range handlers {
			go func(h Handler) {
				var output strings.Builder
				err := h.ServeFlow(ctx, strings.NewReader(string(input)), &output)
				results <- result{output.String(), err}
			}(handler)
		}

		var outputs []string
		for range handlers {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case res := <-results:
				if res.err != nil {
					return res.err
				}
				outputs = append(outputs, res.output)
			}
		}

		// Combine results
		combined := strings.Join(outputs, "\n---\n")
		_, err = w.Write([]byte(combined))
		return err
	})
}

// TeeReader creates a handler that copies input to multiple destinations
func TeeReader(destinations ...io.Writer) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Create MultiWriter to write to all destinations plus the output
		allWriters := append(destinations, w)
		multiWriter := io.MultiWriter(allWriters...)

		_, err := io.Copy(multiWriter, r)
		return err
	})
}

// Transform creates a transformation handler
func Transform(fn func(string) string) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		output := fn(string(input))
		_, err = w.Write([]byte(output))
		return err
	})
}

// Filter creates a conditional handler
func Filter(condition func(string) bool, handler Handler) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		if condition(string(input)) {
			return handler.ServeFlow(ctx, strings.NewReader(string(input)), w)
		}

		// Pass through unchanged
		_, err = w.Write(input)
		return err
	})
}

// Logger adds logging middleware
func Logger(prefix string) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		fmt.Printf("[%s] Processing: %s\n", prefix, string(input))

		// Pass through unchanged
		_, err = w.Write(input)
		return err
	})
}
