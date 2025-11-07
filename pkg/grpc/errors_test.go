package grpc

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		grpcErr  *Error
		expected string
	}{
		{
			name: "error with underlying error",
			grpcErr: &Error{
				Code:    codes.InvalidArgument,
				Message: "invalid input",
				Err:     errors.New("underlying error"),
			},
			expected: "grpc error [InvalidArgument]: invalid input: underlying error",
		},
		{
			name: "error without underlying error",
			grpcErr: &Error{
				Code:    codes.NotFound,
				Message: "resource not found",
			},
			expected: "grpc error [NotFound]: resource not found",
		},
		{
			name: "error with details",
			grpcErr: &Error{
				Code:    codes.Internal,
				Message: "internal error",
				Details: []interface{}{"detail1", "detail2"},
			},
			expected: "grpc error [Internal]: internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.grpcErr.Error()
			if result != tt.expected {
				t.Errorf("Error() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	grpcErr := &Error{
		Code:    codes.InvalidArgument,
		Message: "test error",
		Err:     underlyingErr,
	}

	result := grpcErr.Unwrap()
	if result != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", result, underlyingErr)
	}

	// Test with nil underlying error
	grpcErrNoUnderlying := &Error{
		Code:    codes.InvalidArgument,
		Message: "test error",
		Err:     nil,
	}

	result = grpcErrNoUnderlying.Unwrap()
	if result != nil {
		t.Errorf("Unwrap() = %v, want nil", result)
	}
}

func TestError_IsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		code     codes.Code
		expected bool
	}{
		{
			name:     "Unavailable is retryable",
			code:     codes.Unavailable,
			expected: true,
		},
		{
			name:     "DeadlineExceeded is retryable",
			code:     codes.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "ResourceExhausted is retryable",
			code:     codes.ResourceExhausted,
			expected: true,
		},
		{
			name:     "InvalidArgument is not retryable",
			code:     codes.InvalidArgument,
			expected: false,
		},
		{
			name:     "NotFound is not retryable",
			code:     codes.NotFound,
			expected: false,
		},
		{
			name:     "Internal is not retryable",
			code:     codes.Internal,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcErr := &Error{
				Code:    tt.code,
				Message: "test error",
			}

			result := grpcErr.IsRetryable()
			if result != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		message  string
		details  []interface{}
		expected *Error
	}{
		{
			name:     "nil error returns nil",
			err:      nil,
			message:  "test message",
			details:  nil,
			expected: nil,
		},
		{
			name:    "non-gRPC error",
			err:     errors.New("test error"),
			message: "wrapped message",
			details: []interface{}{"detail1"},
			expected: &Error{
				Code:    codes.Unknown,
				Message: "wrapped message",
				Details: []interface{}{"detail1"},
				Err:     errors.New("test error"),
			},
		},
		{
			name:    "gRPC error with status",
			err:     status.Error(codes.InvalidArgument, "invalid argument"),
			message: "wrapped message",
			details: nil,
			expected: &Error{
				Code:    codes.InvalidArgument,
				Message: "wrapped message",
				Details: nil,
				Err:     status.Error(codes.InvalidArgument, "invalid argument"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapError(tt.err, tt.message, tt.details...)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("WrapError() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Errorf("WrapError() = nil, want %v", tt.expected)
				return
			}

			if result.Code != tt.expected.Code {
				t.Errorf("WrapError() Code = %v, want %v", result.Code, tt.expected.Code)
			}

			if result.Message != tt.expected.Message {
				t.Errorf("WrapError() Message = %v, want %v", result.Message, tt.expected.Message)
			}

			if len(result.Details) != len(tt.expected.Details) {
				t.Errorf("WrapError() Details length = %v, want %v", len(result.Details), len(tt.expected.Details))
			}

			if result.Err.Error() != tt.expected.Err.Error() {
				t.Errorf("WrapError() Err = %v, want %v", result.Err, tt.expected.Err)
			}
		})
	}
}

func TestIsGRPCError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "gRPC error",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: true,
		},
		{
			name:     "non-gRPC error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: true, // status.FromError(nil) returns OK status, which is considered a gRPC error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGRPCError(tt.err)
			if result != tt.expected {
				t.Errorf("IsGRPCError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetGRPCCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected codes.Code
	}{
		{
			name:     "gRPC error with code",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: codes.InvalidArgument,
		},
		{
			name:     "non-gRPC error",
			err:      errors.New("regular error"),
			expected: codes.Unknown,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: codes.OK, // status.FromError(nil) returns OK status
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetGRPCCode(tt.err)
			if result != tt.expected {
				t.Errorf("GetGRPCCode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewUnavailableError(t *testing.T) {
	underlyingErr := errors.New("connection failed")
	message := "service unavailable"

	result := NewUnavailableError(message, underlyingErr)

	if result.Code != codes.Unavailable {
		t.Errorf("NewUnavailableError() Code = %v, want %v", result.Code, codes.Unavailable)
	}

	if result.Message != message {
		t.Errorf("NewUnavailableError() Message = %v, want %v", result.Message, message)
	}

	if result.Err != underlyingErr {
		t.Errorf("NewUnavailableError() Err = %v, want %v", result.Err, underlyingErr)
	}
}

func TestNewDeadlineExceededError(t *testing.T) {
	underlyingErr := errors.New("timeout")
	message := "deadline exceeded"

	result := NewDeadlineExceededError(message, underlyingErr)

	if result.Code != codes.DeadlineExceeded {
		t.Errorf("NewDeadlineExceededError() Code = %v, want %v", result.Code, codes.DeadlineExceeded)
	}

	if result.Message != message {
		t.Errorf("NewDeadlineExceededError() Message = %v, want %v", result.Message, message)
	}

	if result.Err != underlyingErr {
		t.Errorf("NewDeadlineExceededError() Err = %v, want %v", result.Err, underlyingErr)
	}
}

func TestNewInvalidArgumentError(t *testing.T) {
	underlyingErr := errors.New("invalid input")
	message := "invalid argument"

	result := NewInvalidArgumentError(message, underlyingErr)

	if result.Code != codes.InvalidArgument {
		t.Errorf("NewInvalidArgumentError() Code = %v, want %v", result.Code, codes.InvalidArgument)
	}

	if result.Message != message {
		t.Errorf("NewInvalidArgumentError() Message = %v, want %v", result.Message, message)
	}

	if result.Err != underlyingErr {
		t.Errorf("NewInvalidArgumentError() Err = %v, want %v", result.Err, underlyingErr)
	}
}

func TestNewNotFoundError(t *testing.T) {
	underlyingErr := errors.New("not found")
	message := "resource not found"

	result := NewNotFoundError(message, underlyingErr)

	if result.Code != codes.NotFound {
		t.Errorf("NewNotFoundError() Code = %v, want %v", result.Code, codes.NotFound)
	}

	if result.Message != message {
		t.Errorf("NewNotFoundError() Message = %v, want %v", result.Message, message)
	}

	if result.Err != underlyingErr {
		t.Errorf("NewNotFoundError() Err = %v, want %v", result.Err, underlyingErr)
	}
}

func TestNewInternalError(t *testing.T) {
	underlyingErr := errors.New("internal failure")
	message := "internal error"

	result := NewInternalError(message, underlyingErr)

	if result.Code != codes.Internal {
		t.Errorf("NewInternalError() Code = %v, want %v", result.Code, codes.Internal)
	}

	if result.Message != message {
		t.Errorf("NewInternalError() Message = %v, want %v", result.Message, message)
	}

	if result.Err != underlyingErr {
		t.Errorf("NewInternalError() Err = %v, want %v", result.Err, underlyingErr)
	}
}

func TestWrapErrorSimple(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		message  string
		expected string
	}{
		{
			name:     "nil error returns nil",
			err:      nil,
			message:  "test message",
			expected: "",
		},
		{
			name:     "error with message",
			err:      errors.New("original error"),
			message:  "wrapped message",
			expected: "wrapped message: original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapErrorSimple(tt.err, tt.message)

			if tt.err == nil {
				if result != nil {
					t.Errorf("WrapErrorSimple() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Errorf("WrapErrorSimple() = nil, want error")
				return
			}

			if result.Error() != tt.expected {
				t.Errorf("WrapErrorSimple() = %v, want %v", result.Error(), tt.expected)
			}
		})
	}
}

func TestWrapErrorfSimple(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "nil error returns nil",
			err:      nil,
			format:   "test %s",
			args:     []interface{}{"message"},
			expected: "",
		},
		{
			name:     "error with formatted message",
			err:      errors.New("original error"),
			format:   "wrapped %s %d",
			args:     []interface{}{"message", 42},
			expected: "wrapped message 42: original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapErrorfSimple(tt.err, tt.format, tt.args...)

			if tt.err == nil {
				if result != nil {
					t.Errorf("WrapErrorfSimple() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Errorf("WrapErrorfSimple() = nil, want error")
				return
			}

			if result.Error() != tt.expected {
				t.Errorf("WrapErrorfSimple() = %v, want %v", result.Error(), tt.expected)
			}
		})
	}
}

func TestNewErrorSimple(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "simple message",
			format:   "test message",
			args:     nil,
			expected: "test message",
		},
		{
			name:     "formatted message",
			format:   "test %s %d",
			args:     []interface{}{"message", 42},
			expected: "test message 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewErrorSimple(tt.format, tt.args...)

			if result.Error() != tt.expected {
				t.Errorf("NewErrorSimple() = %v, want %v", result.Error(), tt.expected)
			}
		})
	}
}
