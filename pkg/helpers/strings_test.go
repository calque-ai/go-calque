package helpers

import (
	"testing"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "hello world", "hello world", true},
		{"substring at start", "hello world", "hello", true},
		{"substring at end", "hello world", "world", true},
		{"substring in middle", "hello world", "lo wo", true},
		{"not found", "hello world", "xyz", false},
		{"empty string", "", "test", false},
		{"empty substring", "hello", "", true},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("Contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"tabs only", "\t\t", true},
		{"newlines only", "\n\n", true},
		{"mixed whitespace", " \t\n ", true},
		{"non-empty", "hello", false},
		{"with leading space", " hello", false},
		{"with trailing space", "hello ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmpty(tt.input)
			if result != tt.expected {
				t.Errorf("IsEmpty(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultString(t *testing.T) {
	tests := []struct {
		name     string
		options  []string
		expected string
	}{
		{"first non-empty", []string{"", "fallback", "primary"}, "fallback"},
		{"all empty", []string{"", "", ""}, ""},
		{"single option", []string{"single"}, "single"},
		{"no options", []string{}, ""},
		{"first option", []string{"first", "second"}, "first"},
		{"with whitespace", []string{"  ", "valid", "another"}, "valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefaultString(tt.options...)
			if result != tt.expected {
				t.Errorf("DefaultString(%v) = %q, want %q", tt.options, result, tt.expected)
			}
		})
	}
}
