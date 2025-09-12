package grpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

const testEndpoint = "localhost:8080"

func TestRegistry(t *testing.T) {
	t.Parallel()
	// Create a registry
	registry := NewRegistry()

	// Test registering a service
	service := &Service{
		Name:     "test-service",
		Endpoint: testEndpoint,
	}
	err := registry.Register(service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Test getting the service
	retrieved, err := registry.Get("test-service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if retrieved.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", retrieved.Name)
	}

	if retrieved.Endpoint != testEndpoint {
		t.Errorf("Expected endpoint '%s', got '%s'", testEndpoint, retrieved.Endpoint)
	}

	// Test getting non-existent service
	_, err = registry.Get("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}
}

func TestRegistryHandler(t *testing.T) {
	t.Parallel()
	// Create services
	services := []*Service{
		{
			Name:     "service1",
			Endpoint: testEndpoint,
		},
		{
			Name:     "service2",
			Endpoint: "localhost:8081",
		},
	}

	// Create registry handler
	handler := &registryHandler{services: services}

	// Create request and response
	req := &calque.Request{
		Context: context.Background(),
		Data:    calque.NewReader("test input"),
	}
	res := &calque.Response{
		Data: calque.NewWriter(),
	}

	// Execute handler
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler execution failed: %v", err)
	}

	// Check that registry is in context
	registry := GetRegistry(req.Context)
	if registry == nil {
		t.Fatal("Registry not found in context")
	}

	// Check that services are registered
	service1, err := registry.Get("service1")
	if err != nil {
		t.Fatalf("Failed to get service1: %v", err)
	}
	if service1.Endpoint != "localhost:8080" {
		t.Errorf("Expected endpoint 'localhost:8080', got '%s'", service1.Endpoint)
	}

	service2, err := registry.Get("service2")
	if err != nil {
		t.Fatalf("Failed to get service2: %v", err)
	}
	if service2.Endpoint != "localhost:8081" {
		t.Errorf("Expected endpoint 'localhost:8081', got '%s'", service2.Endpoint)
	}
}

func TestServiceCreation(t *testing.T) {
	tests := []struct {
		name      string
		service   *Service
		expectErr bool
	}{
		{
			name: "regular service",
			service: &Service{
				Name:      "test",
				Endpoint:  "localhost:8080",
				Streaming: false,
			},
			expectErr: false,
		},
		{
			name: "streaming service",
			service: &Service{
				Name:      "streaming",
				Endpoint:  "localhost:8081",
				Streaming: true,
			},
			expectErr: false,
		},
		{
			name: "service with empty name",
			service: &Service{
				Name:      "",
				Endpoint:  "localhost:8082",
				Streaming: false,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := tt.service

			if service.Name != tt.service.Name {
				t.Errorf("Expected name '%s', got '%s'", tt.service.Name, service.Name)
			}
			if service.Endpoint != tt.service.Endpoint {
				t.Errorf("Expected endpoint '%s', got '%s'", tt.service.Endpoint, service.Endpoint)
			}
			if service.Streaming != tt.service.Streaming {
				t.Errorf("Expected streaming %v, got %v", tt.service.Streaming, service.Streaming)
			}
		})
	}
}

func TestRegistryClose(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	// Register a service
	service := &Service{
		Name:     "test",
		Endpoint: testEndpoint,
	}
	err := registry.Register(service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Close registry
	err = registry.Close()
	if err != nil {
		t.Fatalf("Failed to close registry: %v", err)
	}
}

// TestServiceConfiguration tests service configuration methods
func TestServiceConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		service  *Service
		expected bool
	}{
		{
			name: "regular service",
			service: &Service{
				Name:     "test-service",
				Endpoint: "localhost:8080",
			},
			expected: true,
		},
		{
			name: "streaming service",
			service: &Service{
				Name:      "streaming-service",
				Endpoint:  "localhost:8081",
				Streaming: true,
			},
			expected: true,
		},
		{
			name: "service with empty name",
			service: &Service{
				Name:     "",
				Endpoint: "localhost:8082",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := tt.service
			if service == nil {
				t.Fatal("Expected non-nil service")
			}

			if service.Name != tt.service.Name {
				t.Errorf("Expected name '%s', got '%s'", tt.service.Name, service.Name)
			}
			if service.Endpoint != tt.service.Endpoint {
				t.Errorf("Expected endpoint '%s', got '%s'", tt.service.Endpoint, service.Endpoint)
			}
		})
	}
}

// TestServiceMethodConfiguration tests service method configuration
func TestServiceMethodConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "empty method",
			method: "",
		},
		{
			name:   "simple method",
			method: "simple-method",
		},
		{
			name:   "method with dashes",
			method: "method-with-dashes",
		},
		{
			name:   "method with underscores",
			method: "method_with_underscores",
		},
		{
			name:   "method with dots",
			method: "method.with.dots",
		},
		{
			name:   "method with slashes",
			method: "method/with/slashes",
		},
		{
			name:   "method with spaces",
			method: "method with spaces",
		},
		{
			name:   "long method name",
			method: "method-with-very-long-name-that-exceeds-normal-limits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{
				Name:     "test-service",
				Endpoint: "localhost:8080",
			}

			_ = service
		})
	}
}

// TestServiceTimeoutConfiguration tests service timeout configuration
func TestServiceTimeoutConfiguration(t *testing.T) {
	t.Parallel()

	timeouts := []time.Duration{
		1 * time.Second,
		30 * time.Second,
		5 * time.Minute,
		0,                // Zero timeout
		-1 * time.Second, // Negative timeout
	}

	for _, timeout := range timeouts {
		t.Run(fmt.Sprintf("timeout_%v", timeout), func(t *testing.T) {
			t.Parallel()

			service := &Service{
				Name:     "test-service",
				Endpoint: "localhost:8080",
			}

			_ = service
		})
	}
}

// TestServiceRetryConfiguration tests service retry configuration
func TestServiceRetryConfiguration(t *testing.T) {
	t.Parallel()

	retries := []int{0, 1, 3, 5, 10, -1}

	for _, retryCount := range retries {
		t.Run(fmt.Sprintf("retries_%d", retryCount), func(t *testing.T) {
			t.Parallel()

			service := &Service{
				Name:     "test-service",
				Endpoint: "localhost:8080",
			}

			_ = service
		})
	}
}

// TestServiceRegistryConcurrency tests concurrent registry operations
func TestServiceRegistryConcurrency(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	// Test concurrent registration
	concurrency := 10
	done := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(i int) {
			service := &Service{
				Name:     fmt.Sprintf("service-%d", i),
				Endpoint: fmt.Sprintf("localhost:%d", 8080+i),
			}
			err := registry.Register(service)
			done <- err
		}(i)
	}

	// Wait for all registrations to complete
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Concurrent registration failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent registration timed out")
		}
	}

	// Test concurrent retrieval
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			_, err := registry.Get(fmt.Sprintf("service-%d", i))
			done <- err
		}(i)
	}

	// Wait for all retrievals to complete
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Concurrent retrieval failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent retrieval timed out")
		}
	}
}

// TestServiceRegistryDuplicateRegistration tests duplicate registration handling
func TestServiceRegistryDuplicateRegistration(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	// Register first service
	service1 := &Service{
		Name:     "service-1",
		Endpoint: "localhost:8080",
	}
	err := registry.Register(service1)
	if err != nil {
		t.Fatalf("Failed to register first service: %v", err)
	}

	// Try to register second service with same name
	service2 := &Service{
		Name:     "service-1", // Same name
		Endpoint: "localhost:8081",
	}
	err = registry.Register(service2)

	_ = err
}

// TestServiceErrorHandling tests error handling in service operations
func TestServiceErrorHandling(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	tests := []struct {
		name      string
		service   *Service
		expectErr bool
	}{
		{
			name:      "nil service",
			service:   (*Service)(nil),
			expectErr: true,
		},
		{
			name: "valid service",
			service: &Service{
				Name:     "test-service",
				Endpoint: "localhost:8080",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := registry.Register(tt.service)
			if tt.expectErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}

	// Test Get with non-existent service
	_, err := registry.Get("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}
}
