package calque

import (
	"io"
	"strings"
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

// NewReader creates a new reader from a string for testing
func NewReader(s string) io.Reader {
	return strings.NewReader(s)
}

// NewWriter creates a new writer that can be used for testing
func NewWriter() *Buffer {
	return &Buffer{}
}

// Buffer is a simple buffer implementation for testing
type Buffer struct {
	data []byte
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *Buffer) String() string {
	return string(b.data)
}

// Bytes returns the buffer contents as a byte slice.
func (b *Buffer) Bytes() []byte {
	return b.data
}
