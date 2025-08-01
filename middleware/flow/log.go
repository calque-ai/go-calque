package flow

import (
	"bufio"
	"fmt"
	"io"
	"log"

	"github.com/calque-ai/calque-pipe/core"
)

// LoggerInterface allows custom logging implementations
type LoggerInterface interface {
	Printf(format string, v ...any)
}

// Logger provides non-intrusive logging of input stream with preview.
//
// Input: any data type (streaming - uses bufio.Reader.Peek for preview)
// Output: same as input (pass-through)
// Behavior: STREAMING - peeks at first N bytes without consuming, then streams
//
// Logs a preview of the input with the specified prefix, then passes the
// complete input through unchanged. Uses buffered peeking to avoid consuming
// the input stream. Optionally accepts a custom logger, defaults to log.Default().
//
// Example:
//
//	logger := flow.Logger("STEP1", 100)                    // Default logger, 100 bytes
//	customLogger := flow.Logger("STEP1", 200, myLogger)   // Custom logger, 200 bytes
//	pipe.Use(logger) // Logs: [STEP1]: Hello, world!
func Logger(prefix string, peekBytes int, logger ...LoggerInterface) core.Handler {
	var l LoggerInterface
	if len(logger) > 0 {
		l = logger[0]
	} else {
		l = log.Default()
	}

	return core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		bufReader := bufio.NewReader(req.Data)

		// Peek at first N bytes for logging without consuming
		firstLine, err := bufReader.Peek(peekBytes)
		if err != nil && err != io.EOF {
			return err
		}

		// Log smart formatted preview
		preview := formatPreview(firstLine)
		l.Printf("[%s]: %s\n", prefix, preview)

		// Pass through unchanged using buffered reader
		_, err = io.Copy(res.Data, bufReader)
		return err
	})
}

// formatPreview creates a readable preview of data, handling both text and binary content
func formatPreview(data []byte) string {
	if len(data) == 0 {
		return "<empty>"
	}

	// Try to detect if it's printable text
	if isPrintable(data) {
		preview := string(data)
		if len(data) == 100 {
			preview += "..."
		}
		return preview
	}

	// For binary data, show hex summary
	if len(data) > 20 {
		return fmt.Sprintf("binary data (%d bytes): %x...", len(data), data[:20])
	}
	return fmt.Sprintf("binary data: %x", data)
}

// isPrintable checks if all bytes are printable ASCII characters
func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 {
			// Allow common whitespace characters
			if b != '\t' && b != '\n' && b != '\r' {
				return false
			}
		}
	}
	return true
}
