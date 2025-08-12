package convert

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string `json:"event,omitempty"`
	Data  any    `json:"data"`
	ID    string `json:"id,omitempty"`
	Retry int    `json:"retry,omitempty"`
}

// SSEEventFormatter defines how to format SSE event data
type SSEEventFormatter func(content string, done bool) any

// SSEConverter streams data as Server-Sent Events to an HTTP response writer
type SSEConverter struct {
	writer      http.ResponseWriter
	flusher     http.Flusher
	chunkBy     SSEChunkMode
	formatter   SSEEventFormatter // RawContentFormatter or MapEventFormatter
	eventFields map[string]any    // Additional fields to include in events
}

// SSEChunkMode defines how to chunk the streaming data
type SSEChunkMode int

const (
	SSEChunkByWord SSEChunkMode = iota // Stream word by word (default)
	SSEChunkByChar                     // Stream character by character
	SSEChunkByLine                     // Stream line by line
	SSEChunkNone                       // Stream entire response as single event
)

// RawContentFormatter sends content directly without wrapping (default)
func RawContentFormatter(content string, done bool) any {
	return content
}

// MapEventFormatter creates events as maps with the provided base fields
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

// ToSSE creates an SSE converter that streams to the given HTTP response writer
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
		writer:      w,
		flusher:     flusher,
		chunkBy:     SSEChunkByWord,
		formatter:   RawContentFormatter, // Default: send raw content
		eventFields: nil,                 // No additional fields by default
	}
}

// WithChunkMode sets how the data should be chunked for streaming
func (s *SSEConverter) WithChunkMode(mode SSEChunkMode) *SSEConverter {
	s.chunkBy = mode
	return s
}

// WithEventFields sets additional fields to include in each event
func (s *SSEConverter) WithEventFields(fields map[string]any) *SSEConverter {
	s.eventFields = fields
	s.formatter = MapEventFormatter(fields)
	return s
}

// FromReader implements OutputConverter interface for streaming SSE responses
func (s *SSEConverter) FromReader(reader io.Reader) error {
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

// streamByWord streams content word by word
func (s *SSEConverter) streamByWord(reader io.Reader) error {
	buffer := make([]byte, 1)
	var currentWord []byte

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			char := buffer[0]

			// If we hit a delimiter, send the current word
			if char == ' ' || char == '\n' || char == '\t' {
				if len(currentWord) > 0 {
					if sendErr := s.sendChunk(string(currentWord) + " "); sendErr != nil {
						return sendErr
					}
					currentWord = currentWord[:0] // Reset slice
				}
			} else {
				currentWord = append(currentWord, char)
			}
		}

		if err == io.EOF {
			// Send any remaining word
			if len(currentWord) > 0 {
				if sendErr := s.sendChunk(string(currentWord)); sendErr != nil {
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

// WriteError sends an error event (public method for external use)
func (s *SSEConverter) WriteError(err error) error {
	return s.sendError(err)
}

// writeSSEEvent writes an SSE event to the response writer
func (s *SSEConverter) writeSSEEvent(event string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal SSE data: %w", err)
	}

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
