package config

import (
	"reflect"
	"testing"

	"github.com/calque-ai/go-calque/pkg/helpers"
)

// TestConfig represents a typical AI client config for testing
type TestConfig struct {
	APIKey      string
	Temperature *float32
	MaxTokens   *int
	Host        string
	Stream      *bool
	Stop        []string
	Options     map[string]interface{}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name           string
		target         *TestConfig
		source         *TestConfig
		expectedResult *TestConfig
		description    string
	}{
		{
			name: "partial_config_merge",
			target: &TestConfig{
				APIKey:      "default-key",
				Temperature: helpers.Float32Ptr(0.7),
				MaxTokens:   helpers.IntPtr(1000),
				Host:        "default-host",
				Stream:      helpers.BoolPtr(true),
				Stop:        []string{"default"},
				Options:     map[string]interface{}{"default": "value"},
			},
			source: &TestConfig{
				Temperature: helpers.Float32Ptr(0.9),
				MaxTokens:   helpers.IntPtr(2000),
			},
			expectedResult: &TestConfig{
				APIKey:      "default-key",
				Temperature: helpers.Float32Ptr(0.9),
				MaxTokens:   helpers.IntPtr(2000),
				Host:        "default-host",
				Stream:      helpers.BoolPtr(true),
				Stop:        []string{"default"},
				Options:     map[string]interface{}{"default": "value"},
			},
			description: "Only specified fields should override, others preserved",
		},
		{
			name: "empty_fields_preserve_defaults",
			target: &TestConfig{
				APIKey:      "default-key",
				Temperature: helpers.Float32Ptr(0.7),
				MaxTokens:   helpers.IntPtr(1000),
				Host:        "default-host",
			},
			source: &TestConfig{
				APIKey:      "",                   // Empty string - should not override
				Temperature: nil,                  // Nil pointer - should not override
				MaxTokens:   helpers.IntPtr(2000), // Set value - should override
				Host:        "",                   // Empty string - should not override
			},
			expectedResult: &TestConfig{
				APIKey:      "default-key",           // Preserved
				Temperature: helpers.Float32Ptr(0.7), // Preserved
				MaxTokens:   helpers.IntPtr(2000),    // Overridden
				Host:        "default-host",          // Preserved
			},
			description: "Empty/nil fields should not override defaults",
		},
		{
			name: "complete_config_override",
			target: &TestConfig{
				APIKey:      "default-key",
				Temperature: helpers.Float32Ptr(0.7),
				MaxTokens:   helpers.IntPtr(1000),
			},
			source: &TestConfig{
				APIKey:      "new-key",
				Temperature: helpers.Float32Ptr(1.0),
				MaxTokens:   helpers.IntPtr(3000),
				Host:        "new-host",
				Stream:      helpers.BoolPtr(false),
				Stop:        []string{"new"},
				Options:     map[string]interface{}{"new": "value"},
			},
			expectedResult: &TestConfig{
				APIKey:      "new-key",
				Temperature: helpers.Float32Ptr(1.0),
				MaxTokens:   helpers.IntPtr(3000),
				Host:        "new-host",
				Stream:      helpers.BoolPtr(false),
				Stop:        []string{"new"},
				Options:     map[string]interface{}{"new": "value"},
			},
			description: "All fields should be overridden when provided",
		},
		{
			name: "nil_source_no_change",
			target: &TestConfig{
				APIKey:      "default-key",
				Temperature: helpers.Float32Ptr(0.7),
				MaxTokens:   helpers.IntPtr(1000),
			},
			source: nil,
			expectedResult: &TestConfig{
				APIKey:      "default-key",
				Temperature: helpers.Float32Ptr(0.7),
				MaxTokens:   helpers.IntPtr(1000),
			},
			description: "Nil source should not modify target",
		},
		{
			name: "zero_values_override",
			target: &TestConfig{
				APIKey:      "default-key",
				Temperature: helpers.Float32Ptr(0.7),
				MaxTokens:   helpers.IntPtr(1000),
				Stream:      helpers.BoolPtr(true),
			},
			source: &TestConfig{
				APIKey:      "zero-key",
				Temperature: helpers.Float32Ptr(0.0), // Zero value - should override
				MaxTokens:   helpers.IntPtr(0),       // Zero value - should override
				Stream:      helpers.BoolPtr(false),  // Zero value - should override
			},
			expectedResult: &TestConfig{
				APIKey:      "zero-key",
				Temperature: helpers.Float32Ptr(0.0),
				MaxTokens:   helpers.IntPtr(0),
				Stream:      helpers.BoolPtr(false),
			},
			description: "Zero values should override defaults (not treated as empty)",
		},
		{
			name: "slice_and_map_merge",
			target: &TestConfig{
				Stop:    []string{"default1", "default2"},
				Options: map[string]interface{}{"default": "value", "keep": "me"},
			},
			source: &TestConfig{
				Stop:    []string{"new1", "new2"},
				Options: map[string]interface{}{"new": "value", "override": "me"},
			},
			expectedResult: &TestConfig{
				Stop:    []string{"new1", "new2"},
				Options: map[string]interface{}{"new": "value", "override": "me"},
			},
			description: "Slices and maps should be completely replaced",
		},
		{
			name: "empty_slice_and_map_preserve",
			target: &TestConfig{
				Stop:    []string{"default1", "default2"},
				Options: map[string]interface{}{"default": "value"},
			},
			source: &TestConfig{
				Stop:    []string{},               // Empty slice - should not override
				Options: map[string]interface{}{}, // Empty map - should not override
			},
			expectedResult: &TestConfig{
				Stop:    []string{"default1", "default2"},
				Options: map[string]interface{}{"default": "value"},
			},
			description: "Empty slices and maps should not override defaults",
		},
		{
			name: "nil_slice_and_map_preserve",
			target: &TestConfig{
				Stop:    []string{"default1", "default2"},
				Options: map[string]interface{}{"default": "value"},
			},
			source: &TestConfig{
				Stop:    nil, // Nil slice - should not override
				Options: nil, // Nil map - should not override
			},
			expectedResult: &TestConfig{
				Stop:    []string{"default1", "default2"},
				Options: map[string]interface{}{"default": "value"},
			},
			description: "Nil slices and maps should not override defaults",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of target to avoid modifying the original
			targetCopy := *tt.target
			if tt.target.Stop != nil {
				targetCopy.Stop = make([]string, len(tt.target.Stop))
				copy(targetCopy.Stop, tt.target.Stop)
			}
			if tt.target.Options != nil {
				targetCopy.Options = make(map[string]interface{})
				for k, v := range tt.target.Options {
					targetCopy.Options[k] = v
				}
			}

			// Perform merge
			Merge(&targetCopy, tt.source)

			// Verify results using reflect.DeepEqual
			if !reflect.DeepEqual(&targetCopy, tt.expectedResult) {
				t.Errorf("Test: %s\nGot: %+v\nExpected: %+v", tt.name, &targetCopy, tt.expectedResult)
			}
		})
	}
}

func TestConfigMerger(t *testing.T) {
	tests := []struct {
		name           string
		target         *TestConfig
		source         *TestConfig
		expectedResult *TestConfig
		description    string
	}{
		{
			name: "merger_instance_works",
			target: &TestConfig{
				APIKey:      "default",
				Temperature: helpers.Float32Ptr(0.7),
				MaxTokens:   helpers.IntPtr(1000),
			},
			source: &TestConfig{
				Temperature: helpers.Float32Ptr(0.9),
				MaxTokens:   helpers.IntPtr(2000),
			},
			expectedResult: &TestConfig{
				APIKey:      "default",
				Temperature: helpers.Float32Ptr(0.9),
				MaxTokens:   helpers.IntPtr(2000),
			},
			description: "Merger instance should work same as package function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := New()
			targetCopy := *tt.target
			if tt.target.Stop != nil {
				targetCopy.Stop = make([]string, len(tt.target.Stop))
				copy(targetCopy.Stop, tt.target.Stop)
			}
			if tt.target.Options != nil {
				targetCopy.Options = make(map[string]interface{})
				for k, v := range tt.target.Options {
					targetCopy.Options[k] = v
				}
			}

			merger.Merge(&targetCopy, tt.source)

			if !reflect.DeepEqual(&targetCopy, tt.expectedResult) {
				t.Errorf("Test: %s\nGot: %+v\nExpected: %+v", tt.name, &targetCopy, tt.expectedResult)
			}
		})
	}
}
