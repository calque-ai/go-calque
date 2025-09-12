package grpc

import (
	"context"
	"testing"

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
