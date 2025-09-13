// Package tools provides tool execution middleware with gRPC support for distributed tool services.
package tools

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
	calquepb "github.com/calque-ai/go-calque/proto"
)

// GRPCExecutor implements distributed tool execution via gRPC services.
//
// Provides distributed tool capabilities by connecting to remote gRPC tool services.
// Supports tool discovery, execution, and result streaming with protobuf serialization.
//
// Example:
//
//	executor, _ := tools.NewGRPCExecutor("tools-service:8080")
//	flow.Use(executor.Execute())
type GRPCExecutor struct {
	conn   *grpc.ClientConn
	client calquepb.ToolsServiceClient
}

// NewGRPCExecutor creates a new gRPC tool executor.
func NewGRPCExecutor(endpoint string) (*GRPCExecutor, error) {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, helpers.WrapGRPCError(err, "failed to connect to tools service", endpoint)
	}

	client := calquepb.NewToolsServiceClient(conn)
	return &GRPCExecutor{
		conn:   conn,
		client: client,
	}, nil
}

// createContext creates a context with timeout for gRPC calls
func (e *GRPCExecutor) createContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 60*time.Second) // Longer timeout for tool execution
}

// Execute creates a handler that executes tools via gRPC.
//
// Input: JSON tool calls from AI (e.g., {"tool_calls": [{"name": "search", "arguments": "{\"query\": \"golang\"}"}]})
// Output: Tool execution results
// Behavior: TRANSFORM - executes tools remotely via gRPC
//
// Example:
//
//	executor, _ := tools.NewGRPCExecutor("tools-service:8080")
//	flow.Use(executor.Execute())
func (e *GRPCExecutor) Execute() calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Read tool calls from input
		var input []byte
		if err := calque.Read(req, &input); err != nil {
			return helpers.WrapError(err, "failed to read input")
		}

		// Parse tool calls
		toolCalls, err := e.parseToolCalls(input)
		if err != nil {
			return helpers.WrapError(err, "failed to parse tool calls")
		}

		// Check if there are any tool calls to execute
		if len(toolCalls) == 0 {
			// Return empty results if no tool calls
			_, err = res.Data.Write([]byte("[]"))
			return err
		}

		// Execute tools via gRPC
		results, err := e.executeTools(req.Context, toolCalls)
		if err != nil {
			return helpers.WrapError(err, "failed to execute tools")
		}

		// Write results
		resultJSON, err := json.Marshal(results)
		if err != nil {
			return helpers.WrapError(err, "failed to marshal results")
		}

		_, err = res.Data.Write(resultJSON)
		return err
	})
}

// parseToolCalls parses tool calls from JSON input
func (e *GRPCExecutor) parseToolCalls(input []byte) ([]*calquepb.ToolCall, error) {
	var toolCallData struct {
		ToolCalls []struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			ID        string `json:"id"`
		} `json:"tool_calls"`
	}

	if err := json.Unmarshal(input, &toolCallData); err != nil {
		return nil, helpers.WrapError(err, "failed to unmarshal tool calls")
	}

	toolCalls := make([]*calquepb.ToolCall, len(toolCallData.ToolCalls))
	for i, tc := range toolCallData.ToolCalls {
		toolCalls[i] = &calquepb.ToolCall{
			Name:      tc.Name,
			Arguments: tc.Arguments,
			Id:        tc.ID,
		}
	}

	return toolCalls, nil
}

// executeTools executes multiple tools via gRPC
func (e *GRPCExecutor) executeTools(ctx context.Context, toolCalls []*calquepb.ToolCall) ([]map[string]interface{}, error) {
	// Create context with timeout for tool execution
	execCtx, cancel := e.createContext(ctx)
	defer cancel()

	results := make([]map[string]interface{}, len(toolCalls))

	for i, toolCall := range toolCalls {
		req := &calquepb.ToolRequest{
			Name:      toolCall.Name,
			Arguments: toolCall.Arguments,
			Id:        toolCall.Id,
		}

		resp, err := e.client.ExecuteTool(execCtx, req)
		if err != nil {
			return nil, helpers.WrapGRPCError(err, "failed to execute tool", toolCall.Name)
		}

		if !resp.Success {
			return nil, helpers.NewGRPCInternalError("tool execution failed", nil)
		}

		// Parse result
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(resp.Result), &result); err != nil {
			return nil, helpers.WrapError(err, "failed to parse tool result")
		}

		results[i] = result
	}

	return results, nil
}

// Close closes the gRPC connection.
func (e *GRPCExecutor) Close() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}
