package calque

import (
	"context"
	"fmt"
	"log/slog"
)

// Error is a context-aware error that carries metadata for logging and tracing.
//
// It implements the standard error interface and supports Go's error wrapping
// (errors.Is, errors.As, errors.Unwrap). Metadata includes trace ID, request ID,
// and arbitrary tags as slog.Attr for structured logging.
//
// Example:
//
//	err := calque.WrapErr(ctx, originalErr, "failed to process request")
//	err.Tag(slog.String("user_id", userID))
//	err.Tag(slog.Int("retry_count", 3))
//	return err
type Error struct {
	msg       string
	cause     error
	traceID   string
	requestID string
	attrs     []slog.Attr
}

// WrapErr wraps an existing error with context metadata.
//
// The trace ID and request ID are automatically extracted from context.
// Use Tag() to add additional metadata.
//
// Example:
//
//	if err != nil {
//	    return calque.WrapErr(ctx, err, "failed to connect to database")
//	}
//
//	// With tags
//	return calque.WrapErr(ctx, err, "query failed").
//	    Tag(slog.String("table", tableName)).
//	    Tag(slog.Duration("timeout", timeout))
func WrapErr(ctx context.Context, err error, msg string) *Error {
	return &Error{
		msg:       msg,
		cause:     err,
		traceID:   TraceID(ctx),
		requestID: RequestID(ctx),
		attrs:     make([]slog.Attr, 0),
	}
}

// NewErr creates a new error with context metadata (no underlying cause).
//
// The trace ID and request ID are automatically extracted from context.
// Use Tag() to add additional metadata.
//
// Example:
//
//	if input == nil {
//	    return calque.NewErr(ctx, "input cannot be nil")
//	}
//
//	// With tags
//	return calque.NewErr(ctx, "validation failed").
//	    Tag(slog.String("field", "email")).
//	    Tag(slog.String("reason", "invalid format"))
func NewErr(ctx context.Context, msg string) *Error {
	return &Error{
		msg:       msg,
		cause:     nil,
		traceID:   TraceID(ctx),
		requestID: RequestID(ctx),
		attrs:     make([]slog.Attr, 0),
	}
}

// Tag adds a slog.Attr to the error for structured logging.
//
// Returns the error for fluent chaining. Use slog.String, slog.Int,
// slog.Bool, slog.Duration, slog.Any, etc. to create attributes.
//
// Example:
//
//	return calque.WrapErr(ctx, err, "operation failed").
//	    Tag(slog.String("operation", "create_user")).
//	    Tag(slog.Int("user_id", 123)).
//	    Tag(slog.Bool("retryable", true)).
//	    Tag(slog.Duration("elapsed", elapsed))
func (e *Error) Tag(attr slog.Attr) *Error {
	e.attrs = append(e.attrs, attr)
	return e
}

// Tags adds multiple slog.Attr to the error.
//
// Returns the error for fluent chaining.
//
// Example:
//
//	return calque.WrapErr(ctx, err, "request failed").
//	    Tags(
//	        slog.String("method", "POST"),
//	        slog.String("path", "/api/users"),
//	        slog.Int("status", 500),
//	    )
func (e *Error) Tags(attrs ...slog.Attr) *Error {
	e.attrs = append(e.attrs, attrs...)
	return e
}

// Error implements the error interface.
//
// Returns the message with the cause error if present.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

// Unwrap returns the underlying error.
//
// This enables errors.Is and errors.As to work with wrapped errors.
func (e *Error) Unwrap() error {
	return e.cause
}

// TraceID returns the trace ID associated with this error.
func (e *Error) TraceID() string {
	return e.traceID
}

// RequestID returns the request ID associated with this error.
func (e *Error) RequestID() string {
	return e.requestID
}

// Attrs returns the slog attributes associated with this error.
//
// Useful for logging the error with all its metadata.
func (e *Error) Attrs() []slog.Attr {
	return e.attrs
}

// Message returns the error message without the cause.
func (e *Error) Message() string {
	return e.msg
}

// Cause returns the underlying error (alias for Unwrap).
func (e *Error) Cause() error {
	return e.cause
}

// LogAttrs returns all attributes including trace_id and request_id.
//
// This is useful for logging the error with all context metadata.
//
// Example:
//
//	calque.LogErrorAttr(ctx, "operation failed", err.LogAttrs()...)
func (e *Error) LogAttrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, len(e.attrs)+3)

	// Add error itself
	if e.cause != nil {
		attrs = append(attrs, slog.Any("error", e.cause))
	}

	// Add trace and request IDs if present
	if e.traceID != "" {
		attrs = append(attrs, slog.String("trace_id", e.traceID))
	}
	if e.requestID != "" {
		attrs = append(attrs, slog.String("request_id", e.requestID))
	}

	// Add custom attrs
	attrs = append(attrs, e.attrs...)

	return attrs
}

// Log logs this error at error level with all metadata.
//
// Uses the logger from context or slog.Default().
//
// Example:
//
//	err := calque.WrapErr(ctx, originalErr, "failed to process").
//	    Tag(slog.String("item_id", itemID))
//	err.Log(ctx)  // Logs with all metadata
func (e *Error) Log(ctx context.Context) {
	LogErrorAttr(ctx, e.msg, e.LogAttrs()...)
}

// LogWithLevel logs this error at the specified level with all metadata.
//
// Example:
//
//	err.LogWithLevel(ctx, slog.LevelWarn)  // Log as warning instead of error
func (e *Error) LogWithLevel(ctx context.Context, level slog.Level) {
	LogAttr(ctx, level, e.msg, e.LogAttrs()...)
}

// WithMessage returns a copy of the error with a new message.
//
// Useful when you want to add context without losing the original error.
// The attrs slice is copied to prevent mutation of the original error.
//
// Example:
//
//	return err.WithMessage("failed in handler")
func (e *Error) WithMessage(msg string) *Error {
	attrsCopy := make([]slog.Attr, len(e.attrs))
	copy(attrsCopy, e.attrs)
	return &Error{
		msg:       msg,
		cause:     e,
		traceID:   e.traceID,
		requestID: e.requestID,
		attrs:     attrsCopy,
	}
}

// Is implements errors.Is for this error.
//
// Returns true if target is the same type and has the same message.
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.msg == t.msg
	}
	return false
}
