package inspect

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestBasicLogging(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}

	// Create logger instance
	log := New(mockLogger)

	// Test basic info logging
	handler := log.Info().Head("TEST", 50)

	// Create test request/response
	input := strings.NewReader("Hello, world! This is a test message.")
	var output bytes.Buffer

	req := &calque.Request{
		Context: context.Background(),
		Data:    input,
	}
	res := &calque.Response{Data: &output}

	// Execute handler
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	// Verify output was passed through
	if output.String() != "Hello, world! This is a test message." {
		t.Errorf("Output mismatch: got %q", output.String())
	}

	// Verify logging occurred
	logOutput := buf.String()
	if !strings.Contains(logOutput, "[INFO]") {
		t.Errorf("Expected [INFO] in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "[TEST]") {
		t.Errorf("Expected [TEST] in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Hello, world!") {
		t.Errorf("Expected message preview in log output, got: %s", logOutput)
	}
}

func TestLevelFiltering(t *testing.T) {
	// Create mock logger that only enables INFO and above
	mockLogger := &MockLogger{buffer: &bytes.Buffer{}, enabledLevel: InfoLevel}
	log := New(mockLogger)

	// Debug handler should be disabled
	debugHandler := log.Debug().Head("DEBUG_TEST", 10)

	input := strings.NewReader("debug message")
	var output bytes.Buffer

	req := &calque.Request{Context: context.Background(), Data: input}
	res := &calque.Response{Data: &output}

	err := debugHandler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	// Should pass through data but not log
	if output.String() != "debug message" {
		t.Errorf("Output should pass through: got %q", output.String())
	}

	// Should not have logged anything
	if mockLogger.buffer.Len() > 0 {
		t.Errorf("Debug should not log when level disabled, got: %s", mockLogger.buffer.String())
	}
}

func TestStructuredFields(t *testing.T) {
	var buf bytes.Buffer
	mockLogger := &MockLogger{buffer: &buf}
	log := New(mockLogger)

	// Test with structured fields
	handler := log.Info().Head("STRUCTURED", 20,
		Attribute{"user_id", "123"},
		Attribute{"session", "abc-def"},
	)

	input := strings.NewReader("test data")
	var output bytes.Buffer

	req := &calque.Request{Context: context.Background(), Data: input}
	res := &calque.Response{Data: &output}

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "user_id=123") {
		t.Errorf("Expected user_id field in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "session=abc-def") {
		t.Errorf("Expected session field in log output, got: %s", logOutput)
	}
}

// MockLogger for testing
type MockLogger struct {
	buffer       *bytes.Buffer
	enabledLevel LogLevel
}

func (m *MockLogger) Log(_ context.Context, level LogLevel, msg string, attrs ...Attribute) {
	levelStr := map[LogLevel]string{
		DebugLevel: "[DEBUG]",
		InfoLevel:  "[INFO]",
		WarnLevel:  "[WARN]",
		ErrorLevel: "[ERROR]",
	}[level]

	m.buffer.WriteString(levelStr + " " + msg)

	for _, attr := range attrs {
		fmt.Fprintf(m.buffer, " %s=%v", attr.Key, attr.Value)
	}
	m.buffer.WriteString("\n")
}

func (m *MockLogger) IsLevelEnabled(_ context.Context, level LogLevel) bool {
	return level >= m.enabledLevel
}

func (m *MockLogger) Printf(_ string, _ ...any) {
	// Not used in tests
}
