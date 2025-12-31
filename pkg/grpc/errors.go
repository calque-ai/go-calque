// Package grpc provides common gRPC utilities and client management for go-calque.
package grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/calque-ai/go-calque/pkg/calque"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error wraps gRPC errors with additional context and status codes.
// It embeds calque.Error to provide trace_id and request_id automatically.
type Error struct {
	calqueErr *calque.Error // Embed calque.Error for context metadata
	Code      codes.Code
	Details   []interface{}
}

// Error implements the error interface (overrides calque.Error.Error()).
func (e *Error) Error() string {
	msg := e.calqueErr.Message()
	if e.calqueErr.Cause() != nil {
		return fmt.Sprintf("grpc error [%s]: %s: %v", e.Code, msg, e.calqueErr.Cause())
	}
	return fmt.Sprintf("grpc error [%s]: %s", e.Code, msg)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.calqueErr.Unwrap()
}

// TraceID returns the trace ID from the embedded calque.Error.
func (e *Error) TraceID() string {
	return e.calqueErr.TraceID()
}

// RequestID returns the request ID from the embedded calque.Error.
func (e *Error) RequestID() string {
	return e.calqueErr.RequestID()
}

// IsRetryable returns true if the error is retryable.
func (e *Error) IsRetryable() bool {
	switch e.Code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

// LogAttrs returns all attributes including gRPC code, trace_id, and request_id.
// This is useful for logging the error with all metadata.
func (e *Error) LogAttrs() []slog.Attr {
	attrs := e.calqueErr.LogAttrs()
	attrs = append(attrs, slog.String("grpc_code", e.Code.String()))
	if len(e.Details) > 0 {
		attrs = append(attrs, slog.Any("grpc_details", e.Details))
	}
	return attrs
}

// WrapError wraps a gRPC error with context metadata using calque errors.
// The error includes trace_id and request_id from context, plus gRPC status code.
func WrapError(ctx context.Context, err error, message string, details ...interface{}) *Error {
	if err == nil {
		return nil
	}

	// Create calque error with context metadata
	calqueErr := calque.WrapErr(ctx, err, message)

	// Extract gRPC status code
	code := codes.Unknown
	if st, ok := status.FromError(err); ok {
		code = st.Code()
	}

	return &Error{
		calqueErr: calqueErr,
		Code:      code,
		Details:   details,
	}
}

// IsGRPCError checks if an error is a gRPC error.
func IsGRPCError(err error) bool {
	_, ok := status.FromError(err)
	return ok
}

// GetGRPCCode returns the gRPC status code from an error.
func GetGRPCCode(err error) codes.Code {
	if st, ok := status.FromError(err); ok {
		return st.Code()
	}
	return codes.Unknown
}

// NewUnavailableError creates a new gRPC unavailable error with context metadata.
func NewUnavailableError(ctx context.Context, message string, err error) *Error {
	calqueErr := calque.WrapErr(ctx, err, message)
	return &Error{
		calqueErr: calqueErr,
		Code:      codes.Unavailable,
	}
}

// NewDeadlineExceededError creates a new gRPC deadline exceeded error with context metadata.
func NewDeadlineExceededError(ctx context.Context, message string, err error) *Error {
	calqueErr := calque.WrapErr(ctx, err, message)
	return &Error{
		calqueErr: calqueErr,
		Code:      codes.DeadlineExceeded,
	}
}

// NewInvalidArgumentError creates a new gRPC invalid argument error with context metadata.
func NewInvalidArgumentError(ctx context.Context, message string, err error) *Error {
	calqueErr := calque.WrapErr(ctx, err, message)
	return &Error{
		calqueErr: calqueErr,
		Code:      codes.InvalidArgument,
	}
}

// NewNotFoundError creates a new gRPC not found error with context metadata.
func NewNotFoundError(ctx context.Context, message string, err error) *Error {
	calqueErr := calque.WrapErr(ctx, err, message)
	return &Error{
		calqueErr: calqueErr,
		Code:      codes.NotFound,
	}
}

// NewInternalError creates a new gRPC internal error with context metadata.
func NewInternalError(ctx context.Context, message string, err error) *Error {
	calqueErr := calque.WrapErr(ctx, err, message)
	return &Error{
		calqueErr: calqueErr,
		Code:      codes.Internal,
	}
}

// WrapErrorSimple wraps an error with context metadata (non-gRPC specific).
// This uses calque errors for consistent error handling.
func WrapErrorSimple(ctx context.Context, err error, message string) error {
	if err == nil {
		return nil
	}
	return calque.WrapErr(ctx, err, message)
}

// WrapErrorfSimple wraps an error with formatted message and context metadata.
func WrapErrorfSimple(ctx context.Context, err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	message := fmt.Sprintf(format, args...)
	return calque.WrapErr(ctx, err, message)
}

// NewErrorSimple creates a new error with context metadata.
func NewErrorSimple(ctx context.Context, message string) error {
	return calque.NewErr(ctx, message)
}
