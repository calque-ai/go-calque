package logger

import (
	"github.com/rs/zerolog"
)

// ZerologAdapter adapts zerolog.Logger to our LoggerInterface
type ZerologAdapter struct {
	logger zerolog.Logger
}

// NewZerologAdapter creates a new adapter for zerolog
func NewZerologAdapter(logger zerolog.Logger) *ZerologAdapter {
	return &ZerologAdapter{logger: logger}
}

// Log implements LoggerInterface for structured logging with zerolog
func (z *ZerologAdapter) Log(level LogLevel, msg string, attrs ...Attribute) {
	var evt *zerolog.Event

	switch level {
	case DebugLevel:
		evt = z.logger.Debug()
	case InfoLevel:
		evt = z.logger.Info()
	case WarnLevel:
		evt = z.logger.Warn()
	case ErrorLevel:
		evt = z.logger.Error()
	default:
		evt = z.logger.Info()
	}

	// Add structured attributes
	for _, attr := range attrs {
		evt = evt.Interface(attr.Key, attr.Value)
	}

	evt.Msg(msg)
}

// IsLevelEnabled checks if the given level is enabled in zerolog
func (z *ZerologAdapter) IsLevelEnabled(level LogLevel) bool {
	zerologLevel := logLevelToZerolog(level)
	return z.logger.GetLevel() <= zerologLevel
}

// Printf implements backward compatibility (though not commonly used with zerolog)
func (z *ZerologAdapter) Printf(format string, v ...any) {
	z.logger.Printf(format, v...)
}

// logLevelToZerolog converts our LogLevel to zerolog.Level
func logLevelToZerolog(level LogLevel) zerolog.Level {
	switch level {
	case DebugLevel:
		return zerolog.DebugLevel
	case InfoLevel:
		return zerolog.InfoLevel
	case WarnLevel:
		return zerolog.WarnLevel
	case ErrorLevel:
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
