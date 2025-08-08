package calque

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Handler is the core abstraction
type Handler interface {
	ServeFlow(*Request, *Response) error
}

// HandlerFunc allows regular functions to be used as Handlers
type HandlerFunc func(req *Request, res *Response) error

func (f HandlerFunc) ServeFlow(req *Request, res *Response) error {
	return f(req, res)
}

type Request struct {
	Context context.Context
	Data    io.Reader
}

func NewRequest(ctx context.Context, data io.Reader) *Request {
	return &Request{Context: ctx, Data: data}
}

// Convenience methods
func (r *Request) WithContext(ctx context.Context) *Request {
	return &Request{Context: ctx, Data: r.Data}
}

func (r *Request) Deadline() (time.Time, bool) {
	return r.Context.Deadline()
}

func (r *Request) Done() <-chan struct{} {
	return r.Context.Done()
}

type Response struct {
	Data io.Writer
}

func NewResponse(data io.Writer) *Response {
	return &Response{Data: data}
}

// Read is a generic utility function for reading data from a Request in handlers.
// It simplifies the common pattern of reading all data and converting it to string or []byte.
//
// Supported types: string, []byte
//
// Usage in handlers:
//
//	func myHandler() core.Handler {
//	    return core.HandlerFunc(func(req *core.Request, res *core.Response) error {
//	        var input string
//	        if err := core.Read(req, &input); err != nil {
//	            return err
//	        }
//
//	        // Process input...
//	        processed := strings.ToUpper(input)
//
//	        return core.Write(res, processed)
//	    })
//	}
func Read[T string | []byte](req *Request, outPtr *T) error {
	data, err := io.ReadAll(req.Data)
	if err != nil {
		return err
	}

	switch ptr := any(outPtr).(type) {
	case *string:
		*ptr = string(data)
	case *[]byte:
		*ptr = data
	default:
		return fmt.Errorf("unsupported type %T", outPtr)
	}
	return nil
}

// Write is a generic utility function for writing data to a Response in handlers.
// It simplifies the common pattern of converting string or []byte data to bytes for writing.
//
// Supported types: string, []byte
//
// Usage in handlers:
//
//	func myHandler() core.Handler {
//	    return core.HandlerFunc(func(req *core.Request, res *core.Response) error {
//	        var input string
//	        core.Read(req, &input)
//
//	        processed := strings.ToUpper(input)
//
//	        return core.Write(res, processed)  // Handles string -> []byte conversion
//	    })
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
