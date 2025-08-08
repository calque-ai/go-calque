package calque

import (
	"io"
)

// Pipe creates a connected pair of readers and writers, like io.Pipe
func Pipe() (*PipeReader, *PipeWriter) {
	r, w := io.Pipe()
	return &PipeReader{PipeReader: r}, &PipeWriter{PipeWriter: w}
}

// PipeReader wraps io.PipeReader with flow-specific methods
type PipeReader struct {
	*io.PipeReader
}

// PipeWriter wraps io.PipeWriter with flow-specific methods
type PipeWriter struct {
	*io.PipeWriter
}
