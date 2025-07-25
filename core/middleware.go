package core

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"
)

// Retry wraps a handler with retry logic and exponential backoff.
//
// Input: any data type (buffered - reads entire input into memory)
// Output: same type as wrapped handler's output
// Behavior: BUFFERED - must read entire input to replay on retries
//
// The function attempts to execute the wrapped handler up to maxAttempts times.
// If the handler fails, it retries with exponential backoff (100ms, 200ms, 400ms, etc.).
// The same input is replayed for each retry attempt.
//
// Example:
//
//	retryHandler := core.Retry(someHandler, 3)
//	flow.Use(retryHandler)
func Retry[T any](handler Handler, maxAttempts int) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		var lastErr error
		for attempt := range maxAttempts {
			var output bytes.Buffer
			err := handler.ServeFlow(ctx, bytes.NewReader(input), &output)
			if err == nil {
				_, writeErr := w.Write(output.Bytes())
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

// Parallel executes multiple handlers concurrently with the same input stream.
//
// Input: any data type (streaming - uses io.Pipe for efficient fan-out)
// Output: bytes containing all handler outputs separated by "\n---\n"
// Behavior: STREAMING - input is fanned out to all handlers simultaneously
//
// Each handler receives the same input stream via io.Pipe. All handlers start
// processing immediately as data arrives. Results are collected and combined
// in the order handlers complete (not necessarily input order).
//
// If any handler fails, the entire operation fails. Empty handler list
// results in pass-through behavior.
//
// Example:
//
//	parallel := core.Parallel[[]byte](handler1, handler2, handler3)
//	// All three handlers process the same input concurrently
func Parallel[T any](handlers ...Handler) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		if len(handlers) == 0 {
			_, err := io.Copy(w, r)
			return err
		}

		// Create pipe pairs for each handler
		readers := make([]*io.PipeReader, len(handlers))
		writers := make([]*io.PipeWriter, len(handlers))

		for i := range handlers {
			readers[i], writers[i] = io.Pipe()
		}

		// Create a MultiWriter to fan out input to all handlers
		multiWriter := io.MultiWriter(func() []io.Writer {
			ws := make([]io.Writer, len(writers))
			for i, pw := range writers {
				ws[i] = pw
			}
			return ws
		}()...)

		// Fan out input to all handlers in background
		go func() {
			defer func() {
				for _, pw := range writers {
					pw.Close()
				}
			}()
			io.Copy(multiWriter, r)
		}()

		type result struct {
			output []byte
			err    error
		}

		results := make(chan result, len(handlers))

		// Run handlers concurrently on their streams
		for i, handler := range handlers {
			go func(h Handler, reader io.Reader) {
				var output bytes.Buffer
				err := h.ServeFlow(ctx, reader, &output)
				results <- result{output.Bytes(), err}
			}(handler, readers[i])
		}

		var outputs [][]byte
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

		combined := bytes.Join(outputs, []byte("\n---\n"))
		_, err := w.Write(combined)
		return err
	})
}

// TeeReader copies input stream to multiple destinations while passing through.
//
// Input: any data type (streaming - uses io.Copy for efficient copying)
// Output: same as input (pass-through)
// Behavior: STREAMING - copies data as it flows through
//
// Input is simultaneously written to all specified destinations AND the output.
// Useful for logging, debugging, or saving copies while maintaining the flow.
// Uses io.MultiWriter for efficient simultaneous copying.
//
// Example:
//
//	logFile, _ := os.Create("flow.log")
//	tee := core.TeeReader(logFile, os.Stdout)
//	flow.Use(tee) // Input goes to logFile, stdout, AND next handler
func TeeReader(destinations ...io.Writer) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Create MultiWriter to write to all destinations plus the output
		allWriters := append(destinations, w)
		multiWriter := io.MultiWriter(allWriters...)

		_, err := io.Copy(multiWriter, r)
		return err
	})
}

// Logger provides non-intrusive logging of input stream with preview.
//
// Input: any data type (streaming - uses bufio.Reader.Peek for preview)
// Output: same as input (pass-through)
// Behavior: STREAMING - peeks at first 100 bytes without consuming, then streams
//
// Logs a preview of the input (first 100 bytes) with the specified prefix,
// then passes the complete input through unchanged. Uses buffered peeking
// to avoid consuming the input stream. Optionally accepts a custom logger,
// defaults to log.Default() if none provided.
//
// Example:
//
//	logger := core.Logger("STEP1")                    // Uses default logger
//	customLogger := core.Logger("STEP1", myLogger)   // Uses custom logger
//	flow.Use(logger) // Logs: [STEP1]: Hello, world!
func Logger(prefix string, logger ...LoggerInterface) Handler {
	var l LoggerInterface
	if len(logger) > 0 {
		l = logger[0]
	} else {
		l = log.Default()
	}

	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		bufReader := bufio.NewReader(r)

		// Peek at first line for logging without consuming
		firstLine, err := bufReader.Peek(100)
		if err != nil && err != io.EOF {
			return err
		}

		// Log smart formatted preview
		preview := formatPreview(firstLine)
		l.Printf("[%s]: %s\n", prefix, preview)

		// Pass through unchanged using buffered reader
		_, err = io.Copy(w, bufReader)
		return err
	})
}

// LoggerInterface allows custom logging implementations
type LoggerInterface interface {
	Printf(format string, v ...any)
}

// formatPreview creates a readable preview of data, handling both text and binary content
func formatPreview(data []byte) string {
	if len(data) == 0 {
		return "<empty>"
	}

	// Try to detect if it's printable text
	if isPrintable(data) {
		preview := string(data)
		if len(data) == 100 {
			preview += "..."
		}
		return preview
	}

	// For binary data, show hex summary
	if len(data) > 20 {
		return fmt.Sprintf("binary data (%d bytes): %x...", len(data), data[:20])
	}
	return fmt.Sprintf("binary data: %x", data)
}

// isPrintable checks if all bytes are printable ASCII characters
func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 {
			// Allow common whitespace characters
			if b != '\t' && b != '\n' && b != '\r' {
				return false
			}
		}
	}
	return true
}

// Timeout wraps a handler with timeout protection.
//
// Input: any data type (passes through unchanged)
// Output: same as wrapped handler's output
// Behavior: STREAMING - cancels context if timeout exceeded
//
// The handler execution is cancelled if it takes longer than the specified timeout.
// Uses context cancellation for clean shutdown. If the context is already cancelled
// or has a deadline, the shorter timeout takes precedence.
//
// Example:
//
//	timeoutHandler := core.Timeout[string](someHandler, 30*time.Second)
//	flow.Use(timeoutHandler)
func Timeout[T any](handler Handler, timeout time.Duration) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- handler.ServeFlow(timeoutCtx, r, w)
		}()

		select {
		case err := <-done:
			return err
		case <-timeoutCtx.Done():
			return fmt.Errorf("handler timeout after %v: %w", timeout, timeoutCtx.Err())
		}
	})
}

// Branch creates conditional routing based on input content evaluation.
//
// Input: any data type (buffered - reads entire input into memory)
// Output: depends on which handler is executed (ifHandler or elseHandler)
// Behavior: BUFFERED - must read entire input to evaluate condition
//
// The condition function receives the entire input as bytes and returns a boolean.
// If true, ifHandler is executed; if false, elseHandler is executed.
// Both handlers receive the same original input.
//
// Example:
//
//	jsonBranch := core.Branch[[]byte](
//	  func(b []byte) bool { return bytes.HasPrefix(b, []byte("{")) },
//	  jsonHandler,
//	  textHandler,
//	)
func Branch[T any](condition func([]byte) bool, ifHandler Handler, elseHandler Handler) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		if condition(input) {
			return ifHandler.ServeFlow(ctx, bytes.NewReader(input), w)
		}
		return elseHandler.ServeFlow(ctx, bytes.NewReader(input), w)
	})
}
