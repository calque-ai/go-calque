package flow

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/calque-pipe/core"
)

func TestBranch(t *testing.T) {
	mockIfHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		_, err := res.Data.Write([]byte("if-executed"))
		return err
	})

	mockElseHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		_, err := res.Data.Write([]byte("else-executed"))
		return err
	})

	errorHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		return errors.New("handler error")
	})

	tests := []struct {
		name        string
		input       string
		condition   func([]byte) bool
		ifHandler   core.Handler
		elseHandler core.Handler
		expected    string
		wantErr     bool
	}{
		{
			name:        "condition true - executes if handler",
			input:       "json data",
			condition:   func(b []byte) bool { return bytes.Contains(b, []byte("json")) },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "if-executed",
			wantErr:     false,
		},
		{
			name:        "condition false - executes else handler",
			input:       "plain text",
			condition:   func(b []byte) bool { return bytes.Contains(b, []byte("json")) },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "else-executed",
			wantErr:     false,
		},
		{
			name:        "empty input - condition false",
			input:       "",
			condition:   func(b []byte) bool { return len(b) > 0 },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "else-executed",
			wantErr:     false,
		},
		{
			name:        "prefix check - condition true",
			input:       "{\"key\": \"value\"}",
			condition:   func(b []byte) bool { return bytes.HasPrefix(b, []byte("{")) },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "if-executed",
			wantErr:     false,
		},
		{
			name:        "if handler error",
			input:       "trigger condition",
			condition:   func(b []byte) bool { return true },
			ifHandler:   errorHandler,
			elseHandler: mockElseHandler,
			expected:    "",
			wantErr:     true,
		},
		{
			name:        "else handler error",
			input:       "trigger condition",
			condition:   func(b []byte) bool { return false },
			ifHandler:   mockIfHandler,
			elseHandler: errorHandler,
			expected:    "",
			wantErr:     true,
		},
		{
			name:        "binary data condition",
			input:       "\x00\x01\x02\x03",
			condition:   func(b []byte) bool { return b[0] == 0x00 },
			ifHandler:   mockIfHandler,
			elseHandler: mockElseHandler,
			expected:    "if-executed",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Branch[[]byte](tt.condition, tt.ifHandler, tt.elseHandler)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Branch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Branch() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTeeReader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "basic tee functionality",
			input:    "hello world",
			expected: "hello world",
			wantErr:  false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "multiline input",
			input:    "line 1\nline 2\nline 3",
			expected: "line 1\nline 2\nline 3",
			wantErr:  false,
		},
		{
			name:     "binary data",
			input:    "\x00\x01\x02\x03",
			expected: "\x00\x01\x02\x03",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var teeBuffer1, teeBuffer2, output bytes.Buffer

			handler := TeeReader(&teeBuffer1, &teeBuffer2)
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&output)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("TeeReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := output.String(); got != tt.expected {
				t.Errorf("TeeReader() output = %q, want %q", got, tt.expected)
			}

			if got := teeBuffer1.String(); got != tt.expected {
				t.Errorf("TeeReader() teeBuffer1 = %q, want %q", got, tt.expected)
			}

			if got := teeBuffer2.String(); got != tt.expected {
				t.Errorf("TeeReader() teeBuffer2 = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTeeReaderNoDestinations(t *testing.T) {
	input := "test data"
	var output bytes.Buffer

	handler := TeeReader()
	reader := strings.NewReader(input)

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&output)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("TeeReader() with no destinations error = %v", err)
	}

	if got := output.String(); got != input {
		t.Errorf("TeeReader() with no destinations = %q, want %q", got, input)
	}
}

func TestParallel(t *testing.T) {
	handler1 := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("handler1:" + string(input)))
		return err
	})

	handler2 := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("handler2:" + string(input)))
		return err
	})

	slowHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		time.Sleep(50 * time.Millisecond)
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("slow:" + string(input)))
		return err
	})

	errorHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		return errors.New("handler error")
	})

	tests := []struct {
		name         string
		input        string
		handlers     []core.Handler
		expectedParts []string
		wantErr      bool
	}{
		{
			name:          "two handlers parallel execution",
			input:         "test",
			handlers:      []core.Handler{handler1, handler2},
			expectedParts: []string{"handler1:test", "handler2:test"},
			wantErr:       false,
		},
		{
			name:          "single handler",
			input:         "single",
			handlers:      []core.Handler{handler1},
			expectedParts: []string{"handler1:single"},
			wantErr:       false,
		},
		{
			name:          "empty input",
			input:         "",
			handlers:      []core.Handler{handler1, handler2},
			expectedParts: []string{"handler1:", "handler2:"},
			wantErr:       false,
		},
		{
			name:          "mixed speed handlers",
			input:         "data",
			handlers:      []core.Handler{handler1, slowHandler},
			expectedParts: []string{"handler1:data", "slow:data"},
			wantErr:       false,
		},
		{
			name:     "handler error fails entire operation",
			input:    "error test",
			handlers: []core.Handler{handler1, errorHandler},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Parallel[[]byte](tt.handlers...)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parallel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			output := buf.String()
			parts := strings.Split(output, "\n---\n")

			if len(parts) != len(tt.expectedParts) {
				t.Errorf("Parallel() got %d parts, want %d", len(parts), len(tt.expectedParts))
				return
			}

			for _, expectedPart := range tt.expectedParts {
				found := false
				for _, part := range parts {
					if part == expectedPart {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Parallel() missing expected part %q in output %q", expectedPart, output)
				}
			}
		})
	}
}

func TestParallelNoHandlers(t *testing.T) {
	input := "pass through"
	var buf bytes.Buffer

	handler := Parallel[[]byte]()
	reader := strings.NewReader(input)

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Parallel() with no handlers error = %v", err)
	}

	if got := buf.String(); got != input {
		t.Errorf("Parallel() with no handlers = %q, want %q", got, input)
	}
}

func TestTimeout(t *testing.T) {
	fastHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("fast:" + string(input)))
		return err
	})

	slowHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		time.Sleep(200 * time.Millisecond)
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("slow:" + string(input)))
		return err
	})

	contextCheckHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		select {
		case <-req.Context.Done():
			return req.Context.Err()
		case <-time.After(50 * time.Millisecond):
			input, err := io.ReadAll(req.Data)
			if err != nil {
				return err
			}
			_, err = res.Data.Write([]byte("completed:" + string(input)))
			return err
		}
	})

	tests := []struct {
		name     string
		handler  core.Handler
		timeout  time.Duration
		input    string
		expected string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "fast handler completes within timeout",
			handler:  fastHandler,
			timeout:  100 * time.Millisecond,
			input:    "test",
			expected: "fast:test",
			wantErr:  false,
		},
		{
			name:    "slow handler times out",
			handler: slowHandler,
			timeout: 50 * time.Millisecond,
			input:   "test",
			wantErr: true,
			errMsg:  "handler timeout",
		},
		{
			name:     "context cancellation respected",
			handler:  contextCheckHandler,
			timeout:  100 * time.Millisecond,
			input:    "test",
			expected: "completed:test",
			wantErr:  false,
		},
		{
			name:     "zero timeout immediate cancellation",
			handler:  fastHandler,
			timeout:  0,
			input:    "test",
			wantErr:  true,
			errMsg:   "handler timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Timeout[string](tt.handler, tt.timeout)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Timeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Timeout() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Timeout() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRetry(t *testing.T) {
	attemptCount := 0
	failingHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		attemptCount++
		if attemptCount < 3 {
			return errors.New("temporary failure")
		}
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("success:" + string(input)))
		return err
	})

	alwaysFailHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		return errors.New("persistent failure")
	})

	successHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("first-try:" + string(input)))
		return err
	})

	tests := []struct {
		name        string
		handler     core.Handler
		maxAttempts int
		input       string
		expected    string
		wantErr     bool
		errMsg      string
		setup       func()
	}{
		{
			name:        "success on first attempt",
			handler:     successHandler,
			maxAttempts: 3,
			input:       "test",
			expected:    "first-try:test",
			wantErr:     false,
		},
		{
			name:        "success after retries",
			handler:     failingHandler,
			maxAttempts: 3,
			input:       "retry-test",
			expected:    "success:retry-test",
			wantErr:     false,
			setup:       func() { attemptCount = 0 },
		},
		{
			name:        "all attempts fail",
			handler:     alwaysFailHandler,
			maxAttempts: 2,
			input:       "fail-test",
			wantErr:     true,
			errMsg:      "retry exhausted",
		},
		{
			name:        "zero attempts",
			handler:     successHandler,
			maxAttempts: 0,
			input:       "no-attempts",
			wantErr:     true,
			errMsg:      "retry exhausted",
		},
		{
			name:        "single attempt success",
			handler:     successHandler,
			maxAttempts: 1,
			input:       "single",
			expected:    "first-try:single",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			handler := Retry[string](tt.handler, tt.maxAttempts)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			start := time.Now()
			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			elapsed := time.Since(start)

			if (err != nil) != tt.wantErr {
				t.Errorf("Retry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Retry() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Retry() = %q, want %q", got, tt.expected)
			}

			if tt.name == "success after retries" {
				if elapsed < 300*time.Millisecond {
					t.Errorf("Retry() expected exponential backoff, but completed in %v", elapsed)
				}
			}
		})
	}
}