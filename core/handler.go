package core

import (
	"context"
	"fmt"
	"io"
)

// Handler is the core abstraction
type Handler interface {
	ServeFlow(ctx context.Context, r io.Reader, w io.Writer) error
}

// HandlerFunc allows regular functions to be used as Handlers
type HandlerFunc func(ctx context.Context, r io.Reader, w io.Writer) error

func (f HandlerFunc) ServeFlow(ctx context.Context, r io.Reader, w io.Writer) error {
	return f(ctx, r, w)
}

// Read is a generic utility function for reading data from an io.Reader in handlers.
// It simplifies the common pattern of reading all data and converting it to string or []byte.
//
// Supported types: string, []byte
//
// Usage in handlers:
//
//	func myHandler() core.Handler {
//	    return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
//	        var input string
//	        if err := core.Read(r, &input); err != nil {
//	            return err
//	        }
//
//	        // Process input...
//	        processed := strings.ToUpper(input)
//
//	        return core.Write(w, processed)
//	    })
//	}
func Read[T string | []byte](r io.Reader, outPtr *T) error {
	data, err := io.ReadAll(r)
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

// Write is a generic utility function for writing data to an io.Writer in handlers.
// It simplifies the common pattern of converting string or []byte data to bytes for writing.
//
// Supported types: string, []byte
//
// Usage in handlers:
//
//	func myHandler() core.Handler {
//	    return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
//	        var input string
//	        core.Read(r, &input)
//
//	        processed := strings.ToUpper(input)
//
//	        return core.Write(w, processed)  // Handles string -> []byte conversion
//	    })
//	}
func Write[T string | []byte](w io.Writer, data T) error {
	switch v := any(data).(type) {
	case string:
		_, err := w.Write([]byte(v))
		return err
	case []byte:
		_, err := w.Write(v)
		return err
	default:
		return fmt.Errorf("unsupported type %T", data)
	}
}
