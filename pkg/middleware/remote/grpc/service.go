// Package grpc provides gRPC middleware for remote service integration in go-calque flows.
//
// This package enables flows to communicate with remote gRPC services, providing:
// - Service registry for managing multiple gRPC connections
// - Type-safe service calls with protobuf serialization
// - Bidirectional streaming support
// - Integration with existing flow patterns
//
// Example usage:
//
//	// Register services
//	flow := calque.NewFlow().
//		Use(grpc.Registry(
//			grpc.Service("ai-service", "localhost:8080"),
//			grpc.Service("memory-service", "localhost:8081"),
//		)).
//		Use(grpc.Call("ai-service")).
//		Use(grpc.Call("memory-service"))
//
//	// Type-safe calls
//	flow := calque.NewFlow().
//		Use(grpc.ServiceWithTypes[Request, Response]("ai-service", "localhost:8080")).
//		Use(grpc.CallWithTypes[Request, Response]("ai-service"))
//
//	// Streaming services
//	flow := calque.NewFlow().
//		Use(grpc.StreamingService("streaming-service", "localhost:8082")).
//		Use(grpc.Stream("streaming-service"))
package grpc

import (
	"context"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
)

// Service represents a registered gRPC service with connection and metadata.
type Service struct {
	Name       string
	Endpoint   string
	Conn       *grpc.ClientConn
	Streaming  bool
	Method     string        // gRPC method name to call (e.g., "FlowService/ExecuteFlow")
	Timeout    time.Duration // Timeout for gRPC calls
	MaxRetries int           // Maximum number of retries for failed calls
	RetryDelay time.Duration // Delay between retries
}

// Registry manages multiple gRPC services and their connections.
type Registry struct {
	services map[string]*Service
	mu       sync.RWMutex
}

// NewRegistry creates a new gRPC service registry.
func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]*Service),
	}
}

// Register adds a service to the registry.
func (r *Registry) Register(service *Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Connect to the service if not already connected
	if service.Conn == nil {
		conn, err := grpc.NewClient(service.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return helpers.WrapErrorf(err, "failed to connect to service %s at %s", service.Name, service.Endpoint)
		}
		service.Conn = conn
	}

	r.services[service.Name] = service
	return nil
}

// Get retrieves a service by name.
func (r *Registry) Get(name string) (*Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[name]
	if !exists {
		return nil, helpers.NewError("service %s not found in registry", name)
	}
	return service, nil
}

// Close closes all service connections.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, service := range r.services {
		if service.Conn != nil {
			if err := service.Conn.Close(); err != nil {
				errs = append(errs, helpers.WrapErrorf(err, "failed to close connection for service %s", service.Name))
			}
		}
	}

	if len(errs) > 0 {
		return helpers.NewError("errors closing services: %v", errs)
	}
	return nil
}

// NewService creates a new gRPC service configuration.
func NewService(name, endpoint string) *Service {
	return &Service{
		Name:       name,
		Endpoint:   endpoint,
		Streaming:  false,
		Method:     "FlowService/ExecuteFlow", // Default method
		Timeout:    30 * time.Second,          // Default timeout
		MaxRetries: 3,                         // Default retries
		RetryDelay: 1 * time.Second,           // Default retry delay
	}
}

// ServiceWithTypes creates a type-safe gRPC service configuration.
func ServiceWithTypes[TReq, TResp proto.Message](name, endpoint string) *Service {
	return &Service{
		Name:       name,
		Endpoint:   endpoint,
		Streaming:  false,
		Method:     "FlowService/ExecuteFlow", // Default method
		Timeout:    30 * time.Second,          // Default timeout
		MaxRetries: 3,                         // Default retries
		RetryDelay: 1 * time.Second,           // Default retry delay
	}
}

// StreamingService creates a streaming gRPC service configuration.
func StreamingService(name, endpoint string) *Service {
	return &Service{
		Name:       name,
		Endpoint:   endpoint,
		Streaming:  true,
		Method:     "FlowService/StreamFlow", // Default streaming method
		Timeout:    60 * time.Second,         // Longer timeout for streaming
		MaxRetries: 3,                        // Default retries
		RetryDelay: 1 * time.Second,          // Default retry delay
	}
}

// NewRegistryHandler creates a handler that registers multiple gRPC services.
//
// Input: any data (streaming - passes through unchanged)
// Output: same as input (pass-through)
// Behavior: STREAMING - registers services in context for downstream handlers
//
// Example:
//
//	flow := calque.NewFlow().
//		Use(grpc.NewRegistryHandler(
//			grpc.NewService("ai-service", "localhost:8080"),
//			grpc.NewService("memory-service", "localhost:8081"),
//		))
func NewRegistryHandler(services ...*Service) calque.Handler {
	return &registryHandler{services: services}
}

// registryHandler implements the registry with services stored as instance data
type registryHandler struct {
	services []*Service
}

func (rh *registryHandler) ServeFlow(req *calque.Request, res *calque.Response) error {
	// Create a registry and register all services
	registry := NewRegistry()
	for _, service := range rh.services {
		if err := registry.Register(service); err != nil {
			return helpers.WrapErrorf(err, "failed to register service %s", service.Name)
		}
	}

	// Store registry in context for downstream handlers
	ctx := context.WithValue(req.Context, registryContextKey{}, registry)
	req.Context = ctx

	// Pass input through unchanged
	_, err := io.Copy(res.Data, req.Data)
	return err
}

// registryContextKey is used to store the gRPC registry in context
type registryContextKey struct{}

// GetRegistry retrieves the gRPC registry from the context.
func GetRegistry(ctx context.Context) *Registry {
	if registry, ok := ctx.Value(registryContextKey{}).(*Registry); ok {
		return registry
	}
	return nil
}

// WithMethod sets the gRPC method for a service.
func (s *Service) WithMethod(method string) *Service {
	s.Method = method
	return s
}

// WithTimeout sets the timeout for gRPC calls.
func (s *Service) WithTimeout(timeout time.Duration) *Service {
	s.Timeout = timeout
	return s
}

// WithRetries sets the retry configuration for gRPC calls.
func (s *Service) WithRetries(maxRetries int, retryDelay time.Duration) *Service {
	s.MaxRetries = maxRetries
	s.RetryDelay = retryDelay
	return s
}
