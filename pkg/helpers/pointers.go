// Package helpers provides common utility functions used across the project.
package helpers

// PtrOf creates a pointer to any value type.
//
// Input: T value of any type
// Output: *T pointer to the value
// Behavior: Generic helper for creating pointers to any type, useful for optional config fields
//
// Example:
//
//	config.MaxTokens = helpers.PtrOf(1500)        // *int
//	config.Temperature = helpers.PtrOf(0.9)       // *float64
//	config.Streaming = helpers.PtrOf(false)       // *bool
//	config.Name = helpers.PtrOf("default")        // *string
func PtrOf[T any](t T) *T { return &t }
