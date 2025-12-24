package calque

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// testLogHandler captures log output for testing
type testLogHandler struct {
	buf    *bytes.Buffer
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

func newTestLogHandler(level slog.Level) *testLogHandler {
	return &testLogHandler{
		buf:   &bytes.Buffer{},
		level: level,
	}
}

func (h *testLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.buf.WriteString(r.Level.String())
	h.buf.WriteString(": ")
	h.buf.WriteString(r.Message)
	// Write pre-attached attrs first
	for _, a := range h.attrs {
		h.buf.WriteString(" ")
		h.buf.WriteString(a.Key)
		h.buf.WriteString("=")
		h.buf.WriteString(a.Value.String())
	}
	// Then write record attrs
	r.Attrs(func(a slog.Attr) bool {
		h.buf.WriteString(" ")
		h.buf.WriteString(a.Key)
		h.buf.WriteString("=")
		h.buf.WriteString(a.Value.String())
		return true
	})
	h.buf.WriteString("\n")
	return nil
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := &testLogHandler{
		buf:    h.buf,
		level:  h.level,
		attrs:  append(h.attrs, attrs...),
		groups: h.groups,
	}
	return newH
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	return &testLogHandler{
		buf:    h.buf,
		level:  h.level,
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
}

func (h *testLogHandler) String() string {
	return h.buf.String()
}

func (h *testLogHandler) Reset() {
	h.buf.Reset()
}

func TestLogInfo(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	t.Run("basic message", func(t *testing.T) {
		handler.Reset()
		LogInfo(ctx, "test message")
		output := handler.String()
		if !strings.Contains(output, "INFO") {
			t.Errorf("expected INFO level, got: %s", output)
		}
		if !strings.Contains(output, "test message") {
			t.Errorf("expected message, got: %s", output)
		}
	})

	t.Run("with args", func(t *testing.T) {
		handler.Reset()
		LogInfo(ctx, "user action", "user_id", 123, "action", "login")
		output := handler.String()
		if !strings.Contains(output, "user_id=123") {
			t.Errorf("expected user_id=123, got: %s", output)
		}
		if !strings.Contains(output, "action=login") {
			t.Errorf("expected action=login, got: %s", output)
		}
	})

	t.Run("with context fields", func(t *testing.T) {
		handler.Reset()
		ctx := WithLogger(context.Background(), logger)
		ctx = WithTraceID(ctx, "trace-123")
		ctx = WithRequestID(ctx, "req-456")

		LogInfo(ctx, "request started")
		output := handler.String()
		if !strings.Contains(output, "trace_id=trace-123") {
			t.Errorf("expected trace_id, got: %s", output)
		}
		if !strings.Contains(output, "request_id=req-456") {
			t.Errorf("expected request_id, got: %s", output)
		}
	})
}

func TestLogDebug(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	t.Run("basic message", func(t *testing.T) {
		handler.Reset()
		LogDebug(ctx, "debug message", "key", "value")
		output := handler.String()
		if !strings.Contains(output, "DEBUG") {
			t.Errorf("expected DEBUG level, got: %s", output)
		}
		if !strings.Contains(output, "debug message") {
			t.Errorf("expected message, got: %s", output)
		}
	})

	t.Run("filtered when level too high", func(t *testing.T) {
		infoHandler := newTestLogHandler(slog.LevelInfo)
		infoLogger := slog.New(infoHandler)
		infoCtx := WithLogger(context.Background(), infoLogger)

		LogDebug(infoCtx, "should not appear")
		output := infoHandler.String()
		if output != "" {
			t.Errorf("expected no output for debug when level is info, got: %s", output)
		}
	})
}

func TestLogWarn(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	t.Run("basic message", func(t *testing.T) {
		handler.Reset()
		LogWarn(ctx, "warning message", "attempt", 2)
		output := handler.String()
		if !strings.Contains(output, "WARN") {
			t.Errorf("expected WARN level, got: %s", output)
		}
		if !strings.Contains(output, "warning message") {
			t.Errorf("expected message, got: %s", output)
		}
		if !strings.Contains(output, "attempt=2") {
			t.Errorf("expected attempt=2, got: %s", output)
		}
	})
}

func TestLogError(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	t.Run("with error", func(t *testing.T) {
		handler.Reset()
		err := &testError{msg: "connection failed"}
		LogError(ctx, "operation failed", err, "retry", true)
		output := handler.String()
		if !strings.Contains(output, "ERROR") {
			t.Errorf("expected ERROR level, got: %s", output)
		}
		if !strings.Contains(output, "operation failed") {
			t.Errorf("expected message, got: %s", output)
		}
		if !strings.Contains(output, "error=connection failed") {
			t.Errorf("expected error field, got: %s", output)
		}
	})

	t.Run("with nil error", func(t *testing.T) {
		handler.Reset()
		LogError(ctx, "completed with warning", nil, "status", "ok")
		output := handler.String()
		if strings.Contains(output, "error=") {
			t.Errorf("expected no error field for nil error, got: %s", output)
		}
	})

	t.Run("with context fields", func(t *testing.T) {
		handler.Reset()
		ctx := WithLogger(context.Background(), logger)
		ctx = WithTraceID(ctx, "trace-err")
		ctx = WithRequestID(ctx, "req-err")

		err := &testError{msg: "test error"}
		LogError(ctx, "failed", err)
		output := handler.String()
		if !strings.Contains(output, "trace_id=trace-err") {
			t.Errorf("expected trace_id, got: %s", output)
		}
		if !strings.Contains(output, "request_id=req-err") {
			t.Errorf("expected request_id, got: %s", output)
		}
	})
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestLogWith(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)
	ctx = WithTraceID(ctx, "trace-with")

	childLogger := LogWith(ctx, "component", "test")

	handler.Reset()
	childLogger.Info("message from child")
	output := handler.String()

	if !strings.Contains(output, "component=test") {
		t.Errorf("expected component=test, got: %s", output)
	}
	if !strings.Contains(output, "trace_id=trace-with") {
		t.Errorf("expected trace_id from context, got: %s", output)
	}
}

func TestLogAttr(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)
	ctx = WithTraceID(ctx, "trace-attr")

	t.Run("with attributes", func(t *testing.T) {
		handler.Reset()
		LogAttr(ctx, slog.LevelInfo, "test message",
			slog.String("key", "value"),
			slog.Int("count", 42),
		)
		output := handler.String()
		if !strings.Contains(output, "key=value") {
			t.Errorf("expected key=value, got: %s", output)
		}
		if !strings.Contains(output, "count=42") {
			t.Errorf("expected count=42, got: %s", output)
		}
		if !strings.Contains(output, "trace_id=trace-attr") {
			t.Errorf("expected trace_id, got: %s", output)
		}
	})
}

func TestLogInfoAttr(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	LogInfoAttr(ctx, "info with attr", slog.String("key", "value"))
	output := handler.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected INFO level, got: %s", output)
	}
}

func TestLogDebugAttr(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	LogDebugAttr(ctx, "debug with attr", slog.String("key", "value"))
	output := handler.String()
	if !strings.Contains(output, "DEBUG") {
		t.Errorf("expected DEBUG level, got: %s", output)
	}
}

func TestLogWarnAttr(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	LogWarnAttr(ctx, "warn with attr", slog.String("key", "value"))
	output := handler.String()
	if !strings.Contains(output, "WARN") {
		t.Errorf("expected WARN level, got: %s", output)
	}
}

func TestLogErrorAttr(t *testing.T) {
	handler := newTestLogHandler(slog.LevelDebug)
	logger := slog.New(handler)
	ctx := WithLogger(context.Background(), logger)

	LogErrorAttr(ctx, "error with attr", slog.Any("error", "test error"))
	output := handler.String()
	if !strings.Contains(output, "ERROR") {
		t.Errorf("expected ERROR level, got: %s", output)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	// Test that logs are filtered based on level
	t.Run("info handler filters debug", func(t *testing.T) {
		handler := newTestLogHandler(slog.LevelInfo)
		logger := slog.New(handler)
		ctx := WithLogger(context.Background(), logger)

		LogDebug(ctx, "should not appear")
		if handler.String() != "" {
			t.Error("debug should be filtered when level is info")
		}

		LogInfo(ctx, "should appear")
		if !strings.Contains(handler.String(), "should appear") {
			t.Error("info should not be filtered")
		}
	})

	t.Run("error handler filters all below error", func(t *testing.T) {
		handler := newTestLogHandler(slog.LevelError)
		logger := slog.New(handler)
		ctx := WithLogger(context.Background(), logger)

		LogDebug(ctx, "debug")
		LogInfo(ctx, "info")
		LogWarn(ctx, "warn")

		if handler.String() != "" {
			t.Error("debug/info/warn should be filtered when level is error")
		}

		LogError(ctx, "error", nil)
		if !strings.Contains(handler.String(), "error") {
			t.Error("error should not be filtered")
		}
	})
}

func TestAppendContextFields(t *testing.T) {
	t.Run("appends both trace and request ID", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithTraceID(ctx, "t123")
		ctx = WithRequestID(ctx, "r456")

		args := appendContextFields(ctx, []any{"key", "value"})

		// Should have original + trace_id + request_id (6 elements)
		if len(args) != 6 {
			t.Errorf("expected 6 args, got %d", len(args))
		}
	})

	t.Run("appends only trace ID when no request ID", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithTraceID(ctx, "t123")

		args := appendContextFields(ctx, []any{"key", "value"})

		if len(args) != 4 {
			t.Errorf("expected 4 args, got %d", len(args))
		}
	})

	t.Run("no append when no context fields", func(t *testing.T) {
		ctx := context.Background()
		args := appendContextFields(ctx, []any{"key", "value"})

		if len(args) != 2 {
			t.Errorf("expected 2 args, got %d", len(args))
		}
	})
}

func TestLogDefaultLogger(_ *testing.T) {
	// When no logger is set in context, should use slog.Default()
	ctx := context.Background()

	// This should not panic
	LogInfo(ctx, "using default logger")
	LogDebug(ctx, "debug with default")
	LogWarn(ctx, "warn with default")
	LogError(ctx, "error with default", nil)
}
