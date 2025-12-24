package calque

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestWrapErr(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "calque-trace-test-wrap-err")
	ctx = WithRequestID(ctx, "calque-req-test-wrap-err")
	originalErr := errors.New("original error")

	err := WrapErr(ctx, originalErr, "operation failed")

	if err.Message() != "operation failed" {
		t.Errorf("Message() = %q, want %q", err.Message(), "operation failed")
	}
	if err.Cause() != originalErr {
		t.Error("Cause() did not return original error")
	}
	if err.TraceID() != "calque-trace-test-wrap-err" {
		t.Errorf("TraceID() = %q, want %q", err.TraceID(), "calque-trace-test-wrap-err")
	}
	if err.RequestID() != "calque-req-test-wrap-err" {
		t.Errorf("RequestID() = %q, want %q", err.RequestID(), "calque-req-test-wrap-err")
	}
}

func TestWrapErr_ErrorString(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("original error")
	err := WrapErr(ctx, originalErr, "operation failed")
	errStr := err.Error()

	if !strings.Contains(errStr, "operation failed") {
		t.Errorf("Error() = %q, want to contain %q", errStr, "operation failed")
	}
	if !strings.Contains(errStr, "original error") {
		t.Errorf("Error() = %q, want to contain %q", errStr, "original error")
	}
}

func TestWrapErr_Unwrap(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("original error")
	err := WrapErr(ctx, originalErr, "wrapped")

	unwrapped := errors.Unwrap(err)
	if unwrapped != originalErr {
		t.Error("Unwrap() did not return original error")
	}
}

func TestWrapErr_ErrorsIs(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("original error")
	err := WrapErr(ctx, originalErr, "wrapped")

	if !errors.Is(err, originalErr) {
		t.Error("errors.Is() = false, want true for original error")
	}
}

func TestWrapErr_WithoutContextIDs(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("original error")
	err := WrapErr(ctx, originalErr, "no context")

	if err.TraceID() != "" {
		t.Errorf("TraceID() = %q, want empty string", err.TraceID())
	}
	if err.RequestID() != "" {
		t.Errorf("RequestID() = %q, want empty string", err.RequestID())
	}
}

func TestNewErr(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "calque-trace-test-new-err")
	ctx = WithRequestID(ctx, "calque-req-test-new-err")

	err := NewErr(ctx, "validation failed")

	if err.Message() != "validation failed" {
		t.Errorf("Message() = %q, want %q", err.Message(), "validation failed")
	}
	if err.Cause() != nil {
		t.Error("Cause() = non-nil, want nil")
	}
	if err.TraceID() != "calque-trace-test-new-err" {
		t.Errorf("TraceID() = %q, want %q", err.TraceID(), "calque-trace-test-new-err")
	}
}

func TestNewErr_ErrorString(t *testing.T) {
	ctx := context.Background()
	err := NewErr(ctx, "simple error")

	if err.Error() != "simple error" {
		t.Errorf("Error() = %q, want %q", err.Error(), "simple error")
	}
}

func TestNewErr_UnwrapNil(t *testing.T) {
	ctx := context.Background()
	err := NewErr(ctx, "no cause")

	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		t.Error("Unwrap() = non-nil, want nil for NewErr")
	}
}

func TestError_Tag(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("base error")

	err := WrapErr(ctx, originalErr, "failed").
		Tag(slog.String("user_id", "user-123"))

	attrs := err.Attrs()
	if len(attrs) != 1 {
		t.Fatalf("Attrs() len = %d, want 1", len(attrs))
	}
	if attrs[0].Key != "user_id" || attrs[0].Value.String() != "user-123" {
		t.Errorf("Attrs()[0] = %s=%s, want user_id=user-123", attrs[0].Key, attrs[0].Value.String())
	}
}

func TestError_Tag_Multiple(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("base error")

	err := WrapErr(ctx, originalErr, "failed").
		Tag(slog.String("key1", "value1")).
		Tag(slog.Int("key2", 42)).
		Tag(slog.Bool("key3", true))

	attrs := err.Attrs()
	if len(attrs) != 3 {
		t.Fatalf("Attrs() len = %d, want 3", len(attrs))
	}
}

func TestError_Tag_VariousTypes(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("base error")
	duration := 5 * time.Second

	err := WrapErr(ctx, originalErr, "failed").
		Tag(slog.String("string", "value")).
		Tag(slog.Int("int", 123)).
		Tag(slog.Int64("int64", 9999999999)).
		Tag(slog.Float64("float", 3.14)).
		Tag(slog.Bool("bool", true)).
		Tag(slog.Duration("duration", duration)).
		Tag(slog.Any("any", []string{"a", "b"}))

	attrs := err.Attrs()
	if len(attrs) != 7 {
		t.Fatalf("Attrs() len = %d, want 7", len(attrs))
	}
}

func TestError_Tags(t *testing.T) {
	ctx := context.Background()
	originalErr := errors.New("base error")

	err := WrapErr(ctx, originalErr, "failed").
		Tags(
			slog.String("method", "POST"),
			slog.String("path", "/api"),
			slog.Int("status", 500),
		)

	attrs := err.Attrs()
	if len(attrs) != 3 {
		t.Fatalf("Attrs() len = %d, want 3", len(attrs))
	}
	if attrs[0].Key != "method" {
		t.Errorf("Attrs()[0].Key = %q, want %q", attrs[0].Key, "method")
	}
}

func TestError_LogAttrs(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "calque-trace-test-error-log")
	ctx = WithRequestID(ctx, "calque-req-test-error-log")
	originalErr := errors.New("base error")

	err := WrapErr(ctx, originalErr, "failed").
		Tag(slog.String("custom", "value"))

	logAttrs := err.LogAttrs()

	// Should have: error, trace_id, request_id, custom
	if len(logAttrs) != 4 {
		t.Fatalf("LogAttrs() len = %d, want 4", len(logAttrs))
	}

	found := make(map[string]bool)
	for _, attr := range logAttrs {
		found[attr.Key] = true
	}

	if !found["error"] {
		t.Error("LogAttrs() missing 'error' attr")
	}
	if !found["trace_id"] {
		t.Error("LogAttrs() missing 'trace_id' attr")
	}
	if !found["request_id"] {
		t.Error("LogAttrs() missing 'request_id' attr")
	}
	if !found["custom"] {
		t.Error("LogAttrs() missing 'custom' attr")
	}
}

func TestError_LogAttrs_NoCause(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "calque-trace-test-error-log-no-cause")
	err := NewErr(ctx, "no cause").
		Tag(slog.String("key", "value"))

	logAttrs := err.LogAttrs()

	found := make(map[string]bool)
	for _, attr := range logAttrs {
		found[attr.Key] = true
	}

	if found["error"] {
		t.Error("LogAttrs() should not have 'error' attr when no cause")
	}
}

func TestError_LogAttrs_EmptyContextIDs(t *testing.T) {
	ctx := context.Background()
	err := NewErr(ctx, "no context")

	logAttrs := err.LogAttrs()

	for _, attr := range logAttrs {
		if attr.Key == "trace_id" || attr.Key == "request_id" {
			t.Errorf("LogAttrs() should not include empty %s", attr.Key)
		}
	}
}

func TestError_Log(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)
	ctx = WithTraceID(ctx, "calque-trace-test-error-log-with-level")

	originalErr := errors.New("underlying error")
	err := WrapErr(ctx, originalErr, "operation failed").
		Tag(slog.String("component", "test"))

	err.Log(ctx)

	output := handler.String()
	if !strings.Contains(output, "ERROR") {
		t.Errorf("Log() output = %q, want to contain ERROR", output)
	}
	if !strings.Contains(output, "operation failed") {
		t.Errorf("Log() output = %q, want to contain message", output)
	}
}

func TestError_LogWithLevel(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	err := NewErr(ctx, "warning condition")
	err.LogWithLevel(ctx, slog.LevelWarn)

	output := handler.String()
	if !strings.Contains(output, "WARN") {
		t.Errorf("LogWithLevel() output = %q, want to contain WARN", output)
	}
}

func TestError_WithMessage(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "calque-trace-test-error-with-message")

	originalErr := errors.New("base")
	err1 := WrapErr(ctx, originalErr, "first wrapper").
		Tag(slog.String("key", "value"))

	err2 := err1.WithMessage("second wrapper")

	if err2.Message() != "second wrapper" {
		t.Errorf("Message() = %q, want %q", err2.Message(), "second wrapper")
	}
	if err2.Cause() != err1 {
		t.Error("Cause() should be the first error")
	}
	if err2.TraceID() != "calque-trace-test-error-with-message" {
		t.Errorf("TraceID() = %q, want %q", err2.TraceID(), "calque-trace-test-error-with-message")
	}
	if len(err2.Attrs()) != 1 {
		t.Errorf("Attrs() len = %d, want 1", len(err2.Attrs()))
	}
	if !errors.Is(err2, originalErr) {
		t.Error("errors.Is() should find original error through chain")
	}
}

func TestError_Is(t *testing.T) {
	ctx := context.Background()

	// Same message matches
	err1 := NewErr(ctx, "same message")
	err2 := NewErr(ctx, "same message")
	if !err1.Is(err2) {
		t.Error("Is() = false for same message, want true")
	}

	// Different message does not match
	err3 := NewErr(ctx, "different message")
	if err1.Is(err3) {
		t.Error("Is() = true for different message, want false")
	}

	// Different error type does not match
	stdErr := errors.New("standard error")
	if err1.Is(stdErr) {
		t.Error("Is() = true for different error type, want false")
	}
}

func TestError_ErrorsAs(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "calque-trace-test-error-is")

	originalErr := NewErr(ctx, "calque error").
		Tag(slog.String("key", "value"))

	wrappedErr := WrapErr(ctx, originalErr, "wrapped")

	var calqueErr *Error
	if !errors.As(wrappedErr, &calqueErr) {
		t.Error("errors.As() = false, want true")
	}

	if calqueErr.Message() != "wrapped" {
		t.Errorf("Message() = %q, want %q", calqueErr.Message(), "wrapped")
	}
}

func TestError_ChainedWrapping(t *testing.T) {
	ctx := context.Background()

	baseErr := errors.New("database connection failed")
	err1 := WrapErr(ctx, baseErr, "repository error")
	err2 := WrapErr(ctx, err1, "service error")
	err3 := WrapErr(ctx, err2, "handler error")

	if err3.Message() != "handler error" {
		t.Errorf("Message() = %q, want %q", err3.Message(), "handler error")
	}
	if !errors.Is(err3, baseErr) {
		t.Error("errors.Is() should find base error in chain")
	}

	errStr := err3.Error()
	if !strings.Contains(errStr, "handler error") {
		t.Errorf("Error() = %q, want to contain 'handler error'", errStr)
	}
	if !strings.Contains(errStr, "service error") {
		t.Errorf("Error() = %q, want to contain 'service error'", errStr)
	}
}
