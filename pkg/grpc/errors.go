// Package grpc provides common gRPC utilities and client management for go-calque.
package grpc

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error wraps gRPC errors with additional context and status codes.
type Error struct {
	Code    codes.Code
	Message string
	Details []interface{}
	Err     error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("grpc error [%s]: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("grpc error [%s]: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
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

// WrapError wraps a gRPC error with additional context.
func WrapError(err error, message string, details ...interface{}) *Error {
	if err == nil {
		return nil
	}

	grpcErr := &Error{
		Message: message,
		Details: details,
		Err:     err,
	}

	// Extract gRPC status if available
	if st, ok := status.FromError(err); ok {
		grpcErr.Code = st.Code()
	} else {
		grpcErr.Code = codes.Unknown
	}

	return grpcErr
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

// NewUnavailableError creates a new gRPC unavailable error.
func NewUnavailableError(message string, err error) *Error {
	return &Error{
		Code:    codes.Unavailable,
		Message: message,
		Err:     err,
	}
}

// NewDeadlineExceededError creates a new gRPC deadline exceeded error.
func NewDeadlineExceededError(message string, err error) *Error {
	return &Error{
		Code:    codes.DeadlineExceeded,
		Message: message,
		Err:     err,
	}
}

// NewInvalidArgumentError creates a new gRPC invalid argument error.
func NewInvalidArgumentError(message string, err error) *Error {
	return &Error{
		Code:    codes.InvalidArgument,
		Message: message,
		Err:     err,
	}
}

// NewNotFoundError creates a new gRPC not found error.
func NewNotFoundError(message string, err error) *Error {
	return &Error{
		Code:    codes.NotFound,
		Message: message,
		Err:     err,
	}
}

// NewInternalError creates a new gRPC internal error.
func NewInternalError(message string, err error) *Error {
	return &Error{
		Code:    codes.Internal,
		Message: message,
		Err:     err,
	}
}

// WrapErrorSimple wraps an error with additional context message (non-gRPC specific).
// This is a general error wrapping function for use within the gRPC package.
func WrapErrorSimple(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WrapErrorfSimple wraps an error with a formatted context message (non-gRPC specific).
func WrapErrorfSimple(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	// Append the error to the args for the %w verb
	args = append(args, err)
	return fmt.Errorf(format+": %w", args...)
}

// NewErrorSimple creates a new error with the given message (non-gRPC specific).
func NewErrorSimple(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
