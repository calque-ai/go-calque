// Package logger provides structured logging middleware for the calque framework.
// It implements level-based logging with context support and adapters for various
// logging backends to provide consistent logging across the application.
package logger

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// StandardAdapter adapts the standard library log package to our LoggerInterface
type StandardAdapter struct {
	logger *log.Logger
}

// NewStandardAdapter creates a new adapter for the standard log package
func NewStandardAdapter(logger *log.Logger) *StandardAdapter {
	return &StandardAdapter{logger: logger}
}

// Log implements LoggerInterface - ignores level and context for standard log
func (s *StandardAdapter) Log(_ context.Context, _ LogLevel, msg string, attrs ...Attribute) {
	if len(attrs) == 0 {
		s.logger.Printf("%s", msg)
		return
	}

	// Format fields as key=value pairs
	attrStrs := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		attrStrs = append(attrStrs, fmt.Sprintf("%s=%v", attr.Key, attr.Value))
	}

	s.logger.Printf("%s %s", msg, strings.Join(attrStrs, " "))
}

// IsLevelEnabled always returns true for standard log (no level filtering)
func (s *StandardAdapter) IsLevelEnabled(_ context.Context, _ LogLevel) bool {
	return true // Standard log doesn't have level filtering
}

// Printf implements backward compatibility
func (s *StandardAdapter) Printf(format string, v ...any) {
	s.logger.Printf(format, v...)
}
