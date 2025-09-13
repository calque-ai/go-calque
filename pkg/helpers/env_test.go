package helpers

import (
	"os"
	"testing"
	"time"
)

func TestGetStringFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue string
		expected     string
	}{
		{"with env value", "TEST_STRING", "test-value", "default", "test-value"},
		{"without env value", "NON_EXISTENT", "", "default", "default"},
		{"empty env value", "EMPTY_STRING", "", "default", "default"},
		{"with spaces", "SPACED_STRING", "  value  ", "default", "  value  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Clean up before test
			os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := GetStringFromEnv(tt.envKey, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetStringFromEnv(%q, %q) = %q, want %q", tt.envKey, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestGetIntFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue int
		expected     int
	}{
		{"valid int", "TEST_INT", "42", 10, 42},
		{"invalid int", "INVALID_INT", "not-a-number", 10, 10},
		{"empty env", "EMPTY_INT", "", 10, 10},
		{"zero value", "ZERO_INT", "0", 10, 0},
		{"negative value", "NEG_INT", "-5", 10, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Clean up before test
			os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := GetIntFromEnv(tt.envKey, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetIntFromEnv(%q, %d) = %d, want %d", tt.envKey, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestGetBoolFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue bool
		expected     bool
	}{
		{"true value", "TEST_BOOL_TRUE", "true", false, true},
		{"false value", "TEST_BOOL_FALSE", "false", true, false},
		{"1 value", "TEST_BOOL_1", "1", false, true},
		{"0 value", "TEST_BOOL_0", "0", true, false},
		{"invalid value", "INVALID_BOOL", "maybe", true, true},
		{"empty env", "EMPTY_BOOL", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Clean up before test
			os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := GetBoolFromEnv(tt.envKey, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetBoolFromEnv(%q, %t) = %t, want %t", tt.envKey, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestGetDurationFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{"valid duration seconds", "TEST_DUR_S", "30s", 10 * time.Second, 30 * time.Second},
		{"valid duration minutes", "TEST_DUR_M", "5m", 1 * time.Minute, 5 * time.Minute},
		{"valid duration hours", "TEST_DUR_H", "2h", 1 * time.Hour, 2 * time.Hour},
		{"invalid duration", "INVALID_DUR", "not-a-duration", 10 * time.Second, 10 * time.Second},
		{"empty env", "EMPTY_DUR", "", 10 * time.Second, 10 * time.Second},
		{"zero duration", "ZERO_DUR", "0s", 10 * time.Second, 0 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Clean up before test
			os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := GetDurationFromEnv(tt.envKey, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetDurationFromEnv(%q, %v) = %v, want %v", tt.envKey, tt.defaultValue, result, tt.expected)
			}
		})
	}
}
