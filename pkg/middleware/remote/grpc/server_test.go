package grpc

import (
	"context"
	"io"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/calque-ai/go-calque/pkg/calque"
	calquepb "github.com/calque-ai/go-calque/proto"
)

func TestNewServer(t *testing.T) {
	t.Parallel()
	server := NewServer(":8080")
	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.addr != ":8080" {
		t.Errorf("Expected addr ':8080', got '%s'", server.addr)
	}
	if server.flows == nil {
		t.Error("Expected non-nil flows map")
	}
	if server.server == nil {
		t.Error("Expected non-nil gRPC server")
	}
}

func TestServerRegisterFlow(t *testing.T) {
	t.Parallel()
	server := NewServer(":8080")
	flow := calque.NewFlow()

	server.RegisterFlow("test-flow", flow)

	retrieved, err := server.GetFlow("test-flow")
	if err != nil {
		t.Fatalf("Failed to get flow: %v", err)
	}
	if retrieved != flow {
		t.Error("Retrieved flow does not match registered flow")
	}
}

func TestServerGetFlow(t *testing.T) {
	tests := []struct {
		name        string
		flowName    string
		register    bool
		expectError bool
	}{
		{
			name:        "non-existent flow",
			flowName:    "non-existent",
			register:    false,
			expectError: true,
		},
		{
			name:        "existing flow",
			flowName:    "test-flow",
			register:    true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := NewServer(":8080")

			if tt.register {
				flow := calque.NewFlow()
				server.RegisterFlow(tt.flowName, flow)
			}

			retrieved, err := server.GetFlow(tt.flowName)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error for non-existent flow")
				}
			} else {
				if err != nil {
					t.Fatalf("Failed to get flow: %v", err)
				}
				if retrieved == nil {
					t.Error("Expected non-nil flow")
				}
			}
		})
	}
}

func TestServerGetServer(t *testing.T) {
	t.Parallel()
	server := NewServer(":8080")
	grpcServer := server.GetServer()
	if grpcServer == nil {
		t.Error("Expected non-nil gRPC server")
	}
}

func TestNewFlowService(t *testing.T) {
	t.Parallel()
	server := NewServer(":8080")
	flowService := NewFlowService(server)
	if flowService == nil {
		t.Fatal("Expected non-nil flow service")
	}
	if flowService.server != server {
		t.Error("Flow service server does not match")
	}
}

func TestFlowServiceExecuteFlow(t *testing.T) {
	t.Parallel()
	server := NewServer(":8080")
	flowService := NewFlowService(server)

	// Create a simple flow
	flow := calque.NewFlow().
		UseFunc(func(_ *calque.Request, res *calque.Response) error {
			input := "test input"
			_, err := res.Data.Write([]byte(input))
			return err
		})

	server.RegisterFlow("test-flow", flow)

	// Create request
	req := &calquepb.FlowRequest{
		FlowName: "test-flow",
		Input:    "test input",
		Metadata: map[string]string{},
	}

	// Execute flow
	resp, err := flowService.ExecuteFlow(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to execute flow: %v", err)
	}

	if !resp.Success {
		t.Error("Expected successful execution")
	}
	if resp.Output == "" {
		t.Error("Expected non-empty output")
	}
}

func TestFlowServiceExecuteFlowNonExistent(t *testing.T) {
	tests := []struct {
		name           string
		flowName       string
		input          string
		expectSuccess  bool
		expectErrorMsg bool
	}{
		{
			name:           "non-existent flow",
			flowName:       "non-existent-flow",
			input:          "test input",
			expectSuccess:  false,
			expectErrorMsg: true,
		},
		{
			name:           "empty flow name",
			flowName:       "",
			input:          "test input",
			expectSuccess:  false,
			expectErrorMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := NewServer(":8080")
			flowService := NewFlowService(server)

			// Create request
			req := &calquepb.FlowRequest{
				FlowName: tt.flowName,
				Input:    tt.input,
				Metadata: map[string]string{},
			}

			// Execute flow
			resp, err := flowService.ExecuteFlow(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resp.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, resp.Success)
			}
			if (resp.ErrorMessage != "") != tt.expectErrorMsg {
				t.Errorf("Expected error message %v, got %v", tt.expectErrorMsg, resp.ErrorMessage != "")
			}
		})
	}
}

func TestFlowServiceStreamFlow(t *testing.T) {
	t.Parallel()
	server := NewServer(":8080")
	flowService := NewFlowService(server)

	// Create a simple flow
	flow := calque.NewFlow().
		UseFunc(func(_ *calque.Request, res *calque.Response) error {
			input := "test input"
			_, err := res.Data.Write([]byte(input))
			return err
		})

	server.RegisterFlow("test-flow", flow)

	// Create a mock stream
	mockStream := &mockFlowServiceStreamFlowServer{
		requests:  make([]*calquepb.StreamingFlowRequest, 0),
		responses: make([]*calquepb.StreamingFlowResponse, 0),
	}

	// Create request
	req := &calquepb.StreamingFlowRequest{
		FlowName: "test-flow",
		Input:    "test input",
		Metadata: map[string]string{},
	}

	// Send request
	mockStream.requests = []*calquepb.StreamingFlowRequest{req}

	// Execute stream flow
	err := flowService.StreamFlow(mockStream)
	if err != nil {
		t.Fatalf("Failed to execute stream flow: %v", err)
	}

	// Check response
	if len(mockStream.responses) == 0 {
		t.Error("Expected at least one response")
	}

	lastResp := mockStream.responses[len(mockStream.responses)-1]
	if !lastResp.Success {
		t.Error("Expected successful execution")
	}
	if !lastResp.IsFinal {
		t.Error("Expected final response")
	}
}

// Mock implementation of FlowService_StreamFlowServer
type mockFlowServiceStreamFlowServer struct {
	requests  []*calquepb.StreamingFlowRequest
	responses []*calquepb.StreamingFlowResponse
	index     int
}

func (m *mockFlowServiceStreamFlowServer) Send(resp *calquepb.StreamingFlowResponse) error {
	m.responses = append(m.responses, resp)
	return nil
}

func (m *mockFlowServiceStreamFlowServer) Recv() (*calquepb.StreamingFlowRequest, error) {
	if m.index >= len(m.requests) {
		return nil, io.EOF // End of stream
	}
	req := m.requests[m.index]
	m.index++
	return req, nil
}

func (m *mockFlowServiceStreamFlowServer) SetHeader(metadata.MD) error {
	return nil
}

func (m *mockFlowServiceStreamFlowServer) SendHeader(metadata.MD) error {
	return nil
}

func (m *mockFlowServiceStreamFlowServer) SetTrailer(metadata.MD) {
}

func (m *mockFlowServiceStreamFlowServer) Context() context.Context {
	return context.Background()
}

func (m *mockFlowServiceStreamFlowServer) SendMsg(_ interface{}) error {
	return nil
}

func (m *mockFlowServiceStreamFlowServer) RecvMsg(_ interface{}) error {
	return nil
}
