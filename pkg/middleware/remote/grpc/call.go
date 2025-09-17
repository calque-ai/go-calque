// Package grpc provides gRPC middleware for remote service integration in go-calque flows.
package grpc

import (
	"context"
	"io"
	"reflect"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/calque-ai/go-calque/pkg/calque"
	grpcerrors "github.com/calque-ai/go-calque/pkg/grpc"
	calquepb "github.com/calque-ai/go-calque/proto"
)

// Call creates a handler that calls a registered gRPC service.
//
// Input: protobuf message data (streaming)
// Output: protobuf message response (streaming)
// Behavior: STREAMING - converts input to protobuf, calls service, converts response
//
// The service must be registered using grpc.NewRegistryHandler() before this handler.
// The input data is expected to be a protobuf message that can be unmarshaled.
//
// Example:
//
//	flow := calque.NewFlow().
//		Use(grpc.NewRegistryHandler(grpc.NewService("ai-service", "localhost:8080"))).
//		Use(grpc.Call("ai-service"))
func Call(serviceName string) calque.Handler {
	return &callHandler{serviceName: serviceName}
}

// CallWithTypes creates a type-safe handler that calls a registered gRPC service.
//
// Input: TReq protobuf message data (streaming)
// Output: TResp protobuf message response (streaming)
// Behavior: STREAMING - type-safe protobuf conversion and service call
//
// This provides compile-time type safety for the request and response types.
// The service must be registered using grpc.NewRegistryHandler() before this handler.
//
// Example:
//
//	type Request struct {
//		Text string `protobuf:"bytes,1,opt,name=text,proto3"`
//	}
//	type Response struct {
//		Result string `protobuf:"bytes,1,opt,name=result,proto3"`
//	}
//
//	flow := calque.NewFlow().
//		Use(grpcerrors.ServiceWithTypes[Request, Response]("ai-service", "localhost:8080")).
//		Use(grpcerrors.CallWithTypes[Request, Response]("ai-service"))
func CallWithTypes[TReq, TResp proto.Message](serviceName string) calque.Handler {
	return &typedCallHandler[TReq, TResp]{serviceName: serviceName}
}

// callHandler implements generic gRPC service calls
type callHandler struct {
	serviceName string
}

func (ch *callHandler) ServeFlow(req *calque.Request, res *calque.Response) error {
	// Get registry from context
	registry := GetRegistry(req.Context)
	if registry == nil {
		return grpcerrors.NewErrorSimple("gRPC registry not found in context, ensure grpcmw.NewRegistryHandler() is used before grpcmw.Call()")
	}

	// Get service from registry
	service, err := registry.Get(ch.serviceName)
	if err != nil {
		return grpcerrors.WrapErrorfSimple(err, "failed to get service %s", ch.serviceName)
	}

	// Read input data
	var inputData []byte
	if err := calque.Read(req, &inputData); err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to read input data")
	}

	// Create a flow request
	// Map service name to flow name (e.g., "ai-service" -> "ai-flow")
	flowName := strings.TrimSuffix(ch.serviceName, "-service") + "-flow"
	flowReq := &calquepb.FlowRequest{
		Version:  1,
		FlowName: flowName,
		Input:    string(inputData),
		Metadata: map[string]string{
			"service": ch.serviceName,
		},
	}

	// Marshal the request
	reqData, err := proto.Marshal(flowReq)
	if err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to marshal request")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(req.Context, service.Timeout)
	defer cancel()

	// Make the gRPC call with retries
	var flowResp *calquepb.FlowResponse
	for attempt := 0; attempt <= service.MaxRetries; attempt++ {
		flowResp, err = ch.makeGRPCCall(ctx, service, reqData)
		if err == nil {
			break
		}

		// Check if error is retryable
		if !isRetryableError(err) || attempt == service.MaxRetries {
			return grpcerrors.WrapErrorSimple(err, "gRPC call failed")
		}

		// Wait before retry
		time.Sleep(service.RetryDelay)
	}

	// Marshal the response
	respData, err := proto.Marshal(flowResp)
	if err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to marshal response")
	}

	// Write response
	_, err = res.Data.Write(respData)
	return err
}

// makeGRPCCall performs the actual gRPC call
func (ch *callHandler) makeGRPCCall(ctx context.Context, service *Service, reqData []byte) (*calquepb.FlowResponse, error) {
	// Create a new gRPC client for the service
	client := calquepb.NewFlowServiceClient(service.Conn)

	// Unmarshal the request data back to FlowRequest
	var flowReq calquepb.FlowRequest
	if err := proto.Unmarshal(reqData, &flowReq); err != nil {
		return nil, grpcerrors.WrapErrorSimple(err, "failed to unmarshal request")
	}

	// Make the unary gRPC call
	flowResp, err := client.ExecuteFlow(ctx, &flowReq)
	if err != nil {
		return nil, grpcerrors.WrapError(err, "gRPC ExecuteFlow failed")
	}

	return flowResp, nil
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a GRPCError from grpc package
	if grpcErr, ok := err.(*grpcerrors.Error); ok {
		return grpcErr.IsRetryable()
	}

	// Check gRPC status codes
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
			return true
		case codes.Internal, codes.Aborted:
			return true
		default:
			return false
		}
	}

	// Check for context errors
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false
	}

	return false
}

// typedCallHandler implements type-safe gRPC service calls
type typedCallHandler[TReq, TResp proto.Message] struct {
	serviceName string
}

func (tch *typedCallHandler[TReq, TResp]) ServeFlow(req *calque.Request, res *calque.Response) error {
	// Get registry from context
	registry := GetRegistry(req.Context)
	if registry == nil {
		return grpcerrors.NewErrorSimple("gRPC registry not found in context, ensure grpcmw.NewRegistryHandler() is used before grpcmw.CallWithTypes()")
	}

	// Get service from registry
	service, err := registry.Get(tch.serviceName)
	if err != nil {
		return grpcerrors.WrapErrorfSimple(err, "failed to get service %s", tch.serviceName)
	}

	// Read input data as string first, then convert to bytes if needed
	var inputStr string
	if err := calque.Read(req, &inputStr); err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to read input data")
	}
	inputData := []byte(inputStr)

	// Create typed request from input data
	var reqMsg TReq
	reqMsg = reflect.New(reflect.TypeOf(reqMsg).Elem()).Interface().(TReq)

	// For now, we'll create a simple request with the input data
	// In a real implementation, you might want to parse the input data differently
	if aiReq, ok := any(reqMsg).(*calquepb.AIRequest); ok {
		aiReq.Prompt = string(inputData)
	} else if memReq, ok := any(reqMsg).(*calquepb.MemoryRequest); ok {
		memReq.Operation = "get"
		memReq.Key = "default"
		memReq.Value = string(inputData)
	} else {
		// For other types, try to unmarshal as protobuf
		if err := proto.Unmarshal(inputData, reqMsg); err != nil {
			return grpcerrors.WrapErrorSimple(err, "failed to unmarshal request")
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(req.Context, service.Timeout)
	defer cancel()

	// Make the gRPC call with retries
	var respMsg TResp
	for attempt := 0; attempt <= service.MaxRetries; attempt++ {
		respMsg, err = tch.makeTypedGRPCCall(ctx, service, reqMsg)
		if err == nil {
			break
		}

		// Check if error is retryable
		if !isRetryableError(err) || attempt == service.MaxRetries {
			return grpcerrors.WrapErrorSimple(err, "typed gRPC call failed")
		}

		// Wait before retry
		time.Sleep(service.RetryDelay)
	}

	// Convert response to string for the flow
	var responseStr string
	if aiResp, ok := any(respMsg).(*calquepb.AIResponse); ok {
		responseStr = aiResp.Response
	} else if memResp, ok := any(respMsg).(*calquepb.MemoryResponse); ok {
		responseStr = memResp.Value
	} else if toolResp, ok := any(respMsg).(*calquepb.ToolResponse); ok {
		responseStr = toolResp.Result
	} else {
		// For other types, marshal to JSON for readability
		respData, err := proto.Marshal(respMsg)
		if err != nil {
			return grpcerrors.WrapErrorSimple(err, "failed to marshal response")
		}
		responseStr = string(respData)
	}

	// Write response as string
	return calque.Write(res, responseStr)
}

// makeTypedGRPCCall performs the actual typed gRPC call
func (tch *typedCallHandler[TReq, TResp]) makeTypedGRPCCall(ctx context.Context, service *Service, reqMsg TReq) (TResp, error) {
	// Call the appropriate service method based on service name
	switch tch.serviceName {
	case "ai-service":
		return tch.callAIService(ctx, service, reqMsg)
	case "memory-service":
		return tch.callMemoryService(ctx, service, reqMsg)
	case "tools-service":
		return tch.callToolsService(ctx, service, reqMsg)
	default:
		// Fallback to FlowService for unknown services
		return tch.callFlowService(ctx, service, reqMsg)
	}
}

// callAIService calls the AI service StreamChat method
func (tch *typedCallHandler[TReq, TResp]) callAIService(ctx context.Context, service *Service, reqMsg TReq) (TResp, error) {
	client := calquepb.NewAIServiceClient(service.Conn)

	// Convert to AIRequest
	aiReq, ok := any(reqMsg).(*calquepb.AIRequest)
	if !ok {
		return *new(TResp), grpcerrors.NewErrorSimple("invalid request type for AI service")
	}

	// Call StreamChat and get the streaming client
	stream, err := client.StreamChat(ctx, aiReq)
	if err != nil {
		return *new(TResp), grpcerrors.WrapError(err, "AI service StreamChat failed", tch.serviceName)
	}

	// Read the first response from the stream
	resp, err := stream.Recv()
	if err != nil {
		return *new(TResp), grpcerrors.WrapError(err, "failed to receive AI response", tch.serviceName)
	}

	// Convert response
	respMsg, ok := any(resp).(TResp)
	if !ok {
		return *new(TResp), grpcerrors.NewErrorSimple("invalid response type for AI service")
	}

	return respMsg, nil
}

// callMemoryService calls the Memory service ProcessMemory method
func (tch *typedCallHandler[TReq, TResp]) callMemoryService(ctx context.Context, service *Service, reqMsg TReq) (TResp, error) {
	client := calquepb.NewMemoryServiceClient(service.Conn)

	// Convert to MemoryRequest
	memReq, ok := any(reqMsg).(*calquepb.MemoryRequest)
	if !ok {
		return *new(TResp), grpcerrors.NewErrorSimple("invalid request type for Memory service")
	}

	// Make the unary gRPC call
	memResp, err := client.ProcessMemory(ctx, memReq)
	if err != nil {
		return *new(TResp), grpcerrors.WrapError(err, "Memory service ProcessMemory failed", tch.serviceName)
	}

	// Convert response
	respMsg, ok := any(memResp).(TResp)
	if !ok {
		return *new(TResp), grpcerrors.NewErrorSimple("invalid response type for Memory service")
	}

	return respMsg, nil
}

// callToolsService calls the Tools service ExecuteTool method
func (tch *typedCallHandler[TReq, TResp]) callToolsService(ctx context.Context, service *Service, reqMsg TReq) (TResp, error) {
	client := calquepb.NewToolsServiceClient(service.Conn)

	// Convert to ToolRequest
	toolReq, ok := any(reqMsg).(*calquepb.ToolRequest)
	if !ok {
		return *new(TResp), grpcerrors.NewErrorSimple("invalid request type for Tools service")
	}

	// Make the unary gRPC call
	toolResp, err := client.ExecuteTool(ctx, toolReq)
	if err != nil {
		return *new(TResp), grpcerrors.WrapError(err, "Tools service ExecuteTool failed", tch.serviceName)
	}

	// Convert response
	respMsg, ok := any(toolResp).(TResp)
	if !ok {
		return *new(TResp), grpcerrors.NewErrorSimple("invalid response type for Tools service")
	}

	return respMsg, nil
}

// callFlowService calls the FlowService ExecuteFlow method (fallback)
func (tch *typedCallHandler[TReq, TResp]) callFlowService(ctx context.Context, service *Service, reqMsg TReq) (TResp, error) {
	client := calquepb.NewFlowServiceClient(service.Conn)

	// Convert typed request to FlowRequest
	reqData, err := proto.Marshal(reqMsg)
	if err != nil {
		return *new(TResp), grpcerrors.WrapErrorSimple(err, "failed to marshal typed request")
	}

	flowReq := &calquepb.FlowRequest{
		Input: string(reqData),
		Metadata: map[string]string{
			"service": tch.serviceName,
		},
	}

	// Make the unary gRPC call
	flowResp, err := client.ExecuteFlow(ctx, flowReq)
	if err != nil {
		return *new(TResp), grpcerrors.WrapError(err, "gRPC ExecuteFlow failed", tch.serviceName)
	}

	// Convert FlowResponse to typed response
	var respMsg TResp
	respMsg = reflect.New(reflect.TypeOf(respMsg).Elem()).Interface().(TResp)
	if err := proto.Unmarshal([]byte(flowResp.Output), respMsg); err != nil {
		return *new(TResp), grpcerrors.WrapErrorSimple(err, "failed to unmarshal typed response")
	}

	return respMsg, nil
}

// Stream creates a handler for streaming gRPC service calls.
//
// Input: protobuf message data (streaming)
// Output: protobuf message response (streaming)
// Behavior: STREAMING - bidirectional streaming with gRPC service
//
// The service must be registered as a streaming service using grpcerrors.StreamingService().
//
// Example:
//
//	flow := calque.NewFlow().
//		Use(grpcerrors.Registry(grpcerrors.StreamingService("streaming-service", "localhost:8082"))).
//		Use(grpcerrors.Stream("streaming-service"))
func Stream(serviceName string) calque.Handler {
	return &streamHandler{serviceName: serviceName}
}

// streamHandler implements streaming gRPC service calls
type streamHandler struct {
	serviceName string
}

func (sh *streamHandler) ServeFlow(req *calque.Request, res *calque.Response) error {
	// Get registry from context
	registry := GetRegistry(req.Context)
	if registry == nil {
		return grpcerrors.NewErrorSimple("gRPC registry not found in context, ensure grpcmw.NewRegistryHandler() is used before grpcmw.Stream()")
	}

	// Get service from registry
	service, err := registry.Get(sh.serviceName)
	if err != nil {
		return grpcerrors.WrapErrorfSimple(err, "failed to get service %s", sh.serviceName)
	}

	if !service.Streaming {
		return grpcerrors.NewErrorSimple("service %s is not configured for streaming", sh.serviceName)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(req.Context, service.Timeout)
	defer cancel()

	// Read input data as string
	var inputStr string
	if err := calque.Read(req, &inputStr); err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to read input data")
	}

	// Call the appropriate streaming service method based on service name
	switch sh.serviceName {
	case "ai-service":
		return sh.streamAIService(ctx, service, inputStr, res)
	case "memory-service":
		return sh.streamMemoryService(ctx, service, inputStr, res)
	case "tools-service":
		return sh.streamToolsService(ctx, service, inputStr, res)
	default:
		return grpcerrors.NewErrorSimple("streaming not supported for service %s", sh.serviceName)
	}
}

// streamAIService streams from the AI service
func (sh *streamHandler) streamAIService(ctx context.Context, service *Service, input string, res *calque.Response) error {
	client := calquepb.NewAIServiceClient(service.Conn)

	// Create AI request
	aiReq := &calquepb.AIRequest{
		Prompt: input,
	}

	// Call StreamChat and get the streaming client
	stream, err := client.StreamChat(ctx, aiReq)
	if err != nil {
		return grpcerrors.WrapError(err, "failed to create AI streaming client", sh.serviceName)
	}

	// Read responses from the stream
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return grpcerrors.WrapError(err, "failed to receive AI streaming response", sh.serviceName)
		}

		// Write response data as string
		if err := calque.Write(res, resp.Response); err != nil {
			return grpcerrors.WrapErrorSimple(err, "failed to write AI response")
		}
	}

	return nil
}

// streamMemoryService streams from the Memory service (simulated streaming)
func (sh *streamHandler) streamMemoryService(ctx context.Context, service *Service, input string, res *calque.Response) error {
	client := calquepb.NewMemoryServiceClient(service.Conn)

	// Create memory request
	memReq := &calquepb.MemoryRequest{
		Operation: "get",
		Key:       "streaming-key",
		Value:     input,
	}

	// Call ProcessMemory (unary call, but we'll simulate streaming)
	memResp, err := client.ProcessMemory(ctx, memReq)
	if err != nil {
		return grpcerrors.WrapError(err, "failed to process memory", sh.serviceName)
	}

	// Write response as string (simulated streaming)
	if err := calque.Write(res, memResp.Value); err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to write memory response")
	}

	return nil
}

// streamToolsService streams from the Tools service (simulated streaming)
func (sh *streamHandler) streamToolsService(ctx context.Context, service *Service, input string, res *calque.Response) error {
	client := calquepb.NewToolsServiceClient(service.Conn)

	// Create tool request
	toolReq := &calquepb.ToolRequest{
		Name:      "streaming-tool",
		Arguments: input,
	}

	// Call ExecuteTool (unary call, but we'll simulate streaming)
	toolResp, err := client.ExecuteTool(ctx, toolReq)
	if err != nil {
		return grpcerrors.WrapError(err, "failed to execute tool", sh.serviceName)
	}

	// Write response as string (simulated streaming)
	if err := calque.Write(res, toolResp.Result); err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to write tool response")
	}

	return nil
}
