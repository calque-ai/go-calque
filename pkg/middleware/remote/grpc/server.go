// Package grpc provides gRPC middleware for remote service integration in go-calque flows.
package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/calque-ai/go-calque/pkg/calque"
	calquepb "github.com/calque-ai/go-calque/proto"
)

// Server represents a gRPC server that can host calque flows.
type Server struct {
	server    *grpc.Server
	flows     map[string]*calque.Flow
	addr      string
	healthSrv *health.Server
	startTime time.Time
}

// NewServer creates a new gRPC server for hosting flows.
func NewServer(addr string) *Server {
	healthSrv := health.NewServer()
	return &Server{
		server:    grpc.NewServer(),
		flows:     make(map[string]*calque.Flow),
		addr:      addr,
		healthSrv: healthSrv,
		startTime: time.Now(),
	}
}

// RegisterFlow registers a flow with the server under a given name.
func (s *Server) RegisterFlow(name string, flow *calque.Flow) {
	s.flows[name] = flow
}

// GetFlow retrieves a registered flow by name.
func (s *Server) GetFlow(ctx context.Context, name string) (*calque.Flow, error) {
	flow, exists := s.flows[name]
	if !exists {
		return nil, calque.NewErr(ctx, fmt.Sprintf("flow %s not found", name))
	}
	return flow, nil
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	ctx := context.Background()
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return calque.WrapErr(ctx, err, fmt.Sprintf("failed to listen on %s", s.addr))
	}

	// Register health service
	grpc_health_v1.RegisterHealthServer(s.server, s.healthSrv)

	// Set all services as serving
	for service := range s.flows {
		s.healthSrv.SetServingStatus(service, grpc_health_v1.HealthCheckResponse_SERVING)
	}
	s.healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service for debugging
	reflection.Register(s.server)

	// Start serving
	return s.server.Serve(lis)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	s.server.GracefulStop()
}

// GetServer returns the underlying gRPC server for advanced configuration.
func (s *Server) GetServer() *grpc.Server {
	return s.server
}

// GetUptime returns the server uptime.
func (s *Server) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// GetHealthServer returns the health server for status management.
func (s *Server) GetHealthServer() *health.Server {
	return s.healthSrv
}

// SetServiceStatus sets the health status for a specific service.
func (s *Server) SetServiceStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	s.healthSrv.SetServingStatus(service, status)
}

// FlowService is a gRPC service that can execute calque flows.
type FlowService struct {
	calquepb.UnimplementedFlowServiceServer
	server *Server
}

// NewFlowService creates a new flow service.
func NewFlowService(server *Server) *FlowService {
	return &FlowService{server: server}
}

// ExecuteFlow executes a registered flow with the given input.
func (fs *FlowService) ExecuteFlow(ctx context.Context, req *calquepb.FlowRequest) (*calquepb.FlowResponse, error) {
	// Get the flow
	flow, err := fs.server.GetFlow(ctx, req.FlowName)
	if err != nil {
		return &calquepb.FlowResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to get flow %s: %v", req.FlowName, err),
		}, nil
	}

	// Execute the flow
	var result string
	err = flow.Run(ctx, req.Input, &result)
	if err != nil {
		return &calquepb.FlowResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to execute flow: %v", err),
		}, nil
	}

	return &calquepb.FlowResponse{
		Output:   result,
		Success:  true,
		Metadata: req.Metadata,
	}, nil
}

// StreamFlow executes a registered flow with bidirectional streaming.
func (fs *FlowService) StreamFlow(stream calquepb.FlowService_StreamFlowServer) error {
	// This is a placeholder implementation for streaming
	// In practice, this would handle bidirectional streaming with the flow

	for {
		req, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}

		// Get the flow
		flow, err := fs.server.GetFlow(stream.Context(), req.FlowName)
		if err != nil {
			resp := &calquepb.StreamingFlowResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to get flow %s: %v", req.FlowName, err),
				IsFinal:      true,
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			continue
		}

		// Execute the flow
		var result string
		err = flow.Run(stream.Context(), req.Input, &result)
		if err != nil {
			resp := &calquepb.StreamingFlowResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to execute flow: %v", err),
				IsFinal:      true,
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			continue
		}

		// Send response
		resp := &calquepb.StreamingFlowResponse{
			Output:   result,
			Success:  true,
			Metadata: req.Metadata,
			IsFinal:  true,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}
