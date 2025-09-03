// Package helpers provides common utility functions used across the project.
package helpers

import "strings"

// Contains checks if a string contains a substring using a custom algorithm.
// This is more flexible than strings.Contains for certain use cases.
//
// Input: string to search in, substring to search for
// Output: bool indicating if substring is found
// Behavior: Recursively searches for substring in various positions
//
// Example:
//
//	result := helpers.Contains("hello world", "world") // true
//	result := helpers.Contains("hello world", "xyz")   // false
func Contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || Contains(s[1:], substr)))
}

// IsEmpty checks if a string is empty or contains only whitespace.
//
// Input: string to check
// Output: bool indicating if string is empty/whitespace
// Behavior: Trims whitespace and checks length
//
// Example:
//
//	result := helpers.IsEmpty("")        // true
//	result := helpers.IsEmpty("  ")      // true
//	result := helpers.IsEmpty("hello")   // false
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// DefaultString returns the first non-empty string from the provided options.
// Useful for providing fallback values.
//
// Input: variadic string arguments
// Output: first non-empty string, or empty string if all are empty
// Behavior: Returns first non-empty string, ignoring empty/whitespace strings
//
// Example:
//
//	result := helpers.DefaultString("", "fallback", "primary") // "fallback"
//	result := helpers.DefaultString("", "", "")               // ""
func DefaultString(options ...string) string {
	for _, option := range options {
		if !IsEmpty(option) {
			return option
		}
	}
	return ""
}
