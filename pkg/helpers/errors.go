// Package helpers provides common utility functions used across the project.
// All error functions use calque errors for context-aware error handling with
// automatic trace_id and request_id propagation.
package helpers

import (
	"context"
	"fmt"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// WrapError wraps an error with context metadata using calque errors.
//
// Input: context, error to wrap, and context message
// Output: calque.Error with trace_id and request_id, or nil if input error is nil
//
// Example:
//
//	err := helpers.WrapError(ctx, originalErr, "failed to connect to service")
//	err := helpers.WrapError(ctx, dbErr, "failed to save user data")
func WrapError(ctx context.Context, err error, message string) error {
	if err == nil {
		return nil
	}
	return calque.WrapErr(ctx, err, message)
}

// WrapErrorf wraps an error with formatted message and context metadata.
//
// Input: context, error to wrap, format string, and format arguments
// Output: calque.Error with trace_id and request_id, or nil if input error is nil
//
// Example:
//
//	err := helpers.WrapErrorf(ctx, originalErr, "failed to connect to %s service", serviceName)
//	err := helpers.WrapErrorf(ctx, dbErr, "failed to save user %d data", userID)
func WrapErrorf(ctx context.Context, err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	message := fmt.Sprintf(format, args...)
	return calque.WrapErr(ctx, err, message)
}

// NewError creates a new error with context metadata using calque errors.
//
// Input: context and error message
// Output: calque.Error with trace_id and request_id
//
// Example:
//
//	err := helpers.NewError(ctx, "invalid configuration")
//	err := helpers.NewError(ctx, "user not found")
func NewError(ctx context.Context, message string) error {
	return calque.NewErr(ctx, message)
}
