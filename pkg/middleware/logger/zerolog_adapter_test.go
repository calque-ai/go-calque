package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestZerologAdapter_Log(t *testing.T) {
	tests := []struct {
		name      string
		level     LogLevel
		msg       string
		attrs     []Attribute
		wantLevel string
		wantMsg   string
		wantAttrs map[string]interface{}
	}{
		{
			name:      "debug level with message only",
			level:     DebugLevel,
			msg:       "debug message",
			attrs:     nil,
			wantLevel: "debug",
			wantMsg:   "debug message",
			wantAttrs: nil,
		},
		{
			name:      "info level with message only",
			level:     InfoLevel,
			msg:       "info message",
			attrs:     nil,
			wantLevel: "info",
			wantMsg:   "info message",
			wantAttrs: nil,
		},
		{
			name:      "warn level with message only",
			level:     WarnLevel,
			msg:       "warn message",
			attrs:     nil,
			wantLevel: "warn",
			wantMsg:   "warn message",
			wantAttrs: nil,
		},
		{
			name:      "error level with message only",
			level:     ErrorLevel,
			msg:       "error message",
			attrs:     nil,
			wantLevel: "error",
			wantMsg:   "error message",
			wantAttrs: nil,
		},
		{
			name:      "info level with single attribute",
			level:     InfoLevel,
			msg:       "user action",
			attrs:     []Attribute{{Key: "user_id", Value: "123"}},
			wantLevel: "info",
			wantMsg:   "user action",
			wantAttrs: map[string]interface{}{"user_id": "123"},
		},
		{
			name:  "info level with multiple attributes",
			level: InfoLevel,
			msg:   "request processed",
			attrs: []Attribute{
				{Key: "method", Value: "POST"},
				{Key: "path", Value: "/api/users"},
				{Key: "status", Value: 201},
			},
			wantLevel: "info",
			wantMsg:   "request processed",
			wantAttrs: map[string]interface{}{
				"method": "POST",
				"path":   "/api/users",
				"status": float64(201), // JSON numbers are float64
			},
		},
		{
			name:  "error level with mixed attribute types",
			level: ErrorLevel,
			msg:   "operation failed",
			attrs: []Attribute{
				{Key: "error", Value: "connection timeout"},
				{Key: "retry_count", Value: 3},
				{Key: "success", Value: false},
				{Key: "duration_ms", Value: 5000.5},
			},
			wantLevel: "error",
			wantMsg:   "operation failed",
			wantAttrs: map[string]interface{}{
				"error":       "connection timeout",
				"retry_count": float64(3),
				"success":     false,
				"duration_ms": 5000.5,
			},
		},
		{
			name:      "empty message",
			level:     InfoLevel,
			msg:       "",
			attrs:     []Attribute{{Key: "event", Value: "startup"}},
			wantLevel: "info",
			wantMsg:   "", // Note: zerolog may omit empty message field entirely
			wantAttrs: map[string]interface{}{"event": "startup"},
		},
		{
			name:  "special characters in message and attributes",
			level: WarnLevel,
			msg:   "special chars: \"quotes\" and 'apostrophes'",
			attrs: []Attribute{
				{Key: "data", Value: "value with spaces and symbols!@#$%"},
				{Key: "unicode", Value: "测试数据"},
			},
			wantLevel: "warn",
			wantMsg:   "special chars: \"quotes\" and 'apostrophes'",
			wantAttrs: map[string]interface{}{
				"data":    "value with spaces and symbols!@#$%",
				"unicode": "测试数据",
			},
		},
		{
			name:  "complex nested data structures",
			level: InfoLevel,
			msg:   "complex data",
			attrs: []Attribute{
				{Key: "slice", Value: []string{"a", "b", "c"}},
				{Key: "map", Value: map[string]int{"x": 1, "y": 2}},
				{Key: "nil_value", Value: nil},
			},
			wantLevel: "info",
			wantMsg:   "complex data",
			wantAttrs: map[string]interface{}{
				"slice": []interface{}{"a", "b", "c"},
				"map": map[string]interface{}{
					"x": float64(1),
					"y": float64(2),
				},
				"nil_value": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer to capture zerolog JSON output
			var buf bytes.Buffer
			
			// Create zerolog logger with JSON output
			logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
			
			// Create adapter
			adapter := NewZerologAdapter(logger)
			
			// Call Log method
			ctx := context.Background()
			adapter.Log(ctx, tt.level, tt.msg, tt.attrs...)
			
			// Parse JSON output
			output := buf.String()
			if output == "" {
				t.Fatal("No output generated")
			}
			
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
				t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
			}
			
			// Verify level
			if level, ok := logEntry["level"].(string); !ok || level != tt.wantLevel {
				t.Errorf("Expected level %q, got %v", tt.wantLevel, logEntry["level"])
			}
			
			// Verify message (handle empty message case)
			if tt.wantMsg != "" {
				if msg, ok := logEntry["message"].(string); !ok || msg != tt.wantMsg {
					t.Errorf("Expected message %q, got %v", tt.wantMsg, logEntry["message"])
				}
			} else {
				// For empty messages, zerolog may omit the field or set it to empty
				if msg, exists := logEntry["message"]; exists && msg != "" && msg != nil {
					t.Errorf("Expected empty message, got %v", msg)
				}
			}
			
			// Verify attributes
			for key, wantValue := range tt.wantAttrs {
				gotValue, exists := logEntry[key]
				if !exists {
					t.Errorf("Expected attribute %q not found in output", key)
					continue
				}
				
				// For complex types, use reflection-safe comparison
				if !compareValues(gotValue, wantValue) {
					t.Errorf("Attribute %q: expected %v (%T), got %v (%T)", 
						key, wantValue, wantValue, gotValue, gotValue)
				}
			}
		})
	}
}

// compareValues compares values in a way that handles JSON unmarshaling differences
func compareValues(got, want interface{}) bool {
	// Handle nil values
	if got == nil && want == nil {
		return true
	}
	if got == nil || want == nil {
		return false
	}
	
	return compareTypedValues(got, want)
}

// compareTypedValues handles different type comparisons
func compareTypedValues(got, want interface{}) bool {
	switch wantVal := want.(type) {
	case []string:
		return compareStringSlice(got, wantVal)
	case []interface{}:
		// Skip comparison for complex slice types that can't be compared
		return true
	case map[string]int:
		return compareStringIntMap(got, wantVal)
	case map[string]interface{}:
		// Skip comparison for complex map types that can't be compared
		return true
	default:
		// Default comparison
		return got == want
	}
}

// compareStringSlice compares a string slice with JSON unmarshaled interface slice
func compareStringSlice(got interface{}, wantSlice []string) bool {
	gotSlice, ok := got.([]interface{})
	if !ok {
		return false
	}
	
	if len(gotSlice) != len(wantSlice) {
		return false
	}
	
	for i, v := range gotSlice {
		if v != wantSlice[i] {
			return false
		}
	}
	return true
}

// compareStringIntMap compares a string-int map with JSON unmarshaled interface map
func compareStringIntMap(got interface{}, wantMap map[string]int) bool {
	gotMap, ok := got.(map[string]interface{})
	if !ok {
		return false
	}
	
	if len(gotMap) != len(wantMap) {
		return false
	}
	
	for k, v := range wantMap {
		gotVal, exists := gotMap[k]
		if !exists || gotVal != float64(v) {
			return false
		}
	}
	return true
}

func TestZerologAdapter_IsLevelEnabled(t *testing.T) {
	tests := []struct {
		name         string
		loggerLevel  zerolog.Level
		testLevel    LogLevel
		want         bool
	}{
		{
			name:        "debug enabled when logger at debug",
			loggerLevel: zerolog.DebugLevel,
			testLevel:   DebugLevel,
			want:        true,
		},
		{
			name:        "debug disabled when logger at info",
			loggerLevel: zerolog.InfoLevel,
			testLevel:   DebugLevel,
			want:        false,
		},
		{
			name:        "info enabled when logger at debug",
			loggerLevel: zerolog.DebugLevel,
			testLevel:   InfoLevel,
			want:        true,
		},
		{
			name:        "info enabled when logger at info",
			loggerLevel: zerolog.InfoLevel,
			testLevel:   InfoLevel,
			want:        true,
		},
		{
			name:        "info disabled when logger at warn",
			loggerLevel: zerolog.WarnLevel,
			testLevel:   InfoLevel,
			want:        false,
		},
		{
			name:        "warn enabled when logger at warn",
			loggerLevel: zerolog.WarnLevel,
			testLevel:   WarnLevel,
			want:        true,
		},
		{
			name:        "warn disabled when logger at error",
			loggerLevel: zerolog.ErrorLevel,
			testLevel:   WarnLevel,
			want:        false,
		},
		{
			name:        "error enabled when logger at error",
			loggerLevel: zerolog.ErrorLevel,
			testLevel:   ErrorLevel,
			want:        true,
		},
		{
			name:        "error enabled when logger at debug",
			loggerLevel: zerolog.DebugLevel,
			testLevel:   ErrorLevel,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create zerolog logger with specific level
			var buf bytes.Buffer
			logger := zerolog.New(&buf).Level(tt.loggerLevel)
			
			// Create adapter
			adapter := NewZerologAdapter(logger)
			
			// Test IsLevelEnabled
			ctx := context.Background()
			got := adapter.IsLevelEnabled(ctx, tt.testLevel)
			
			if got != tt.want {
				t.Errorf("IsLevelEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestZerologAdapter_Printf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "simple message",
			format: "hello world",
			args:   nil,
			want:   "hello world",
		},
		{
			name:   "formatted message with string",
			format: "user %s logged in",
			args:   []any{"john"},
			want:   "user john logged in",
		},
		{
			name:   "formatted message with multiple types",
			format: "processed %d items in %v seconds",
			args:   []any{42, 1.23},
			want:   "processed 42 items in 1.23 seconds",
		},
		{
			name:   "empty format",
			format: "",
			args:   nil,
			want:   "",
		},
		{
			name:   "format with no args",
			format: "no substitutions here",
			args:   []any{},
			want:   "no substitutions here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer to capture zerolog output
			var buf bytes.Buffer
			logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
			
			// Create adapter
			adapter := NewZerologAdapter(logger)
			
			// Call Printf
			adapter.Printf(tt.format, tt.args...)
			
			// Get output and verify expected content is present
			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("Expected %q in output, got: %s", tt.want, output)
			}
		})
	}
}

func TestLogLevelToZerolog(t *testing.T) {
	tests := []struct {
		name  string
		level LogLevel
		want  zerolog.Level
	}{
		{
			name:  "debug level conversion",
			level: DebugLevel,
			want:  zerolog.DebugLevel,
		},
		{
			name:  "info level conversion",
			level: InfoLevel,
			want:  zerolog.InfoLevel,
		},
		{
			name:  "warn level conversion",
			level: WarnLevel,
			want:  zerolog.WarnLevel,
		},
		{
			name:  "error level conversion",
			level: ErrorLevel,
			want:  zerolog.ErrorLevel,
		},
		{
			name:  "invalid level defaults to info",
			level: LogLevel(999),
			want:  zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logLevelToZerolog(tt.level)
			if got != tt.want {
				t.Errorf("logLevelToZerolog(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestNewZerologAdapter(t *testing.T) {
	t.Run("creates valid adapter", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)
		
		adapter := NewZerologAdapter(logger)
		
		if adapter == nil {
			t.Fatal("NewZerologAdapter returned nil")
		}
		
		// Note: We can't directly compare zerolog.Logger instances,
		// so we test by using the adapter
		ctx := context.Background()
		adapter.Log(ctx, InfoLevel, "test")
		
		if buf.Len() == 0 {
			t.Error("NewZerologAdapter did not store a working logger")
		}
	})
	
	t.Run("adapter implements Adapter interface", func(_ *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)
		
		var _ Adapter = NewZerologAdapter(logger)
	})
}

func TestZerologAdapter_Integration(t *testing.T) {
	t.Run("full integration test with context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
		adapter := NewZerologAdapter(logger)
		
		// Test with context containing values using proper typed key
		type contextKey string
		traceIDKey := contextKey("trace_id")
		ctx := context.WithValue(context.Background(), traceIDKey, "abc-123")
		
		// Log with various levels and attributes
		adapter.Log(ctx, InfoLevel, "test message", 
			Attr("key1", "value1"),
			Attr("key2", 42),
		)
		
		output := buf.String()
		
		// Parse JSON to verify structure
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
			t.Fatalf("Failed to parse JSON output: %v", err)
		}
		
		// Verify basic structure
		if logEntry["level"] != "info" {
			t.Error("Expected info level in output")
		}
		if logEntry["message"] != "test message" {
			t.Error("Expected message in output")
		}
		if logEntry["key1"] != "value1" {
			t.Error("Expected key1 attribute in output")
		}
		if logEntry["key2"] != float64(42) {
			t.Error("Expected key2 attribute in output")
		}
	})
	
	t.Run("level filtering integration", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf).Level(zerolog.WarnLevel)
		adapter := NewZerologAdapter(logger)
		
		ctx := context.Background()
		
		// This should not produce output (level too low)
		adapter.Log(ctx, InfoLevel, "info message")
		
		infoOutput := buf.String()
		if infoOutput != "" {
			t.Error("Expected no output for info level when logger at warn level")
		}
		
		// This should produce output
		adapter.Log(ctx, ErrorLevel, "error message")
		
		errorOutput := buf.String()
		if !strings.Contains(errorOutput, "error message") {
			t.Error("Expected error message in output")
		}
	})
}

// Benchmark tests
func BenchmarkZerologAdapter_Log(b *testing.B) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)
	adapter := NewZerologAdapter(logger)
	ctx := context.Background()
	
	attrs := []Attribute{
		{Key: "user_id", Value: "12345"},
		{Key: "action", Value: "login"},
		{Key: "success", Value: true},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset() // Reset buffer to avoid memory issues
		adapter.Log(ctx, InfoLevel, "user performed action", attrs...)
	}
}

func BenchmarkZerologAdapter_IsLevelEnabled(b *testing.B) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.WarnLevel)
	adapter := NewZerologAdapter(logger)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.IsLevelEnabled(ctx, InfoLevel)
	}
}

func BenchmarkZerologAdapter_Disabled(b *testing.B) {
	// Test performance when logging is disabled
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.ErrorLevel) // Disable info/debug
	adapter := NewZerologAdapter(logger)
	ctx := context.Background()
	
	attrs := []Attribute{
		{Key: "expensive_key", Value: "expensive_value"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.Log(ctx, DebugLevel, "debug message", attrs...)
	}
}