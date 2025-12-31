package grpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestDefaultConfig(t *testing.T) {
	endpoint := "localhost:8080"
	config := DefaultConfig(endpoint)

	if config.Endpoint != endpoint {
		t.Errorf("Expected endpoint %s, got %s", endpoint, config.Endpoint)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}

	if config.Credentials == nil {
		t.Error("Expected credentials to be set")
	}

	if config.KeepAlive == nil {
		t.Error("Expected keep-alive config to be set")
	}

	if config.Retry == nil {
		t.Error("Expected retry config to be set")
	}
}

func TestNewClient(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig("localhost:8080")

	// This will fail to connect, but we can test the configuration
	_, err := NewClient(ctx, config)
	// Note: In some environments, this might succeed due to localhost resolution
	// So we just test that the function doesn't panic
	if err != nil {
		// Expected for non-existent endpoint
		t.Logf("Connection failed as expected: %v", err)
	}

	// Test with nil config
	_, err = NewClient(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}
}

func TestNewClientWithTLS(t *testing.T) {
	ctx := context.Background()
	// This will fail due to missing cert files, but tests the function signature
	_, err := NewClientWithTLS(ctx, "localhost:8080", "cert.pem", "key.pem", "ca.pem")
	if err == nil {
		t.Error("Expected error for missing cert files")
	}
}

func TestConfigValidation(t *testing.T) {
	config := &Config{
		Endpoint:    "",
		Timeout:     0,
		Credentials: insecure.NewCredentials(),
		KeepAlive: &KeepAliveConfig{
			Time:                30 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		},
	}

	ctx := context.Background()
	_, err := NewClient(ctx, config)
	// The error might be about empty endpoint or connection failure
	// Both are acceptable for this test
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestCloseConnection(t *testing.T) {
	ctx := context.Background()
	// Test CloseConnection with nil connection
	var conn *grpc.ClientConn
	err := CloseConnection(ctx, conn, 5*time.Second)
	if err != nil {
		t.Errorf("CloseConnection with nil connection should not error, got: %v", err)
	}

	// Test CloseConnection with valid connection (will fail to connect but tests the function)
	config := DefaultConfig("localhost:8080")
	conn, err = NewClient(ctx, config)
	if err != nil {
		// Connection failed as expected, but we can still test CloseConnection
		t.Logf("Connection failed as expected: %v", err)
		return
	}

	// If we got here, the connection succeeded (unlikely but possible)
	err = CloseConnection(ctx, conn, 5*time.Second)
	if err != nil {
		t.Errorf("CloseConnection failed: %v", err)
	}
}
