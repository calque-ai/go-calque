package ctrl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// PassThrough creates a simple pass-through handler
func PassThrough() calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
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
//	jsonBranch := ctrl.Branch(
//	  func(b []byte) bool { return bytes.HasPrefix(b, []byte("{")) },
//	  jsonHandler,
//	  textHandler,
//	)
func Branch(condition func([]byte) bool, ifHandler calque.Handler, elseHandler calque.Handler) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input []byte
		err := calque.Read(req, &input)
		if err != nil {
			return err
		}

		req.Data = bytes.NewReader(input)

		if condition(input) {
			return ifHandler.ServeFlow(req, res)
		}
		return elseHandler.ServeFlow(req, res)
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
//	tee := ctrl.TeeReader(logFile, os.Stdout)
//	pipe.Use(tee) // Input goes to logFile, stdout, AND next handler
func TeeReader(destinations ...io.Writer) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Create MultiWriter to write to all destinations plus the output
		allWriters := append(destinations, res.Data)
		multiWriter := io.MultiWriter(allWriters...)

		_, err := io.Copy(multiWriter, req.Data)
		return err
	})
}

// Parallel executes multiple handlers concurrently with the same input stream.
//
// Input: any data type (streaming - uses TeeReader + MultiWriter for efficient fan-out)
// Output: bytes containing all handler outputs separated by "\n---\n"
// Behavior: STREAMING - input flows through TeeReader to all handlers simultaneously
//
// Uses io.TeeReader with io.MultiWriter to efficiently split the input stream
// to all handlers without complex pipe chains. Each handler processes the stream
// as data arrives. Results are collected and combined in completion order.
//
// If any handler fails, the entire operation fails. Empty handler list
// results in pass-through behavior.
//
// Example:
//
//	parallel := ctrl.Parallel(handler1, handler2, handler3)
//	// All three handlers process the same input concurrently via TeeReader
func Parallel(handlers ...calque.Handler) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		if len(handlers) == 0 {
			_, err := io.Copy(res.Data, req.Data)
			return err
		}

		// Create pipes for each handler
		writers := make([]io.Writer, len(handlers))
		readers := make([]*io.PipeReader, len(handlers))

		for i := range handlers {
			r, w := io.Pipe()
			readers[i] = r
			writers[i] = w
		}

		// Single TeeReader with MultiWriter - much simpler than pipe chains!
		multiWriter := io.MultiWriter(writers...)
		teeReader := io.TeeReader(req.Data, multiWriter)

		// Consume the tee'd stream and close writers when done
		go func() {
			defer func() {
				for _, w := range writers {
					w.(*io.PipeWriter).Close()
				}
			}()
			io.Copy(io.Discard, teeReader)
		}()

		type result struct {
			index  int
			output []byte
			err    error
		}

		results := make(chan result, len(handlers))

		// Run handlers concurrently on their streams
		for i, handler := range handlers {
			go func(idx int, h calque.Handler, reader *io.PipeReader) {
				var output bytes.Buffer
				handlerReq := &calque.Request{Context: req.Context, Data: reader}
				handlerRes := &calque.Response{Data: &output}

				err := h.ServeFlow(handlerReq, handlerRes)
				results <- result{idx, output.Bytes(), err}
			}(i, handler, readers[i])
		}

		// Collect results preserving original order
		outputs := make([][]byte, len(handlers))
		for range handlers {
			select {
			case <-req.Context.Done():
				return req.Context.Err()
			case res := <-results:
				if res.err != nil {
					return res.err
				}
				outputs[res.index] = res.output
			}
		}

		// Combine results
		combined := bytes.Join(outputs, []byte("\n---\n"))
		err := calque.Write(res, combined)
		return err
	})
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
//	timeoutHandler := ctrl.Timeout(someHandler, 30*time.Second)
//	pipe.Use(timeoutHandler)
func Timeout(handler calque.Handler, timeout time.Duration) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		timeoutCtx, cancel := context.WithTimeout(req.Context, timeout)
		defer cancel()

		req.Context = timeoutCtx //update req context

		done := make(chan error, 1)
		go func() {
			done <- handler.ServeFlow(req, res)
		}()

		select {
		case err := <-done:
			return err
		case <-timeoutCtx.Done():
			return fmt.Errorf("handler timeout after %v: %w", timeout, timeoutCtx.Err())
		}
	})
}

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
//	retryHandler := ctrl.Retry(someHandler, 3)
//	pipe.Use(retryHandler)
func Retry(handler calque.Handler, maxAttempts int) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input []byte
		err := calque.Read(req, &input)
		if err != nil {
			return err
		}

		var lastErr error
		for attempt := range maxAttempts {
			// Reset the request data for each attempt
			req.Data = bytes.NewReader(input)

			var output bytes.Buffer
			tempRes := &calque.Response{Data: &output}
			err := handler.ServeFlow(req, tempRes)
			if err == nil {
				_, writeErr := res.Data.Write(output.Bytes())
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
