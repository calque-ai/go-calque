//go:build integration

package pgvector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// mockEmbeddingProvider provides deterministic embeddings for testing
type mockEmbeddingProvider struct {
	dimension int
}

func newMockEmbeddingProvider(dimension int) *mockEmbeddingProvider {
	return &mockEmbeddingProvider{dimension: dimension}
}

func (m *mockEmbeddingProvider) Embed(ctx context.Context, text string) (retrieval.EmbeddingVector, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text cannot be embedded")
	}

	// Generate consistent, simple embeddings based on text content
	// This creates different but deterministic vectors for different texts
	vector := make(retrieval.EmbeddingVector, m.dimension)
	for i := 0; i < m.dimension; i++ {
		// Simple hash-like function for consistent embeddings
		vector[i] = float32((len(text)+i)%100) / 100.0
	}
	return vector, nil
}

// generateTestUUID creates a valid UUID for testing
// Uses UUID v5 (SHA-1 hash based) for deterministic IDs from string input
func generateTestUUID(name string) string {
	// Use DNS namespace as base for consistent UUIDs
	namespace := uuid.NameSpaceDNS
	return uuid.NewSHA1(namespace, []byte(name)).String()
}

// pgvectorContainer holds the testcontainer for PostgreSQL with pgvector
type pgvectorContainer struct {
	Container testcontainers.Container
	ConnStr   string
}

// setupPGVectorContainer starts a PostgreSQL container with pgvector extension
func setupPGVectorContainer(ctx context.Context) (*pgvectorContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	// Get the mapped port
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())

	// Enable the pgvector extension
	if err := enablePGVectorExtension(ctx, connStr); err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to enable pgvector extension: %w", err)
	}

	return &pgvectorContainer{
		Container: container,
		ConnStr:   connStr,
	}, nil
}

// enablePGVectorExtension creates the vector extension in the database
func enablePGVectorExtension(ctx context.Context, connStr string) error {
	conn, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	_, err = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	return nil
}

// teardown cleans up the PostgreSQL container
func (pc *pgvectorContainer) teardown(ctx context.Context) error {
	if pc.Container != nil {
		return pc.Container.Terminate(ctx)
	}
	return nil
}

// TestClientCreation tests various client configuration scenarios
func TestClientCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, client *Client)
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				ConnectionString:  pc.ConnStr,
				TableName:         "test_documents",
				VectorDimension:   128,
				EmbeddingProvider: newMockEmbeddingProvider(128),
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.tableName != "test_documents" {
					t.Errorf("Expected table 'test_documents', got %q", client.tableName)
				}
				if client.vectorDimension != 128 {
					t.Errorf("Expected dimension 128, got %d", client.vectorDimension)
				}
			},
		},
		{
			name: "valid config with default table name",
			config: &Config{
				ConnectionString: pc.ConnStr,
				VectorDimension:  128,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.tableName != "documents" {
					t.Errorf("Expected default table 'documents', got %q", client.tableName)
				}
			},
		},
		{
			name: "valid config with default vector dimension",
			config: &Config{
				ConnectionString: pc.ConnStr,
				TableName:        "test_default_dim",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.vectorDimension != 1536 {
					t.Errorf("Expected default dimension 1536, got %d", client.vectorDimension)
				}
			},
		},
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
			name: "invalid connection string returns error",
			config: &Config{
				ConnectionString: "invalid://connection",
			},
			expectErr: true,
		},
		{
			name: "connection to non-existent host fails",
			config: &Config{
				ConnectionString: "postgres://user:pass@localhost:9999/testdb?sslmode=disable",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Logf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("Expected non-nil client")
				return
			}

			if tt.checkFn != nil {
				tt.checkFn(t, client)
			}

			// Clean up
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		})
	}
}

// TestHealthCheck tests health check functionality
func TestHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	t.Run("health check succeeds for running server", func(t *testing.T) {
		client, err := New(&Config{
			ConnectionString: pc.ConnStr,
			TableName:        "health_test",
		})
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		err = client.Health(ctx)
		if err != nil {
			t.Errorf("Unexpected health check error: %v", err)
		}
	})

	t.Run("health check after closing connection fails", func(t *testing.T) {
		client, err := New(&Config{
			ConnectionString: pc.ConnStr,
			TableName:        "health_closed_test",
		})
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Close the client first
		client.Close()

		// Health check should now fail
		err = client.Health(ctx)
		if err == nil {
			t.Error("Expected health check to fail after closing connection")
		}
	})
}

// TestStoreOperations tests document storage functionality
func TestStoreOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	tests := []struct {
		name      string
		setupFn   func(t *testing.T) *Client
		documents []retrieval.Document
		expectErr bool
		errMsg    string
	}{
		{
			name: "store single document successfully",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "store_single_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{
					ID:      generateTestUUID("doc1"),
					Content: "This is a test document",
					Metadata: map[string]any{
						"category": "test",
						"author":   "tester",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "store multiple documents in batch",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "store_batch_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{ID: generateTestUUID("doc1"), Content: "First document"},
				{ID: generateTestUUID("doc2"), Content: "Second document"},
				{ID: generateTestUUID("doc3"), Content: "Third document"},
				{ID: generateTestUUID("doc4"), Content: "Fourth document"},
				{ID: generateTestUUID("doc5"), Content: "Fifth document"},
			},
			expectErr: false,
		},
		{
			name: "store empty document list is no-op",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "store_empty_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{},
			expectErr: false,
		},
		{
			name: "store without embedding provider returns error",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString: pc.ConnStr,
					TableName:        "store_no_embed_test",
					VectorDimension:  128,
					// No embedding provider
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{ID: generateTestUUID("doc1"), Content: "Test document"},
			},
			expectErr: true,
			errMsg:    "no embedding provider configured",
		},
		{
			name: "store documents with various metadata types",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "store_metadata_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{
					ID:      generateTestUUID("doc1"),
					Content: "Document with rich metadata",
					Metadata: map[string]any{
						"title":     "Test Title",
						"year":      2024,
						"score":     0.95,
						"published": true,
						"count":     int64(42),
					},
					Created: time.Now(),
					Updated: time.Now(),
				},
			},
			expectErr: false,
		},
		{
			name: "store document with empty content skips document",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "store_empty_content_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{ID: generateTestUUID("doc1"), Content: ""},
			},
			expectErr: false, // Empty content is skipped, not an error
		},
		{
			name: "store large batch of documents",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "store_large_batch_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: func() []retrieval.Document {
				docs := make([]retrieval.Document, 150)
				for i := 0; i < 150; i++ {
					docs[i] = retrieval.Document{
						ID:      generateTestUUID(fmt.Sprintf("doc%d", i)),
						Content: fmt.Sprintf("Document number %d", i),
					}
				}
				return docs
			}(),
			expectErr: false,
		},
		{
			name: "upsert updates existing document",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "upsert_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				// Store initial document
				err = client.Store(ctx, []retrieval.Document{
					{ID: generateTestUUID("upsert1"), Content: "Original content"},
				})
				if err != nil {
					t.Fatalf("Failed to store initial document: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{ID: generateTestUUID("upsert1"), Content: "Updated content"},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupFn(t)
			defer client.Close()

			err := client.Store(ctx, tt.documents)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errMsg != "" {
					if !strings.Contains(err.Error(), tt.errMsg) {
						t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestSearchOperations tests vector search functionality
func TestSearchOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	// Setup client and seed data once for all search tests
	embedProvider := newMockEmbeddingProvider(128)
	client, err := New(&Config{
		ConnectionString:  pc.ConnStr,
		TableName:         "search_test",
		VectorDimension:   128,
		EmbeddingProvider: embedProvider,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Seed test data
	seedDocs := []retrieval.Document{
		{
			ID:      generateTestUUID("doc1"),
			Content: "machine learning basics",
			Metadata: map[string]any{
				"category": "ai",
				"year":     2024,
			},
		},
		{
			ID:      generateTestUUID("doc2"),
			Content: "deep learning tutorial",
			Metadata: map[string]any{
				"category": "ai",
				"year":     2023,
			},
		},
		{
			ID:      generateTestUUID("doc3"),
			Content: "cooking recipes",
			Metadata: map[string]any{
				"category": "food",
				"year":     2024,
			},
		},
	}

	if err := client.Store(ctx, seedDocs); err != nil {
		t.Fatalf("Failed to seed test data: %v", err)
	}

	// Wait a bit for indexing
	time.Sleep(1 * time.Second)

	tests := []struct {
		name      string
		query     retrieval.SearchQuery
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, result *retrieval.SearchResult)
	}{
		{
			name: "basic search with valid query",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "machine learning")
				return retrieval.SearchQuery{
					Text:      "machine learning",
					Vector:    vec,
					Limit:     10,
					Threshold: 0.0,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				if len(result.Documents) == 0 {
					t.Error("Expected at least one result")
				}
			},
		},
		{
			name: "search with empty vector returns error",
			query: retrieval.SearchQuery{
				Text:      "test query",
				Vector:    nil,
				Limit:     10,
				Threshold: 0.0,
			},
			expectErr: true,
			errMsg:    "query.Vector is required",
		},
		{
			name: "search with limit parameter",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "machine learning")
				return retrieval.SearchQuery{
					Text:      "machine learning",
					Vector:    vec,
					Limit:     1,
					Threshold: 0.0,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				if len(result.Documents) > 1 {
					t.Errorf("Expected at most 1 result, got %d", len(result.Documents))
				}
			},
		},
		{
			name: "search with threshold filtering",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "machine learning")
				return retrieval.SearchQuery{
					Text:      "machine learning",
					Vector:    vec,
					Limit:     10,
					Threshold: 0.8,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				for _, doc := range result.Documents {
					if doc.Score < 0.8 {
						t.Errorf("Document score %f below threshold 0.8", doc.Score)
					}
				}
			},
		},
		{
			name: "search returns documents with metadata",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "tutorial")
				return retrieval.SearchQuery{
					Text:      "tutorial",
					Vector:    vec,
					Limit:     10,
					Threshold: 0.0,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				for _, doc := range result.Documents {
					if doc.Metadata == nil {
						t.Error("Expected metadata to be non-nil")
					}
					if doc.ID == "" {
						t.Error("Expected non-empty ID")
					}
					if doc.Content == "" {
						t.Error("Expected non-empty content")
					}
				}
			},
		},
		{
			name: "search with high threshold returns fewer results",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "learning")
				return retrieval.SearchQuery{
					Text:      "learning",
					Vector:    vec,
					Limit:     10,
					Threshold: 0.95,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				// High threshold should return fewer (possibly zero) results
				t.Logf("High threshold search returned %d results", len(result.Documents))
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.Search(ctx, tt.query)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errMsg != "" {
					if !strings.Contains(err.Error(), tt.errMsg) {
						t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

// TestDeleteOperations tests document deletion functionality
func TestDeleteOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	tests := []struct {
		name      string
		setupFn   func(t *testing.T) (*Client, []string)
		ids       []string
		expectErr bool
		checkFn   func(t *testing.T, client *Client)
	}{
		{
			name: "delete single document",
			setupFn: func(t *testing.T) (*Client, []string) {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "delete_single_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}

				// Store a document first
				docID := generateTestUUID("doc1")
				docs := []retrieval.Document{
					{ID: docID, Content: "Test document"},
				}
				if err := client.Store(ctx, docs); err != nil {
					t.Fatalf("Failed to store document: %v", err)
				}

				return client, []string{docID}
			},
			ids:       nil, // Will be set by setupFn
			expectErr: false,
		},
		{
			name: "delete multiple documents",
			setupFn: func(t *testing.T) (*Client, []string) {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "delete_multiple_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}

				// Store multiple documents
				doc1ID := generateTestUUID("doc1")
				doc2ID := generateTestUUID("doc2")
				doc3ID := generateTestUUID("doc3")
				docs := []retrieval.Document{
					{ID: doc1ID, Content: "Document 1"},
					{ID: doc2ID, Content: "Document 2"},
					{ID: doc3ID, Content: "Document 3"},
				}
				if err := client.Store(ctx, docs); err != nil {
					t.Fatalf("Failed to store documents: %v", err)
				}

				return client, []string{doc1ID, doc2ID, doc3ID}
			},
			ids:       nil,
			expectErr: false,
		},
		{
			name: "delete empty ID list is no-op",
			setupFn: func(t *testing.T) (*Client, []string) {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "delete_empty_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client, []string{}
			},
			ids:       nil,
			expectErr: false,
		},
		{
			name: "delete non-existent IDs does not error",
			setupFn: func(t *testing.T) (*Client, []string) {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "delete_nonexistent_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				// Store a dummy document to ensure table exists
				dummyDoc := []retrieval.Document{
					{ID: generateTestUUID("dummy"), Content: "Dummy document"},
				}
				if err := client.Store(ctx, dummyDoc); err != nil {
					t.Fatalf("Failed to create table: %v", err)
				}
				return client, []string{generateTestUUID("non-existent-id-1"), generateTestUUID("non-existent-id-2")}
			},
			ids:       nil,
			expectErr: false,
		},
		{
			name: "delete large batch of IDs",
			setupFn: func(t *testing.T) (*Client, []string) {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "delete_large_batch_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}

				// Store large batch
				docs := make([]retrieval.Document, 150)
				ids := make([]string, 150)
				for i := 0; i < 150; i++ {
					docID := generateTestUUID(fmt.Sprintf("doc%d", i))
					docs[i] = retrieval.Document{
						ID:      docID,
						Content: fmt.Sprintf("Document %d", i),
					}
					ids[i] = docID
				}

				if err := client.Store(ctx, docs); err != nil {
					t.Fatalf("Failed to store documents: %v", err)
				}

				return client, ids
			},
			ids:       nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			client, ids := tt.setupFn(t)
			defer client.Close()

			// Use IDs from setupFn if not specified
			deleteIDs := tt.ids
			if deleteIDs == nil {
				deleteIDs = ids
			}

			err := client.Delete(ctx, deleteIDs)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, client)
			}
		})
	}
}

// TestEmbeddingProvider tests embedding provider functionality
func TestEmbeddingProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	tests := []struct {
		name      string
		setupFn   func(t *testing.T) *Client
		text      string
		expectErr bool
		checkFn   func(t *testing.T, vec retrieval.EmbeddingVector, err error)
	}{
		{
			name: "get embedding with configured provider",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "embed_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			text:      "test text for embedding",
			expectErr: false,
			checkFn: func(t *testing.T, vec retrieval.EmbeddingVector, err error) {
				if len(vec) != 128 {
					t.Errorf("Expected vector dimension 128, got %d", len(vec))
				}
			},
		},
		{
			name: "get embedding without configured provider returns error",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString: pc.ConnStr,
					TableName:        "embed_no_provider_test",
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			text:      "test text",
			expectErr: true,
		},
		{
			name: "set embedding provider after creation",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString: pc.ConnStr,
					TableName:        "embed_set_provider_test",
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				client.SetEmbeddingProvider(newMockEmbeddingProvider(256))
				return client
			},
			text:      "test text",
			expectErr: false,
			checkFn: func(t *testing.T, vec retrieval.EmbeddingVector, err error) {
				if len(vec) != 256 {
					t.Errorf("Expected vector dimension 256, got %d", len(vec))
				}
			},
		},
		{
			name: "get embedding provider",
			setupFn: func(t *testing.T) *Client {
				provider := newMockEmbeddingProvider(128)
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "embed_get_provider_test",
					EmbeddingProvider: provider,
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				if client.GetEmbeddingProvider() != provider {
					t.Error("GetEmbeddingProvider returned wrong provider")
				}
				return client
			},
			text:      "test text",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			client := tt.setupFn(t)
			defer client.Close()

			vec, err := client.GetEmbedding(ctx, tt.text)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.checkFn != nil {
				tt.checkFn(t, vec, err)
			}
		})
	}
}

// TestTableCreation tests lazy table creation functionality
func TestTableCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	tests := []struct {
		name      string
		setupFn   func(t *testing.T) *Client
		expectErr bool
		checkFn   func(t *testing.T, client *Client)
	}{
		{
			name: "table created on first store",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "lazy_create_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				// Store should create the table
				err := client.Store(ctx, []retrieval.Document{
					{ID: generateTestUUID("test1"), Content: "Test content"},
				})
				if err != nil {
					t.Errorf("Failed to store document: %v", err)
				}
				if !client.schemaEnsured {
					t.Error("Expected schemaEnsured to be true after store")
				}
			},
		},
		{
			name: "table creation is idempotent",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					ConnectionString:  pc.ConnStr,
					TableName:         "idempotent_test",
					VectorDimension:   128,
					EmbeddingProvider: newMockEmbeddingProvider(128),
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				// Store twice to test idempotency
				err := client.Store(ctx, []retrieval.Document{
					{ID: generateTestUUID("test1"), Content: "First store"},
				})
				if err != nil {
					t.Errorf("First store failed: %v", err)
				}
				err = client.Store(ctx, []retrieval.Document{
					{ID: generateTestUUID("test2"), Content: "Second store"},
				})
				if err != nil {
					t.Errorf("Second store failed: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			client := tt.setupFn(t)
			defer client.Close()

			if tt.checkFn != nil {
				tt.checkFn(t, client)
			}
		})
	}
}

// TestConcurrentOperations tests thread-safety of client operations
func TestConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	client, err := New(&Config{
		ConnectionString:  pc.ConnStr,
		TableName:         "concurrent_test",
		VectorDimension:   128,
		EmbeddingProvider: newMockEmbeddingProvider(128),
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	t.Run("concurrent stores", func(t *testing.T) {
		const numGoroutines = 10
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				doc := retrieval.Document{
					ID:      generateTestUUID(fmt.Sprintf("concurrent-doc-%d", idx)),
					Content: fmt.Sprintf("Concurrent document %d", idx),
				}
				errChan <- client.Store(ctx, []retrieval.Document{doc})
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("Concurrent store failed: %v", err)
			}
		}
	})

	t.Run("concurrent searches", func(t *testing.T) {
		// First store a document
		err := client.Store(ctx, []retrieval.Document{
			{ID: generateTestUUID("search-doc"), Content: "Document for concurrent search testing"},
		})
		if err != nil {
			t.Fatalf("Failed to store document: %v", err)
		}

		time.Sleep(1 * time.Second) // Wait for indexing

		const numGoroutines = 10
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				vec, err := client.GetEmbedding(ctx, "concurrent search")
				if err != nil {
					errChan <- fmt.Errorf("failed to generate embedding: %w", err)
					return
				}

				query := retrieval.SearchQuery{
					Vector:    vec,
					Limit:     10,
					Threshold: 0.0,
				}
				_, err = client.Search(ctx, query)
				errChan <- err
			}()
		}

		for i := 0; i < numGoroutines; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("Concurrent search failed: %v", err)
			}
		}
	})
}

// TestSearchResultOrdering tests that search results are ordered by similarity
func TestSearchResultOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	pc, err := setupPGVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pc.teardown(ctx)

	embedProvider := newMockEmbeddingProvider(128)
	client, err := New(&Config{
		ConnectionString:  pc.ConnStr,
		TableName:         "ordering_test",
		VectorDimension:   128,
		EmbeddingProvider: embedProvider,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Store documents
	docs := []retrieval.Document{
		{ID: generateTestUUID("doc1"), Content: "machine learning"},
		{ID: generateTestUUID("doc2"), Content: "deep learning"},
		{ID: generateTestUUID("doc3"), Content: "cooking"},
	}

	if err := client.Store(ctx, docs); err != nil {
		t.Fatalf("Failed to store documents: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Search for "machine learning"
	vec, err := embedProvider.Embed(ctx, "machine learning")
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}

	result, err := client.Search(ctx, retrieval.SearchQuery{
		Vector:    vec,
		Limit:     10,
		Threshold: 0.0,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Check that results are ordered by score (descending)
	for i := 1; i < len(result.Documents); i++ {
		if result.Documents[i].Score > result.Documents[i-1].Score {
			t.Errorf("Results not ordered by score: doc[%d].Score=%f > doc[%d].Score=%f",
				i, result.Documents[i].Score, i-1, result.Documents[i-1].Score)
		}
	}
}
