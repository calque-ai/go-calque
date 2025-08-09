package logger

import (
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

// Log implements LoggerInterface - ignores level for standard log
func (s *StandardAdapter) Log(level LogLevel, msg string, attrs ...Attribute) {
	if len(attrs) == 0 {
		s.logger.Printf("%s", msg)
		return
	}

	// Format fields as key=value pairs
	var attrStrs []string
	for _, attr := range attrs {
		attrStrs = append(attrStrs, fmt.Sprintf("%s=%v", attr.Key, attr.Value))
	}

	s.logger.Printf("%s %s", msg, strings.Join(attrStrs, " "))
}

// IsLevelEnabled always returns true for standard log (no level filtering)
func (s *StandardAdapter) IsLevelEnabled(level LogLevel) bool {
	return true // Standard log doesn't have level filtering
}

// Printf implements backward compatibility
func (s *StandardAdapter) Printf(format string, v ...any) {
	s.logger.Printf(format, v...)
}
