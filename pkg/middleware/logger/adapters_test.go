package logger

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
)

func TestStandardAdapter_Log(t *testing.T) {
	tests := []struct {
		name      string
		level     LogLevel
		msg       string
		attrs     []Attribute
		wantMsg   string
		wantAttrs []string
	}{
		{
			name:      "debug level with message only",
			level:     DebugLevel,
			msg:       "debug message",
			attrs:     nil,
			wantMsg:   "debug message",
			wantAttrs: nil,
		},
		{
			name:      "info level with message only",
			level:     InfoLevel,
			msg:       "info message",
			attrs:     nil,
			wantMsg:   "info message",
			wantAttrs: nil,
		},
		{
			name:      "warn level with message only",
			level:     WarnLevel,
			msg:       "warn message",
			attrs:     nil,
			wantMsg:   "warn message",
			wantAttrs: nil,
		},
		{
			name:      "error level with message only",
			level:     ErrorLevel,
			msg:       "error message",
			attrs:     nil,
			wantMsg:   "error message",
			wantAttrs: nil,
		},
		{
			name:      "info level with single attribute",
			level:     InfoLevel,
			msg:       "user action",
			attrs:     []Attribute{{Key: "user_id", Value: "123"}},
			wantMsg:   "user action",
			wantAttrs: []string{"user_id=123"},
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
			wantMsg:   "request processed",
			wantAttrs: []string{"method=POST", "path=/api/users", "status=201"},
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
			wantMsg:   "operation failed",
			wantAttrs: []string{"error=connection timeout", "retry_count=3", "success=false", "duration_ms=5000.5"},
		},
		{
			name:      "empty message",
			level:     InfoLevel,
			msg:       "",
			attrs:     []Attribute{{Key: "event", Value: "startup"}},
			wantMsg:   "",
			wantAttrs: []string{"event=startup"},
		},
		{
			name:  "special characters in message and attributes",
			level: WarnLevel,
			msg:   "special chars: \"quotes\" and 'apostrophes'",
			attrs: []Attribute{
				{Key: "data", Value: "value with spaces and symbols!@#$%"},
				{Key: "unicode", Value: "测试数据"},
			},
			wantMsg:   "special chars: \"quotes\" and 'apostrophes'",
			wantAttrs: []string{"data=value with spaces and symbols!@#$%", "unicode=测试数据"},
		},
		{
			name:  "attributes with nil value",
			level: InfoLevel,
			msg:   "test message",
			attrs: []Attribute{
				{Key: "valid_key", Value: "valid_value"},
				{Key: "nil_key", Value: nil},
			},
			wantMsg:   "test message",
			wantAttrs: []string{"valid_key=valid_value", "nil_key=<nil>"},
		},
		{
			name:  "complex data structures in attributes",
			level: InfoLevel,
			msg:   "complex data",
			attrs: []Attribute{
				{Key: "slice", Value: []string{"a", "b", "c"}},
				{Key: "map", Value: map[string]int{"x": 1, "y": 2}},
			},
			wantMsg: "complex data",
			// Note: exact format depends on Go's %v formatting
			wantAttrs: []string{"slice=", "map="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer to capture log output
			var buf bytes.Buffer
			
			// Create standard logger writing to buffer
			stdLogger := log.New(&buf, "", 0) // No prefix, no flags for cleaner testing
			
			// Create adapter
			adapter := NewStandardAdapter(stdLogger)
			
			// Call Log method (context and level are ignored by StandardAdapter)
			ctx := context.Background()
			adapter.Log(ctx, tt.level, tt.msg, tt.attrs...)
			
			// Get output
			output := buf.String()
			
			// Verify message is present
			if !strings.Contains(output, tt.wantMsg) {
				t.Errorf("Expected message %q in output, got: %s", tt.wantMsg, output)
			}
			
			// Verify all attributes are present
			for _, wantAttr := range tt.wantAttrs {
				if !strings.Contains(output, wantAttr) {
					t.Errorf("Expected attribute %q in output, got: %s", wantAttr, output)
				}
			}
		})
	}
}

func TestStandardAdapter_IsLevelEnabled(t *testing.T) {
	tests := []struct {
		name      string
		testLevel LogLevel
		want      bool
	}{
		{
			name:      "debug level always enabled",
			testLevel: DebugLevel,
			want:      true,
		},
		{
			name:      "info level always enabled",
			testLevel: InfoLevel,
			want:      true,
		},
		{
			name:      "warn level always enabled",
			testLevel: WarnLevel,
			want:      true,
		},
		{
			name:      "error level always enabled",
			testLevel: ErrorLevel,
			want:      true,
		},
		{
			name:      "invalid level always enabled",
			testLevel: LogLevel(999),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create standard logger
			var buf bytes.Buffer
			stdLogger := log.New(&buf, "", 0)
			
			// Create adapter
			adapter := NewStandardAdapter(stdLogger)
			
			// Test IsLevelEnabled (context is ignored)
			ctx := context.Background()
			got := adapter.IsLevelEnabled(ctx, tt.testLevel)
			
			if got != tt.want {
				t.Errorf("IsLevelEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStandardAdapter_Printf(t *testing.T) {
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
		{
			name:   "format with extra args",
			format: "only one %s placeholder",
			args:   []any{"first", "second", "third"},
			want:   "only one first placeholder",
		},
		{
			name:   "format with missing args",
			format: "missing %s and %d args",
			args:   []any{"only one"},
			want:   "missing only one and %!d(MISSING) args",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer to capture log output
			var buf bytes.Buffer
			stdLogger := log.New(&buf, "", 0) // No prefix, no flags
			
			// Create adapter
			adapter := NewStandardAdapter(stdLogger)
			
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

func TestNewStandardAdapter(t *testing.T) {
	t.Run("creates valid adapter", func(t *testing.T) {
		var buf bytes.Buffer
		stdLogger := log.New(&buf, "", 0)
		
		adapter := NewStandardAdapter(stdLogger)
		
		if adapter == nil {
			t.Fatal("NewStandardAdapter returned nil")
		}
		
		if adapter.logger != stdLogger {
			t.Error("NewStandardAdapter did not store the provided logger")
		}
	})
	
	t.Run("adapter implements Adapter interface", func(t *testing.T) {
		var buf bytes.Buffer
		stdLogger := log.New(&buf, "", 0)
		
		var _ Adapter = NewStandardAdapter(stdLogger)
	})
	
	t.Run("works with default logger", func(t *testing.T) {
		adapter := NewStandardAdapter(log.Default())
		
		if adapter == nil {
			t.Fatal("NewStandardAdapter with default logger returned nil")
		}
		
		// Test that it works (output goes to stderr, so we can't easily capture it)
		ctx := context.Background()
		adapter.Log(ctx, InfoLevel, "test message")
		
		// Just verify no panic occurred
	})
}

func TestStandardAdapter_Integration(t *testing.T) {
	t.Run("full integration test with various scenarios", func(t *testing.T) {
		var buf bytes.Buffer
		stdLogger := log.New(&buf, "[TEST] ", log.LstdFlags)
		adapter := NewStandardAdapter(stdLogger)
		ctx := context.Background()
		
		// Test different levels (should all behave the same)
		adapter.Log(ctx, DebugLevel, "debug message")
		adapter.Log(ctx, InfoLevel, "info message") 
		adapter.Log(ctx, WarnLevel, "warn message")
		adapter.Log(ctx, ErrorLevel, "error message")
		
		output := buf.String()
		
		// All messages should be present
		messages := []string{"debug message", "info message", "warn message", "error message"}
		for _, msg := range messages {
			if !strings.Contains(output, msg) {
				t.Errorf("Expected %q in output, got: %s", msg, output)
			}
		}
		
		// Should have TEST prefix for all entries
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if !strings.Contains(line, "[TEST]") {
				t.Errorf("Expected [TEST] prefix in line: %s", line)
			}
		}
	})
	
	t.Run("context values are ignored", func(t *testing.T) {
		var buf bytes.Buffer
		stdLogger := log.New(&buf, "", 0)
		adapter := NewStandardAdapter(stdLogger)
		
		// Create context with values
		ctx := context.WithValue(context.Background(), "trace_id", "abc-123")
		ctx = context.WithValue(ctx, "user_id", "user-456")
		
		adapter.Log(ctx, InfoLevel, "test message", Attr("key", "value"))
		
		output := buf.String()
		
		// Context values should NOT appear in output (StandardAdapter ignores context)
		if strings.Contains(output, "trace_id") || strings.Contains(output, "abc-123") {
			t.Error("Context values should not appear in StandardAdapter output")
		}
		if strings.Contains(output, "user_id") || strings.Contains(output, "user-456") {
			t.Error("Context values should not appear in StandardAdapter output")
		}
		
		// But explicit attributes should appear
		if !strings.Contains(output, "key=value") {
			t.Error("Expected explicit attribute in output")
		}
	})
	
	t.Run("level parameter is ignored", func(t *testing.T) {
		var buf bytes.Buffer
		stdLogger := log.New(&buf, "", 0)
		adapter := NewStandardAdapter(stdLogger)
		ctx := context.Background()
		
		// Log same message at different levels
		adapter.Log(ctx, DebugLevel, "same message")
		buf.Reset()
		
		adapter.Log(ctx, ErrorLevel, "same message") 
		
		// Both should produce identical output (ignoring any timestamps)
		output := buf.String()
		if !strings.Contains(output, "same message") {
			t.Error("Expected message in output regardless of level")
		}
	})
}

func TestStandardAdapter_AttributeFormatting(t *testing.T) {
	tests := []struct {
		name  string
		attrs []Attribute
		want  []string // substrings that should be present
	}{
		{
			name:  "single string attribute",
			attrs: []Attribute{{Key: "name", Value: "john"}},
			want:  []string{"name=john"},
		},
		{
			name:  "multiple attributes maintain order",
			attrs: []Attribute{{Key: "a", Value: 1}, {Key: "b", Value: 2}, {Key: "c", Value: 3}},
			want:  []string{"a=1 b=2 c=3"},
		},
		{
			name:  "attributes with spaces in values",
			attrs: []Attribute{{Key: "desc", Value: "a long description"}},
			want:  []string{"desc=a long description"},
		},
		{
			name:  "boolean attributes",
			attrs: []Attribute{{Key: "enabled", Value: true}, {Key: "disabled", Value: false}},
			want:  []string{"enabled=true", "disabled=false"},
		},
		{
			name:  "numeric attributes",
			attrs: []Attribute{{Key: "count", Value: 42}, {Key: "rate", Value: 3.14}},
			want:  []string{"count=42", "rate=3.14"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			stdLogger := log.New(&buf, "", 0)
			adapter := NewStandardAdapter(stdLogger)
			
			ctx := context.Background()
			adapter.Log(ctx, InfoLevel, "test message", tt.attrs...)
			
			output := buf.String()
			
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("Expected %q in output, got: %s", want, output)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkStandardAdapter_Log(b *testing.B) {
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0)
	adapter := NewStandardAdapter(stdLogger)
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

func BenchmarkStandardAdapter_IsLevelEnabled(b *testing.B) {
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0)
	adapter := NewStandardAdapter(stdLogger)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.IsLevelEnabled(ctx, InfoLevel)
	}
}

func BenchmarkStandardAdapter_Printf(b *testing.B) {
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0)
	adapter := NewStandardAdapter(stdLogger)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		adapter.Printf("user %s performed %s with result %v", "john", "login", true)
	}
}

// Test edge cases
func TestStandardAdapter_EdgeCases(t *testing.T) {
	t.Run("nil logger should panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when creating adapter with nil logger")
			}
		}()
		
		adapter := NewStandardAdapter(nil)
		// Try to use the adapter to trigger the panic
		adapter.Printf("test")
	})
	
	t.Run("empty attributes slice", func(t *testing.T) {
		var buf bytes.Buffer
		stdLogger := log.New(&buf, "", 0)
		adapter := NewStandardAdapter(stdLogger)
		
		ctx := context.Background()
		adapter.Log(ctx, InfoLevel, "message", []Attribute{}...)
		
		output := buf.String()
		if !strings.Contains(output, "message") {
			t.Error("Expected message in output")
		}
	})
	
	t.Run("many attributes", func(t *testing.T) {
		var buf bytes.Buffer
		stdLogger := log.New(&buf, "", 0)
		adapter := NewStandardAdapter(stdLogger)
		
		// Create many attributes
		attrs := make([]Attribute, 100)
		for i := 0; i < 100; i++ {
			attrs[i] = Attribute{Key: fmt.Sprintf("key%d", i), Value: i}
		}
		
		ctx := context.Background()
		adapter.Log(ctx, InfoLevel, "many attrs", attrs...)
		
		output := buf.String()
		if !strings.Contains(output, "many attrs") {
			t.Error("Expected message in output")
		}
		
		// Verify at least some attributes are present
		if !strings.Contains(output, "key0=0") {
			t.Error("Expected first attribute in output")
		}
	})
}