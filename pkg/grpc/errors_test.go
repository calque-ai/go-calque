package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestError_Error(t *testing.T) {
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-grpc-error")

	tests := []struct {
		name     string
		grpcErr  *Error
		expected string
	}{
		{
			name: "error with underlying error",
			grpcErr: &Error{
				calqueErr: calque.WrapErr(ctx, errors.New("underlying error"), "invalid input"),
				Code:      codes.InvalidArgument,
			},
			expected: "grpc error [InvalidArgument]: invalid input: underlying error",
		},
		{
			name: "error without underlying error",
			grpcErr: &Error{
				calqueErr: calque.NewErr(ctx, "resource not found"),
				Code:      codes.NotFound,
			},
			expected: "grpc error [NotFound]: resource not found",
		},
		{
			name: "error with details",
			grpcErr: &Error{
				calqueErr: calque.NewErr(ctx, "internal error"),
				Code:      codes.Internal,
				Details:   []interface{}{"detail1", "detail2"},
			},
			expected: "grpc error [Internal]: internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.grpcErr.Error()
			if result != tt.expected {
				t.Errorf("Error() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	ctx := context.Background()
	underlyingErr := errors.New("underlying error")
	grpcErr := &Error{
		calqueErr: calque.WrapErr(ctx, underlyingErr, "test error"),
		Code:      codes.InvalidArgument,
	}

	result := grpcErr.Unwrap()
	if result != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", result, underlyingErr)
	}

	// Test with nil underlying error
	grpcErrNoUnderlying := &Error{
		calqueErr: calque.NewErr(ctx, "test error"),
		Code:      codes.InvalidArgument,
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
			t.Parallel()
			ctx := context.Background()
			grpcErr := &Error{
				calqueErr: calque.NewErr(ctx, "test error"),
				Code:      tt.code,
			}

			result := grpcErr.IsRetryable()
			if result != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-grpc-wrap")

	tests := []struct {
		name     string
		ctx      context.Context
		err      error
		message  string
		details  []interface{}
		expected *Error
	}{
		{
			name:     "nil error returns nil",
			ctx:      ctx,
			err:      nil,
			message:  "test message",
			details:  nil,
			expected: nil,
		},
		{
			name:    "non-gRPC error",
			ctx:     ctx,
			err:     errors.New("test error"),
			message: "wrapped message",
			details: []interface{}{"detail1"},
			expected: &Error{
				Code:    codes.Unknown,
				Details: []interface{}{"detail1"},
			},
		},
		{
			name:    "gRPC error with status",
			ctx:     ctx,
			err:     status.Error(codes.InvalidArgument, "invalid argument"),
			message: "wrapped message",
			details: nil,
			expected: &Error{
				Code:    codes.InvalidArgument,
				Details: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WrapError(tt.ctx, tt.err, tt.message, tt.details...)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("WrapError() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Errorf("WrapError() = nil, want non-nil error")
				return
			}

			if result.Code != tt.expected.Code {
				t.Errorf("WrapError() Code = %v, want %v", result.Code, tt.expected.Code)
			}

			if len(result.Details) != len(tt.expected.Details) {
				t.Errorf("WrapError() Details length = %v, want %v", len(result.Details), len(tt.expected.Details))
			}

			if result.Error() == "" {
				t.Error("WrapError() should return non-empty error message")
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
	ctx := context.Background()
	underlyingErr := errors.New("connection failed")
	message := "service unavailable"

	result := NewUnavailableError(ctx, message, underlyingErr)

	if result.Code != codes.Unavailable {
		t.Errorf("NewUnavailableError() Code = %v, want %v", result.Code, codes.Unavailable)
	}

	if result.Error() == "" {
		t.Error("NewUnavailableError() should return non-empty error message")
	}
}

func TestNewDeadlineExceededError(t *testing.T) {
	ctx := context.Background()
	underlyingErr := errors.New("timeout")
	message := "deadline exceeded"

	result := NewDeadlineExceededError(ctx, message, underlyingErr)

	if result.Code != codes.DeadlineExceeded {
		t.Errorf("NewDeadlineExceededError() Code = %v, want %v", result.Code, codes.DeadlineExceeded)
	}

	if result.Error() == "" {
		t.Error("NewDeadlineExceededError() should return non-empty error message")
	}
}

func TestNewInvalidArgumentError(t *testing.T) {
	ctx := context.Background()
	underlyingErr := errors.New("invalid input")
	message := "invalid argument"

	result := NewInvalidArgumentError(ctx, message, underlyingErr)

	if result.Code != codes.InvalidArgument {
		t.Errorf("NewInvalidArgumentError() Code = %v, want %v", result.Code, codes.InvalidArgument)
	}

	if result.Error() == "" {
		t.Error("NewInvalidArgumentError() should return non-empty error message")
	}
}

func TestNewNotFoundError(t *testing.T) {
	ctx := context.Background()
	underlyingErr := errors.New("not found")
	message := "resource not found"

	result := NewNotFoundError(ctx, message, underlyingErr)

	if result.Code != codes.NotFound {
		t.Errorf("NewNotFoundError() Code = %v, want %v", result.Code, codes.NotFound)
	}

	if result.Error() == "" {
		t.Error("NewNotFoundError() should return non-empty error message")
	}
}

func TestNewInternalError(t *testing.T) {
	ctx := context.Background()
	underlyingErr := errors.New("internal failure")
	message := "internal error"

	result := NewInternalError(ctx, message, underlyingErr)

	if result.Code != codes.Internal {
		t.Errorf("NewInternalError() Code = %v, want %v", result.Code, codes.Internal)
	}

	if result.Error() == "" {
		t.Error("NewInternalError() should return non-empty error message")
	}
}

func TestWrapErrorSimple(t *testing.T) {
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-grpc-wrap-simple")

	tests := []struct {
		name     string
		ctx      context.Context
		err      error
		message  string
		expected string
	}{
		{
			name:     "nil error returns nil",
			ctx:      ctx,
			err:      nil,
			message:  "test message",
			expected: "",
		},
		{
			name:     "error with message",
			ctx:      ctx,
			err:      errors.New("original error"),
			message:  "wrapped message",
			expected: "wrapped message: original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WrapErrorSimple(tt.ctx, tt.err, tt.message)

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
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-grpc-wrapf-simple")

	tests := []struct {
		name     string
		ctx      context.Context
		err      error
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "nil error returns nil",
			ctx:      ctx,
			err:      nil,
			format:   "test %s",
			args:     []interface{}{"message"},
			expected: "",
		},
		{
			name:     "error with formatted message",
			ctx:      ctx,
			err:      errors.New("original error"),
			format:   "wrapped %s %d",
			args:     []interface{}{"message", 42},
			expected: "wrapped message 42: original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WrapErrorfSimple(tt.ctx, tt.err, tt.format, tt.args...)

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
	ctx := context.Background()
	ctx = calque.WithTraceID(ctx, "calque-trace-test-grpc-new-simple")

	tests := []struct {
		name     string
		ctx      context.Context
		message  string
		expected string
	}{
		{
			name:     "simple message",
			ctx:      ctx,
			message:  "test message",
			expected: "test message",
		},
		{
			name:     "error message",
			ctx:      ctx,
			message:  "invalid configuration",
			expected: "invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := NewErrorSimple(tt.ctx, tt.message)

			if result.Error() != tt.expected {
				t.Errorf("NewErrorSimple() = %v, want %v", result.Error(), tt.expected)
			}
		})
	}
}
