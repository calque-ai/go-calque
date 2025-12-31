// Package remote provides common interfaces and utilities for remote service communication.
package remote

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Client represents a remote service client interface.
type Client interface {
	// Call performs a unary RPC call.
	Call(ctx context.Context, method string, req, resp proto.Message) error

	// Stream performs a streaming RPC call.
	Stream(ctx context.Context, method string) (Stream, error)

	// Close closes the client connection.
	Close() error

	// IsHealthy checks if the client is healthy.
	IsHealthy(ctx context.Context) error
}

// Stream represents a bidirectional stream interface.
type Stream interface {
	// Send sends a message through the stream.
	Send(msg proto.Message) error

	// Recv receives a message from the stream.
	Recv() (proto.Message, error)

	// CloseSend closes the send side of the stream.
	CloseSend() error

	// Context returns the stream context.
	Context() context.Context
}

// Config holds configuration for remote clients.
type Config struct {
	Endpoint    string
	Timeout     time.Duration
	Retry       *RetryConfig
	HealthCheck *HealthCheckConfig
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
	MaxBackoff  time.Duration
}

// HealthCheckConfig configures health checking.
type HealthCheckConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

// DefaultConfig returns a default remote client configuration.
func DefaultConfig(endpoint string) *Config {
	return &Config{
		Endpoint: endpoint,
		Timeout:  30 * time.Second,
		Retry: &RetryConfig{
			MaxAttempts: 3,
			Backoff:     100 * time.Millisecond,
			MaxBackoff:  5 * time.Second,
		},
		HealthCheck: &HealthCheckConfig{
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
		},
	}
}

// ClientManager manages multiple remote clients.
type ClientManager struct {
	clients map[string]Client
	configs map[string]*Config
}

// NewClientManager creates a new client manager.
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]Client),
		configs: make(map[string]*Config),
	}
}

// RegisterClient registers a client with the manager.
func (cm *ClientManager) RegisterClient(name string, client Client, config *Config) {
	cm.clients[name] = client
	cm.configs[name] = config
}

// GetClient retrieves a client by name.
func (cm *ClientManager) GetClient(ctx context.Context, name string) (Client, error) {
	client, exists := cm.clients[name]
	if !exists {
		return nil, calque.NewErr(ctx, fmt.Sprintf("client %s not found", name))
	}
	return client, nil
}

// CloseAll closes all managed clients.
func (cm *ClientManager) CloseAll() error {
	ctx := context.Background()
	var errs []error
	for name, client := range cm.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, calque.WrapErr(ctx, err, fmt.Sprintf("failed to close client %s", name)))
		}
	}

	if len(errs) > 0 {
		return calque.NewErr(ctx, fmt.Sprintf("errors closing clients: %v", errs))
	}
	return nil
}

// HealthCheck performs health checks on all clients.
func (cm *ClientManager) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)

	for name, client := range cm.clients {
		results[name] = client.IsHealthy(ctx)
	}

	return results
}
