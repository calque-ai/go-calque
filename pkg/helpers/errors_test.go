package helpers

import (
	"context"
	"errors"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestWrapError(t *testing.T) {
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-wrap-error")
	ctx = calque.WithRequestID(ctx, "calque-req-test-wrap-error")

	tests := []struct {
		name     string
		ctx      context.Context
		err      error
		message  string
		expected string
		isNil    bool
	}{
		{
			name:     "wrap non-nil error",
			ctx:      ctx,
			err:      errors.New("original error"),
			message:  "failed to process",
			expected: "failed to process: original error",
			isNil:    false,
		},
		{
			name:     "wrap nil error",
			ctx:      ctx,
			err:      nil,
			message:  "failed to process",
			expected: "",
			isNil:    true,
		},
		{
			name:     "wrap with empty message",
			ctx:      ctx,
			err:      errors.New("original error"),
			message:  "",
			expected: ": original error",
			isNil:    false,
		},
		{
			name:     "wrap without context metadata",
			ctx:      context.Background(),
			err:      errors.New("original error"),
			message:  "failed to process",
			expected: "failed to process: original error",
			isNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WrapError(tt.ctx, tt.err, tt.message)

			if tt.isNil {
				if result != nil {
					t.Errorf("WrapError() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("WrapError() = nil, want non-nil error")
			}

			if result.Error() != tt.expected {
				t.Errorf("WrapError() = %q, want %q", result.Error(), tt.expected)
			}

			// Test that the original error can be unwrapped
			if tt.err != nil && !errors.Is(result, tt.err) {
				t.Error("WrapError() should preserve original error for errors.Is()")
			}

			// Test that calque.Error has context metadata
			if calqueErr, ok := result.(*calque.Error); ok && tt.ctx != nil {
				if traceID := calque.TraceID(tt.ctx); traceID != "" {
					if calqueErr.TraceID() != traceID {
						t.Errorf("WrapError() traceID = %q, want %q", calqueErr.TraceID(), traceID)
					}
				}
			}
		})
	}
}

func TestWrapErrorf(t *testing.T) {
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-wrap-errorf")
	ctx = calque.WithRequestID(ctx, "calque-req-test-wrap-errorf")

	tests := []struct {
		name     string
		ctx      context.Context
		err      error
		format   string
		args     []interface{}
		expected string
		isNil    bool
	}{
		{
			name:     "wrap with formatted message",
			ctx:      ctx,
			err:      errors.New("connection failed"),
			format:   "failed to connect to %s service",
			args:     []interface{}{"database"},
			expected: "failed to connect to database service: connection failed",
			isNil:    false,
		},
		{
			name:     "wrap nil error",
			ctx:      ctx,
			err:      nil,
			format:   "failed to connect to %s service",
			args:     []interface{}{"database"},
			expected: "",
			isNil:    true,
		},
		{
			name:     "wrap with multiple args",
			ctx:      ctx,
			err:      errors.New("not found"),
			format:   "user %d in organization %s not found",
			args:     []interface{}{123, "acme"},
			expected: "user 123 in organization acme not found: not found",
			isNil:    false,
		},
		{
			name:     "wrap with no args",
			ctx:      ctx,
			err:      errors.New("timeout"),
			format:   "operation timed out",
			args:     []interface{}{},
			expected: "operation timed out: timeout",
			isNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WrapErrorf(tt.ctx, tt.err, tt.format, tt.args...)

			if tt.isNil {
				if result != nil {
					t.Errorf("WrapErrorf() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("WrapErrorf() = nil, want non-nil error")
			}

			if result.Error() != tt.expected {
				t.Errorf("WrapErrorf() = %q, want %q", result.Error(), tt.expected)
			}

			// Test that the original error can be unwrapped
			if tt.err != nil && !errors.Is(result, tt.err) {
				t.Error("WrapErrorf() should preserve original error for errors.Is()")
			}
		})
	}
}

func TestNewError(t *testing.T) {
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-new-error")
	ctx = calque.WithRequestID(ctx, "calque-req-test-new-error")

	tests := []struct {
		name     string
		ctx      context.Context
		message  string
		expected string
	}{
		{
			name:     "simple message",
			ctx:      ctx,
			message:  "something went wrong",
			expected: "something went wrong",
		},
		{
			name:     "error message",
			ctx:      ctx,
			message:  "user not found",
			expected: "user not found",
		},
		{
			name:     "empty message",
			ctx:      ctx,
			message:  "",
			expected: "",
		},
		{
			name:     "without context metadata",
			ctx:      context.Background(),
			message:  "invalid configuration",
			expected: "invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := NewError(tt.ctx, tt.message)

			if result == nil {
				t.Fatal("NewError() = nil, want non-nil error")
			}

			if result.Error() != tt.expected {
				t.Errorf("NewError() = %q, want %q", result.Error(), tt.expected)
			}

			// Test that calque.Error has context metadata
			if calqueErr, ok := result.(*calque.Error); ok {
				if traceID := calque.TraceID(tt.ctx); traceID != "" {
					if calqueErr.TraceID() != traceID {
						t.Errorf("NewError() traceID = %q, want %q", calqueErr.TraceID(), traceID)
					}
				}
			}
		})
	}
}
