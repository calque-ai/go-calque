// Package grpc provides gRPC middleware for remote service integration in go-calque flows.
package grpc

import (
	"context"
	"io"
	"reflect"
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
		return grpcerrors.NewErrorSimple("gRPC registry not found in context, ensure grpcerrors.NewRegistryHandler() is used before grpcerrors.Call()")
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
	flowReq := &calquepb.FlowRequest{
		Input: string(inputData),
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

	// Create the request
	flowReq := &calquepb.FlowRequest{
		Input: string(reqData),
		Metadata: map[string]string{
			"service": ch.serviceName,
		},
	}

	// Make the unary gRPC call
	flowResp, err := client.ExecuteFlow(ctx, flowReq)
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
		return grpcerrors.NewErrorSimple("gRPC registry not found in context, ensure grpcerrors.NewRegistryHandler() is used before grpcerrors.CallWithTypes()")
	}

	// Get service from registry
	service, err := registry.Get(tch.serviceName)
	if err != nil {
		return grpcerrors.WrapErrorfSimple(err, "failed to get service %s", tch.serviceName)
	}

	// Read input data
	var inputData []byte
	if err := calque.Read(req, &inputData); err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to read input data")
	}

	// Unmarshal input to typed request
	var reqMsg TReq
	reqMsg = reflect.New(reflect.TypeOf(reqMsg).Elem()).Interface().(TReq)
	if err := proto.Unmarshal(inputData, reqMsg); err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to unmarshal request")
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

	// Marshal the response
	respData, err := proto.Marshal(respMsg)
	if err != nil {
		return grpcerrors.WrapErrorSimple(err, "failed to marshal response")
	}

	// Write response
	_, err = res.Data.Write(respData)
	return err
}

// makeTypedGRPCCall performs the actual typed gRPC call
func (tch *typedCallHandler[TReq, TResp]) makeTypedGRPCCall(ctx context.Context, service *Service, reqMsg TReq) (TResp, error) {
	// Create a new gRPC client for the service
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
		return grpcerrors.NewErrorSimple("gRPC registry not found in context, ensure grpcerrors.NewRegistryHandler() is used before grpcerrors.Stream()")
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

	// Create a new gRPC client for the service
	client := calquepb.NewFlowServiceClient(service.Conn)

	// Create streaming client
	stream, err := client.StreamFlow(ctx)
	if err != nil {
		return grpcerrors.WrapError(err, "failed to create streaming client", sh.serviceName)
	}
	defer func() {
		_ = stream.CloseSend()
	}()

	// Channel to handle streaming responses
	responseChan := make(chan *calquepb.StreamingFlowResponse, 10)
	errorChan := make(chan error, 1)

	// Start goroutine to receive responses
	go func() {
		defer close(responseChan)
		defer close(errorChan)

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				errorChan <- grpcerrors.WrapError(err, "failed to receive streaming response", sh.serviceName)
				return
			}
			responseChan <- resp
		}
	}()

	// Start goroutine to send requests
	go func() {
		defer func() {
			_ = stream.CloseSend()
		}()

		// Read input data in chunks
		buffer := make([]byte, 4096)
		for {
			n, err := req.Data.Read(buffer)
			if err != nil {
				if err == io.EOF {
					break
				}
				errorChan <- grpcerrors.WrapErrorSimple(err, "failed to read input data")
				return
			}

			// Create streaming request
			streamReq := &calquepb.StreamingFlowRequest{
				Input: string(buffer[:n]),
				Metadata: map[string]string{
					"service": sh.serviceName,
				},
			}

			// Send the request
			if err := stream.Send(streamReq); err != nil {
				errorChan <- grpcerrors.WrapError(err, "failed to send streaming request", sh.serviceName)
				return
			}
		}
	}()

	// Handle responses and errors
	for {
		select {
		case resp, ok := <-responseChan:
			if !ok {
				// Channel closed, streaming finished
				return nil
			}

			// Write response data
			if _, err := res.Data.Write([]byte(resp.Output)); err != nil {
				return grpcerrors.WrapErrorSimple(err, "failed to write response")
			}

			// Check if this is the final response
			if resp.IsFinal {
				return nil
			}

		case err := <-errorChan:
			return err

		case <-ctx.Done():
			return grpcerrors.WrapErrorSimple(ctx.Err(), "streaming context cancelled")
		}
	}
}
