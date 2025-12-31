package config

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDefaultGRPCConfig(t *testing.T) {
	config := DefaultGRPCConfig()

	tests := []struct {
		name     string
		check    func() bool
		expected bool
		message  string
	}{
		{
			name:     "services map initialized",
			check:    func() bool { return config.Services != nil },
			expected: true,
			message:  "Expected services map to be initialized",
		},
		{
			name:     "TLS disabled by default",
			check:    func() bool { return !config.Security.TLS.Enabled },
			expected: true,
			message:  "Expected TLS to be disabled by default",
		},
		{
			name:     "retry max attempts",
			check:    func() bool { return config.Retry.MaxAttempts == 3 },
			expected: true,
			message:  "Expected max attempts 3",
		},
		{
			name:     "log level",
			check:    func() bool { return config.Logging.Level == "info" },
			expected: true,
			message:  "Expected log level 'info'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.check() != tt.expected {
				t.Error(tt.message)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		checks      []struct {
			name     string
			check    func(*GRPCConfig) bool
			expected bool
			message  string
		}
	}{
		{
			name: "valid configuration",
			envVars: map[string]string{
				"GRPC_SERVICES":                "ai-service,memory-service",
				"GRPC_AI-SERVICE_ENDPOINT":     "localhost:8080",
				"GRPC_MEMORY-SERVICE_ENDPOINT": "localhost:8081",
				"GRPC_TLS_ENABLED":             "true",
				"GRPC_RETRY_MAX_ATTEMPTS":      "5",
			},
			expectError: false,
			checks: []struct {
				name     string
				check    func(*GRPCConfig) bool
				expected bool
				message  string
			}{
				{
					name:     "services count",
					check:    func(c *GRPCConfig) bool { return len(c.Services) == 2 },
					expected: true,
					message:  "Expected 2 services",
				},
				{
					name:     "ai-service exists",
					check:    func(c *GRPCConfig) bool { _, exists := c.Services["ai-service"]; return exists },
					expected: true,
					message:  "Expected ai-service to be configured",
				},
				{
					name:     "ai-service endpoint",
					check:    func(c *GRPCConfig) bool { return c.Services["ai-service"].Endpoint == "localhost:8080" },
					expected: true,
					message:  "Expected ai-service endpoint 'localhost:8080'",
				},
				{
					name:     "TLS enabled",
					check:    func(c *GRPCConfig) bool { return c.Security.TLS.Enabled },
					expected: true,
					message:  "Expected TLS to be enabled",
				},
				{
					name:     "retry max attempts",
					check:    func(c *GRPCConfig) bool { return c.Retry.MaxAttempts == 5 },
					expected: true,
					message:  "Expected max attempts 5",
				},
			},
		},
		{
			name: "empty services list",
			envVars: map[string]string{
				"GRPC_SERVICES": "",
			},
			expectError: false,
			checks: []struct {
				name     string
				check    func(*GRPCConfig) bool
				expected bool
				message  string
			}{
				{
					name:     "no services",
					check:    func(c *GRPCConfig) bool { return len(c.Services) == 0 },
					expected: true,
					message:  "Expected no services",
				},
			},
		},
		{
			name: "invalid TLS configuration",
			envVars: map[string]string{
				"GRPC_SERVICES":    "test-service",
				"GRPC_TLS_ENABLED": "invalid",
			},
			expectError: false,
			checks: []struct {
				name     string
				check    func(*GRPCConfig) bool
				expected bool
				message  string
			}{
				{
					name:     "TLS disabled on invalid value",
					check:    func(c *GRPCConfig) bool { return !c.Security.TLS.Enabled },
					expected: true,
					message:  "Expected TLS to be disabled on invalid value",
				},
			},
		},
		{
			name: "invalid retry configuration",
			envVars: map[string]string{
				"GRPC_SERVICES":           "test-service",
				"GRPC_RETRY_MAX_ATTEMPTS": "invalid",
			},
			expectError: false,
			checks: []struct {
				name     string
				check    func(*GRPCConfig) bool
				expected bool
				message  string
			}{
				{
					name:     "retry max attempts default",
					check:    func(c *GRPCConfig) bool { return c.Retry.MaxAttempts == 3 },
					expected: true,
					message:  "Expected default retry max attempts",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Clean up after test
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			config := DefaultGRPCConfig()
			err := config.LoadFromEnv()

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Run checks
			for _, check := range tt.checks {
				t.Run(check.name, func(t *testing.T) {
					if check.check(config) != check.expected {
						t.Error(check.message)
					}
				})
			}
		})
	}
}

func TestGetServiceConfig(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		setupConfig func(*GRPCConfig)
		expectError bool
		checks      []struct {
			name     string
			check    func(*ServiceConfig) bool
			expected bool
			message  string
		}
	}{
		{
			name:        "existing service",
			serviceName: "test-service",
			setupConfig: func(c *GRPCConfig) {
				c.Services["test-service"] = ServiceConfig{
					Endpoint: "localhost:8080",
					Timeout:  30 * time.Second,
				}
			},
			expectError: false,
			checks: []struct {
				name     string
				check    func(*ServiceConfig) bool
				expected bool
				message  string
			}{
				{
					name:     "endpoint",
					check:    func(s *ServiceConfig) bool { return s.Endpoint == "localhost:8080" },
					expected: true,
					message:  "Expected endpoint 'localhost:8080'",
				},
				{
					name:     "timeout",
					check:    func(s *ServiceConfig) bool { return s.Timeout == 30*time.Second },
					expected: true,
					message:  "Expected timeout 30s",
				},
			},
		},
		{
			name:        "non-existent service",
			serviceName: "non-existent",
			setupConfig: func(_ *GRPCConfig) {
			},
			expectError: true,
			checks:      nil,
		},
		{
			name:        "empty service name",
			serviceName: "",
			setupConfig: func(_ *GRPCConfig) {
			},
			expectError: true,
			checks:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultGRPCConfig()
			tt.setupConfig(config)

			serviceConfig, err := config.GetServiceConfig(context.Background(), tt.serviceName)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Run checks if no error expected
			if !tt.expectError {
				for _, check := range tt.checks {
					t.Run(check.name, func(t *testing.T) {
						if check.check(serviceConfig) != check.expected {
							t.Error(check.message)
						}
					})
				}
			}
		})
	}
}

func TestGetCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials CredentialsConfig
		expectError bool
	}{
		{
			name: "insecure",
			credentials: CredentialsConfig{
				Type: "insecure",
			},
			expectError: false,
		},
		{
			name: "tls with ca file",
			credentials: CredentialsConfig{
				Type:   "tls",
				CAFile: "ca.pem",
			},
			expectError: true,
		},
		{
			name: "tls without ca file",
			credentials: CredentialsConfig{
				Type: "tls",
			},
			expectError: true,
		},
		{
			name: "mtls with all files",
			credentials: CredentialsConfig{
				Type:     "mtls",
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
				CAFile:   "ca.pem",
			},
			expectError: true,
		},
		{
			name: "mtls missing files",
			credentials: CredentialsConfig{
				Type: "mtls",
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serviceConfig := &ServiceConfig{
				Credentials: test.credentials,
			}

			_, err := serviceConfig.GetCredentials(context.Background())
			if test.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
