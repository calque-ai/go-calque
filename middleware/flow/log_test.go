package flow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/calque-pipe/core"
)

type mockLogger struct {
	logs []string
}

func (m *mockLogger) Printf(format string, v ...any) {
	m.logs = append(m.logs, fmt.Sprintf(format, v...))
}

func TestLogger(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		peekBytes   int
		input       string
		expectedLog string
		expectedOut string
	}{
		{
			name:        "simple text logging",
			prefix:      "TEST",
			peekBytes:   50,
			input:       "hello world",
			expectedLog: "[TEST]: hello world\n",
			expectedOut: "hello world",
		},
		{
			name:        "empty input",
			prefix:      "EMPTY",
			peekBytes:   100,
			input:       "",
			expectedLog: "[EMPTY]: <empty>\n",
			expectedOut: "",
		},
		{
			name:        "multiline input",
			prefix:      "MULTI",
			peekBytes:   50,
			input:       "line 1\nline 2\nline 3",
			expectedLog: "[MULTI]: line 1\nline 2\nline 3\n",
			expectedOut: "line 1\nline 2\nline 3",
		},
		{
			name:        "long input truncated",
			prefix:      "LONG",
			peekBytes:   10,
			input:       "this is a very long input that should be truncated",
			expectedLog: "[LONG]: this is a \n",
			expectedOut: "this is a very long input that should be truncated",
		},
		{
			name:        "input with tabs and newlines",
			prefix:      "TAB",
			peekBytes:   50,
			input:       "hello\tworld\ntest",
			expectedLog: "[TAB]: hello\tworld\ntest\n",
			expectedOut: "hello\tworld\ntest",
		},
		{
			name:        "binary data detection",
			prefix:      "BIN",
			peekBytes:   20,
			input:       "\x00\x01\x02\x03binary data",
			expectedLog: "[BIN]: binary data: 0001020362696e6172792064617461\n",
			expectedOut: "\x00\x01\x02\x03binary data",
		},
		{
			name:        "large binary data with hex preview",
			prefix:      "BIGBIN",
			peekBytes:   50,
			input:       string(bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03}, 20)),
			expectedLog: "[BIGBIN]: binary data (50 bytes): 0001020300010203000102030001020300010203...\n",
			expectedOut: string(bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03}, 20)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLog := &mockLogger{}
			handler := Logger(tt.prefix, tt.peekBytes, mockLog)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			if err != nil {
				t.Errorf("Logger() error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expectedOut {
				t.Errorf("Logger() output = %q, want %q", got, tt.expectedOut)
			}

			if len(mockLog.logs) != 1 {
				t.Errorf("Expected 1 log entry, got %d", len(mockLog.logs))
				return
			}

			if mockLog.logs[0] != tt.expectedLog {
				t.Errorf("Logger() log = %q, want %q", mockLog.logs[0], tt.expectedLog)
			}
		})
	}
}

func TestLoggerDefaultLogger(t *testing.T) {
	handler := Logger("DEFAULT", 50)

	var buf bytes.Buffer
	reader := strings.NewReader("test with default logger")

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Logger() with default logger error = %v", err)
		return
	}

	expected := "test with default logger"
	if got := buf.String(); got != expected {
		t.Errorf("Logger() output = %q, want %q", got, expected)
	}
}

func TestFormatPreview(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: "<empty>",
		},
		{
			name:     "printable text",
			input:    []byte("hello world"),
			expected: "hello world",
		},
		{
			name:     "text with whitespace",
			input:    []byte("hello\tworld\ntest\r"),
			expected: "hello\tworld\ntest\r",
		},
		{
			name:     "binary data short",
			input:    []byte{0x00, 0x01, 0x02},
			expected: "binary data: 000102",
		},
		{
			name:     "binary data long",
			input:    bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03}, 10),
			expected: "binary data (40 bytes): 0001020300010203000102030001020300010203...",
		},
		{
			name:     "mixed printable and non-printable",
			input:    []byte("hello\x00world"),
			expected: "binary data: 68656c6c6f00776f726c64",
		},
		{
			name:     "100 byte limit with ellipsis",
			input:    bytes.Repeat([]byte("a"), 100),
			expected: strings.Repeat("a", 100) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPreview(tt.input)
			if got != tt.expected {
				t.Errorf("formatPreview() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsPrintable(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: true,
		},
		{
			name:     "basic ascii text",
			input:    []byte("hello world"),
			expected: true,
		},
		{
			name:     "text with allowed whitespace",
			input:    []byte("hello\tworld\ntest\r"),
			expected: true,
		},
		{
			name:     "text with numbers and symbols",
			input:    []byte("Hello123!@#$%^&*()"),
			expected: true,
		},
		{
			name:     "contains null byte",
			input:    []byte("hello\x00world"),
			expected: false,
		},
		{
			name:     "contains control character",
			input:    []byte("hello\x01world"),
			expected: false,
		},
		{
			name:     "contains high ascii",
			input:    []byte("hello\x80world"),
			expected: false,
		},
		{
			name:     "only whitespace characters",
			input:    []byte("\t\n\r   "),
			expected: true,
		},
		{
			name:     "printable range boundaries",
			input:    []byte{32, 126}, // space and tilde
			expected: true,
		},
		{
			name:     "non-printable boundaries",
			input:    []byte{31, 127}, // below space and above tilde
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPrintable(tt.input)
			if got != tt.expected {
				t.Errorf("isPrintable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLoggerWithIOError(t *testing.T) {
	mockLog := &mockLogger{}
	handler := Logger("ERROR", 50, mockLog)

	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	req := core.NewRequest(context.Background(), errorReader)
	res := core.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Logger() error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func TestLoggerPreservesStreamIntegrity(t *testing.T) {
	handler := Logger("STREAM", 10)

	largeInput := strings.Repeat("0123456789", 1000) // 10KB
	var buf bytes.Buffer
	reader := strings.NewReader(largeInput)

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Logger() error = %v", err)
		return
	}

	if got := buf.String(); got != largeInput {
		t.Errorf("Logger() corrupted large stream, length got %d, want %d", len(got), len(largeInput))
	}
}
