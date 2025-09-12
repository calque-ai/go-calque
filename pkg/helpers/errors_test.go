package helpers

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWrapError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		message  string
		expected string
		isNil    bool
	}{
		{
			name:     "wrap non-nil error",
			err:      errors.New("original error"),
			message:  "failed to process",
			expected: "failed to process: original error",
			isNil:    false,
		},
		{
			name:     "wrap nil error",
			err:      nil,
			message:  "failed to process",
			expected: "",
			isNil:    true,
		},
		{
			name:     "wrap with empty message",
			err:      errors.New("original error"),
			message:  "",
			expected: ": original error",
			isNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WrapError(tt.err, tt.message)

			if tt.isNil {
				if result != nil {
					t.Errorf("WrapError() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("WrapError() = nil, want non-nil error")
			}

			if result.Error() != tt.expected {
				t.Errorf("WrapError() = %q, want %q", result.Error(), tt.expected)
			}

			// Test that the original error can be unwrapped
			if tt.err != nil && !errors.Is(result, tt.err) {
				t.Error("WrapError() should preserve original error for errors.Is()")
			}
		})
	}
}

func TestWrapErrorf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		format   string
		args     []interface{}
		expected string
		isNil    bool
	}{
		{
			name:     "wrap with formatted message",
			err:      errors.New("connection failed"),
			format:   "failed to connect to %s service",
			args:     []interface{}{"database"},
			expected: "failed to connect to database service: connection failed",
			isNil:    false,
		},
		{
			name:     "wrap nil error",
			err:      nil,
			format:   "failed to connect to %s service",
			args:     []interface{}{"database"},
			expected: "",
			isNil:    true,
		},
		{
			name:     "wrap with multiple args",
			err:      errors.New("not found"),
			format:   "user %d in organization %s not found",
			args:     []interface{}{123, "acme"},
			expected: "user 123 in organization acme not found: not found",
			isNil:    false,
		},
		{
			name:     "wrap with no args",
			err:      errors.New("timeout"),
			format:   "operation timed out",
			args:     []interface{}{},
			expected: "operation timed out: timeout",
			isNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WrapErrorf(tt.err, tt.format, tt.args...)

			if tt.isNil {
				if result != nil {
					t.Errorf("WrapErrorf() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("WrapErrorf() = nil, want non-nil error")
			}

			if result.Error() != tt.expected {
				t.Errorf("WrapErrorf() = %q, want %q", result.Error(), tt.expected)
			}

			// Test that the original error can be unwrapped
			if tt.err != nil && !errors.Is(result, tt.err) {
				t.Error("WrapErrorf() should preserve original error for errors.Is()")
			}
		})
	}
}

func TestNewError(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "simple message",
			format:   "something went wrong",
			args:     []interface{}{},
			expected: "something went wrong",
		},
		{
			name:     "formatted message",
			format:   "user %d not found",
			args:     []interface{}{123},
			expected: "user 123 not found",
		},
		{
			name:     "multiple args",
			format:   "failed to connect to %s:%d",
			args:     []interface{}{"localhost", 8080},
			expected: "failed to connect to localhost:8080",
		},
		{
			name:     "empty message",
			format:   "",
			args:     []interface{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := NewError(tt.format, tt.args...)

			if result == nil {
				t.Fatal("NewError() = nil, want non-nil error")
			}

			if result.Error() != tt.expected {
				t.Errorf("NewError() = %q, want %q", result.Error(), tt.expected)
			}
		})
	}
}

func TestWrapGRPCError(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WrapGRPCError(originalErr, "wrapped message", "detail1", "detail2")

	if wrappedErr == nil {
		t.Fatal("Expected wrapped error to not be nil")
	}

	if wrappedErr.Err != originalErr {
		t.Error("Expected wrapped error to contain original error")
	}

	if wrappedErr.Message != "wrapped message" {
		t.Errorf("Expected message 'wrapped message', got '%s'", wrappedErr.Message)
	}

	if len(wrappedErr.Details) != 2 {
		t.Errorf("Expected 2 details, got %d", len(wrappedErr.Details))
	}
}

func TestWrapGRPCErrorWithGRPCStatus(t *testing.T) {
	grpcErr := status.New(codes.Unavailable, "service unavailable").Err()
	wrappedErr := WrapGRPCError(grpcErr, "wrapped message")

	if wrappedErr.Code != codes.Unavailable {
		t.Errorf("Expected code %v, got %v", codes.Unavailable, wrappedErr.Code)
	}
}

func TestGRPCErrorIsRetryable(t *testing.T) {
	tests := []struct {
		code     codes.Code
		expected bool
	}{
		{codes.Unavailable, true},
		{codes.DeadlineExceeded, true},
		{codes.ResourceExhausted, true},
		{codes.InvalidArgument, false},
		{codes.NotFound, false},
		{codes.Internal, false},
	}

	for _, test := range tests {
		err := &GRPCError{Code: test.code}
		if err.IsRetryable() != test.expected {
			t.Errorf("Expected IsRetryable() to return %v for code %v", test.expected, test.code)
		}
	}
}

func TestIsGRPCError(t *testing.T) {
	grpcErr := status.New(codes.Internal, "internal error").Err()
	regularErr := errors.New("regular error")

	if !IsGRPCError(grpcErr) {
		t.Error("Expected grpc error to be detected")
	}

	if IsGRPCError(regularErr) {
		t.Error("Expected regular error to not be detected as grpc error")
	}
}

func TestGetGRPCCode(t *testing.T) {
	grpcErr := status.New(codes.NotFound, "not found").Err()
	regularErr := errors.New("regular error")

	if GetGRPCCode(grpcErr) != codes.NotFound {
		t.Error("Expected to get NotFound code from grpc error")
	}

	if GetGRPCCode(regularErr) != codes.Unknown {
		t.Error("Expected to get Unknown code from regular error")
	}
}

func TestGRPCErrorConstructors(t *testing.T) {
	originalErr := errors.New("original")

	tests := []struct {
		name         string
		constructor  func(string, error) *GRPCError
		expectedCode codes.Code
	}{
		{"Unavailable", NewGRPCUnavailableError, codes.Unavailable},
		{"DeadlineExceeded", NewGRPCDeadlineExceededError, codes.DeadlineExceeded},
		{"InvalidArgument", NewGRPCInvalidArgumentError, codes.InvalidArgument},
		{"NotFound", NewGRPCNotFoundError, codes.NotFound},
		{"Internal", NewGRPCInternalError, codes.Internal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.constructor("test message", originalErr)
			if err.Code != test.expectedCode {
				t.Errorf("Expected code %v, got %v", test.expectedCode, err.Code)
			}
			if err.Err != originalErr {
				t.Error("Expected original error to be preserved")
			}
		})
	}
}
