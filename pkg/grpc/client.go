// Package grpc provides common gRPC utilities and client management for go-calque.
package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Config holds configuration for gRPC client connections.
type Config struct {
	Endpoint    string
	Timeout     time.Duration
	Credentials credentials.TransportCredentials
	KeepAlive   *KeepAliveConfig
	Retry       *RetryConfig
}

// KeepAliveConfig configures gRPC keep-alive settings.
type KeepAliveConfig struct {
	Time                time.Duration
	Timeout             time.Duration
	PermitWithoutStream bool
}

// RetryConfig configures retry behavior for gRPC calls.
type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
}

// DefaultConfig returns a default gRPC client configuration.
func DefaultConfig(endpoint string) *Config {
	return &Config{
		Endpoint:    endpoint,
		Timeout:     30 * time.Second,
		Credentials: insecure.NewCredentials(),
		KeepAlive: &KeepAliveConfig{
			Time:                30 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		},
		Retry: &RetryConfig{
			MaxAttempts: 3,
			Backoff:     100 * time.Millisecond,
		},
	}
}

// NewClient creates a new gRPC client connection with the given configuration.
func NewClient(ctx context.Context, config *Config) (*grpc.ClientConn, error) {
	if config == nil {
		return nil, NewInvalidArgumentError(ctx, "grpc config cannot be nil", nil)
	}

	if config.Endpoint == "" {
		return nil, NewInvalidArgumentError(ctx, "grpc endpoint cannot be empty", nil)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(config.Credentials),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepAlive.Time,
			Timeout:             config.KeepAlive.Timeout,
			PermitWithoutStream: config.KeepAlive.PermitWithoutStream,
		}),
	}

	conn, err := grpc.NewClient(config.Endpoint, opts...)
	if err != nil {
		return nil, WrapError(ctx, err, "failed to connect to gRPC service", config.Endpoint)
	}

	return conn, nil
}

// NewClientWithTLS creates a new gRPC client with TLS credentials.
func NewClientWithTLS(ctx context.Context, endpoint string, _, _, caFile string) (*grpc.ClientConn, error) {
	creds, err := credentials.NewClientTLSFromFile(caFile, "")
	if err != nil {
		return nil, WrapError(ctx, err, "failed to load TLS credentials", caFile)
	}

	config := DefaultConfig(endpoint)
	config.Credentials = creds

	return NewClient(ctx, config)
}

// CloseConnection safely closes a gRPC connection with timeout.
func CloseConnection(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	if conn == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- conn.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			return WrapError(ctx, err, "failed to close gRPC connection")
		}
		return err
	case <-time.After(timeout):
		return NewDeadlineExceededError(ctx, "connection close timeout", nil)
	}
}
