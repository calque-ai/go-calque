//go:build integration

package qdrant

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	"github.com/google/uuid"
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

// qdrantContainer holds the testcontainer for Qdrant
type qdrantContainer struct {
	Container testcontainers.Container
	URL       string
}

// setupQdrantContainer starts a Qdrant container for testing
func setupQdrantContainer(ctx context.Context) (*qdrantContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "qdrant/qdrant:latest",
		ExposedPorts: []string{"6333/tcp", "6334/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("6333/tcp"),
			wait.ForLog("Qdrant gRPC listening"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Qdrant container: %w", err)
	}

	// Get the mapped port for gRPC API (6334)
	grpcPort, err := container.MappedPort(ctx, "6334")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped gRPC port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	// Qdrant Go client uses gRPC port (6334) not HTTP port (6333)
	// Format as http://host:port so URL parsing works correctly
	url := fmt.Sprintf("http://%s:%s", host, grpcPort.Port())

	return &qdrantContainer{
		Container: container,
		URL:       url,
	}, nil
}

// teardown cleans up the Qdrant container
func (qc *qdrantContainer) teardown(ctx context.Context) error {
	if qc.Container != nil {
		return qc.Container.Terminate(ctx)
	}
	return nil
}

// TestClientCreation tests various client configuration scenarios
func TestClientCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	qc, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer qc.teardown(ctx)

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
				URL:               qc.URL,
				CollectionName:    "test_collection",
				EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.collectionName != "test_collection" {
					t.Errorf("Expected collection 'test_collection', got %q", client.collectionName)
				}
				if client.url != qc.URL {
					t.Errorf("Expected URL %q, got %q", qc.URL, client.url)
				}
			},
		},
		{
			name: "valid config with default collection name",
			config: &Config{
				URL: qc.URL,
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.collectionName != "documents" {
					t.Errorf("Expected default collection 'documents', got %q", client.collectionName)
				}
			},
		},
		{
			name:      "missing URL returns error",
			config:    &Config{},
			expectErr: true,
			errMsg:    "qdrant URL is required",
		},
		{
			name: "empty URL returns error",
			config: &Config{
				URL: "",
			},
			expectErr: true,
			errMsg:    "qdrant URL is required",
		},
		{
			name: "invalid URL format returns error",
			config: &Config{
				URL: "ht!tp://invalid url",
			},
			expectErr: true,
		},
		{
			name: "URL with API key",
			config: &Config{
				URL:    qc.URL,
				APIKey: "test-api-key",
			},
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.apiKey != "test-api-key" {
					t.Errorf("Expected API key 'test-api-key', got %q", client.apiKey)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Logf("Expected error %q, got %q", tt.errMsg, err.Error())
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
	qc, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer qc.teardown(ctx)

	tests := []struct {
		name      string
		setupFn   func(t *testing.T) *Client
		expectErr bool
		checkFn   func(t *testing.T, err error)
	}{
		{
			name: "health check succeeds for running server",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					URL:            qc.URL,
					CollectionName: "health_test",
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			expectErr: false,
		},
		{
			name: "health check fails for non-existent server",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					URL:            "http://localhost:9999",
					CollectionName: "health_test",
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			expectErr: true,
			checkFn: func(t *testing.T, err error) {
				if err == nil {
					t.Error("Expected health check to fail for non-existent server")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupFn(t)
			defer client.Close()

			err := client.Health(ctx)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if tt.checkFn != nil {
					tt.checkFn(t, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestStoreOperations tests document storage functionality
func TestStoreOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	qc, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer qc.teardown(ctx)

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
					URL:               qc.URL,
					CollectionName:    "store_single_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
					URL:               qc.URL,
					CollectionName:    "store_batch_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
					URL:               qc.URL,
					CollectionName:    "store_empty_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
					URL:            qc.URL,
					CollectionName: "store_no_embed_test",
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
					URL:               qc.URL,
					CollectionName:    "store_metadata_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
			name: "store large batch of documents",
			setupFn: func(t *testing.T) *Client {
				client, err := New(&Config{
					URL:               qc.URL,
					CollectionName:    "store_large_batch_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: func() []retrieval.Document {
				docs := make([]retrieval.Document, 150) // More than batch size (100)
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
	}

	for _, tt := range tests {
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
	qc, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer qc.teardown(ctx)

	// Setup client and seed data once for all search tests
	embedProvider := newMockEmbeddingProvider(128)
	client, err := New(&Config{
		URL:               qc.URL,
		CollectionName:    "search_test",
		EmbeddingProvider: embedProvider,
		VectorDimension:   128,
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
	time.Sleep(2 * time.Second)

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
					Text:   "machine learning",
					Vector: vec,
					Limit:  10,
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
				Text:   "test query",
				Vector: nil,
				Limit:  10,
			},
			expectErr: true,
			errMsg:    "query vector is required",
		},
		{
			name: "search with limit parameter",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "machine learning")
				return retrieval.SearchQuery{
					Text:   "machine learning",
					Vector: vec,
					Limit:  1,
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
					Threshold: 0.9,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				for _, doc := range result.Documents {
					if doc.Score < 0.9 {
						t.Errorf("Document score %f below threshold 0.9", doc.Score)
					}
				}
			},
		},
		{
			name: "search with metadata filters",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "tutorial")
				return retrieval.SearchQuery{
					Text:   "tutorial",
					Vector: vec,
					Limit:  10,
					Filter: map[string]any{
						"category": "ai",
					},
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				for _, doc := range result.Documents {
					if category, ok := doc.Metadata["category"].(string); !ok || category != "ai" {
						t.Errorf("Expected category 'ai', got %v", doc.Metadata["category"])
					}
				}
			},
		},
		{
			name: "search non-existent collection returns error",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "test")
				return retrieval.SearchQuery{
					Text:       "test",
					Vector:     vec,
					Limit:      10,
					Collection: "non_existent_collection",
				}
			}(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
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
	qc, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer qc.teardown(ctx)

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
					URL:               qc.URL,
					CollectionName:    "delete_single_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
					URL:               qc.URL,
					CollectionName:    "delete_multiple_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
					URL:               qc.URL,
					CollectionName:    "delete_empty_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
					URL:               qc.URL,
					CollectionName:    "delete_nonexistent_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
					VectorDimension:   128,
				})
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				// Store a dummy document to ensure collection exists
				dummyDoc := []retrieval.Document{
					{ID: generateTestUUID("dummy"), Content: "Dummy document"},
				}
				if err := client.Store(ctx, dummyDoc); err != nil {
					t.Fatalf("Failed to create collection: %v", err)
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
					URL:               qc.URL,
					CollectionName:    "delete_large_batch_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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

// TestSearchWithDiversification tests diversification functionality
func TestSearchWithDiversification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	qc, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer qc.teardown(ctx)

	embedProvider := newMockEmbeddingProvider(128)
	client, err := New(&Config{
		URL:               qc.URL,
		CollectionName:    "diversification_test",
		EmbeddingProvider: embedProvider,
		VectorDimension:   128,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Seed diverse test data
	seedDocs := []retrieval.Document{
		{ID: generateTestUUID("doc1"), Content: "machine learning basics"},
		{ID: generateTestUUID("doc2"), Content: "deep learning tutorial"},
		{ID: generateTestUUID("doc3"), Content: "neural networks explained"},
		{ID: generateTestUUID("doc4"), Content: "data science fundamentals"},
		{ID: generateTestUUID("doc5"), Content: "cooking italian recipes"},
	}

	if err := client.Store(ctx, seedDocs); err != nil {
		t.Fatalf("Failed to seed test data: %v", err)
	}

	time.Sleep(2 * time.Second)

	tests := []struct {
		name      string
		query     retrieval.SearchQuery
		opts      retrieval.DiversificationOptions
		expectErr bool
		checkFn   func(t *testing.T, result *retrieval.SearchResult)
	}{
		{
			name: "diversification with valid options",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "machine learning")
				return retrieval.SearchQuery{
					Text:   "machine learning",
					Vector: vec,
					Limit:  3,
				}
			}(),
			opts: retrieval.DiversificationOptions{
				CandidatesLimit: 10,
				Diversity:       0.5,
			},
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
			name: "diversification with empty vector returns error",
			query: retrieval.SearchQuery{
				Text:   "test",
				Vector: nil,
				Limit:  3,
			},
			opts: retrieval.DiversificationOptions{
				CandidatesLimit: 10,
			},
			expectErr: true,
		},
		{
			name: "diversification with high diversity parameter",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "learning")
				return retrieval.SearchQuery{
					Text:   "learning",
					Vector: vec,
					Limit:  3,
				}
			}(),
			opts: retrieval.DiversificationOptions{
				CandidatesLimit: 20,
				Diversity:       0.9,
			},
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				if len(result.Documents) == 0 {
					t.Error("Expected at least one result")
				}
			},
		},
		{
			name: "diversification with low candidates limit",
			query: func() retrieval.SearchQuery {
				vec, _ := embedProvider.Embed(ctx, "tutorial")
				return retrieval.SearchQuery{
					Text:   "tutorial",
					Vector: vec,
					Limit:  2,
				}
			}(),
			opts: retrieval.DiversificationOptions{
				CandidatesLimit: 5,
				Diversity:       0.5,
			},
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				if len(result.Documents) > 2 {
					t.Errorf("Expected at most 2 results, got %d", len(result.Documents))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.SearchWithDiversification(ctx, tt.query, tt.opts)

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
				tt.checkFn(t, result)
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
	qc, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer qc.teardown(ctx)

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
					URL:               qc.URL,
					CollectionName:    "embed_test",
					EmbeddingProvider: newMockEmbeddingProvider(128),
				VectorDimension:   128,
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
					URL:            qc.URL,
					CollectionName: "embed_no_provider_test",
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
					URL:            qc.URL,
					CollectionName: "embed_set_provider_test",
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
	}

	for _, tt := range tests {
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
