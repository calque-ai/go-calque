package logger

import (
	"context"
	"log/slog"
)

// SlogAdapter adapts slog.Logger to our LoggerInterface
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new adapter for slog
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: logger}
}

// Log implements LoggerInterface for structured logging with slog
func (s *SlogAdapter) Log(ctx context.Context, level LogLevel, msg string, attrs ...Attribute) {
	slogLevel := logLevelToSlog(level)

	// Convert our Attributes to slog.Attr
	slogAttrs := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		slogAttrs[i] = slog.Any(attr.Key, attr.Value)
	}

	// Use provided context for tracing/observability
	s.logger.LogAttrs(ctx, slogLevel, msg, slogAttrs...)
}

// IsLevelEnabled checks if the given level is enabled in slog
func (s *SlogAdapter) IsLevelEnabled(ctx context.Context, level LogLevel) bool {
	slogLevel := logLevelToSlog(level)
	return s.logger.Enabled(ctx, slogLevel)
}

// Printf implements backward compatibility (though not idiomatic for slog)
func (s *SlogAdapter) Printf(format string, v ...any) {
	s.logger.Info(format, v...)
}

// logLevelToSlog converts our LogLevel to slog.Level
func logLevelToSlog(level LogLevel) slog.Level {
	switch level {
	case DebugLevel:
		return slog.LevelDebug
	case InfoLevel:
		return slog.LevelInfo
	case WarnLevel:
		return slog.LevelWarn
	case ErrorLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
