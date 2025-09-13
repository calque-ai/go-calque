// Package helpers provides common utility functions used across the project.
package helpers

import (
	"os"
	"strconv"
	"time"
)

// GetStringFromEnv returns the environment variable value or default if not set or empty.
//
// Input: environment variable key and default value
// Output: string value from environment or default
// Behavior: Returns default if env var is empty or not set
//
// Example:
//
//	endpoint := helpers.GetStringFromEnv("API_ENDPOINT", "localhost:8080")
//	level := helpers.GetStringFromEnv("LOG_LEVEL", "info")
func GetStringFromEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetIntFromEnv returns the environment variable value as int or default if not set or invalid.
//
// Input: environment variable key and default int value
// Output: int value from environment or default
// Behavior: Returns default if env var is empty, not set, or not a valid integer
//
// Example:
//
//	maxRetries := helpers.GetIntFromEnv("MAX_RETRIES", 3)
//	port := helpers.GetIntFromEnv("PORT", 8080)
func GetIntFromEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetBoolFromEnv returns the environment variable value as bool or default if not set or invalid.
//
// Input: environment variable key and default bool value
// Output: bool value from environment or default
// Behavior: Returns default if env var is empty, not set, or not a valid boolean
//
// Example:
//
//	debug := helpers.GetBoolFromEnv("DEBUG", false)
//	tlsEnabled := helpers.GetBoolFromEnv("TLS_ENABLED", true)
func GetBoolFromEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// GetDurationFromEnv returns the environment variable value as duration or default if not set or invalid.
//
// Input: environment variable key and default duration value
// Output: time.Duration value from environment or default
// Behavior: Returns default if env var is empty, not set, or not a valid duration string
//
// Example:
//
//	timeout := helpers.GetDurationFromEnv("TIMEOUT", 30*time.Second)
//	interval := helpers.GetDurationFromEnv("POLL_INTERVAL", 5*time.Minute)
func GetDurationFromEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
