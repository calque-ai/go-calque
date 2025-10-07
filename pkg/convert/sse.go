package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"sync"
	"time"
)

// SSEEvent represents a Server-Sent Event.
//
// Defines the structure for SSE messages with optional event type,
// data payload, ID for reconnection, and retry timing.
//
// Example:
//
//	event := SSEEvent{
//		Event: "message",
//		Data: "Hello world",
//		ID: "123",
//	}
type SSEEvent struct {
	Event string `json:"event,omitempty"`
	Data  any    `json:"data"`
	ID    string `json:"id,omitempty"`
	Retry int    `json:"retry,omitempty"`
}

// SSEEventFormatter defines how to format SSE event data.
//
// Input: content string, done flag
// Output: formatted data for SSE event
// Behavior: Transforms content into desired event format
//
// Example:
//
//	formatter := func(content string, done bool) any {
//		return map[string]any{"text": content, "finished": done}
//	}
type SSEEventFormatter func(content string, done bool) any

// SSEChunkMode defines how to chunk the streaming data.
//
// Controls granularity of streaming: by character, word, line, or complete.
// Affects real-time user experience and bandwidth usage.
type SSEChunkMode int

const (
	// SSEChunkByWord streams content word by word (default mode)
	SSEChunkByWord SSEChunkMode = iota // Stream word by word (default)
	// SSEChunkByChar streams content character by character
	SSEChunkByChar // Stream character by character
	// SSEChunkByLine streams content line by line
	SSEChunkByLine // Stream line by line
	// SSEChunkNone streams entire response as single event
	SSEChunkNone // Stream entire response as single event
)

// RawContentFormatter sends content directly without wrapping (default).
//
// Input: content string, done flag (ignored)
// Output: content as-is
// Behavior: Simple pass-through formatter
//
// Returns content without any wrapper structure. Use for simple
// text streaming where clients expect raw content.
//
// Example:
//
//	formatter := convert.RawContentFormatter
//	data := formatter("hello", false) // returns "hello"
func RawContentFormatter(content string, _ bool) any {
	return content
}

// MapEventFormatter creates events as maps with the provided base fields.
//
// Input: base fields map
// Output: SSEEventFormatter function
// Behavior: Returns formatter that wraps content in structured map
//
// Creates structured events with custom fields plus content and done status.
// Useful for clients expecting consistent JSON structure.
//
// Example:
//
//	formatter := convert.MapEventFormatter(map[string]any{
//		"type": "chat", "user_id": "123",
//	})
//	// Results in: {"type": "chat", "user_id": "123", "content": "...", "done": false}
func MapEventFormatter(baseFields map[string]any) SSEEventFormatter {
	return func(content string, done bool) any {
		event := make(map[string]any)

		// Copy base fields
		maps.Copy(event, baseFields)

		// Add content and done status
		event["content"] = content
		event["done"] = done

		return event
	}
}

// ToSSE creates an SSE converter that streams to the given HTTP response writer.
//
// Input: HTTP response writer
// Output: configured SSEConverter
// Behavior: Sets SSE headers and configures streaming
//
// Automatically sets required SSE headers (Content-Type, Cache-Control, etc.)
// and configures default word-by-word chunking with raw content formatting.
//
// Example:
//
//	sse := convert.ToSSE(w)
//	err := sse.FromReader(reader)
func ToSSE(w http.ResponseWriter) *SSEConverter {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		// If no flusher available, we'll still work but without immediate streaming
		flusher = &noopFlusher{}
	}

	return &SSEConverter{
		writer:           w,
		flusher:          flusher,
		chunkBy:          SSEChunkByWord,
		formatter:        RawContentFormatter, // Default: send raw content
		eventFields:      nil,                 // No additional fields by default
		keepAliveEnabled: false,               // Keep-alive disabled by default
	}
}

// SSEConverter streams data as Server-Sent Events to an HTTP response writer.
//
// Handles real-time streaming of data chunks with configurable chunking modes
// and event formatting. Automatically sets SSE headers and manages flushing.
//
// Example:
//
//	sse := convert.ToSSE(w).WithChunkMode(convert.SSEChunkByWord)
//	err := sse.FromReader(dataReader)
type SSEConverter struct {
	writer      http.ResponseWriter
	flusher     http.Flusher
	chunkBy     SSEChunkMode
	formatter   SSEEventFormatter // RawContentFormatter or MapEventFormatter
	eventFields map[string]any    // Additional fields to include in events

	// Keep-alive configuration
	keepAliveInterval time.Duration
	keepAliveEnabled  bool
	keepAliveCancel   context.CancelFunc
	mu                sync.Mutex
}

// Close forcefully terminates the SSE connection and releases resources.
//
// Input: none
// Output: error if cleanup fails
// Behavior: Flushes data, hijacks connection, forces close
//
// Performs aggressive cleanup of SSE streaming connection. First attempts
// to hijack the underlying HTTP connection for immediate termination, then
// falls back to io.Closer if available. Use for force-close scenarios or
// when handlers don't return immediately after streaming.
//
// Note: Most SSE streams close naturally when the HTTP handler returns.
// This method is primarily for edge cases requiring explicit connection
// termination (misbehaving clients, long-running handlers, etc.).
//
// Example:
//
//	defer sse.Close() // Force cleanup on function exit
//	if err := sse.Close(); err != nil {
//		log.Printf("SSE close error: %v", err)
//	}
func (s *SSEConverter) Close() error {
	// Stop keep-alive first
	s.stopKeepAlive()

	// Final flush
	if s.flusher != nil {
		s.flusher.Flush()
	}

	// Try to hijack and close underlying connection first
	if hijacker, ok := s.writer.(http.Hijacker); ok {
		conn, _, err := hijacker.Hijack()
		if err == nil {
			return conn.Close()
		}
	}

	// Fallback: try to close if response writer supports io.Closer
	if s.writer != nil {
		if closer, ok := s.writer.(io.Closer); ok {
			return closer.Close()
		}
	}

	return nil
}

// WithChunkMode sets how the data should be chunked for streaming.
//
// Input: SSEChunkMode enum value
// Output: *SSEConverter for chaining
// Behavior: Configures streaming granularity
//
// Example:
//
//	sse.WithChunkMode(convert.SSEChunkByChar) // Character-by-character
//	sse.WithChunkMode(convert.SSEChunkByLine) // Line-by-line
func (s *SSEConverter) WithChunkMode(mode SSEChunkMode) *SSEConverter {
	s.chunkBy = mode
	return s
}

// WithEventFields sets additional fields to include in each event.
//
// Input: map of additional fields
// Output: *SSEConverter for chaining
// Behavior: Switches to map formatter with custom fields
//
// Automatically switches from raw content to structured map format
// with the provided base fields plus content and done status.
//
// Example:
//
//	sse.WithEventFields(map[string]any{
//		"stream_id": "abc123",
//		"model": "gpt-4",
//	})
func (s *SSEConverter) WithEventFields(fields map[string]any) *SSEConverter {
	s.eventFields = fields
	s.formatter = MapEventFormatter(fields)
	return s
}

// WithKeepAlive enables periodic keep-alive messages to prevent connection timeouts.
//
// Input: keep-alive interval (recommended: 30 * time.Second)
// Output: *SSEConverter for chaining
// Behavior: Sends periodic comment messages to maintain connection
//
// Keep-alive messages are sent as SSE comments (": keep-alive\n\n") which
// are ignored by clients but prevent proxy/firewall timeouts. Common
// interval is 30 seconds to handle most proxy timeout configurations.
//
// Example:
//
//	sse.WithKeepAlive(30 * time.Second) // Recommended interval
//	sse.WithKeepAlive(15 * time.Second) // More frequent
func (s *SSEConverter) WithKeepAlive(interval time.Duration) *SSEConverter {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.keepAliveEnabled = true
	s.keepAliveInterval = interval
	return s
}

// FromReader implements OutputConverter interface for streaming SSE responses.
//
// Input: io.Reader data source
// Output: error if streaming fails
// Behavior: STREAMING - reads and sends data in real-time chunks
//
// Streams data according to configured chunk mode, sending SSE events
// as data arrives. Handles completion and error events automatically.
// Starts keep-alive if enabled.
//
// Example:
//
//	err := sse.FromReader(llmResponseStream)
func (s *SSEConverter) FromReader(reader io.Reader) error {
	// Start keep-alive if enabled
	if s.keepAliveEnabled {
		s.startKeepAlive()
		defer s.stopKeepAlive()
	}

	switch s.chunkBy {
	case SSEChunkByWord:
		return s.streamByWord(reader)
	case SSEChunkByChar:
		return s.streamByChar(reader)
	case SSEChunkByLine:
		return s.streamByLine(reader)
	case SSEChunkNone:
		return s.streamComplete(reader)
	default:
		return s.streamByWord(reader)
	}
}

// startKeepAlive starts the keep-alive goroutine
func (s *SSEConverter) startKeepAlive() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.keepAliveEnabled || s.keepAliveCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.keepAliveCancel = cancel

	go func() {
		ticker := time.NewTicker(s.keepAliveInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.sendKeepAlive(); err != nil {
					// Keep-alive failed, likely connection is broken
					return
				}
			}
		}
	}()
}

// sendKeepAlive safely sends a keep-alive message with proper locking
func (s *SSEConverter) sendKeepAlive() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send keep-alive as SSE comment
	if _, err := fmt.Fprintf(s.writer, ": keep-alive\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// stopKeepAlive stops the keep-alive goroutine
func (s *SSEConverter) stopKeepAlive() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.keepAliveCancel != nil {
		s.keepAliveCancel()
		s.keepAliveCancel = nil
	}
}

// streamByWord streams content word by word
func (s *SSEConverter) streamByWord(reader io.Reader) error {
	buffer := make([]byte, 1)
	var currentWord []byte

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if s.processWordChar(buffer[0], &currentWord) {
				return s.sendError(fmt.Errorf("failed to process word character"))
			}
		}

		if err == io.EOF {
			return s.handleWordEOF(currentWord)
		}

		if err != nil {
			return s.sendError(err)
		}
	}
}

// processWordChar processes a single character for word streaming
func (s *SSEConverter) processWordChar(char byte, currentWord *[]byte) bool {
	// Word boundary on space, newline, or tab
	if char == ' ' || char == '\n' || char == '\t' {
		return s.handleWhitespace(char, currentWord)
	}
	*currentWord = append(*currentWord, char)
	return false
}

// handleWhitespace processes whitespace characters for word streaming
func (s *SSEConverter) handleWhitespace(char byte, currentWord *[]byte) bool {
	if len(*currentWord) > 0 {
		// Send word with its delimiter
		if sendErr := s.sendChunk(string(*currentWord) + string(char)); sendErr != nil {
			return true
		}
		*currentWord = (*currentWord)[:0]
		return false
	}

	// Send standalone whitespace character (handles consecutive/leading whitespace)
	return s.sendChunk(string(char)) != nil
}

// handleWordEOF handles end-of-file for word streaming
func (s *SSEConverter) handleWordEOF(currentWord []byte) error {
	// Send any remaining word
	if len(currentWord) > 0 {
		if sendErr := s.sendChunk(string(currentWord)); sendErr != nil {
			return sendErr
		}
	}
	return s.sendCompletion()
}

// streamByChar streams content character by character
func (s *SSEConverter) streamByChar(reader io.Reader) error {
	buffer := make([]byte, 1)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if sendErr := s.sendChunk(string(buffer[0])); sendErr != nil {
				return sendErr
			}
		}

		if err == io.EOF {
			return s.sendCompletion()
		}

		if err != nil {
			return s.sendError(err)
		}
	}
}

// streamByLine streams content line by line
func (s *SSEConverter) streamByLine(reader io.Reader) error {
	buffer := make([]byte, 1)
	var currentLine []byte

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			char := buffer[0]
			currentLine = append(currentLine, char)

			if char == '\n' {
				if sendErr := s.sendChunk(string(currentLine)); sendErr != nil {
					return sendErr
				}
				currentLine = currentLine[:0] // Reset slice
			}
		}

		if err == io.EOF {
			// Send any remaining content
			if len(currentLine) > 0 {
				if sendErr := s.sendChunk(string(currentLine)); sendErr != nil {
					return sendErr
				}
			}
			return s.sendCompletion()
		}

		if err != nil {
			return s.sendError(err)
		}
	}
}

// streamComplete sends the entire content as a single event
func (s *SSEConverter) streamComplete(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return s.sendError(err)
	}

	if sendErr := s.sendChunk(string(data)); sendErr != nil {
		return sendErr
	}

	return s.sendCompletion()
}

// sendChunk sends a data chunk as an SSE event
func (s *SSEConverter) sendChunk(content string) error {
	eventData := s.formatter(content, false)

	if err := s.writeSSEEvent("message", eventData); err != nil {
		return err
	}

	return nil
}

// sendCompletion sends the completion event
func (s *SSEConverter) sendCompletion() error {
	eventData := s.formatter("", true)
	return s.writeSSEEvent("completion", eventData)
}

// sendError sends an error event (always uses simple format for errors)
func (s *SSEConverter) sendError(err error) error {
	event := map[string]any{
		"error": err.Error(),
	}

	return s.writeSSEEvent("error", event)
}

// WriteError sends an error event (public method for external use).
//
// Input: error to send
// Output: error if writing fails
// Behavior: Sends structured error event to client
//
// Example:
//
//	if err := processData(); err != nil {
//		sse.WriteError(err)
//	}
func (s *SSEConverter) WriteError(err error) error {
	return s.sendError(err)
}

// writeSSEEvent writes an SSE event to the response writer
func (s *SSEConverter) writeSSEEvent(event string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal SSE data: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", event, jsonData)
	if err != nil {
		return fmt.Errorf("failed to write SSE event: %w", err)
	}

	s.flusher.Flush()
	return nil
}

// noopFlusher is a no-op implementation of http.Flusher
type noopFlusher struct{}

func (nf *noopFlusher) Flush() {
	// No-op
}
