// Package grpc provides streaming utilities for gRPC communication.
package grpc

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// Stream represents a bidirectional gRPC stream.
type Stream interface {
	Send(msg proto.Message) error
	Recv() (proto.Message, error)
	CloseSend() error
	Context() context.Context
}

// StreamHandler handles streaming operations.
type StreamHandler struct {
	stream Stream
}

// NewStreamHandler creates a new stream handler.
func NewStreamHandler(stream Stream) *StreamHandler {
	return &StreamHandler{stream: stream}
}

// SendMessage sends a protobuf message through the stream.
func (sh *StreamHandler) SendMessage(msg proto.Message) error {
	if err := sh.stream.Send(msg); err != nil {
		return WrapError(sh.stream.Context(), err, "failed to send message")
	}
	return nil
}

// ReceiveMessage receives a protobuf message from the stream.
func (sh *StreamHandler) ReceiveMessage(_ proto.Message) error {
	_, err := sh.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return io.EOF
		}
		return WrapError(sh.stream.Context(), err, "failed to receive message")
	}
	return nil
}

// Close closes the send side of the stream.
func (sh *StreamHandler) Close() error {
	if err := sh.stream.CloseSend(); err != nil {
		return WrapError(sh.stream.Context(), err, "failed to close stream")
	}
	return nil
}

// Context returns the stream context.
func (sh *StreamHandler) Context() context.Context {
	return sh.stream.Context()
}

// StreamReader implements io.Reader for gRPC streams.
type StreamReader struct {
	stream   Stream
	msgType  proto.Message
	buffer   []byte
	position int
}

// NewStreamReader creates a new stream reader.
func NewStreamReader(stream Stream, msgType proto.Message) *StreamReader {
	return &StreamReader{
		stream:  stream,
		msgType: msgType,
		buffer:  make([]byte, 0),
	}
}

// Read implements io.Reader interface.
func (sr *StreamReader) Read(p []byte) (n int, err error) {
	// If buffer is empty, try to read from stream
	if len(sr.buffer) == 0 {
		if err := sr.readFromStream(); err != nil {
			return 0, err
		}
	}

	// Copy from buffer to p
	n = copy(p, sr.buffer[sr.position:])
	sr.position += n

	// If we've read all the buffer, clear it
	if sr.position >= len(sr.buffer) {
		sr.buffer = sr.buffer[:0]
		sr.position = 0
	}

	return n, nil
}

// readFromStream reads a message from the gRPC stream.
func (sr *StreamReader) readFromStream() error {
	// Create a new message instance
	msg := proto.Clone(sr.msgType)
	if msg == nil {
		return fmt.Errorf("failed to clone message type")
	}

	// Receive message from stream
	_, err := sr.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return io.EOF
		}
		return WrapError(sr.stream.Context(), err, "failed to receive from stream")
	}

	// Marshal message to bytes
	data, err := proto.Marshal(msg)
	if err != nil {
		return WrapError(sr.stream.Context(), err, "failed to marshal message")
	}

	sr.buffer = data
	sr.position = 0
	return nil
}

// StreamWriter implements io.Writer for gRPC streams.
type StreamWriter struct {
	stream  Stream
	msgType proto.Message
}

// NewStreamWriter creates a new stream writer.
func NewStreamWriter(stream Stream, msgType proto.Message) *StreamWriter {
	return &StreamWriter{
		stream:  stream,
		msgType: msgType,
	}
}

// Write implements io.Writer interface.
func (sw *StreamWriter) Write(p []byte) (n int, err error) {
	// Create a new message instance
	msg := proto.Clone(sw.msgType)
	if msg == nil {
		return 0, fmt.Errorf("failed to clone message type")
	}

	// Unmarshal bytes to message
	if err := proto.Unmarshal(p, msg); err != nil {
		return 0, WrapError(sw.stream.Context(), err, "failed to unmarshal message")
	}

	// Send message through stream
	if err := sw.stream.Send(msg); err != nil {
		return 0, WrapError(sw.stream.Context(), err, "failed to send message")
	}

	return len(p), nil
}

// Close closes the stream writer.
func (sw *StreamWriter) Close() error {
	return sw.stream.CloseSend()
}
