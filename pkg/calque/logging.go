package calque

import (
	"context"
	"log/slog"
)

// LogInfo logs an info-level message with context metadata.
//
// Automatically appends trace_id and request_id from context if present.
// Uses the logger from context, or slog.Default() if not set.
//
// Checks if info level is enabled before building the log message (optimization).
//
// Example:
//
//	calque.LogInfo(ctx, "request started")
//	calque.LogInfo(ctx, "user logged in", "user_id", 123, "ip", "192.168.1.1")
func LogInfo(ctx context.Context, msg string, args ...any) {
	logger := Logger(ctx)
	if !logger.Enabled(ctx, slog.LevelInfo) {
		return
	}
	args = appendContextFields(ctx, args)
	logger.InfoContext(ctx, msg, args...)
}

// LogDebug logs a debug-level message with context metadata.
//
// Automatically appends trace_id and request_id from context if present.
// Uses the logger from context, or slog.Default() if not set.
//
// Checks if debug level is enabled before building the log message (optimization).
//
// Example:
//
//	calque.LogDebug(ctx, "cache hit", "key", cacheKey)
//	calque.LogDebug(ctx, "processing item", "item_id", itemID, "size", len(data))
func LogDebug(ctx context.Context, msg string, args ...any) {
	logger := Logger(ctx)
	if !logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	args = appendContextFields(ctx, args)
	logger.DebugContext(ctx, msg, args...)
}

// LogWarn logs a warning-level message with context metadata.
//
// Automatically appends trace_id and request_id from context if present.
// Uses the logger from context, or slog.Default() if not set.
//
// Checks if warn level is enabled before building the log message (optimization).
//
// Example:
//
//	calque.LogWarn(ctx, "retrying operation", "attempt", 2, "max_attempts", 3)
//	calque.LogWarn(ctx, "deprecated API called", "endpoint", "/v1/old")
func LogWarn(ctx context.Context, msg string, args ...any) {
	logger := Logger(ctx)
	if !logger.Enabled(ctx, slog.LevelWarn) {
		return
	}
	args = appendContextFields(ctx, args)
	logger.WarnContext(ctx, msg, args...)
}

// LogError logs an error-level message with context metadata.
//
// Automatically appends trace_id, request_id, and error from context if present.
// Uses the logger from context, or slog.Default() if not set.
// If err is not nil, it's added to the log with key "error".
//
// Checks if error level is enabled before building the log message (optimization).
//
// Example:
//
//	calque.LogError(ctx, "request failed", err)
//	calque.LogError(ctx, "database error", err, "query", sqlQuery, "duration_ms", duration)
func LogError(ctx context.Context, msg string, err error, args ...any) {
	logger := Logger(ctx)
	if !logger.Enabled(ctx, slog.LevelError) {
		return
	}
	args = appendContextFields(ctx, args)
	if err != nil {
		args = append(args, "error", err)
	}
	logger.ErrorContext(ctx, msg, args...)
}

// appendContextFields adds trace_id and request_id to args if present in context.
//
// This is called by all log functions to ensure consistent context propagation.
func appendContextFields(ctx context.Context, args []any) []any {
	if traceID := TraceID(ctx); traceID != "" {
		args = append(args, "trace_id", traceID)
	}
	if requestID := RequestID(ctx); requestID != "" {
		args = append(args, "request_id", requestID)
	}
	return args
}

// LogWith returns a logger with pre-attached context fields.
//
// Useful when you need to log multiple messages with the same context fields.
// The returned logger includes trace_id and request_id from context.
//
// Example:
//
//	logger := calque.LogWith(ctx, "component", "ai-agent")
//	logger.Info("starting")
//	logger.Info("processing", "item", itemID)
//	logger.Info("completed", "duration_ms", elapsed)
func LogWith(ctx context.Context, args ...any) *slog.Logger {
	logger := Logger(ctx)
	args = appendContextFields(ctx, args)
	return logger.With(args...)
}

// LogAttr logs with slog.Attr for structured logging.
//
// Use this when you need type-safe attributes or want to use slog.Attr directly.
// Automatically appends trace_id and request_id as attributes.
//
// Example:
//
//	calque.LogAttr(ctx, slog.LevelInfo, "user action",
//	    slog.String("action", "login"),
//	    slog.Int("user_id", 123),
//	    slog.Duration("latency", time.Since(start)),
//	)
func LogAttr(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	logger := Logger(ctx)
	if !logger.Enabled(ctx, level) {
		return
	}

	// Add context fields as attributes
	if traceID := TraceID(ctx); traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if requestID := RequestID(ctx); requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}

	logger.LogAttrs(ctx, level, msg, attrs...)
}

// LogInfoAttr logs info-level with slog.Attr.
//
// Convenience wrapper around LogAttr for info level.
//
// Example:
//
//	calque.LogInfoAttr(ctx, "request completed",
//	    slog.String("method", "POST"),
//	    slog.Int("status", 200),
//	    slog.Duration("duration", elapsed),
//	)
func LogInfoAttr(ctx context.Context, msg string, attrs ...slog.Attr) {
	LogAttr(ctx, slog.LevelInfo, msg, attrs...)
}

// LogDebugAttr logs debug-level with slog.Attr.
//
// Convenience wrapper around LogAttr for debug level.
//
// Example:
//
//	calque.LogDebugAttr(ctx, "cache lookup",
//	    slog.String("key", cacheKey),
//	    slog.Bool("hit", found),
//	)
func LogDebugAttr(ctx context.Context, msg string, attrs ...slog.Attr) {
	LogAttr(ctx, slog.LevelDebug, msg, attrs...)
}

// LogWarnAttr logs warn-level with slog.Attr.
//
// Convenience wrapper around LogAttr for warn level.
//
// Example:
//
//	calque.LogWarnAttr(ctx, "rate limit approaching",
//	    slog.Int("current", current),
//	    slog.Int("limit", limit),
//	)
func LogWarnAttr(ctx context.Context, msg string, attrs ...slog.Attr) {
	LogAttr(ctx, slog.LevelWarn, msg, attrs...)
}

// LogErrorAttr logs error-level with slog.Attr.
//
// Convenience wrapper around LogAttr for error level.
// If you have an error, include it as slog.Any("error", err).
//
// Example:
//
//	calque.LogErrorAttr(ctx, "operation failed",
//	    slog.Any("error", err),
//	    slog.String("operation", "database_query"),
//	    slog.Duration("duration", elapsed),
//	)
func LogErrorAttr(ctx context.Context, msg string, attrs ...slog.Attr) {
	LogAttr(ctx, slog.LevelError, msg, attrs...)
}
