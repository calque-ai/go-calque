// Package config provides configuration management for go-calque services.
package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
)

// GRPCConfig holds configuration for gRPC services.
type GRPCConfig struct {
	Services map[string]ServiceConfig `yaml:"services" json:"services"`
	Security SecurityConfig           `yaml:"security" json:"security"`
	Retry    RetryConfig              `yaml:"retry" json:"retry"`
	Logging  LoggingConfig            `yaml:"logging" json:"logging"`
}

// ServiceConfig holds configuration for a specific gRPC service.
type ServiceConfig struct {
	Endpoint    string            `yaml:"endpoint" json:"endpoint"`
	Timeout     time.Duration     `yaml:"timeout" json:"timeout"`
	Credentials CredentialsConfig `yaml:"credentials" json:"credentials"`
	KeepAlive   KeepAliveConfig   `yaml:"keepalive" json:"keepalive"`
	Retry       *RetryConfig      `yaml:"retry,omitempty" json:"retry,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	TLS TLSConfig `yaml:"tls" json:"tls"`
}

// TLSConfig holds TLS configuration.
type TLSConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	CertFile   string `yaml:"cert_file" json:"cert_file"`
	KeyFile    string `yaml:"key_file" json:"key_file"`
	CAFile     string `yaml:"ca_file" json:"ca_file"`
	ServerName string `yaml:"server_name" json:"server_name"`
	Insecure   bool   `yaml:"insecure" json:"insecure"`
}

// CredentialsConfig holds credentials configuration.
type CredentialsConfig struct {
	Type     string            `yaml:"type" json:"type"` // "insecure", "tls", "mtls"
	CertFile string            `yaml:"cert_file" json:"cert_file"`
	KeyFile  string            `yaml:"key_file" json:"key_file"`
	CAFile   string            `yaml:"ca_file" json:"ca_file"`
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// KeepAliveConfig holds keep-alive configuration.
type KeepAliveConfig struct {
	Time                time.Duration `yaml:"time" json:"time"`
	Timeout             time.Duration `yaml:"timeout" json:"timeout"`
	PermitWithoutStream bool          `yaml:"permit_without_stream" json:"permit_without_stream"`
}

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxAttempts int           `yaml:"max_attempts" json:"max_attempts"`
	Backoff     time.Duration `yaml:"backoff" json:"backoff"`
	MaxBackoff  time.Duration `yaml:"max_backoff" json:"max_backoff"`
	Jitter      bool          `yaml:"jitter" json:"jitter"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"`
	EnableGRPC bool   `yaml:"enable_grpc" json:"enable_grpc"`
}

// DefaultGRPCConfig returns a default gRPC configuration.
func DefaultGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		Services: make(map[string]ServiceConfig),
		Security: SecurityConfig{
			TLS: TLSConfig{
				Enabled: false,
			},
		},
		Retry: RetryConfig{
			MaxAttempts: 3,
			Backoff:     100 * time.Millisecond,
			MaxBackoff:  5 * time.Second,
			Jitter:      true,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			EnableGRPC: false,
		},
	}
}

// LoadFromEnv loads gRPC configuration from environment variables.
func (c *GRPCConfig) LoadFromEnv() error {
	// Load service configurations
	services := os.Getenv("GRPC_SERVICES")
	if services != "" {
		serviceList := strings.Split(services, ",")
		for _, service := range serviceList {
			service = strings.TrimSpace(service)
			if helpers.IsEmpty(service) {
				continue
			}

			endpoint := helpers.GetStringFromEnv(fmt.Sprintf("GRPC_%s_ENDPOINT", strings.ToUpper(service)), "")
			if helpers.IsEmpty(endpoint) {
				continue
			}

			timeout := helpers.GetDurationFromEnv(fmt.Sprintf("GRPC_%s_TIMEOUT", strings.ToUpper(service)), 30*time.Second)

			c.Services[service] = ServiceConfig{
				Endpoint: endpoint,
				Timeout:  timeout,
				Credentials: CredentialsConfig{
					Type: helpers.GetStringFromEnv(fmt.Sprintf("GRPC_%s_CREDENTIALS_TYPE", strings.ToUpper(service)), "insecure"),
				},
				KeepAlive: KeepAliveConfig{
					Time:                helpers.GetDurationFromEnv(fmt.Sprintf("GRPC_%s_KEEPALIVE_TIME", strings.ToUpper(service)), 30*time.Second),
					Timeout:             helpers.GetDurationFromEnv(fmt.Sprintf("GRPC_%s_KEEPALIVE_TIMEOUT", strings.ToUpper(service)), 5*time.Second),
					PermitWithoutStream: helpers.GetBoolFromEnv(fmt.Sprintf("GRPC_%s_KEEPALIVE_PERMIT_WITHOUT_STREAM", strings.ToUpper(service)), true),
				},
			}
		}
	}

	// Load security configuration
	c.Security.TLS.Enabled = helpers.GetBoolFromEnv("GRPC_TLS_ENABLED", false)
	c.Security.TLS.CertFile = helpers.GetStringFromEnv("GRPC_TLS_CERT_FILE", "")
	c.Security.TLS.KeyFile = helpers.GetStringFromEnv("GRPC_TLS_KEY_FILE", "")
	c.Security.TLS.CAFile = helpers.GetStringFromEnv("GRPC_TLS_CA_FILE", "")
	c.Security.TLS.ServerName = helpers.GetStringFromEnv("GRPC_TLS_SERVER_NAME", "")
	c.Security.TLS.Insecure = helpers.GetBoolFromEnv("GRPC_TLS_INSECURE", false)

	// Load retry configuration
	c.Retry.MaxAttempts = helpers.GetIntFromEnv("GRPC_RETRY_MAX_ATTEMPTS", 3)
	c.Retry.Backoff = helpers.GetDurationFromEnv("GRPC_RETRY_BACKOFF", 100*time.Millisecond)
	c.Retry.MaxBackoff = helpers.GetDurationFromEnv("GRPC_RETRY_MAX_BACKOFF", 5*time.Second)
	c.Retry.Jitter = helpers.GetBoolFromEnv("GRPC_RETRY_JITTER", true)

	// Load logging configuration
	c.Logging.Level = helpers.GetStringFromEnv("GRPC_LOG_LEVEL", "info")
	c.Logging.Format = helpers.GetStringFromEnv("GRPC_LOG_FORMAT", "json")
	c.Logging.EnableGRPC = helpers.GetBoolFromEnv("GRPC_LOG_ENABLE_GRPC", false)

	return nil
}

// GetServiceConfig returns configuration for a specific service.
func (c *GRPCConfig) GetServiceConfig(ctx context.Context, serviceName string) (*ServiceConfig, error) {
	config, exists := c.Services[serviceName]
	if !exists {
		return nil, calque.NewErr(ctx, fmt.Sprintf("service %s not found in configuration", serviceName))
	}
	return &config, nil
}

// GetCredentials returns gRPC credentials based on configuration.
func (sc *ServiceConfig) GetCredentials(ctx context.Context) (credentials.TransportCredentials, error) {
	switch sc.Credentials.Type {
	case "insecure":
		return insecure.NewCredentials(), nil
	case "tls":
		if sc.Credentials.CAFile == "" {
			return nil, calque.NewErr(ctx, "CA file required for TLS credentials")
		}
		return credentials.NewClientTLSFromFile(sc.Credentials.CAFile, sc.Credentials.CertFile)
	case "mtls":
		if sc.Credentials.CertFile == "" || sc.Credentials.KeyFile == "" || sc.Credentials.CAFile == "" {
			return nil, calque.NewErr(ctx, "cert file, key file, and CA file required for mTLS credentials")
		}
		return credentials.NewClientTLSFromFile(sc.Credentials.CAFile, sc.Credentials.CertFile)
	default:
		return insecure.NewCredentials(), nil
	}
}
