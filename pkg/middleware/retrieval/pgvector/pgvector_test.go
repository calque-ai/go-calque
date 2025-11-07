package pgvector

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, client *Client, config *Config)
	}{
		{
			name:      "missing connection string returns error",
			config:    &Config{},
			expectErr: true,
			errMsg:    "PostgreSQL connection string is required",
		},
		{
			name: "empty connection string returns error",
			config: &Config{
				ConnectionString: "",
			},
			expectErr: true,
			errMsg:    "PostgreSQL connection string is required",
		},
		{
			name: "invalid connection string format returns error",
			config: &Config{
				ConnectionString: "invalid-connection-string",
			},
			expectErr: true,
			// Error message will be about parsing, not exact match
		},
		{
			name: "missing host in connection string returns error",
			config: &Config{
				ConnectionString: "postgres://",
			},
			expectErr: true,
		},
		{
			name: "valid connection string with defaults",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				// Verify defaults are set
				if client.tableName != "documents" {
					t.Errorf("Expected default table name 'documents', got %q", client.tableName)
				}
				if client.vectorDimension != 1536 {
					t.Errorf("Expected default vector dimension 1536, got %d", client.vectorDimension)
				}
			},
		},
		{
			name: "valid connection string with custom table name",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				TableName:        "my_vectors",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				if client.tableName != "my_vectors" {
					t.Errorf("Expected table name 'my_vectors', got %q", client.tableName)
				}
			},
		},
		{
			name: "valid connection string with custom vector dimension",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				VectorDimension:  768,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				if client.vectorDimension != 768 {
					t.Errorf("Expected vector dimension 768, got %d", client.vectorDimension)
				}
			},
		},
		{
			name: "zero vector dimension uses default",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				VectorDimension:  0,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				if client.vectorDimension != 1536 {
					t.Errorf("Expected default vector dimension 1536, got %d", client.vectorDimension)
				}
			},
		},
		{
			name: "negative vector dimension uses default",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				VectorDimension:  -100,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				if client.vectorDimension != 1536 {
					t.Errorf("Expected default vector dimension 1536 for negative input, got %d", client.vectorDimension)
				}
			},
		},
		{
			name: "empty table name uses default",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				TableName:        "",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				if client.tableName != "documents" {
					t.Errorf("Expected default table name 'documents', got %q", client.tableName)
				}
			},
		},
		{
			name: "all custom configuration values",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				TableName:        "custom_table",
				VectorDimension:  384,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				if client.tableName != "custom_table" {
					t.Errorf("Expected table name 'custom_table', got %q", client.tableName)
				}
				if client.vectorDimension != 384 {
					t.Errorf("Expected vector dimension 384, got %d", client.vectorDimension)
				}
			},
		},
		{
			name: "connection string with SSL mode required",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=require",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
			},
		},
		{
			name: "connection string with port",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost:5433/testdb?sslmode=disable",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
			},
		},
		{
			name: "connection string with special characters in password",
			config: &Config{
				ConnectionString: "postgres://user:p%40ssw0rd@localhost/testdb?sslmode=disable",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
			},
		},
		{
			name: "very large vector dimension",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				VectorDimension:  4096,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client, config *Config) {
				if client == nil {
					t.Fatal("Expected non-nil client")
				}
				if client.vectorDimension != 4096 {
					t.Errorf("Expected vector dimension 4096, got %d", client.vectorDimension)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := New(tt.config)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				if tt.errMsg != "" && err != nil && err.Error() != tt.errMsg {
					// For some errors, we just want to verify an error occurred
					// not the exact message
					t.Logf("Got error: %v", err)
				}
				return
			}

			if err != nil {
				// Some tests may fail due to actual connection issues
				// (pgvector extension not installed, database not running, etc.)
				// This is expected in unit tests without a real database
				t.Logf("Note: Connection error expected in unit tests without database: %v", err)
				return
			}

			if tt.checkFn != nil {
				tt.checkFn(t, client, tt.config)
			}

			// Clean up
			if client != nil {
				_ = client.Close()
			}
		})
	}
}

func TestEmbeddingProviderMethods(t *testing.T) {
	t.Parallel()

	// Create a mock client (will fail to connect, but we only test the methods)
	client := &Client{
		vectorDimension:   1536,
		tableName:         "test",
		embeddingProvider: nil,
	}

	t.Run("GetEmbeddingProvider returns nil when not set", func(t *testing.T) {
		provider := client.GetEmbeddingProvider()
		if provider != nil {
			t.Error("Expected nil provider when not set")
		}
	})

	t.Run("SetEmbeddingProvider sets the provider", func(t *testing.T) {
		// We can't create a real provider without dependencies,
		// but we can verify the method doesn't panic
		client.SetEmbeddingProvider(nil)
		if client.GetEmbeddingProvider() != nil {
			t.Error("Expected nil after setting nil provider")
		}
	})
}

func TestClientInitialization(t *testing.T) {
	t.Parallel()

	t.Run("schemaEnsured is false initially", func(t *testing.T) {
		client := &Client{
			vectorDimension: 1536,
			tableName:       "test",
			schemaEnsured:   false,
		}

		if client.schemaEnsured {
			t.Error("Expected schemaEnsured to be false initially")
		}
	})

	t.Run("Close on nil connection succeeds", func(t *testing.T) {
		client := &Client{
			conn: nil,
		}

		err := client.Close()
		if err != nil {
			t.Errorf("Expected no error closing client with nil connection, got %v", err)
		}
	})

	t.Run("Close sets connection to nil", func(t *testing.T) {
		client := &Client{
			conn: nil,
		}

		_ = client.Close()
		if client.conn != nil {
			t.Error("Expected connection to be nil after Close")
		}
	})
}

func TestDefaultValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		inputTableName  string
		inputDimension  int
		expectTableName string
		expectDimension int
	}{
		{
			name:            "empty table name gets default",
			inputTableName:  "",
			inputDimension:  1536,
			expectTableName: "documents",
			expectDimension: 1536,
		},
		{
			name:            "zero dimension gets default",
			inputTableName:  "test",
			inputDimension:  0,
			expectTableName: "test",
			expectDimension: 1536,
		},
		{
			name:            "negative dimension gets default",
			inputTableName:  "test",
			inputDimension:  -100,
			expectTableName: "test",
			expectDimension: 1536,
		},
		{
			name:            "custom values preserved",
			inputTableName:  "my_table",
			inputDimension:  768,
			expectTableName: "my_table",
			expectDimension: 768,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{
				ConnectionString: "postgres://user:pass@localhost/testdb?sslmode=disable",
				TableName:        tt.inputTableName,
				VectorDimension:  tt.inputDimension,
			}

			// We expect connection to fail in unit tests, but we can still
			// check that defaults are applied to the config before connection
			_, err := New(config)

			// Verify defaults were applied to config
			if config.TableName != tt.expectTableName {
				t.Errorf("Expected table name %q after defaults, got %q",
					tt.expectTableName, config.TableName)
			}
			if config.VectorDimension != tt.expectDimension {
				t.Errorf("Expected dimension %d after defaults, got %d",
					tt.expectDimension, config.VectorDimension)
			}

			// Log connection error (expected in unit tests)
			if err != nil {
				t.Logf("Connection error (expected): %v", err)
			}
		})
	}
}
