package logger

import (
	"context"
	"fmt"
	"log"
)

// LogLevel represents logging levels (Debug < Info < Warn < Error)
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// Attribute represents a structured logging attribute for key-value pairs
type Attribute struct {
	Key   string
	Value any
}

// Attr creates an Attribute
func Attr(key string, value any) Attribute {
	return Attribute{Key: key, Value: value}
}

// Adapter defines the contract for logging backends (zerolog, slog, standard log, etc.)
type Adapter interface {
	Log(ctx context.Context, level LogLevel, msg string, attrs ...Attribute) // Structured logging with level
	IsLevelEnabled(ctx context.Context, level LogLevel) bool                 // Performance check - skip work if disabled
	Printf(format string, v ...any)                                          // Simple printf-style logging
}

// ============================================================================
// LOGGER INSTANCE
// ============================================================================

// Logger wraps any LoggerInterface backend and provides the main API
type Logger struct {
	backend Adapter
}

// New creates a Logger with a custom backend (zerolog, slog, etc.)
func New(backend Adapter) *Logger {
	return &Logger{backend: backend}
}

// Default creates a Logger using the standard library log package (simple, no levels)
func Default() *Logger {
	return New(NewStandardAdapter(log.Default()))
}

// ============================================================================
// LEVEL METHODS - Create HandlerBuilder with specific log levels
// ============================================================================

func (l *Logger) Debug() *HandlerBuilder {
	return &HandlerBuilder{
		logger:  l,
		printer: &LeveledPrinter{backend: l.backend, level: DebugLevel},
	}
}

func (l *Logger) Info() *HandlerBuilder {
	return &HandlerBuilder{
		logger:  l,
		printer: &LeveledPrinter{backend: l.backend, level: InfoLevel},
	}
}

func (l *Logger) Warn() *HandlerBuilder {
	return &HandlerBuilder{
		logger:  l,
		printer: &LeveledPrinter{backend: l.backend, level: WarnLevel},
	}
}

func (l *Logger) Error() *HandlerBuilder {
	return &HandlerBuilder{
		logger:  l,
		printer: &LeveledPrinter{backend: l.backend, level: ErrorLevel},
	}
}

// Print provides level-agnostic logging
func (l *Logger) Print() *HandlerBuilder {
	return &HandlerBuilder{
		logger:  l,
		printer: &SimplePrinter{backend: l.backend},
	}
}

// ============================================================================
// INTERNAL TYPES - HandlerBuilder and Printers
// ============================================================================

// HandlerBuilder provides the specialized logging methods (Head, Chunks, Timing, etc.)
// All Logger level methods return this to enable: log.Info().Head("prefix", 100)
type HandlerBuilder struct {
	logger  *Logger
	printer Printer
	ctx     context.Context // Optional explicit context for tracing/observability
}

// WithContext returns a new HandlerBuilder with the specified context for tracing/observability
func (hb *HandlerBuilder) WithContext(ctx context.Context) *HandlerBuilder {
	return &HandlerBuilder{
		logger:  hb.logger,
		printer: hb.printer,
		ctx:     ctx,
	}
}

// Printer interface abstracts different printing strategies (leveled vs simple)
type Printer interface {
	Print(ctx context.Context, msg string, attrs ...Attribute)
}

// SimplePrinter uses Printf() - no levels, simple formatting
type SimplePrinter struct {
	backend Adapter
}

func (sp *SimplePrinter) Print(ctx context.Context, msg string, attrs ...Attribute) {
	if len(attrs) == 0 {
		sp.backend.Printf("%s", msg)
		return
	}

	// Simple attribute formatting: msg key1=value1 key2=value2
	var attrStr string
	for _, attr := range attrs {
		attrStr += fmt.Sprintf(" %s=%v", attr.Key, attr.Value)
	}
	sp.backend.Printf("%s%s", msg, attrStr)
}

// LeveledPrinter uses Log() with level checking - structured logging
type LeveledPrinter struct {
	backend Adapter
	level   LogLevel
}

func (lp *LeveledPrinter) Print(ctx context.Context, msg string, attrs ...Attribute) {
	if lp.backend.IsLevelEnabled(ctx, lp.level) {
		lp.backend.Log(ctx, lp.level, msg, attrs...)
	}
}
