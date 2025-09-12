package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

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
				Data: calque.NewWriter(),
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
			output := res.Data.(*calque.Buffer).String()
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
					Data: calque.NewWriter(),
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
					Data: calque.NewWriter(),
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
		Data: calque.NewWriter(),
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
		Data: calque.NewWriter(),
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
		Data: calque.NewWriter(),
	}

	// Execute handler - should fail
	err = handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error for non-streaming service")
	}
}
