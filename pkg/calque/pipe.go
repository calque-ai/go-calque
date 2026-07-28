package calque

import (
	"bytes"
	"io"
	"sync"
)

const defaultPipeBufSize = 64 * 1024 // 64KB: covers most robot sensor payloads in one write

// bufpipe is the shared state between PipeReader and PipeWriter.
// Unlike io.Pipe (which uses unbuffered channels requiring a round-trip per write),
// bufpipe holds an internal buffer so the writer can advance without waiting for
// the reader to consume each chunk. The writer only blocks when the buffer is full.
type bufpipe struct {
	mu    sync.Mutex
	rwait sync.Cond // reader waits here when buffer is empty
	wwait sync.Cond // writer waits here when buffer is full

	buf bytes.Buffer
	cap int // max bytes buf may hold; writer blocks when buf.Len() >= cap

	rerr error // set by reader close; poisons subsequent writes
	werr error // set by writer close; returned to reads after buffer drained
}

func newBufPipe(bufSize int) *bufpipe {
	p := &bufpipe{cap: bufSize}
	p.rwait.L = &p.mu
	p.wwait.L = &p.mu
	return p
}

// PipeReader is the read half of a buffered pipe.
type PipeReader struct {
	p *bufpipe
}

// Read reads from the pipe buffer, blocking until data is available or the
// write end is closed.
func (r *PipeReader) Read(b []byte) (int, error) {
	p := r.p
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		// Drain buffered data first, even if the reader or writer has been closed.
		// Preserves all bytes written before Close() was called.
		if p.buf.Len() > 0 {
			n, err := p.buf.Read(b)
			if err == io.EOF {
				err = nil
			}
			p.wwait.Signal() // buffer has space; unblock any waiting writer
			return n, err
		}
		// Buffer empty — check terminal states.
		if p.rerr != nil {
			return 0, p.rerr
		}
		if p.werr != nil {
			return 0, p.werr
		}
		// Buffer empty, no errors — wait. Always loop back to re-check
		// buf.Len() after waking; never return directly from here, to avoid
		// discarding data written before a close signal.
		p.rwait.Wait()
	}
}

// Close closes the read end. Subsequent writes return io.ErrClosedPipe.
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the read end with a specific error.
func (r *PipeReader) CloseWithError(err error) error {
	p := r.p
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rerr == nil {
		if err == nil {
			err = io.ErrClosedPipe
		}
		p.rerr = err
		p.rwait.Signal()
		p.wwait.Signal()
	}
	return nil
}

// PipeWriter is the write half of a buffered pipe.
type PipeWriter struct {
	p *bufpipe
}

// Write copies b into the pipe buffer. Blocks only when the buffer is full.
// Cap is enforced by slicing the input to available space before each
// bytes.Buffer.Write, so the buffer never exceeds cap regardless of input size.
// Returns io.ErrClosedPipe if the read end has been closed.
func (w *PipeWriter) Write(b []byte) (int, error) {
	p := w.p
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.werr != nil {
		return 0, io.ErrClosedPipe
	}

	total := 0
	for len(b) > 0 {
		// Wait while buffer is full and no errors.
		for p.buf.Len() >= p.cap && p.rerr == nil && p.werr == nil {
			p.rwait.Signal() // nudge reader in case it's waiting
			p.wwait.Wait()
		}
		if p.rerr != nil {
			return total, p.rerr
		}
		if p.werr != nil {
			return total, io.ErrClosedPipe
		}

		// Slice input to exactly what fits so bytes.Buffer never exceeds cap.
		space := p.cap - p.buf.Len()
		chunk := b
		if len(chunk) > space {
			chunk = b[:space]
		}
		n, _ := p.buf.Write(chunk)
		total += n
		b = b[n:]
		p.rwait.Signal() // data available; unblock reader
	}
	return total, nil
}

// Close closes the write end, signalling EOF to readers after buffer is drained.
func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

// CloseWithError closes the write end with a specific error.
// Readers drain the buffer first, then receive this error (or io.EOF if nil).
func (w *PipeWriter) CloseWithError(err error) error {
	p := w.p
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.werr == nil {
		if err == nil {
			err = io.EOF
		}
		p.werr = err
		p.rwait.Signal()
		p.wwait.Signal()
	}
	return nil
}

// Pipe creates a buffered pipe with the default buffer size.
// Drop-in for io.Pipe: the writer only blocks when the buffer is full instead
// of blocking on every write, decoupling producer and consumer goroutines.
func Pipe() (*PipeReader, *PipeWriter) {
	return PipeSize(defaultPipeBufSize)
}

// PipeSize creates a buffered pipe with a custom buffer capacity.
// Larger buffers improve throughput for payloads exceeding the default 64KB;
// smaller buffers reduce per-pipe memory when many pipes run concurrently.
func PipeSize(bufSize int) (*PipeReader, *PipeWriter) {
	p := newBufPipe(bufSize)
	return &PipeReader{p: p}, &PipeWriter{p: p}
}
