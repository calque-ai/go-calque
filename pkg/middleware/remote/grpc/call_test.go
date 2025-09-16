package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/calque-ai/go-calque/pkg/calque"
	calquepb "github.com/calque-ai/go-calque/proto"
)

const testInput = "test input"

// setupTestServer creates a test gRPC server and returns the address
func setupTestServer() string {
	return "localhost:0"
}

func TestCallHandler(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "connection failure expected",
			serviceName: "test-service",
			input:       testInput,
			expectError: true,
			errorMsg:    "gRPC call failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Setup test server (will fail connection)
			serverAddr := setupTestServer()

			// Create a registry with a service
			registry := NewRegistry()
			service := &Service{
				Name:     tt.serviceName,
				Endpoint: serverAddr,
				Timeout:  1 * time.Second, // Short timeout for faster tests
			}
			err := registry.Register(service)
			if err != nil {
				t.Fatalf("Failed to register service: %v", err)
			}

			// Create context with registry
			ctx := context.WithValue(context.Background(), registryContextKey{}, registry)

			// Create call handler
			handler := Call(tt.serviceName)

			// Create request and response
			req := &calque.Request{
				Context: ctx,
				Data:    calque.NewReader(tt.input),
			}
			res := &calque.Response{
				Data: calque.NewWriter[string](),
			}

			// Execute handler
			err = handler.ServeFlow(req, res)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Handler execution failed: %v", err)
			}

			// Check response
			output := res.Data.(*calque.Buffer[string]).String()
			if output == "" {
				t.Error("Expected non-empty output")
			}
		})
	}
}

func TestCallHandlerErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (*calque.Request, *calque.Response)
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing registry",
			setupFunc: func() (*calque.Request, *calque.Response) {
				handler := Call("test-service")
				req := &calque.Request{
					Context: context.Background(),
					Data:    calque.NewReader(testInput),
				}
				res := &calque.Response{
					Data: calque.NewWriter[string](),
				}
				_ = handler // Use handler to avoid unused variable
				return req, res
			},
			expectError: true,
			errorMsg:    "registry not found",
		},
		{
			name: "non-existent service",
			setupFunc: func() (*calque.Request, *calque.Response) {
				registry := NewRegistry()
				ctx := context.WithValue(context.Background(), registryContextKey{}, registry)
				handler := Call("non-existent-service")
				req := &calque.Request{
					Context: ctx,
					Data:    calque.NewReader(testInput),
				}
				res := &calque.Response{
					Data: calque.NewWriter[string](),
				}
				_ = handler // Use handler to avoid unused variable
				return req, res
			},
			expectError: true,
			errorMsg:    "service not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, res := tt.setupFunc()

			// Create appropriate handler based on test
			var handler calque.Handler
			if tt.name == "missing registry" {
				handler = Call("test-service")
			} else {
				handler = Call("non-existent-service")
			}

			// Execute handler
			err := handler.ServeFlow(req, res)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestTypedCallHandler(t *testing.T) {
	// Setup test server (will fail connection)
	serverAddr := setupTestServer()

	// Create a registry with a service
	registry := NewRegistry()
	service := &Service{
		Name:     "test-service",
		Endpoint: serverAddr,
		Timeout:  1 * time.Second,
	}
	err := registry.Register(service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Create context with registry
	ctx := context.WithValue(context.Background(), registryContextKey{}, registry)

	// Create typed call handler
	handler := CallWithTypes[*calquepb.FlowRequest, *calquepb.FlowResponse]("test-service")

	// Create a proper protobuf request
	flowReq := &calquepb.FlowRequest{
		Input: testInput,
		Metadata: map[string]string{
			"test": "true",
		},
	}
	reqData, err := proto.Marshal(flowReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Create request and response
	req := &calque.Request{
		Context: ctx,
		Data:    calque.NewReader(string(reqData)),
	}
	res := &calque.Response{
		Data: calque.NewWriter[string](),
	}

	// Execute handler - expect connection failure
	err = handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected connection error but got none")
	} else if !strings.Contains(err.Error(), "typed gRPC call failed") {
		t.Errorf("Expected typed gRPC call error, got: %v", err)
	}
}

func TestStreamHandler(t *testing.T) {
	// Setup test server (will fail connection)
	serverAddr := setupTestServer()

	// Create a registry with a streaming service
	registry := NewRegistry()
	service := &Service{
		Name:      "streaming-service",
		Endpoint:  serverAddr,
		Streaming: true,
		Timeout:   1 * time.Second,
	}
	err := registry.Register(service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Create context with registry
	ctx := context.WithValue(context.Background(), registryContextKey{}, registry)

	// Create stream handler
	handler := Stream("streaming-service")

	// Create request and response
	req := &calque.Request{
		Context: ctx,
		Data:    calque.NewReader(testInput),
	}
	res := &calque.Response{
		Data: calque.NewWriter[string](),
	}

	// Execute handler - expect connection failure
	err = handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected connection error but got none")
	} else if !strings.Contains(err.Error(), "failed to create streaming client") {
		t.Errorf("Expected streaming client error, got: %v", err)
	}
}

func TestStreamHandlerWithNonStreamingService(t *testing.T) {
	// Create a registry with a non-streaming service
	registry := NewRegistry()
	service := &Service{
		Name:      "regular-service",
		Endpoint:  "localhost:8080",
		Streaming: false,
	}
	err := registry.Register(service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Create context with registry
	ctx := context.WithValue(context.Background(), registryContextKey{}, registry)

	// Create stream handler
	handler := Stream("regular-service")

	// Create request and response
	req := &calque.Request{
		Context: ctx,
		Data:    calque.NewReader(testInput),
	}
	res := &calque.Response{
		Data: calque.NewWriter[string](),
	}

	// Execute handler - should fail
	err = handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error for non-streaming service")
	}
}

// TestIsRetryableError tests the retryable error detection logic
func TestIsRetryableError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "non-gRPC error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "Unavailable error",
			err:      status.Error(codes.Unavailable, "service unavailable"),
			expected: true,
		},
		{
			name:     "DeadlineExceeded error",
			err:      status.Error(codes.DeadlineExceeded, "deadline exceeded"),
			expected: true,
		},
		{
			name:     "ResourceExhausted error",
			err:      status.Error(codes.ResourceExhausted, "resource exhausted"),
			expected: true,
		},
		{
			name:     "Internal error",
			err:      status.Error(codes.Internal, "internal error"),
			expected: true,
		},
		{
			name:     "InvalidArgument error",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: false,
		},
		{
			name:     "NotFound error",
			err:      status.Error(codes.NotFound, "not found"),
			expected: false,
		},
		{
			name:     "PermissionDenied error",
			err:      status.Error(codes.PermissionDenied, "permission denied"),
			expected: false,
		},
		{
			name:     "Unauthenticated error",
			err:      status.Error(codes.Unauthenticated, "unauthenticated"),
			expected: false,
		},
		{
			name:     "FailedPrecondition error",
			err:      status.Error(codes.FailedPrecondition, "failed precondition"),
			expected: false,
		},
		{
			name:     "OutOfRange error",
			err:      status.Error(codes.OutOfRange, "out of range"),
			expected: false,
		},
		{
			name:     "Unimplemented error",
			err:      status.Error(codes.Unimplemented, "unimplemented"),
			expected: false,
		},
		{
			name:     "DataLoss error",
			err:      status.Error(codes.DataLoss, "data loss"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

// TestMakeGRPCCallRetryLogic tests the retry logic with different error types
func TestMakeGRPCCallRetryLogic(t *testing.T) {
	t.Parallel()

	// Test isRetryableError function directly since makeGRPCCall is not accessible
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable error",
			err:      status.Error(codes.Unavailable, "unavailable"),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

// TestMakeTypedGRPCCallErrorHandling tests typed gRPC call error handling
func TestMakeTypedGRPCCallErrorHandling(t *testing.T) {
	t.Parallel()

	// Test error types that would be used in typed calls
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable error",
			err:      status.Error(codes.Unavailable, "unavailable"),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

// TestErrorHandlingConcurrency tests error handling under concurrent load
func TestErrorHandlingConcurrency(t *testing.T) {
	t.Parallel()

	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(_ int) {
			// Test isRetryableError concurrently
			err := status.Error(codes.Unavailable, "unavailable")
			result := isRetryableError(err)
			done <- result
		}(i)
	}

	// Wait for all goroutines to complete
	successCount := 0
	for i := 0; i < concurrency; i++ {
		select {
		case result := <-done:
			if result {
				successCount++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent error handling test timed out")
		}
	}

	// All calls should return true for retryable errors
	if successCount != concurrency {
		t.Errorf("Expected %d successes, got %d", concurrency, successCount)
	}
}

// TestErrorHandlingEdgeCases tests edge cases for error handling
func TestErrorHandlingEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "non-gRPC error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "retryable error",
			err:      status.Error(codes.Unavailable, "unavailable"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

// TestErrorHandlingStatusCodes tests all gRPC status codes for retryability
func TestErrorHandlingStatusCodes(t *testing.T) {
	t.Parallel()

	allCodes := []codes.Code{
		codes.OK,
		codes.Canceled,
		codes.Unknown,
		codes.InvalidArgument,
		codes.DeadlineExceeded,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.ResourceExhausted,
		codes.FailedPrecondition,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unimplemented,
		codes.Internal,
		codes.Unavailable,
		codes.DataLoss,
		codes.Unauthenticated,
	}

	expectedRetryable := map[codes.Code]bool{
		codes.Unavailable:       true,
		codes.DeadlineExceeded:  true,
		codes.ResourceExhausted: true,
		codes.Internal:          true,
		codes.Aborted:           true,
	}

	for _, code := range allCodes {
		t.Run(fmt.Sprintf("code_%v", code), func(t *testing.T) {
			t.Parallel()

			err := status.Error(code, "test error")
			result := isRetryableError(err)
			expected := expectedRetryable[code]

			if result != expected {
				t.Errorf("Code %v: expected retryable=%v, got %v", code, expected, result)
			}
		})
	}
}
