package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/invopop/jsonschema"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
	calquepb "github.com/calque-ai/go-calque/proto"
)

// MockAIServiceClient is a mock implementation of AIServiceClient
type MockAIServiceClient struct {
	streamChatError error
	streamResponses []*calquepb.AIResponse
	streamIndex     int
}

func NewMockAIServiceClient() *MockAIServiceClient {
	return &MockAIServiceClient{
		streamResponses: []*calquepb.AIResponse{
			{Response: "Hello"},
			{Response: " World"},
			{Response: "!"},
		},
	}
}

func (m *MockAIServiceClient) StreamChat(_ context.Context, _ *calquepb.AIRequest, _ ...grpc.CallOption) (calquepb.AIService_StreamChatClient, error) {
	if m.streamChatError != nil {
		return nil, m.streamChatError
	}
	return &MockStreamChatClient{
		responses: m.streamResponses,
		index:     &m.streamIndex,
	}, nil
}

func (m *MockAIServiceClient) Chat(_ context.Context, _ *calquepb.AIRequest, _ ...grpc.CallOption) (*calquepb.AIResponse, error) {
	return nil, errors.New("not implemented")
}

// MockStreamChatClient implements AIService_StreamChatClient
type MockStreamChatClient struct {
	responses []*calquepb.AIResponse
	index     *int
	closed    bool
}

func (m *MockStreamChatClient) Recv() (*calquepb.AIResponse, error) {
	if m.closed || *m.index >= len(m.responses) {
		return nil, errors.New("EOF")
	}
	resp := m.responses[*m.index]
	*m.index++
	return resp, nil
}

func (m *MockStreamChatClient) Send(*calquepb.AIRequest) error {
	return errors.New("not implemented")
}

func (m *MockStreamChatClient) CloseAndRecv() (*calquepb.AIResponse, error) {
	m.closed = true
	return nil, nil
}

func (m *MockStreamChatClient) Context() context.Context {
	return context.Background()
}

func (m *MockStreamChatClient) SendMsg(interface{}) error {
	return errors.New("not implemented")
}

func (m *MockStreamChatClient) RecvMsg(interface{}) error {
	return errors.New("not implemented")
}

func (m *MockStreamChatClient) Header() (metadata.MD, error) {
	return nil, errors.New("not implemented")
}

func (m *MockStreamChatClient) Trailer() metadata.MD {
	return nil
}

func (m *MockStreamChatClient) CloseSend() error {
	m.closed = true
	return nil
}

func TestNewGRPCClient(t *testing.T) {
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
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewGRPCClient(tt.endpoint)
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

			if client == nil {
				t.Error("Expected client but got nil")
			}
		})
	}
}

func TestGRPCClient_Chat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		mockClient     *MockAIServiceClient
		expectedOutput string
		wantErr        bool
	}{
		{
			name:  "streaming error",
			input: "Hello",
			mockClient: &MockAIServiceClient{
				streamChatError: status.Error(codes.Unavailable, "service unavailable"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a mock client
			client := &GRPCClient{
				client: tt.mockClient,
			}

			// Create request and response
			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			res := &calque.Response{
				Data: calque.NewWriter(),
			}

			// Execute chat
			err := client.Chat(req, res, nil)
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

			// Check output by reading from response buffer
			output := res.Data.(*calque.Buffer).String()

			if output != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestGRPCClient_Close(t *testing.T) {
	t.Parallel()

	// Create a real connection for testing
	conn, err := grpc.NewClient("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skip("Skipping test - no gRPC server available")
	}
	defer conn.Close()

	client := &GRPCClient{
		conn: conn,
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGrpcRemoteClient_Call(t *testing.T) {
	t.Parallel()

	client := &grpcRemoteClient{}

	err := client.Call(context.Background(), "test", nil, nil)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestGrpcRemoteClient_Stream(t *testing.T) {
	t.Parallel()

	client := &grpcRemoteClient{}

	stream, err := client.Stream(context.Background(), "test")
	if err == nil {
		t.Error("Expected error but got none")
	}
	if stream != nil {
		t.Error("Expected nil stream")
	}
}

func TestGrpcRemoteClient_Close(t *testing.T) {
	t.Parallel()

	// Create a real connection for testing
	conn, err := grpc.NewClient("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skip("Skipping test - no gRPC server available")
	}
	defer conn.Close()

	client := &grpcRemoteClient{
		conn: conn,
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGrpcRemoteClient_IsHealthy(t *testing.T) {
	t.Parallel()

	// Create a real connection for testing
	conn, err := grpc.NewClient("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skip("Skipping test - no gRPC server available")
	}
	defer conn.Close()

	client := &grpcRemoteClient{
		conn: conn,
	}

	// Test that IsHealthy returns an error (since connection will fail)
	err = client.IsHealthy(context.Background())
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestConvertToolsToStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     *AgentOptions
		expected []string
	}{
		{
			name:     "nil options",
			opts:     nil,
			expected: nil,
		},
		{
			name:     "empty tools",
			opts:     &AgentOptions{Tools: []tools.Tool{}},
			expected: nil,
		},
		{
			name: "single tool",
			opts: &AgentOptions{
				Tools: []tools.Tool{
					&mockTool{name: "tool1"},
				},
			},
			expected: []string{"tool1"},
		},
		{
			name: "multiple tools",
			opts: &AgentOptions{
				Tools: []tools.Tool{
					&mockTool{name: "tool1"},
					&mockTool{name: "tool2"},
					&mockTool{name: "tool3"},
				},
			},
			expected: []string{"tool1", "tool2", "tool3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := convertToolsToStrings(tt.opts)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d tools, got %d", len(tt.expected), len(result))
				return
			}

			for i, tool := range result {
				if tool != tt.expected[i] {
					t.Errorf("Expected tool %q at index %d, got %q", tt.expected[i], i, tool)
				}
			}
		})
	}
}

// mockTool is a simple mock implementation of tools.Tool interface
type mockTool struct {
	name string
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return "mock tool description"
}

func (m *mockTool) ParametersSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
	}
}

func (m *mockTool) ServeFlow(_ *calque.Request, res *calque.Response) error {
	_, err := res.Data.Write([]byte("mock result"))
	return err
}
