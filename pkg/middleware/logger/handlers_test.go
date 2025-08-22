package logger

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

const testMessage = "Hello, world!"

type contextKey string

// TestHandlerHead tests the Head handler functionality
func TestHandlerHead(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	// Test basic Head functionality
	handler := log.Info().Head("TEST_HEAD", 10)

	input := strings.NewReader("Hello, world! This is a longer message.")
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Head handler failed: %v", err)
	}

	// Verify output passes through unchanged
	expected := "Hello, world! This is a longer message."
	if output.String() != expected {
		t.Errorf("Output mismatch: got %q, want %q", output.String(), expected)
	}

	// Verify logging occurred with correct preview
	logOutput := buf.String()
	if !strings.Contains(logOutput, "[INFO]") {
		t.Errorf("Expected [INFO] in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "[TEST_HEAD]") {
		t.Errorf("Expected [TEST_HEAD] in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Hello, wor") { // First 10 chars
		t.Errorf("Expected head preview in log output, got: %s", logOutput)
	}
}

// TestHandlerHeadTail tests the HeadTail handler functionality
func TestHandlerHeadTail(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	handler := log.Info().HeadTail("TEST_HEADTAIL", 5, 3)

	input := strings.NewReader(testMessage)
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("HeadTail handler failed: %v", err)
	}

	// Verify output passes through unchanged
	if output.String() != testMessage {
		t.Errorf("Output mismatch: got %q", output.String())
	}

	// Verify logging occurred with head and tail
	logOutput := buf.String()
	if !strings.Contains(logOutput, "[TEST_HEADTAIL]") {
		t.Errorf("Expected [TEST_HEADTAIL] in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "head=Hello") {
		t.Errorf("Expected head preview in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "tail=ld!") {
		t.Errorf("Expected tail preview in log output, got: %s", logOutput)
	}
}

// TestHandlerChunks tests the Chunks handler functionality
func TestHandlerChunks(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	handler := log.Debug().Chunks("TEST_CHUNKS", 5)

	input := strings.NewReader(testMessage)
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Chunks handler failed: %v", err)
	}

	// Verify output passes through unchanged
	if output.String() != testMessage {
		t.Errorf("Output mismatch: got %q", output.String())
	}

	// Verify multiple chunks were logged
	logOutput := buf.String()
	chunkCount := strings.Count(logOutput, "[TEST_CHUNKS] Chunk")
	if chunkCount < 2 {
		t.Errorf("Expected multiple chunks in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "chunk_num=1") {
		t.Errorf("Expected chunk_num in log output, got: %s", logOutput)
	}
}

// TestHandlerTiming tests the Timing handler functionality
func TestHandlerTiming(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	// Create a simple handler that adds a delay
	innerHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		time.Sleep(1 * time.Millisecond) // Small delay to ensure measurable timing

		var data []byte
		err := calque.Read(req, &data)
		if err != nil {
			return err
		}
		return calque.Write(res, data)
	})

	handler := log.Info().Timing("TEST_TIMING", innerHandler)

	input := strings.NewReader(testMessage)
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Timing handler failed: %v", err)
	}

	// Verify output passes through unchanged
	if output.String() != testMessage {
		t.Errorf("Output mismatch: got %q", output.String())
	}

	// Verify timing was logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "[TEST_TIMING] completed") {
		t.Errorf("Expected timing completion in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "duration_") {
		t.Errorf("Expected duration field in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "bytes=13") {
		t.Errorf("Expected bytes count in log output, got: %s", logOutput)
	}
}

// TestHandlerSampling tests the Sampling handler functionality
func TestHandlerSampling(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	handler := log.Info().Sampling("TEST_SAMPLING", 3, 2)

	input := strings.NewReader("Hello, world! This is a longer message for sampling.")
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Sampling handler failed: %v", err)
	}

	// Verify output passes through unchanged
	expected := "Hello, world! This is a longer message for sampling."
	if output.String() != expected {
		t.Errorf("Output mismatch: got %q", output.String())
	}

	// Verify sampling was logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "[TEST_SAMPLING]") {
		t.Errorf("Expected sampling prefix in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "num_samples=3") {
		t.Errorf("Expected num_samples in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "sample_positions") {
		t.Errorf("Expected sample_positions in log output, got: %s", logOutput)
	}
}

// TestHandlerPrint tests the Print handler functionality
func TestHandlerPrint(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	handler := log.Debug().Print("TEST_PRINT")

	input := strings.NewReader(testMessage)
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Print handler failed: %v", err)
	}

	// Verify output passes through unchanged
	if output.String() != testMessage {
		t.Errorf("Output mismatch: got %q", output.String())
	}

	// Verify full content was logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "[TEST_PRINT]") {
		t.Errorf("Expected print prefix in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "content=Hello, world!") {
		t.Errorf("Expected full content in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "total_bytes=13") {
		t.Errorf("Expected total_bytes in log output, got: %s", logOutput)
	}
}

// TestHandlerWithAttributes tests handlers with custom attributes
func TestHandlerWithAttributes(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	handler := log.Info().Head("ATTR_TEST", 10,
		Attr("user_id", "12345"),
		Attr("session", "test-session"),
	)

	input := strings.NewReader("test data")
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler with attributes failed: %v", err)
	}

	// Verify custom attributes were logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "user_id=12345") {
		t.Errorf("Expected user_id attribute in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "session=test-session") {
		t.Errorf("Expected session attribute in log output, got: %s", logOutput)
	}
}

// TestHandlerWithContext tests context handling in handlers
func TestHandlerWithContext(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockContextLogger{buffer: &buf}
	log := New(mockLogger)

	// Test with explicit context
	ctx := context.WithValue(context.Background(), contextKey("trace_id"), "test-trace-123")
	handler := log.Info().WithContext(ctx).Head("CTX_TEST", 10)

	input := strings.NewReader("test data")
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(), // Different context in request
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler with context failed: %v", err)
	}

	// Verify the explicit context was used (MockContextLogger records context values)
	logOutput := buf.String()
	if !strings.Contains(logOutput, "trace_id=test-trace-123") {
		t.Errorf("Expected explicit context to be used, got: %s", logOutput)
	}
}

// TestFormatDuration tests the formatDuration helper function
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration      time.Duration
		expectedField string
		minValue      float64
		maxValue      float64
	}{
		{500 * time.Microsecond, "duration_µs", 400, 600},
		{5 * time.Millisecond, "duration_µs", 4900, 5100}, // 5ms = 5000µs (under 10ms threshold)
		{15 * time.Millisecond, "duration_ms", 14, 16},    // 15ms > 10ms threshold
		{1500 * time.Millisecond, "duration_s", 1.4, 1.6},
	}

	for _, test := range tests {
		field, value := formatDuration(test.duration)
		if field != test.expectedField {
			t.Errorf("For duration %v, expected field %s, got %s", test.duration, test.expectedField, field)
		}
		if value < test.minValue || value > test.maxValue {
			t.Errorf("For duration %v, expected value between %f and %f, got %f", test.duration, test.minValue, test.maxValue, value)
		}
	}
}

// TestFormatPreview tests the formatPreview helper function
func TestFormatPreview(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"empty", []byte{}, "<empty>"},
		{"simple text", []byte("Hello"), "Hello"},
		{"unicode text", []byte("Hello 世界"), "Hello 世界"},
		{"binary data", []byte{0x00, 0x01, 0x02}, "binary data: 000102"},
		{"long binary", make([]byte, 50), "binary data (50 bytes):"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := formatPreview(test.input)
			if test.name == "long binary" {
				if !strings.HasPrefix(result, test.expected) {
					t.Errorf("Expected prefix %q, got %q", test.expected, result)
				}
			} else {
				if result != test.expected {
					t.Errorf("Expected %q, got %q", test.expected, result)
				}
			}
		})
	}
}

// MockContextLogger extends MockLogger to capture context information
type MockContextLogger struct {
	buffer *bytes.Buffer
}

func (m *MockContextLogger) Log(ctx context.Context, level LogLevel, msg string, attrs ...Attribute) {
	levelStr := map[LogLevel]string{
		DebugLevel: "[DEBUG]",
		InfoLevel:  "[INFO]",
		WarnLevel:  "[WARN]",
		ErrorLevel: "[ERROR]",
	}[level]

	m.buffer.WriteString(levelStr + " " + msg)

	// Add context values to log output
	if traceID := ctx.Value(contextKey("trace_id")); traceID != nil {
		m.buffer.WriteString(" trace_id=" + traceID.(string))
	}

	// Add attributes to log output
	for _, attr := range attrs {
		m.buffer.WriteString(" " + attr.Key + "=")
		switch v := attr.Value.(type) {
		case string:
			m.buffer.WriteString(v)
		case int:
			fmt.Fprintf(m.buffer, "%d", v)
		case float64:
			fmt.Fprintf(m.buffer, "%g", v)
		default:
			fmt.Fprintf(m.buffer, "%v", v)
		}
	}
	m.buffer.WriteString("\n")
}

func (m *MockContextLogger) IsLevelEnabled(_ context.Context, _ LogLevel) bool {
	return true // Always enabled for testing
}

func (m *MockContextLogger) Printf(_ string, v ...any) {
	// Not used in tests
}
