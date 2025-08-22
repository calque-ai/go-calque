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
func (s *StandardAdapter) Log(_ context.Context, level LogLevel, msg string, attrs ...Attribute) {
	if len(attrs) == 0 {
		s.logger.Printf("%s", msg)
		return
	}

	// Format fields as key=value pairs
	attrStrs := make([]string, len(attrs))
	for i, attr := range attrs {
		attrStrs[i] = fmt.Sprintf("%s=%v", attr.Key, attr.Value)
	}

	s.logger.Printf("%s %s", msg, strings.Join(attrStrs, " "))
}

// IsLevelEnabled always returns true for standard log (no level filtering)
func (s *StandardAdapter) IsLevelEnabled(_ context.Context, level LogLevel) bool {
	return true // Standard log doesn't have level filtering
}

// Printf implements backward compatibility
func (s *StandardAdapter) Printf(format string, v ...any) {
	s.logger.Printf(format, v...)
}
