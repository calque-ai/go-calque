package memory

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	calquepb "github.com/calque-ai/go-calque/proto"
)

// MockMemoryServiceClient is a mock implementation of MemoryServiceClient
type MockMemoryServiceClient struct {
	responses map[string]*calquepb.MemoryResponse
	errors    map[string]error
}

func NewMockMemoryServiceClient() *MockMemoryServiceClient {
	return &MockMemoryServiceClient{
		responses: make(map[string]*calquepb.MemoryResponse),
		errors:    make(map[string]error),
	}
}

func (m *MockMemoryServiceClient) ProcessMemory(_ context.Context, req *calquepb.MemoryRequest, _ ...grpc.CallOption) (*calquepb.MemoryResponse, error) {
	// Check for errors first
	if err, exists := m.errors[req.Operation]; exists {
		return nil, err
	}

	// Return predefined response
	if resp, exists := m.responses[req.Operation]; exists {
		return resp, nil
	}

	// Default success response
	return &calquepb.MemoryResponse{
		Success: true,
		Value:   "test-value",
	}, nil
}

func (m *MockMemoryServiceClient) SetResponse(operation string, response *calquepb.MemoryResponse) {
	m.responses[operation] = response
}

func (m *MockMemoryServiceClient) SetError(operation string, err error) {
	m.errors[operation] = err
}

// MockGRPCStore wraps the mock client for testing
type MockGRPCStore struct {
	*GRPCStore
	mockClient *MockMemoryServiceClient
}

func NewMockGRPCStore() *MockGRPCStore {
	mockClient := NewMockMemoryServiceClient()
	return &MockGRPCStore{
		GRPCStore: &GRPCStore{
			client: mockClient,
		},
		mockClient: mockClient,
	}
}

func TestGRPCStore_Get(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		key           string
		setupMock     func(*MockMemoryServiceClient)
		expectedValue []byte
		expectedError bool
	}{
		{
			name: "successful get",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("get", &calquepb.MemoryResponse{
					Success: true,
					Value:   "test-value",
				})
			},
			expectedValue: []byte("test-value"),
			expectedError: false,
		},
		{
			name: "get with failure response",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("get", &calquepb.MemoryResponse{
					Success: false,
					Value:   "",
				})
			},
			expectedValue: nil,
			expectedError: true,
		},
		{
			name: "get with gRPC error",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetError("get", status.Error(codes.Internal, "internal error"))
			},
			expectedValue: nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewMockGRPCStore()
			tt.setupMock(store.mockClient)

			value, err := store.Get(tt.key)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if string(value) != string(tt.expectedValue) {
					t.Errorf("Expected value %s, got %s", tt.expectedValue, value)
				}
			}
		})
	}
}

func TestGRPCStore_Set(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		key           string
		value         []byte
		setupMock     func(*MockMemoryServiceClient)
		expectedError bool
	}{
		{
			name:  "successful set",
			key:   "test-key",
			value: []byte("test-value"),
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("set", &calquepb.MemoryResponse{
					Success: true,
				})
			},
			expectedError: false,
		},
		{
			name:  "set with failure response",
			key:   "test-key",
			value: []byte("test-value"),
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("set", &calquepb.MemoryResponse{
					Success: false,
				})
			},
			expectedError: true,
		},
		{
			name:  "set with gRPC error",
			key:   "test-key",
			value: []byte("test-value"),
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetError("set", status.Error(codes.Internal, "internal error"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewMockGRPCStore()
			tt.setupMock(store.mockClient)

			err := store.Set(tt.key, tt.value)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGRPCStore_Delete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		key           string
		setupMock     func(*MockMemoryServiceClient)
		expectedError bool
	}{
		{
			name: "successful delete",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("delete", &calquepb.MemoryResponse{
					Success: true,
				})
			},
			expectedError: false,
		},
		{
			name: "delete with failure response",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("delete", &calquepb.MemoryResponse{
					Success: false,
				})
			},
			expectedError: true,
		},
		{
			name: "delete with gRPC error",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetError("delete", status.Error(codes.Internal, "internal error"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewMockGRPCStore()
			tt.setupMock(store.mockClient)

			err := store.Delete(tt.key)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGRPCStore_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		setupMock    func(*MockMemoryServiceClient)
		expectedKeys []string
	}{
		{
			name: "successful list",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("list", &calquepb.MemoryResponse{
					Success: true,
					Value:   "key1,key2,key3",
				})
			},
			expectedKeys: []string{"key1", "key2", "key3"},
		},
		{
			name: "list with empty response",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("list", &calquepb.MemoryResponse{
					Success: true,
					Value:   "",
				})
			},
			expectedKeys: []string{},
		},
		{
			name: "list with failure response",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("list", &calquepb.MemoryResponse{
					Success: false,
				})
			},
			expectedKeys: []string{},
		},
		{
			name: "list with gRPC error",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetError("list", status.Error(codes.Internal, "internal error"))
			},
			expectedKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewMockGRPCStore()
			tt.setupMock(store.mockClient)

			keys := store.List()

			if len(keys) != len(tt.expectedKeys) {
				t.Errorf("Expected %d keys, got %d", len(tt.expectedKeys), len(keys))
			}

			for i, expectedKey := range tt.expectedKeys {
				if i >= len(keys) || keys[i] != expectedKey {
					t.Errorf("Expected key %s at index %d, got %s", expectedKey, i, keys[i])
				}
			}
		})
	}
}

func TestGRPCStore_Exists(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		key       string
		setupMock func(*MockMemoryServiceClient)
		expected  bool
	}{
		{
			name: "key exists",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("exists", &calquepb.MemoryResponse{
					Success: true,
				})
			},
			expected: true,
		},
		{
			name: "key does not exist",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetResponse("exists", &calquepb.MemoryResponse{
					Success: false,
				})
			},
			expected: false,
		},
		{
			name: "exists with gRPC error",
			key:  "test-key",
			setupMock: func(m *MockMemoryServiceClient) {
				m.SetError("exists", status.Error(codes.Internal, "internal error"))
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewMockGRPCStore()
			tt.setupMock(store.mockClient)

			exists := store.Exists(tt.key)

			if exists != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, exists)
			}
		})
	}
}

func TestGRPCStore_createContext(t *testing.T) {
	t.Parallel()
	store := &GRPCStore{}

	ctx, cancel := store.createContext()
	defer cancel()

	// Check that context has timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected context to have deadline")
	}

	// Check that timeout is approximately 30 seconds
	expectedTimeout := 30 * time.Second
	actualTimeout := time.Until(deadline)
	if actualTimeout < expectedTimeout-time.Second || actualTimeout > expectedTimeout+time.Second {
		t.Errorf("Expected timeout around %v, got %v", expectedTimeout, actualTimeout)
	}
}

func TestNewGRPCStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid endpoint",
			endpoint: "localhost:8080",
			wantErr:  false,
		},
		{
			name:     "empty endpoint",
			endpoint: "",
			wantErr:  false, // NewGRPCStore doesn't validate empty endpoints
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewGRPCStore(tt.endpoint)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if store == nil {
				t.Error("Expected store but got nil")
			}
		})
	}
}
