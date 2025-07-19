package core

import (
	"bufio"
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

// ReadString reads a complete string message
func (r *PipeReader) ReadString() (string, error) {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// PipeWriter wraps io.PipeWriter with flow-specific methods
type PipeWriter struct {
	*io.PipeWriter
}

// WriteString writes a string message
func (w *PipeWriter) WriteString(s string) error {
	_, err := w.Write([]byte(s + "\n"))
	return err
}
