package core

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
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
//   retryHandler := core.Retry(someHandler, 3)
//   flow.Use(retryHandler)
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

// Branch creates conditional routing based on input content evaluation.
//
// Input: any data type (buffered - reads entire input into memory)
// Output: depends on which handler is executed (ifHandler or elseHandler)
// Behavior: BUFFERED - must read entire input to evaluate condition
//
// The condition function receives the entire input as a string and returns a boolean.
// If true, ifHandler is executed; if false, elseHandler is executed.
// Both handlers receive the same original input.
//
// Example:
//   jsonBranch := core.Branch(
//     func(s string) bool { return strings.HasPrefix(s, "{") },
//     jsonHandler,
//     textHandler,
//   )
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

// Parallel executes multiple handlers concurrently with the same input stream.
//
// Input: any data type (streaming - uses io.Pipe for efficient fan-out)
// Output: string containing all handler outputs separated by "\n---\n"
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
//   parallel := core.Parallel(handler1, handler2, handler3)
//   // All three handlers process the same input concurrently
func Parallel(handlers ...Handler) Handler {
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
			output string
			err    error
		}

		results := make(chan result, len(handlers))

		// Run handlers concurrently on their streams
		for i, handler := range handlers {
			go func(h Handler, reader io.Reader) {
				var output strings.Builder
				err := h.ServeFlow(ctx, reader, &output)
				results <- result{output.String(), err}
			}(handler, readers[i])
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
		_, err := w.Write([]byte(combined))
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
//   logFile, _ := os.Create("flow.log")
//   tee := core.TeeReader(logFile, os.Stdout)
//   flow.Use(tee) // Input goes to logFile, stdout, AND next handler
func TeeReader(destinations ...io.Writer) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Create MultiWriter to write to all destinations plus the output
		allWriters := append(destinations, w)
		multiWriter := io.MultiWriter(allWriters...)

		_, err := io.Copy(multiWriter, r)
		return err
	})
}

// Transform applies a function to transform the entire input content.
//
// Input: any data type (buffered - reads entire input into memory)
// Output: string (result of transformation function)
// Behavior: BUFFERED - must read entire input to apply transformation
//
// The transformation function receives the entire input as a string and
// returns the transformed string. Useful for text processing, formatting,
// or content modification that requires the complete input.
//
// Example:
//   upperCase := core.Transform(strings.ToUpper)
//   reverse := core.Transform(func(s string) string {
//     runes := []rune(s)
//     for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
//       runes[i], runes[j] = runes[j], runes[i]
//     }
//     return string(runes)
//   })
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

// Filter conditionally processes input based on content evaluation.
//
// Input: any data type (buffered - reads entire input into memory)
// Output: depends on condition - either handler output or original input
// Behavior: BUFFERED - must read entire input to evaluate condition
//
// If the condition function returns true, the input is processed by the handler.
// If false, the original input passes through unchanged. The condition function
// receives the entire input as a string.
//
// Example:
//   jsonFilter := core.Filter(
//     func(s string) bool { return json.Valid([]byte(s)) },
//     jsonProcessor,
//   )
//   // Only valid JSON gets processed, everything else passes through
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

// Logger provides non-intrusive logging of input stream with preview.
//
// Input: any data type (streaming - uses bufio.Reader.Peek for preview)
// Output: same as input (pass-through)
// Behavior: STREAMING - peeks at first 100 bytes without consuming, then streams
//
// Logs a preview of the input (first 100 bytes) with the specified prefix,
// then passes the complete input through unchanged. Uses buffered peeking
// to avoid consuming the input stream.
//
// Example:
//   logger := core.Logger("STEP1")
//   flow.Use(logger) // Prints: [STEP1] Processing: {"key":"value"}...
func Logger(prefix string) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		bufReader := bufio.NewReader(r)
		
		// Peek at first line for logging without consuming
		firstLine, err := bufReader.Peek(100)
		if err != nil && err != io.EOF {
			return err
		}
		
		// Log truncated preview
		preview := string(firstLine)
		if len(firstLine) == 100 {
			preview += "..."
		}
		fmt.Printf("[%s] Processing: %s\n", prefix, preview)

		// Pass through unchanged using buffered reader
		_, err = io.Copy(w, bufReader)
		return err
	})
}

// LineProcessor transforms input line-by-line using buffered scanning.
//
// Input: any data type (streaming - uses bufio.Scanner for line-by-line processing)
// Output: string (processed lines separated by newlines)
// Behavior: STREAMING - processes each line as it's read, memory efficient
//
// Reads input line by line and applies the transformation function to each line.
// Output lines are written immediately, making this memory efficient for large
// inputs. Each output line ends with a newline character.
//
// Example:
//   addLineNumbers := core.LineProcessor(func(line string) string {
//     return fmt.Sprintf("%d: %s", lineNum, line)
//   })
//   csvProcessor := core.LineProcessor(func(line string) string {
//     return strings.ToUpper(line) // Convert CSV to uppercase
//   })
func LineProcessor(fn func(string) string) Handler {
	return HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		scanner := bufio.NewScanner(r)
		
		for scanner.Scan() {
			line := scanner.Text()
			processed := fn(line)
			if _, err := fmt.Fprintln(w, processed); err != nil {
				return err
			}
		}
		
		return scanner.Err()
	})
}