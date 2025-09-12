// Package ai provides AI agent middleware with gRPC support for distributed AI services.
package ai

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/config"
	grpcutil "github.com/calque-ai/go-calque/pkg/grpc"
	"github.com/calque-ai/go-calque/pkg/helpers"
	"github.com/calque-ai/go-calque/pkg/middleware/remote"
	calquepb "github.com/calque-ai/go-calque/proto"
)

// GRPCClient implements the Client interface for remote gRPC AI services.
//
// Provides distributed AI capabilities by connecting to remote gRPC AI services.
// Supports both unary and streaming AI operations with protobuf serialization.
//
// Example:
//
//	client, _ := ai.NewGRPCClient("ai-service:8080")
//	agent := ai.Agent(client)
type GRPCClient struct {
	remoteClient remote.Client
	client       calquepb.AIServiceClient
	conn         *grpc.ClientConn
}

// NewGRPCClient creates a new gRPC AI client.
func NewGRPCClient(endpoint string) (*GRPCClient, error) {
	return NewGRPCClientWithConfig(&config.ServiceConfig{
		Endpoint: endpoint,
		Timeout:  30 * time.Second,
		Credentials: config.CredentialsConfig{
			Type: "insecure",
		},
	})
}

// NewGRPCClientWithConfig creates a new gRPC AI client with configuration.
func NewGRPCClientWithConfig(serviceConfig *config.ServiceConfig) (*GRPCClient, error) {
	// Create gRPC client configuration
	grpcConfig := &grpcutil.Config{
		Endpoint: serviceConfig.Endpoint,
		Timeout:  serviceConfig.Timeout,
		KeepAlive: &grpcutil.KeepAliveConfig{
			Time:                serviceConfig.KeepAlive.Time,
			Timeout:             serviceConfig.KeepAlive.Timeout,
			PermitWithoutStream: serviceConfig.KeepAlive.PermitWithoutStream,
		},
	}

	// Get credentials
	creds, err := serviceConfig.GetCredentials()
	if err != nil {
		return nil, helpers.WrapGRPCError(err, "failed to get credentials")
	}
	grpcConfig.Credentials = creds

	// Create connection
	conn, err := grpcutil.NewClient(grpcConfig)
	if err != nil {
		return nil, helpers.WrapGRPCError(err, "failed to connect to AI service", serviceConfig.Endpoint)
	}

	client := calquepb.NewAIServiceClient(conn)

	// Create remote client wrapper
	remoteClient := &grpcRemoteClient{
		conn:   conn,
		client: client,
	}

	return &GRPCClient{
		remoteClient: remoteClient,
		client:       client,
		conn:         conn,
	}, nil
}

// Chat implements the Client interface for gRPC AI services.
//
// Input: string prompt/query via calque.Request
// Output: streamed AI response via calque.Response
// Behavior: STREAMING - outputs tokens as they arrive from remote service
//
// Supports tool calling and structured responses through protobuf.
//
// Example:
//
//	err := client.Chat(req, res, &ai.AgentOptions{Tools: tools})
func (c *GRPCClient) Chat(r *calque.Request, w *calque.Response, opts *AgentOptions) error {
	// Read input
	var input string
	if err := calque.Read(r, &input); err != nil {
		return helpers.WrapError(err, "failed to read input")
	}

	// Create AI request
	req := &calquepb.AIRequest{
		Prompt:     input,
		Parameters: make(map[string]string),
		Tools:      convertToolsToStrings(opts),
	}

	// Execute streaming request
	stream, err := c.client.StreamChat(r.Context, req)
	if err != nil {
		return helpers.WrapGRPCError(err, "failed to start streaming chat")
	}

	// Stream responses
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return helpers.WrapGRPCError(err, "streaming error")
		}

		// Write response
		if _, err := w.Data.Write([]byte(resp.Response)); err != nil {
			return helpers.WrapError(err, "failed to write response")
		}
	}

	return nil
}

// Close closes the gRPC connection.
func (c *GRPCClient) Close() error {
	return grpcutil.CloseConnection(c.conn, 5*time.Second)
}

// grpcRemoteClient implements the remote.Client interface for gRPC.
type grpcRemoteClient struct {
	conn   *grpc.ClientConn
	client calquepb.AIServiceClient
}

// Call implements remote.Client interface.
func (c *grpcRemoteClient) Call(_ context.Context, _ string, _, _ proto.Message) error {
	// This is a simplified implementation - in practice, you'd need to handle different methods
	return helpers.NewGRPCInternalError("unary calls not implemented for AI service", nil)
}

// Stream implements remote.Client interface.
func (c *grpcRemoteClient) Stream(_ context.Context, _ string) (remote.Stream, error) {
	// This is a simplified implementation - in practice, you'd need to handle different methods
	return nil, helpers.NewGRPCInternalError("streaming calls not implemented for AI service", nil)
}

// Close implements remote.Client interface.
func (c *grpcRemoteClient) Close() error {
	return grpcutil.CloseConnection(c.conn, 5*time.Second)
}

// IsHealthy implements remote.Client interface.
func (c *grpcRemoteClient) IsHealthy(_ context.Context) error {
	// Simple health check - try to get connection state
	state := c.conn.GetState()
	if state.String() == "READY" {
		return nil
	}
	return helpers.NewGRPCUnavailableError("connection not ready", nil)
}

// convertToolsToStrings converts tools to string array for protobuf
func convertToolsToStrings(opts *AgentOptions) []string {
	if opts == nil || len(opts.Tools) == 0 {
		return nil
	}

	result := make([]string, len(opts.Tools))
	for i, tool := range opts.Tools {
		result[i] = tool.Name()
	}
	return result
}
