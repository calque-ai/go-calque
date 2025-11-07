package calque

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// Handler defines the core interface for processing streaming data in pipelines.
//
// Input: *Request containing context and input data stream
// Output: error if processing fails
// Behavior: STREAMING - processes data from Request.Data io.Reader to Response.Data io.Writer
//
// All pipeline middleware implements this interface. Handlers receive streaming data
// via io.Reader and write results to io.Writer, enabling constant memory usage
// regardless of data size. Context cancellation is available for timeouts and cleanup.
//
// Example implementation:
//
//	type MyHandler struct{}
//
//	func (h MyHandler) ServeFlow(req *calque.Request, res *calque.Response) error {
//		var input string
//		if err := calque.Read(req, &input); err != nil {
//			return err
//		}
//		processed := strings.ToUpper(input)
//		return calque.Write(res, processed)
//	}
type Handler interface {
	ServeFlow(*Request, *Response) error
}

// HandlerFunc allows regular functions to be used as Handlers via type conversion.
//
// Input: function matching the handler signature
// Output: implements Handler interface
// Behavior: Adapter pattern for function-to-interface conversion
//
// This type adapter enables using regular functions as handlers without
// defining custom types. The function signature must match exactly:
// func(req *Request, res *Response) error
//
// Example:
//
//	upperCase := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
//		var input string
//		calque.Read(req, &input)
//		return calque.Write(res, strings.ToUpper(input))
//	})
//
//	pipeline.Use(upperCase)
type HandlerFunc func(req *Request, res *Response) error

// ServeFlow implements the Handler interface for HandlerFunc.
func (f HandlerFunc) ServeFlow(req *Request, res *Response) error {
	return f(req, res)
}

// Request represents input data and context for handler processing.
//
// Context provides cancellation, timeouts, and request-scoped values.
// Data is the input stream that handlers read from using io.Reader methods.
// Both fields are immutable once created to ensure thread safety across goroutines.
//
// Example usage in handlers:
//
//	func myHandler(req *calque.Request, res *calque.Response) error {
//		// Check for cancellation
//		select {
//		case <-req.Done():
//			return req.Context.Err()
//		default:
//		}
//
//		// Read streaming data
//		var input string
//		return calque.Read(req, &input)
//	}
type Request struct {
	Context context.Context
	Data    io.Reader
}

// NewRequest creates a new Request with the provided context and data stream.
//
// Input: context.Context for cancellation, io.Reader for data stream
// Output: *Request ready for handler processing
// Behavior: Creates immutable request wrapper
//
// This constructor is primarily used internally by the pipeline execution engine.
// Most users interact with requests passed to their handler functions.
//
// Example:
//
//	req := calque.NewRequest(context.Background(), strings.NewReader("data"))
func NewRequest(ctx context.Context, data io.Reader) *Request {
	return &Request{Context: ctx, Data: data}
}

// WithContext creates a new Request with the same data but different context.
//
// Input: context.Context to replace the current context
// Output: *Request with new context and same data stream
// Behavior: Creates new Request instance (immutable pattern)
//
// Useful for adding timeouts, cancellation, or request-scoped values
// while preserving the original data stream.
//
// Example:
//
//	timeoutCtx, cancel := context.WithTimeout(req.Context, 30*time.Second)
//	defer cancel()
//	newReq := req.WithContext(timeoutCtx)
func (r *Request) WithContext(ctx context.Context) *Request {
	return &Request{Context: ctx, Data: r.Data}
}

// Deadline returns the context deadline if set.
//
// Output: time.Time deadline and bool indicating if deadline is set
// Behavior: Delegates to underlying context.Context.Deadline()
//
// Convenience method for checking request timeouts without accessing Context directly.
func (r *Request) Deadline() (time.Time, bool) {
	return r.Context.Deadline()
}

// Done returns a channel that closes when the request context is cancelled.
//
// Output: <-chan struct{} that closes on context cancellation
// Behavior: Delegates to underlying context.Context.Done()
//
// Use for non-blocking cancellation checks in handlers:
//
//	select {
//	case <-req.Done():
//		return req.Context.Err()
//	default:
//		// Continue processing
//	}
func (r *Request) Done() <-chan struct{} {
	return r.Context.Done()
}

// Response represents the output destination for handler processing.
//
// Data is the output stream that handlers write to using io.Writer methods.
// The field is immutable once created to ensure thread safety across goroutines.
// Handlers should write their processed results to this stream.
//
// Example usage in handlers:
//
//	func myHandler(req *calque.Request, res *calque.Response) error {
//		// Simple write
//		_, err := res.Data.Write([]byte("processed data"))
//		return err
//
//		// Or use convenience function
//		return calque.Write(res, "processed data")
//	}
type Response struct {
	Data io.Writer
}

// NewResponse creates a new Response with the provided output stream.
//
// Input: io.Writer for output stream
// Output: *Response ready for handler output
// Behavior: Creates immutable response wrapper
//
// This constructor is primarily used internally by the pipeline execution engine.
// Most users interact with responses passed to their handler functions.
//
// Example:
//
//	var buf bytes.Buffer
//	res := calque.NewResponse(&buf)
func NewResponse(data io.Writer) *Response {
	return &Response{Data: data}
}

// Read is a generic utility function for reading data from a Request in handlers.
//
// Input: *Request containing data stream, pointer to output variable (string or []byte)
// Output: error if reading fails
// Behavior: BUFFERED - reads entire input stream into memory then converts to target type
//
// Simplifies the common pattern of reading all data and converting to string or []byte.
// Uses io.ReadAll internally, so entire input is loaded into memory. For streaming
// processing of large data, use req.Data directly with io.Copy or similar methods.
//
// Supported target types: string, []byte
//
// Example usage:
//
//	func myHandler(req *calque.Request, res *calque.Response) error {
//		var input string
//		if err := calque.Read(req, &input); err != nil {
//			return err
//		}
//
//		processed := strings.ToUpper(input)
//		return calque.Write(res, processed)
//	}
func Read[T string | []byte](req *Request, outPtr *T) error {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, req.Data)
	if err != nil {
		return err
	}

	switch ptr := any(outPtr).(type) {
	case *string:
		*ptr = buf.String()
	case *[]byte:
		*ptr = buf.Bytes()
	default:
		return fmt.Errorf("unsupported type %T", outPtr)
	}
	return nil
}

// Write is a generic utility function for writing data to a Response in handlers.
//
// Input: *Response containing output stream, data to write (string or []byte)
// Output: error if writing fails
// Behavior: BUFFERED - converts input to []byte then writes entire content at once
//
// Simplifies the common pattern of converting string or []byte data to bytes for writing.
// Handles type conversion automatically. For streaming large data or when you need
// more control over writing, use res.Data.Write() directly.
//
// Supported input types: string, []byte
//
// Example usage:
//
//	func myHandler(req *calque.Request, res *calque.Response) error {
//		var input string
//		calque.Read(req, &input)
//
//		processed := strings.ToUpper(input)
//		return calque.Write(res, processed)  // Handles string -> []byte conversion
//	}
func Write[T string | []byte](res *Response, data T) error {
	switch v := any(data).(type) {
	case string:
		_, err := res.Data.Write([]byte(v))
		return err
	case []byte:
		_, err := res.Data.Write(v)
		return err
	default:
		return fmt.Errorf("unsupported type %T", data)
	}
}

// NewReader creates a new reader from various input types for testing
func NewReader[T string | []byte](data T) io.Reader {
	switch v := any(data).(type) {
	case string:
		return strings.NewReader(v)
	case []byte:
		return bytes.NewReader(v)
	default:
		return strings.NewReader("")
	}
}

// NewWriter creates a new writer that can be used for testing
func NewWriter[T string | []byte]() *Buffer[T] {
	return &Buffer[T]{}
}

// Buffer is a generic buffer implementation for testing
type Buffer[T string | []byte] struct {
	data []byte
}

func (b *Buffer[T]) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *Buffer[T]) String() string {
	return string(b.data)
}

// Bytes returns the buffer contents as a byte slice.
func (b *Buffer[T]) Bytes() []byte {
	return b.data
}

// Get returns the buffer contents as the specified type
func (b *Buffer[T]) Get() T {
	switch any(*new(T)).(type) {
	case string:
		return any(string(b.data)).(T)
	case []byte:
		return any(b.data).(T)
	default:
		return *new(T)
	}
}
