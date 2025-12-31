package remote

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// MockClient is a mock implementation of the Client interface
type MockClient struct {
	callError   error
	streamError error
	closeError  error
	healthError error
	stream      *MockStream
}

func NewMockClient() *MockClient {
	return &MockClient{
		stream: NewMockStream(),
	}
}

func (m *MockClient) Call(_ context.Context, _ string, _, _ proto.Message) error {
	return m.callError
}

func (m *MockClient) Stream(_ context.Context, _ string) (Stream, error) {
	if m.streamError != nil {
		return nil, m.streamError
	}
	return m.stream, nil
}

func (m *MockClient) Close() error {
	return m.closeError
}

func (m *MockClient) IsHealthy(_ context.Context) error {
	return m.healthError
}

// MockStream is a mock implementation of the Stream interface
type MockStream struct {
	sendError    error
	recvError    error
	closeError   error
	ctx          context.Context
	recvMessages []proto.Message
	recvIndex    int
}

func NewMockStream() *MockStream {
	return &MockStream{
		ctx:          context.Background(),
		recvMessages: make([]proto.Message, 0),
	}
}

func (m *MockStream) Send(_ proto.Message) error {
	return m.sendError
}

func (m *MockStream) Recv() (proto.Message, error) {
	if m.recvError != nil {
		return nil, m.recvError
	}
	if m.recvIndex >= len(m.recvMessages) {
		return nil, errors.New("no more messages")
	}
	msg := m.recvMessages[m.recvIndex]
	m.recvIndex++
	return msg, nil
}

func (m *MockStream) CloseSend() error {
	return m.closeError
}

func (m *MockStream) Context() context.Context {
	return m.ctx
}

func (m *MockStream) AddRecvMessage(msg proto.Message) {
	m.recvMessages = append(m.recvMessages, msg)
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	endpoint := "localhost:8080"
	config := DefaultConfig(endpoint)

	if config.Endpoint != endpoint {
		t.Errorf("Expected endpoint %s, got %s", endpoint, config.Endpoint)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout %v, got %v", 30*time.Second, config.Timeout)
	}

	if config.Retry == nil {
		t.Error("Expected retry config to be set")
	} else {
		if config.Retry.MaxAttempts != 3 {
			t.Errorf("Expected max attempts 3, got %d", config.Retry.MaxAttempts)
		}
		if config.Retry.Backoff != 100*time.Millisecond {
			t.Errorf("Expected backoff %v, got %v", 100*time.Millisecond, config.Retry.Backoff)
		}
		if config.Retry.MaxBackoff != 5*time.Second {
			t.Errorf("Expected max backoff %v, got %v", 5*time.Second, config.Retry.MaxBackoff)
		}
	}

	if config.HealthCheck == nil {
		t.Error("Expected health check config to be set")
	} else {
		if config.HealthCheck.Interval != 30*time.Second {
			t.Errorf("Expected health check interval %v, got %v", 30*time.Second, config.HealthCheck.Interval)
		}
		if config.HealthCheck.Timeout != 5*time.Second {
			t.Errorf("Expected health check timeout %v, got %v", 5*time.Second, config.HealthCheck.Timeout)
		}
	}
}

func TestClientManager_RegisterClient(t *testing.T) {
	t.Parallel()
	manager := NewClientManager()
	client := NewMockClient()
	config := DefaultConfig("localhost:8080")

	manager.RegisterClient("test-client", client, config)

	// Verify client was registered
	retrievedClient, err := manager.GetClient(context.Background(), "test-client")
	if err != nil {
		t.Errorf("Unexpected error retrieving client: %v", err)
	}
	if retrievedClient != client {
		t.Error("Retrieved client does not match registered client")
	}
}

func TestClientManager_GetClient(t *testing.T) {
	t.Parallel()
	manager := NewClientManager()

	tests := []struct {
		name        string
		clientName  string
		expectError bool
	}{
		{
			name:        "existing client",
			clientName:  "test-client",
			expectError: false,
		},
		{
			name:        "non-existing client",
			clientName:  "non-existing",
			expectError: true,
		},
	}

	// Register a test client
	client := NewMockClient()
	config := DefaultConfig("localhost:8080")
	manager.RegisterClient("test-client", client, config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedClient, err := manager.GetClient(context.Background(), tt.clientName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if retrievedClient != client {
					t.Error("Retrieved client does not match registered client")
				}
			}
		})
	}
}

func TestClientManager_CloseAll(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		setupClients func(*ClientManager)
		expectError  bool
	}{
		{
			name: "successful close all",
			setupClients: func(cm *ClientManager) {
				client1 := NewMockClient()
				client2 := NewMockClient()
				config := DefaultConfig("localhost:8080")
				cm.RegisterClient("client1", client1, config)
				cm.RegisterClient("client2", client2, config)
			},
			expectError: false,
		},
		{
			name: "close with errors",
			setupClients: func(cm *ClientManager) {
				client1 := NewMockClient()
				client1.closeError = errors.New("close error")
				config := DefaultConfig("localhost:8080")
				cm.RegisterClient("client1", client1, config)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager := NewClientManager()
			tt.setupClients(manager)

			err := manager.CloseAll()

			if tt.expectError {
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

func TestClientManager_HealthCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		setupClients   func(*ClientManager)
		expectedErrors int
	}{
		{
			name: "all healthy clients",
			setupClients: func(cm *ClientManager) {
				client1 := NewMockClient()
				client2 := NewMockClient()
				config := DefaultConfig("localhost:8080")
				cm.RegisterClient("client1", client1, config)
				cm.RegisterClient("client2", client2, config)
			},
			expectedErrors: 0,
		},
		{
			name: "some unhealthy clients",
			setupClients: func(cm *ClientManager) {
				client1 := NewMockClient()
				client1.healthError = errors.New("health check failed")
				client2 := NewMockClient()
				config := DefaultConfig("localhost:8080")
				cm.RegisterClient("client1", client1, config)
				cm.RegisterClient("client2", client2, config)
			},
			expectedErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager := NewClientManager()
			tt.setupClients(manager)

			results := manager.HealthCheck(context.Background())

			errorCount := 0
			for _, err := range results {
				if err != nil {
					errorCount++
				}
			}

			if errorCount != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectedErrors, errorCount)
			}
		})
	}
}

func TestMockClient_Call(t *testing.T) {
	t.Parallel()
	client := NewMockClient()
	ctx := context.Background()
	req := &anypb.Any{}
	resp := &anypb.Any{}

	// Test successful call
	err := client.Call(ctx, "test-method", req, resp)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test call with error
	expectedError := errors.New("call error")
	client.callError = expectedError
	err = client.Call(ctx, "test-method", req, resp)
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockClient_Stream(t *testing.T) {
	t.Parallel()
	client := NewMockClient()
	ctx := context.Background()

	// Test successful stream
	stream, err := client.Stream(ctx, "test-method")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if stream == nil {
		t.Error("Expected stream to be returned")
	}

	// Test stream with error
	expectedError := errors.New("stream error")
	client.streamError = expectedError
	stream, err = client.Stream(ctx, "test-method")
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
	if stream != nil {
		t.Error("Expected stream to be nil on error")
	}
}

func TestMockClient_Close(t *testing.T) {
	t.Parallel()
	client := NewMockClient()

	// Test successful close
	err := client.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test close with error
	expectedError := errors.New("close error")
	client.closeError = expectedError
	err = client.Close()
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockClient_IsHealthy(t *testing.T) {
	t.Parallel()
	client := NewMockClient()
	ctx := context.Background()

	// Test healthy client
	err := client.IsHealthy(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test unhealthy client
	expectedError := errors.New("health check failed")
	client.healthError = expectedError
	err = client.IsHealthy(ctx)
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockStream_Send(t *testing.T) {
	t.Parallel()
	stream := NewMockStream()
	msg := &anypb.Any{}

	// Test successful send
	err := stream.Send(msg)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test send with error
	expectedError := errors.New("send error")
	stream.sendError = expectedError
	err = stream.Send(msg)
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockStream_Recv(t *testing.T) {
	t.Parallel()
	stream := NewMockStream()
	msg1 := &anypb.Any{}
	msg2 := &anypb.Any{}

	// Add messages to receive
	stream.AddRecvMessage(msg1)
	stream.AddRecvMessage(msg2)

	// Test successful receive
	recvMsg1, err := stream.Recv()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if recvMsg1 != msg1 {
		t.Error("Received message does not match expected message")
	}

	recvMsg2, err := stream.Recv()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if recvMsg2 != msg2 {
		t.Error("Received message does not match expected message")
	}

	// Test receive when no more messages
	_, err = stream.Recv()
	if err == nil {
		t.Error("Expected error when no more messages")
	}

	// Test receive with error
	stream.recvError = errors.New("recv error")
	stream.recvIndex = 0 // Reset index
	_, err = stream.Recv()
	if err != stream.recvError {
		t.Errorf("Expected error %v, got %v", stream.recvError, err)
	}
}

func TestMockStream_CloseSend(t *testing.T) {
	t.Parallel()
	stream := NewMockStream()

	// Test successful close send
	err := stream.CloseSend()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test close send with error
	expectedError := errors.New("close send error")
	stream.closeError = expectedError
	err = stream.CloseSend()
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockStream_Context(t *testing.T) {
	t.Parallel()
	stream := NewMockStream()
	ctx := stream.Context()

	if ctx == nil {
		t.Error("Expected context to be returned")
	}
}
