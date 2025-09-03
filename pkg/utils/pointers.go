// Package utils provides common utility functions used across the project.
package utils

// IntPtr creates a pointer to an int value.
//
// Input: int value
// Output: *int pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.MaxTokens = utils.IntPtr(1500)
func IntPtr(i int) *int { return &i }

// Int32Ptr creates a pointer to an int32 value.
//
// Input: int32 value
// Output: *int32 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Seed = utils.Int32Ptr(1500)
func Int32Ptr(i int32) *int32 { return &i }

// Int64Ptr creates a pointer to an int64 value.
//
// Input: int64 value
// Output: *int64 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.MaxTokens = utils.Int64Ptr(1500)
func Int64Ptr(i int64) *int64 { return &i }

// UintPtr creates a pointer to a uint value.
//
// Input: uint value
// Output: *uint pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.MaxLength = utils.UintPtr(100)
func UintPtr(u uint) *uint { return &u }

// Uint32Ptr creates a pointer to a uint32 value.
//
// Input: uint32 value
// Output: *uint32 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.MaxLength = utils.Uint32Ptr(100)
func Uint32Ptr(u uint32) *uint32 { return &u }

// Uint64Ptr creates a pointer to a uint64 value.
//
// Input: uint64 value
// Output: *uint64 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.MaxLength = utils.Uint64Ptr(100)
func Uint64Ptr(u uint64) *uint64 { return &u }

// Float32Ptr creates a pointer to a float32 value.
//
// Input: float32 value
// Output: *float32 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Temperature = utils.Float32Ptr(0.9)
func Float32Ptr(f float32) *float32 { return &f }

// Float64Ptr creates a pointer to a float64 value.
//
// Input: float64 value
// Output: *float64 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Temperature = utils.Float64Ptr(0.9)
func Float64Ptr(f float64) *float64 { return &f }

// BoolPtr creates a pointer to a bool value.
//
// Input: bool value
// Output: *bool pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Streaming = utils.BoolPtr(false)
func BoolPtr(b bool) *bool { return &b }

// StringPtr creates a pointer to a string value.
//
// Input: string value
// Output: *string pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Name = utils.StringPtr("default")
func StringPtr(s string) *string { return &s }
