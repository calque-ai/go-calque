package convert

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testEvent      = "message"
	testCompletion = "completion"
	testError      = "error"
)

// Mock implementations for testing
type mockFlusher struct {
	flushed int
	mu      sync.Mutex
}

func (m *mockFlusher) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushed++
}

func (m *mockFlusher) FlushCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushed
}

type mockResponseWriter struct {
	*httptest.ResponseRecorder
	flusher *mockFlusher
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		flusher:          &mockFlusher{},
	}
}

func (m *mockResponseWriter) Flush() {
	m.flusher.Flush()
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, e.err
}

type sseSlowReader struct {
	data []byte
	pos  int
}

func (s *sseSlowReader) Read(p []byte) (n int, err error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	if len(p) > 0 && s.pos < len(s.data) {
		p[0] = s.data[s.pos]
		s.pos++
		return 1, nil
	}
	return 0, io.EOF
}

func TestSSEEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    SSEEvent
		expected map[string]any
	}{
		{
			name: "complete event",
			event: SSEEvent{
				Event: "message",
				Data:  "test data",
				ID:    "123",
				Retry: 5000,
			},
			expected: map[string]any{
				"event": "message",
				"data":  "test data",
				"id":    "123",
				"retry": 5000,
			},
		},
		{
			name: "minimal event",
			event: SSEEvent{
				Data: "only data",
			},
			expected: map[string]any{
				"data": "only data",
			},
		},
		{
			name: "event with complex data",
			event: SSEEvent{
				Event: "update",
				Data: map[string]any{
					"user":    "alice",
					"message": "hello",
					"count":   42,
				},
			},
			expected: map[string]any{
				"event": "update",
				"data": map[string]any{
					"user":    "alice",
					"message": "hello",
					"count":   42,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventBytes, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}

			var actual map[string]any
			if err := json.Unmarshal(eventBytes, &actual); err != nil {
				t.Fatalf("Failed to unmarshal event: %v", err)
			}

			verifyEventFields(t, actual, tt.expected)
		})
	}
}

func verifyEventFields(t *testing.T, actual, expected map[string]any) {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("Expected field %s not found", key)
			continue
		}

		switch ev := expectedValue.(type) {
		case map[string]any:
			av, ok := actualValue.(map[string]any)
			if !ok {
				t.Errorf("Field %s: expected map, got %T", key, actualValue)
				continue
			}
			verifyEventFields(t, av, ev)
		default:
			// JSON unmarshaling converts numbers to float64
			if actualValue != expectedValue {
				// Allow for JSON number conversion
				if expectedFloat, ok := expectedValue.(int); ok {
					if actualFloat, ok := actualValue.(float64); ok && float64(expectedFloat) == actualFloat {
						continue
					}
				}
				t.Errorf("Field %s: expected %v, got %v", key, expectedValue, actualValue)
			}
		}
	}
}

func TestSSEChunkMode(t *testing.T) {
	tests := []struct {
		name string
		mode SSEChunkMode
		want int
	}{
		{"word mode", SSEChunkByWord, 0},
		{"char mode", SSEChunkByChar, 1},
		{"line mode", SSEChunkByLine, 2},
		{"none mode", SSEChunkNone, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.mode) != tt.want {
				t.Errorf("SSEChunkMode %s = %d, want %d", tt.name, int(tt.mode), tt.want)
			}
		})
	}
}

func TestRawContentFormatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		done     bool
		expected any
	}{
		{"simple content", "hello world", false, "hello world"},
		{"empty content", "", false, ""},
		{"done flag ignored", "content", true, "content"},
		{"special characters", "hello\nworld\t!", false, "hello\nworld\t!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RawContentFormatter(tt.content, tt.done)
			if result != tt.expected {
				t.Errorf("RawContentFormatter(%q, %v) = %v, want %v", tt.content, tt.done, result, tt.expected)
			}
		})
	}
}

func TestMapEventFormatter(t *testing.T) {
	tests := []struct {
		name       string
		baseFields map[string]any
		content    string
		done       bool
		expected   map[string]any
	}{
		{
			name:       "empty base fields",
			baseFields: map[string]any{},
			content:    "test content",
			done:       false,
			expected: map[string]any{
				"content": "test content",
				"done":    false,
			},
		},
		{
			name: "with base fields",
			baseFields: map[string]any{
				"type":    "chat",
				"user_id": "123",
			},
			content: "hello",
			done:    true,
			expected: map[string]any{
				"type":    "chat",
				"user_id": "123",
				"content": "hello",
				"done":    true,
			},
		},
		{
			name: "base fields override",
			baseFields: map[string]any{
				"content": "base content",
				"extra":   "field",
			},
			content: "new content",
			done:    false,
			expected: map[string]any{
				"content": "new content",
				"extra":   "field",
				"done":    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := MapEventFormatter(tt.baseFields)
			result := formatter(tt.content, tt.done)

			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("Expected map[string]any, got %T", result)
			}

			verifyEventFields(t, resultMap, tt.expected)
		})
	}
}

func TestToSSE(t *testing.T) {
	tests := []struct {
		name        string
		setupWriter func() (http.ResponseWriter, *mockFlusher)
		verifyFunc  func(t *testing.T, w http.ResponseWriter, flusher *mockFlusher, sse *SSEConverter)
	}{
		{
			name: "standard response writer",
			setupWriter: func() (http.ResponseWriter, *mockFlusher) {
				mock := newMockResponseWriter()
				return mock, mock.flusher
			},
			verifyFunc: func(t *testing.T, w http.ResponseWriter, _ *mockFlusher, sse *SSEConverter) {
				verifySSEHeaders(t, w.Header())
				if sse.chunkBy != SSEChunkByWord {
					t.Errorf("Expected default chunk mode SSEChunkByWord, got %v", sse.chunkBy)
				}
			},
		},
		{
			name: "response writer without flusher",
			setupWriter: func() (http.ResponseWriter, *mockFlusher) {
				recorder := httptest.NewRecorder()
				return recorder, nil
			},
			verifyFunc: func(t *testing.T, w http.ResponseWriter, _ *mockFlusher, sse *SSEConverter) {
				verifySSEHeaders(t, w.Header())
				// Should work even without flusher
				if sse.flusher == nil {
					t.Error("Expected non-nil flusher (should use noopFlusher)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, flusher := tt.setupWriter()
			sse := ToSSE(w)

			if sse == nil {
				t.Fatal("ToSSE() returned nil")
			}

			tt.verifyFunc(t, w, flusher, sse)
		})
	}
}

func verifySSEHeaders(t *testing.T, headers http.Header) {
	expectedHeaders := map[string]string{
		"Content-Type":                "text/event-stream",
		"Cache-Control":               "no-cache",
		"Connection":                  "keep-alive",
		"Access-Control-Allow-Origin": "*",
	}

	for key, expected := range expectedHeaders {
		actual := headers.Get(key)
		if actual != expected {
			t.Errorf("Header %s = %q, want %q", key, actual, expected)
		}
	}
}

func TestSSEConverter_WithChunkMode(t *testing.T) {
	tests := []struct {
		name string
		mode SSEChunkMode
	}{
		{"word mode", SSEChunkByWord},
		{"char mode", SSEChunkByChar},
		{"line mode", SSEChunkByLine},
		{"none mode", SSEChunkNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			sse := ToSSE(mock).WithChunkMode(tt.mode)

			if sse.chunkBy != tt.mode {
				t.Errorf("WithChunkMode() chunk mode = %v, want %v", sse.chunkBy, tt.mode)
			}

			// Verify method chaining
			sse2 := sse.WithChunkMode(SSEChunkByChar)
			if sse != sse2 {
				t.Error("WithChunkMode() should return same instance for chaining")
			}
		})
	}
}

func TestSSEConverter_WithEventFields(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]any
	}{
		{
			name:   "nil fields",
			fields: nil,
		},
		{
			name:   "empty fields",
			fields: map[string]any{},
		},
		{
			name: "single field",
			fields: map[string]any{
				"stream_id": "abc123",
			},
		},
		{
			name: "multiple fields",
			fields: map[string]any{
				"stream_id": "abc123",
				"model":     "gpt-4",
				"user_id":   42,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			sse := ToSSE(mock).WithEventFields(tt.fields)

			if len(tt.fields) == 0 && sse.eventFields != nil && len(sse.eventFields) != 0 {
				t.Errorf("WithEventFields() with empty fields should result in empty eventFields")
			}

			// Test the formatter was updated
			if len(tt.fields) > 0 {
				result := sse.formatter("test", false)
				resultMap, ok := result.(map[string]any)
				if !ok {
					t.Fatalf("Expected map[string]any, got %T", result)
				}

				// Should contain base fields plus content and done
				for key, value := range tt.fields {
					if resultMap[key] != value {
						t.Errorf("Field %s = %v, want %v", key, resultMap[key], value)
					}
				}

				if resultMap["content"] != "test" {
					t.Errorf("content = %v, want test", resultMap["content"])
				}
				if resultMap["done"] != false {
					t.Errorf("done = %v, want false", resultMap["done"])
				}
			}

			// Verify method chaining
			sse2 := sse.WithEventFields(map[string]any{"test": "value"})
			if sse != sse2 {
				t.Error("WithEventFields() should return same instance for chaining")
			}
		})
	}
}

func TestSSEConverter_FromReader_ChunkModes(t *testing.T) {
	tests := []struct {
		name       string
		mode       SSEChunkMode
		input      string
		verifyFunc func(t *testing.T, events []SSEEventCapture, input string)
	}{
		{
			name:  "chunk by word",
			mode:  SSEChunkByWord,
			input: "hello world test",
			verifyFunc: func(t *testing.T, events []SSEEventCapture, _ string) {
				// Should have events for each word + completion
				expectedChunks := []string{"hello ", "world ", "test"}
				verifyWordChunks(t, events, expectedChunks)
			},
		},
		{
			name:  "chunk by char",
			mode:  SSEChunkByChar,
			input: "abc",
			verifyFunc: func(t *testing.T, events []SSEEventCapture, _ string) {
				// Should have events for each character + completion
				expectedChunks := []string{"a", "b", "c"}
				verifyCharChunks(t, events, expectedChunks)
			},
		},
		{
			name:  "chunk by line",
			mode:  SSEChunkByLine,
			input: "line1\nline2\nline3",
			verifyFunc: func(t *testing.T, events []SSEEventCapture, _ string) {
				// Should have events for each line + completion
				expectedChunks := []string{"line1\n", "line2\n", "line3"}
				verifyLineChunks(t, events, expectedChunks)
			},
		},
		{
			name:  "chunk none",
			mode:  SSEChunkNone,
			input: "complete message here",
			verifyFunc: func(t *testing.T, events []SSEEventCapture, input string) {
				// Should have single message event + completion
				if len(events) != 2 {
					t.Errorf("Expected 2 events (message + completion), got %d", len(events))
					return
				}
				if events[0].Event != testEvent || events[0].Data != input {
					t.Errorf("Expected complete message, got event=%s data=%s", events[0].Event, events[0].Data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			sse := ToSSE(mock).WithChunkMode(tt.mode)

			reader := strings.NewReader(tt.input)
			err := sse.FromReader(reader)

			if err != nil {
				t.Fatalf("FromReader() error = %v", err)
			}

			events := parseSSEEvents(t, mock.Body.String())
			tt.verifyFunc(t, events, tt.input)

			// Verify completion event
			if len(events) == 0 {
				t.Fatal("No events captured")
			}
			lastEvent := events[len(events)-1]
			if lastEvent.Event != testCompletion {
				t.Errorf("Last event should be completion, got %s", lastEvent.Event)
			}
		})
	}
}

type SSEEventCapture struct {
	Event string
	Data  string
}

func parseSSEEvents(t *testing.T, sseData string) []SSEEventCapture {
	var events []SSEEventCapture
	lines := strings.Split(sseData, "\n")

	var currentEvent SSEEventCapture
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "event: "):
			currentEvent.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			dataJSON := strings.TrimPrefix(line, "data: ")
			currentEvent.Data = extractDataFromJSON(t, dataJSON)
		case line == "" && currentEvent.Event != "":
			events = append(events, currentEvent)
			currentEvent = SSEEventCapture{}
		}
	}

	return events
}

func extractDataFromJSON(t *testing.T, dataJSON string) string {
	// For raw content formatter, data is just a JSON string
	// For map formatter, data is an object with content field
	var data any
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		t.Fatalf("Failed to parse SSE data JSON: %v", err)
	}

	switch d := data.(type) {
	case string:
		return d
	case map[string]any:
		if content, ok := d["content"].(string); ok {
			return content
		}
		return fmt.Sprintf("%v", d)
	default:
		return fmt.Sprintf("%v", d)
	}
}

func verifyWordChunks(t *testing.T, events []SSEEventCapture, expectedChunks []string) {
	messageEvents := filterMessageEvents(events)
	if len(messageEvents) != len(expectedChunks) {
		t.Errorf("Expected %d message events, got %d", len(expectedChunks), len(messageEvents))
		return
	}

	for i, expected := range expectedChunks {
		if messageEvents[i].Data != expected {
			t.Errorf("Event %d: expected %q, got %q", i, expected, messageEvents[i].Data)
		}
	}
}

func verifyCharChunks(t *testing.T, events []SSEEventCapture, expectedChunks []string) {
	messageEvents := filterMessageEvents(events)
	if len(messageEvents) != len(expectedChunks) {
		t.Errorf("Expected %d message events, got %d", len(expectedChunks), len(messageEvents))
		return
	}

	for i, expected := range expectedChunks {
		if messageEvents[i].Data != expected {
			t.Errorf("Event %d: expected %q, got %q", i, expected, messageEvents[i].Data)
		}
	}
}

func verifyLineChunks(t *testing.T, events []SSEEventCapture, expectedChunks []string) {
	messageEvents := filterMessageEvents(events)
	if len(messageEvents) != len(expectedChunks) {
		t.Errorf("Expected %d message events, got %d", len(expectedChunks), len(messageEvents))
		return
	}

	for i, expected := range expectedChunks {
		if messageEvents[i].Data != expected {
			t.Errorf("Event %d: expected %q, got %q", i, expected, messageEvents[i].Data)
		}
	}
}

func filterMessageEvents(events []SSEEventCapture) []SSEEventCapture {
	var messageEvents []SSEEventCapture
	for _, event := range events {
		if event.Event == testEvent {
			messageEvents = append(messageEvents, event)
		}
	}
	return messageEvents
}

func TestSSEConverter_FromReader_ErrorCases(t *testing.T) {
	tests := []struct {
		name         string
		reader       io.Reader
		mode         SSEChunkMode
		expectError  bool
		verifyEvents func(t *testing.T, events []SSEEventCapture)
	}{
		{
			name:        "reader error",
			reader:      &errorReader{err: errors.New("read failed")},
			mode:        SSEChunkByWord,
			expectError: false, // s.sendError() sends event but returns nil (unless writeSSEEvent fails)
			verifyEvents: func(t *testing.T, events []SSEEventCapture) {
				// Reader errors should send SSE error event
				if len(events) == 0 {
					t.Error("Expected error event to be sent")
					return
				}
				lastEvent := events[len(events)-1]
				if lastEvent.Event != testError {
					t.Errorf("Expected error event, got %s", lastEvent.Event)
				}
			},
		},
		{
			name:        "slow reader with valid data",
			reader:      &sseSlowReader{data: []byte("slow data")},
			mode:        SSEChunkByChar,
			expectError: false,
			verifyEvents: func(t *testing.T, events []SSEEventCapture) {
				messageEvents := filterMessageEvents(events)
				expected := []string{"s", "l", "o", "w", " ", "d", "a", "t", "a"}
				if len(messageEvents) != len(expected) {
					t.Errorf("Expected %d events, got %d", len(expected), len(messageEvents))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			sse := ToSSE(mock).WithChunkMode(tt.mode)

			err := sse.FromReader(tt.reader)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			events := parseSSEEvents(t, mock.Body.String())
			if tt.verifyEvents != nil {
				tt.verifyEvents(t, events)
			}
		})
	}
}

func TestSSEConverter_WriteError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"simple error", errors.New("test error")},
		{"formatted error", fmt.Errorf("formatted error: %w", errors.New("wrapped"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			sse := ToSSE(mock)

			err := sse.WriteError(tt.err)
			if err != nil {
				t.Fatalf("WriteError() failed: %v", err)
			}

			events := parseSSEEvents(t, mock.Body.String())
			if len(events) != 1 {
				t.Fatalf("Expected 1 error event, got %d", len(events))
			}

			event := events[0]
			if event.Event != testError {
				t.Errorf("Expected error event, got %s", event.Event)
			}

			// Parse error data
			var errorData map[string]any
			dataJSON := strings.TrimPrefix(strings.Split(mock.Body.String(), "\n")[1], "data: ")
			if err := json.Unmarshal([]byte(dataJSON), &errorData); err != nil {
				t.Fatalf("Failed to parse error data: %v", err)
			}

			expectedError := tt.err.Error()

			if errorData["error"] != expectedError {
				t.Errorf("Error message = %v, want %s", errorData["error"], expectedError)
			}
		})
	}
}

func TestSSEConverter_FlushBehavior(t *testing.T) {
	mock := newMockResponseWriter()
	sse := ToSSE(mock)

	reader := strings.NewReader("test data")
	err := sse.FromReader(reader)

	if err != nil {
		t.Fatalf("FromReader() error = %v", err)
	}

	// Should have flushed for each event (message + completion)
	if mock.flusher.FlushCount() < 2 {
		t.Errorf("Expected at least 2 flushes, got %d", mock.flusher.FlushCount())
	}
}

func TestSSEConverter_Integration_WithEventFields(t *testing.T) {
	mock := newMockResponseWriter()
	baseFields := map[string]any{
		"stream_id": "test-stream",
		"model":     "test-model",
	}

	sse := ToSSE(mock).
		WithEventFields(baseFields).
		WithChunkMode(SSEChunkByWord)

	reader := strings.NewReader("hello world")
	err := sse.FromReader(reader)

	if err != nil {
		t.Fatalf("FromReader() error = %v", err)
	}

	// Parse and verify events have the base fields
	lines := strings.Split(mock.Body.String(), "\n")
	var messageFound bool

	for i, line := range lines {
		if strings.HasPrefix(line, "data: ") && i > 0 && strings.Contains(lines[i-1], "event: message") {
			dataJSON := strings.TrimPrefix(line, "data: ")
			var eventData map[string]any
			if err := json.Unmarshal([]byte(dataJSON), &eventData); err != nil {
				t.Fatalf("Failed to parse event data: %v", err)
			}

			// Verify base fields are present
			for key, expected := range baseFields {
				if actual, ok := eventData[key]; !ok || actual != expected {
					t.Errorf("Field %s = %v, want %v", key, actual, expected)
				}
			}

			// Verify content and done fields
			if _, ok := eventData["content"]; !ok {
				t.Error("Missing content field")
			}
			if _, ok := eventData["done"]; !ok {
				t.Error("Missing done field")
			}

			messageFound = true
			break
		}
	}

	if !messageFound {
		t.Error("No message event found to verify")
	}
}

func TestSSEConverter_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		mode   SSEChunkMode
		verify func(t *testing.T, events []SSEEventCapture, input string)
	}{
		{
			name:  "empty input",
			input: "",
			mode:  SSEChunkByWord,
			verify: func(t *testing.T, events []SSEEventCapture, _ string) {
				// Should only have completion event
				if len(events) != 1 {
					t.Errorf("Expected 1 event (completion), got %d", len(events))
					return
				}
				if events[0].Event != testCompletion {
					t.Errorf("Expected completion event, got %s", events[0].Event)
				}
			},
		},
		{
			name:  "only whitespace",
			input: "   \n\t  ",
			mode:  SSEChunkByWord,
			verify: func(t *testing.T, events []SSEEventCapture, _ string) {
				// Should only have completion event (no words)
				completionEvents := 0
				for _, event := range events {
					if event.Event == testCompletion {
						completionEvents++
					}
				}
				if completionEvents != 1 {
					t.Errorf("Expected 1 completion event, got %d", completionEvents)
				}
			},
		},
		{
			name:  "single character",
			input: "x",
			mode:  SSEChunkByChar,
			verify: func(t *testing.T, events []SSEEventCapture, _ string) {
				messageEvents := filterMessageEvents(events)
				if len(messageEvents) != 1 {
					t.Errorf("Expected 1 message event, got %d", len(messageEvents))
					return
				}
				if messageEvents[0].Data != "x" {
					t.Errorf("Expected 'x', got %q", messageEvents[0].Data)
				}
			},
		},
		{
			name:  "no newline at end",
			input: "line1\nline2",
			mode:  SSEChunkByLine,
			verify: func(t *testing.T, events []SSEEventCapture, _ string) {
				messageEvents := filterMessageEvents(events)
				if len(messageEvents) != 2 {
					t.Errorf("Expected 2 message events, got %d", len(messageEvents))
					return
				}
				if messageEvents[0].Data != "line1\n" {
					t.Errorf("First event: expected 'line1\\n', got %q", messageEvents[0].Data)
				}
				if messageEvents[1].Data != "line2" {
					t.Errorf("Second event: expected 'line2', got %q", messageEvents[1].Data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			sse := ToSSE(mock).WithChunkMode(tt.mode)

			reader := strings.NewReader(tt.input)
			err := sse.FromReader(reader)

			if err != nil {
				t.Fatalf("FromReader() error = %v", err)
			}

			events := parseSSEEvents(t, mock.Body.String())
			tt.verify(t, events, tt.input)
		})
	}
}

func TestSSEConverter_ConcurrentAccess(t *testing.T) {
	// Test concurrent access by creating separate SSE converters
	const numGoroutines = 10
	const numWrites = 5

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numWrites)
	eventChan := make(chan int, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine gets its own mock writer and SSE converter
			mock := newMockResponseWriter()
			sse := ToSSE(mock)

			for j := range numWrites {
				err := sse.WriteError(fmt.Errorf("error %d-%d", id, j))
				if err != nil {
					errChan <- err
				}
			}

			// Count events from this goroutine
			events := parseSSEEvents(t, mock.Body.String())
			eventChan <- len(events)
		}(i)
	}

	wg.Wait()
	close(errChan)
	close(eventChan)

	// Check for any errors
	for err := range errChan {
		t.Errorf("Concurrent access error: %v", err)
	}

	// Verify each goroutine produced the expected number of events
	totalEvents := 0
	for eventCount := range eventChan {
		if eventCount != numWrites {
			t.Errorf("Expected %d error events per goroutine, got %d", numWrites, eventCount)
		}
		totalEvents += eventCount
	}

	expectedTotal := numGoroutines * numWrites
	if totalEvents != expectedTotal {
		t.Errorf("Expected total %d error events, got %d", expectedTotal, totalEvents)
	}
}

func TestNoopFlusher(_ *testing.T) {
	flusher := &noopFlusher{}
	// Should not panic
	flusher.Flush()
}

// Benchmark tests
func BenchmarkSSEConverter_ChunkByWord(b *testing.B) {
	input := strings.Repeat("word ", 1000)

	for b.Loop() {
		mock := newMockResponseWriter()
		sse := ToSSE(mock).WithChunkMode(SSEChunkByWord)
		reader := strings.NewReader(input)
		sse.FromReader(reader)
	}
}

func BenchmarkSSEConverter_ChunkByChar(b *testing.B) {
	input := strings.Repeat("x", 1000)

	for b.Loop() {
		mock := newMockResponseWriter()
		sse := ToSSE(mock).WithChunkMode(SSEChunkByChar)
		reader := strings.NewReader(input)
		sse.FromReader(reader)
	}
}

func BenchmarkSSEConverter_ChunkNone(b *testing.B) {
	input := strings.Repeat("large content block ", 1000)

	for b.Loop() {
		mock := newMockResponseWriter()
		sse := ToSSE(mock).WithChunkMode(SSEChunkNone)
		reader := strings.NewReader(input)
		sse.FromReader(reader)
	}
}

func TestSSEConverter_Close(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *SSEConverter
		wantErr  bool
		verifyFn func(t *testing.T, mock *mockResponseWriter, sse *SSEConverter)
	}{
		{
			name: "close with flusher",
			setup: func() *SSEConverter {
				mock := newMockResponseWriter()
				return ToSSE(mock)
			},
			wantErr: false,
			verifyFn: func(t *testing.T, mock *mockResponseWriter, _ *SSEConverter) {
				// Should have flushed before close
				if mock.flusher.FlushCount() == 0 {
					t.Error("Expected flush to be called during close")
				}
			},
		},
		{
			name: "close without flusher",
			setup: func() *SSEConverter {
				recorder := httptest.NewRecorder()
				return ToSSE(recorder)
			},
			wantErr: false,
			verifyFn: func(_ *testing.T, _ *mockResponseWriter, _ *SSEConverter) {
				// Should not panic even without real flusher
			},
		},
		{
			name: "multiple close calls",
			setup: func() *SSEConverter {
				mock := newMockResponseWriter()
				return ToSSE(mock)
			},
			wantErr: false,
			verifyFn: func(t *testing.T, mock *mockResponseWriter, sse *SSEConverter) {
				// First close
				if err := sse.Close(); err != nil {
					t.Errorf("First close failed: %v", err)
				}
				initialFlushCount := mock.flusher.FlushCount()

				// Second close should be safe
				if err := sse.Close(); err != nil {
					t.Errorf("Second close failed: %v", err)
				}

				// Should have flushed again
				if mock.flusher.FlushCount() <= initialFlushCount {
					t.Error("Expected additional flush on second close")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sse := tt.setup()
			var mock *mockResponseWriter

			// Extract mock if available for verification
			if sse != nil {
				if mockWriter, ok := sse.writer.(*mockResponseWriter); ok {
					mock = mockWriter
				}
			}

			err := sse.Close()

			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.verifyFn != nil {
				tt.verifyFn(t, mock, sse)
			}
		})
	}
}

// mockClosableWriter implements both http.ResponseWriter and io.Closer
type mockClosableWriter struct {
	*mockResponseWriter
	closed     bool
	closeError error
}

func (m *mockClosableWriter) Close() error {
	m.closed = true
	return m.closeError
}

// mockHijackableWriter implements http.ResponseWriter and http.Hijacker
type mockHijackableWriter struct {
	*mockResponseWriter
	hijacked    bool
	hijackError error
	mockConn    *mockConn
}

func (m *mockHijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if m.hijackError != nil {
		return nil, nil, m.hijackError
	}
	m.hijacked = true
	return m.mockConn, nil, nil
}

// mockConn implements net.Conn for testing
type mockConn struct {
	closed     bool
	closeError error
}

func (m *mockConn) Read(_ []byte) (n int, err error)  { return 0, io.EOF }
func (m *mockConn) Write(b []byte) (n int, err error) { return len(b), nil }
func (m *mockConn) Close() error {
	m.closed = true
	return m.closeError
}
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }

func TestSSEConverter_Close_WithClosableWriter(t *testing.T) {
	tests := []struct {
		name       string
		closeError error
		wantErr    bool
	}{
		{
			name:       "successful close",
			closeError: nil,
			wantErr:    false,
		},
		{
			name:       "close with error",
			closeError: errors.New("close failed"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := &mockClosableWriter{
				mockResponseWriter: newMockResponseWriter(),
				closeError:         tt.closeError,
			}

			// Manually create SSE converter with closable writer
			sse := &SSEConverter{
				writer:      mockWriter,
				flusher:     mockWriter.flusher,
				chunkBy:     SSEChunkByWord,
				formatter:   RawContentFormatter,
				eventFields: nil,
			}

			err := sse.Close()

			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify writer was closed
			if !mockWriter.closed {
				t.Error("Expected writer to be closed")
			}

			// Verify flush was called
			if mockWriter.flusher.FlushCount() == 0 {
				t.Error("Expected flush to be called before close")
			}
		})
	}
}

func verifySuccessfulHijack(t *testing.T, mockWriter *mockHijackableWriter, mockConn *mockConn, connError error) {
	// Hijack should have been called
	if !mockWriter.hijacked {
		t.Error("Expected writer to be hijacked")
	}
	// Connection should have been closed (if no conn error)
	if connError == nil && !mockConn.closed {
		t.Error("Expected connection to be closed")
	}
}

func verifyFailedHijack(t *testing.T, mockWriter *mockHijackableWriter, mockConn *mockConn) {
	// Hijack should have failed, no connection interaction
	if mockWriter.hijacked {
		t.Error("Expected hijack to fail, but it succeeded")
	}
	if mockConn.closed {
		t.Error("Expected connection to remain open when hijack fails")
	}
}

func TestSSEConverter_Close_WithHijacker(t *testing.T) {
	tests := []struct {
		name        string
		hijackError error
		connError   error
		wantErr     bool
	}{
		{
			name:        "successful hijack and close",
			hijackError: nil,
			connError:   nil,
			wantErr:     false,
		},
		{
			name:        "successful hijack, connection close error",
			hijackError: nil,
			connError:   errors.New("connection close failed"),
			wantErr:     true,
		},
		{
			name:        "hijack fails, fallback to no-op",
			hijackError: errors.New("hijack failed"),
			connError:   nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &mockConn{closeError: tt.connError}
			mockWriter := &mockHijackableWriter{
				mockResponseWriter: newMockResponseWriter(),
				hijackError:        tt.hijackError,
				mockConn:           mockConn,
			}

			// Manually create SSE converter with hijackable writer
			sse := &SSEConverter{
				writer:      mockWriter,
				flusher:     mockWriter.flusher,
				chunkBy:     SSEChunkByWord,
				formatter:   RawContentFormatter,
				eventFields: nil,
			}

			err := sse.Close()

			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify hijacking behavior
			if tt.hijackError == nil {
				verifySuccessfulHijack(t, mockWriter, mockConn, tt.connError)
			} else {
				verifyFailedHijack(t, mockWriter, mockConn)
			}

			// Verify flush was called
			if mockWriter.flusher.FlushCount() == 0 {
				t.Error("Expected flush to be called before close attempt")
			}
		})
	}
}

func TestSSEConverter_Close_DeferUsage(t *testing.T) {
	// Test the common defer pattern
	testFunc := func() error {
		mock := newMockResponseWriter()
		sse := ToSSE(mock)

		// Simulate defer close with error handling
		defer func() {
			if closeErr := sse.Close(); closeErr != nil {
				// In real code, this would likely be logged
				t.Logf("Close error (expected in defer): %v", closeErr)
			}
		}()

		// Simulate some work
		reader := strings.NewReader("test data")
		return sse.FromReader(reader)
	}

	if err := testFunc(); err != nil {
		t.Errorf("Test function failed: %v", err)
	}
}

func TestSSEConverter_WhitespacePreservation(t *testing.T) {
	tests := []struct {
		name           string
		mode           SSEChunkMode
		input          string
		expectedChunks []string
	}{
		{
			name:           "word mode preserves spaces",
			mode:           SSEChunkByWord,
			input:          "hello world test",
			expectedChunks: []string{"hello ", "world ", "test"},
		},
		{
			name:           "word mode preserves newlines",
			mode:           SSEChunkByWord,
			input:          "line1\nline2\nend",
			expectedChunks: []string{"line1\n", "line2\n", "end"},
		},
		{
			name:           "word mode preserves tabs",
			mode:           SSEChunkByWord,
			input:          "col1\tcol2\tdata",
			expectedChunks: []string{"col1\t", "col2\t", "data"},
		},
		{
			name:           "word mode preserves mixed whitespace",
			mode:           SSEChunkByWord,
			input:          "word \ttab\nnewline end",
			expectedChunks: []string{"word ", "\t", "tab\n", "newline ", "end"},
		},
		{
			name:           "char mode preserves all characters",
			mode:           SSEChunkByChar,
			input:          "a \n\tb",
			expectedChunks: []string{"a", " ", "\n", "\t", "b"},
		},
		{
			name:           "line mode preserves newlines",
			mode:           SSEChunkByLine,
			input:          "line1\nline2\n\nline4",
			expectedChunks: []string{"line1\n", "line2\n", "\n", "line4"},
		},
		{
			name:           "line mode with spaces and tabs",
			mode:           SSEChunkByLine,
			input:          "  spaced\t\tline  \n\ttabbed line\nend",
			expectedChunks: []string{"  spaced\t\tline  \n", "\ttabbed line\n", "end"},
		},
		{
			name:           "none mode preserves everything",
			mode:           SSEChunkNone,
			input:          "multi\nline\t\tcontent with   spaces",
			expectedChunks: []string{"multi\nline\t\tcontent with   spaces"},
		},
		{
			name:           "consecutive whitespace preservation",
			mode:           SSEChunkByWord,
			input:          "word1   \n\n\t  word2",
			expectedChunks: []string{"word1 ", " ", " ", "\n", "\n", "\t", " ", " ", "word2"},
		},
		{
			name:           "leading and trailing whitespace",
			mode:           SSEChunkByWord,
			input:          "  start middle  end  ",
			expectedChunks: []string{" ", " ", "start ", "middle ", " ", "end ", " "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			sse := ToSSE(mock).WithChunkMode(tt.mode)

			reader := strings.NewReader(tt.input)
			err := sse.FromReader(reader)

			if err != nil {
				t.Fatalf("FromReader() error = %v", err)
			}

			events := parseSSEEvents(t, mock.Body.String())
			messageEvents := filterMessageEvents(events)

			if len(messageEvents) != len(tt.expectedChunks) {
				t.Errorf("Expected %d message events, got %d", len(tt.expectedChunks), len(messageEvents))
				t.Logf("Input: %q", tt.input)
				t.Logf("Expected chunks: %+v", tt.expectedChunks)
				actualChunks := make([]string, len(messageEvents))
				for i, event := range messageEvents {
					actualChunks[i] = event.Data
				}
				t.Logf("Actual chunks: %+v", actualChunks)
				return
			}

			// Reconstruct original input from chunks
			var reconstructed strings.Builder
			for i, event := range messageEvents {
				reconstructed.WriteString(event.Data)

				// Verify each chunk matches expected
				if event.Data != tt.expectedChunks[i] {
					t.Errorf("Chunk %d: expected %q, got %q", i, tt.expectedChunks[i], event.Data)
				}
			}

			// Verify complete reconstruction matches original input
			if reconstructed.String() != tt.input {
				t.Errorf("Reconstructed input doesn't match original")
				t.Logf("Original:      %q", tt.input)
				t.Logf("Reconstructed: %q", reconstructed.String())

				// Show byte-by-byte comparison for debugging
				orig := []byte(tt.input)
				recon := []byte(reconstructed.String())
				t.Logf("Original bytes:      %v", orig)
				t.Logf("Reconstructed bytes: %v", recon)
			}
		})
	}
}

func TestSSEIntegration(t *testing.T) {
	t.Run("http server integration", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			sse := ToSSE(w).WithChunkMode(SSEChunkByWord)

			data := "hello world from server"
			reader := strings.NewReader(data)

			if err := sse.FromReader(reader); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		// Make request to server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Verify SSE headers
		verifySSEHeaders(t, resp.Header)

		// Read and parse response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		events := parseSSEEvents(t, string(body))
		if len(events) < 2 { // At least message + completion
			t.Errorf("Expected at least 2 events, got %d", len(events))
		}

		// Verify completion event exists
		completionFound := false
		for _, event := range events {
			if event.Event == testCompletion {
				completionFound = true
				break
			}
		}
		if !completionFound {
			t.Error("Completion event not found")
		}
	})
}
