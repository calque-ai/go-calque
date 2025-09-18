// Package helpers provides common utility functions used across the project.
package helpers

import (
	"fmt"
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
	args = append(args, err)
	return fmt.Errorf(format+": %w", args...)
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
