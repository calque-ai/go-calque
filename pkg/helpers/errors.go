// Package helpers provides common utility functions used across the project.
package helpers

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WrapError wraps an error with additional context message.
// This is a common pattern used throughout the project for error handling.
//
// Input: error to wrap and context message
// Output: wrapped error with context, or nil if input error is nil
// Behavior: Uses fmt.Errorf with %w verb for proper error wrapping
//
// Example:
//
//	err := helpers.WrapError(originalErr, "failed to connect to service")
//	err := helpers.WrapError(dbErr, "failed to save user data")
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WrapErrorf wraps an error with a formatted context message.
// Similar to WrapError but allows formatted message with arguments.
//
// Input: error to wrap, format string, and format arguments
// Output: wrapped error with formatted context, or nil if input error is nil
// Behavior: Uses fmt.Errorf with %w verb for proper error wrapping
//
// Example:
//
//	err := helpers.WrapErrorf(originalErr, "failed to connect to %s service", serviceName)
//	err := helpers.WrapErrorf(dbErr, "failed to save user %d data", userID)
func WrapErrorf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	// Append the error to the args for the %w verb
	allArgs := append(args, err)
	return fmt.Errorf(format+": %w", allArgs...)
}

// NewError creates a new error with the given message.
// This is a simple wrapper around fmt.Errorf for consistency.
//
// Input: error message format string and arguments
// Output: new error with formatted message
// Behavior: Uses fmt.Errorf to create formatted error
//
// Example:
//
//	err := helpers.NewError("invalid configuration: %s", configName)
//	err := helpers.NewError("user %d not found", userID)
func NewError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// GRPCError wraps gRPC errors with additional context and status codes.
type GRPCError struct {
	Code    codes.Code
	Message string
	Details []interface{}
	Err     error
}

// Error implements the error interface.
func (e *GRPCError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("grpc error [%s]: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("grpc error [%s]: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *GRPCError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is retryable.
func (e *GRPCError) IsRetryable() bool {
	switch e.Code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

// WrapGRPCError wraps a gRPC error with additional context.
func WrapGRPCError(err error, message string, details ...interface{}) *GRPCError {
	if err == nil {
		return nil
	}

	grpcErr := &GRPCError{
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

// NewGRPCUnavailableError creates a new gRPC unavailable error.
func NewGRPCUnavailableError(message string, err error) *GRPCError {
	return &GRPCError{
		Code:    codes.Unavailable,
		Message: message,
		Err:     err,
	}
}

// NewGRPCDeadlineExceededError creates a new gRPC deadline exceeded error.
func NewGRPCDeadlineExceededError(message string, err error) *GRPCError {
	return &GRPCError{
		Code:    codes.DeadlineExceeded,
		Message: message,
		Err:     err,
	}
}

// NewGRPCInvalidArgumentError creates a new gRPC invalid argument error.
func NewGRPCInvalidArgumentError(message string, err error) *GRPCError {
	return &GRPCError{
		Code:    codes.InvalidArgument,
		Message: message,
		Err:     err,
	}
}

// NewGRPCNotFoundError creates a new gRPC not found error.
func NewGRPCNotFoundError(message string, err error) *GRPCError {
	return &GRPCError{
		Code:    codes.NotFound,
		Message: message,
		Err:     err,
	}
}

// NewGRPCInternalError creates a new gRPC internal error.
func NewGRPCInternalError(message string, err error) *GRPCError {
	return &GRPCError{
		Code:    codes.Internal,
		Message: message,
		Err:     err,
	}
}
