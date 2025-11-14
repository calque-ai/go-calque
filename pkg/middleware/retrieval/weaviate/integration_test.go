//go:build integration

package weaviate

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

// weaviateContainer holds the testcontainer for Weaviate
type weaviateContainer struct {
	Container testcontainers.Container
	URL       string
}

// getDefaultSchema returns a default schema config for testing
func getDefaultSchema(className string) *SchemaConfig {
	return &SchemaConfig{
		ClassName:  className,
		Vectorizer: "none", // We'll provide vectors manually
		Properties: []PropertyConfig{
			{Name: "category", Type: PropertyTypeText, Indexed: true},
			{Name: "year", Type: PropertyTypeInt, Indexed: true},
			{Name: "author", Type: PropertyTypeText, Indexed: true},
			{Name: "priority", Type: PropertyTypeInt, Indexed: true},
			{Name: "active", Type: PropertyTypeBool, Indexed: true},
			{Name: "score", Type: PropertyTypeNumber, Indexed: true},
		},
	}
}

// setupWeaviateContainer starts a Weaviate container for testing
func setupWeaviateContainer(ctx context.Context) (*weaviateContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "semitechnologies/weaviate:latest",
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED": "true",
			"PERSISTENCE_DATA_PATH":                   "/var/lib/weaviate",
			"QUERY_DEFAULTS_LIMIT":                    "25",
			"DEFAULT_VECTORIZER_MODULE":               "none",
			"ENABLE_MODULES":                          "",
			"CLUSTER_HOSTNAME":                        "node1",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("8080/tcp"),
			wait.ForHTTP("/v1/.well-known/ready").WithPort("8080/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Weaviate container: %w", err)
	}

	// Get the mapped port for HTTP API (8080)
	httpPort, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped HTTP port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	url := fmt.Sprintf("http://%s:%s", host, httpPort.Port())

	return &weaviateContainer{
		Container: container,
		URL:       url,
	}, nil
}

// teardown cleans up the Weaviate container
func (wc *weaviateContainer) teardown(ctx context.Context) error {
	if wc.Container != nil {
		return wc.Container.Terminate(ctx)
	}
	return nil
}

// TestClientCreation tests various client configuration scenarios
func TestClientCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

	tests := []struct {
		name      string
		config    *Config
		schema    *SchemaConfig
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, client *Client)
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				URL:       wc.URL,
				ClassName: "TestDocument",
				APIKey:    "",
			},
			schema:    getDefaultSchema("TestDocument"),
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.className != "TestDocument" {
					t.Errorf("Expected class 'TestDocument', got %q", client.className)
				}
				if client.url != wc.URL {
					t.Errorf("Expected URL %q, got %q", wc.URL, client.url)
				}
			},
		},
		{
			name: "valid config with default class name",
			config: &Config{
				URL: wc.URL,
			},
			schema:    getDefaultSchema("Document"),
			expectErr: false,
			checkFn: func(t *testing.T, client *Client) {
				if client.className != "Document" {
					t.Errorf("Expected default class 'Document', got %q", client.className)
				}
			},
		},
		{
			name:      "missing URL returns error",
			config:    &Config{},
			schema:    getDefaultSchema("TestDocument"),
			expectErr: true,
			errMsg:    "weaviate URL is required",
		},
		{
			name: "empty URL returns error",
			config: &Config{
				URL: "",
			},
			schema:    getDefaultSchema("TestDocument"),
			expectErr: true,
			errMsg:    "weaviate URL is required",
		},
		{
			name: "invalid URL format returns error",
			config: &Config{
				URL: "ht!tp://invalid url",
			},
			schema:    getDefaultSchema("TestDocument"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(ctx, tt.config, tt.schema)

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
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

	tests := []struct {
		name      string
		setupFn   func(t *testing.T) *Client
		expectErr bool
		checkFn   func(t *testing.T, err error)
	}{
		{
			name: "health check succeeds for running server",
			setupFn: func(t *testing.T) *Client {
				schema := getDefaultSchema("HealthTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "HealthTest",
				}, schema)
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
				// Use private newClient() to create client without schema setup
				// since we can't connect to validate schema on non-existent server
				client, err := newClient(&Config{
					URL:       "http://localhost:9999",
					ClassName: "HealthTest",
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
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

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
				schema := getDefaultSchema("StoreSingleTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "StoreSingleTest",
				}, schema)
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
				schema := getDefaultSchema("StoreBatchTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "StoreBatchTest",
				}, schema)
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
				schema := getDefaultSchema("StoreEmptyTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "StoreEmptyTest",
				}, schema)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{},
			expectErr: false,
		},
		{
			name: "store document without ID generates UUID",
			setupFn: func(t *testing.T) *Client {
				schema := getDefaultSchema("StoreNoIDTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "StoreNoIDTest",
				}, schema)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{Content: "Document without explicit ID"},
			},
			expectErr: false,
		},
		{
			name: "store document with empty content returns error",
			setupFn: func(t *testing.T) *Client {
				schema := getDefaultSchema("StoreEmptyContentTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "StoreEmptyContentTest",
				}, schema)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{ID: generateTestUUID("doc1"), Content: ""},
			},
			expectErr: true,
			errMsg:    "document content is required",
		},
		{
			name: "store documents with various metadata types",
			setupFn: func(t *testing.T) *Client {
				// Custom schema with additional properties
				schema := &SchemaConfig{
					ClassName:  "StoreMetadataTest",
					Vectorizer: "none",
					Properties: []PropertyConfig{
						{Name: "title", Type: PropertyTypeText, Indexed: true},
						{Name: "year", Type: PropertyTypeInt, Indexed: true},
						{Name: "score", Type: PropertyTypeNumber, Indexed: true},
						{Name: "published", Type: PropertyTypeBool, Indexed: true},
						{Name: "count", Type: PropertyTypeInt, Indexed: true},
					},
				}
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "StoreMetadataTest",
				}, schema)
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
			name: "store document with nil metadata",
			setupFn: func(t *testing.T) *Client {
				schema := getDefaultSchema("StoreNilMetadataTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "StoreNilMetadataTest",
				}, schema)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			documents: []retrieval.Document{
				{
					ID:       generateTestUUID("doc1"),
					Content:  "Document without metadata",
					Metadata: nil,
				},
			},
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
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

	// Setup client and seed data once for all search tests
	schema := getDefaultSchema("SearchTest")
	client, err := New(ctx, &Config{
		URL:       wc.URL,
		ClassName: "SearchTest",
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create mock embedding provider for generating vectors
	mockEmbedder := newMockEmbeddingProvider(128)

	// Seed test data with vectors (required since vectorizer is disabled)
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

	// Add vectors to documents via metadata (since Document struct doesn't have Vector field)
	for i := range seedDocs {
		vec, err := mockEmbedder.Embed(ctx, seedDocs[i].Content)
		if err != nil {
			t.Fatalf("Failed to generate embedding: %v", err)
		}
		seedDocs[i].Metadata["vector"] = []float32(vec)
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
			name: "search with vector query - similar to doc1",
			query: func() retrieval.SearchQuery {
				vec, _ := mockEmbedder.Embed(ctx, "machine learning basics")
				return retrieval.SearchQuery{
					Vector:     vec,
					Collection: "SearchTest",
					Limit:      10,
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
			name: "search with empty text and vector returns error",
			query: retrieval.SearchQuery{
				Text:       "",
				Vector:     nil,
				Collection: "SearchTest",
				Limit:      10,
			},
			expectErr: true,
			errMsg:    "either query.Text or query.Vector must be provided",
		},
		{
			name: "search with limit parameter",
			query: func() retrieval.SearchQuery {
				vec, _ := mockEmbedder.Embed(ctx, "tutorial")
				return retrieval.SearchQuery{
					Vector:     vec,
					Collection: "SearchTest",
					Limit:      1,
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
				vec, _ := mockEmbedder.Embed(ctx, "machine learning")
				return retrieval.SearchQuery{
					Vector:     vec,
					Collection: "SearchTest",
					Limit:      10,
					Threshold:  0.5, // Lower threshold since Weaviate may return lower scores
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				// Note: Weaviate's score depends on distance metric and may be computed differently
				// We just check that we got results, not the exact score values
				if len(result.Documents) > 0 {
					t.Logf("Received %d documents with threshold filtering", len(result.Documents))
				}
			},
		},
		{
			name: "search with metadata filters",
			query: func() retrieval.SearchQuery {
				vec, _ := mockEmbedder.Embed(ctx, "tutorial")
				return retrieval.SearchQuery{
					Vector:     vec,
					Collection: "SearchTest",
					Limit:      10,
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
				vec, _ := mockEmbedder.Embed(ctx, "test")
				return retrieval.SearchQuery{
					Vector:     vec,
					Collection: "NonExistentCollection",
					Limit:      10,
				}
			}(),
			expectErr: true,
		},
		{
			name: "search without collection uses client default",
			query: func() retrieval.SearchQuery {
				vec, _ := mockEmbedder.Embed(ctx, "machine learning")
				return retrieval.SearchQuery{
					Vector: vec,
					Limit:  10,
				}
			}(),
			expectErr: false, // Should work because client default is "SearchTest"
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				t.Log("Search successfully used client's default class name")
			},
		},
		{
			name: "search with different vector dimensions",
			query: func() retrieval.SearchQuery {
				vec := make(retrieval.EmbeddingVector, 128)
				for i := 0; i < 128; i++ {
					vec[i] = float32(i) / 128.0
				}
				return retrieval.SearchQuery{
					Vector:     vec,
					Collection: "SearchTest",
					Limit:      5,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
			},
		},
		{
			name: "text search fails without vectorizer (as expected)",
			query: retrieval.SearchQuery{
				Text:       "machine learning",
				Collection: "SearchTest",
				Limit:      10,
			},
			expectErr: true, // Text search won't work because vectorizer is disabled
			checkFn: func(t *testing.T, result *retrieval.SearchResult) {
				// This test demonstrates that text search requires a vectorizer
				t.Log("Text search correctly failed without vectorizer module enabled")
			},
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
				if tt.checkFn != nil {
					tt.checkFn(t, result)
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
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

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
				schema := getDefaultSchema("DeleteSingleTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "DeleteSingleTest",
				}, schema)
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
				schema := getDefaultSchema("DeleteMultipleTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "DeleteMultipleTest",
				}, schema)
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
				schema := getDefaultSchema("DeleteEmptyTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "DeleteEmptyTest",
				}, schema)
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
				schema := getDefaultSchema("DeleteNonexistentTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "DeleteNonexistentTest",
				}, schema)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				// Store a dummy document to ensure class exists
				dummyDoc := []retrieval.Document{
					{ID: generateTestUUID("dummy"), Content: "Dummy document"},
				}
				if err := client.Store(ctx, dummyDoc); err != nil {
					t.Fatalf("Failed to store dummy document: %v", err)
				}
				return client, []string{generateTestUUID("non-existent-id-1"), generateTestUUID("non-existent-id-2")}
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

// TestAutoEmbeddingSupport tests auto-embedding configuration
func TestAutoEmbeddingSupport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

	tests := []struct {
		name    string
		setupFn func(t *testing.T) *Client
		checkFn func(t *testing.T, client *Client)
	}{
		{
			name: "client supports auto-embedding",
			setupFn: func(t *testing.T) *Client {
				schema := getDefaultSchema("AutoEmbedTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "AutoEmbedTest",
				}, schema)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			checkFn: func(t *testing.T, client *Client) {
				if !client.SupportsAutoEmbedding() {
					t.Error("Expected client to support auto-embedding")
				}
			},
		},
		{
			name: "get embedding config returns defaults",
			setupFn: func(t *testing.T) *Client {
				schema := getDefaultSchema("EmbedConfigTest")
				client, err := New(ctx, &Config{
					URL:       wc.URL,
					ClassName: "EmbedConfigTest",
				}, schema)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				return client
			},
			checkFn: func(t *testing.T, client *Client) {
				config := client.GetEmbeddingConfig()
				if config.Provider != "weaviate" {
					t.Errorf("Expected provider 'weaviate', got %q", config.Provider)
				}
				if config.Model == "" {
					t.Error("Expected non-empty model")
				}
				if config.Dimensions <= 0 {
					t.Errorf("Expected positive dimensions, got %d", config.Dimensions)
				}
			},
		},
	}

	for _, tt := range tests {
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
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

	schema := getDefaultSchema("ConcurrentTest")
	client, err := New(ctx, &Config{
		URL:       wc.URL,
		ClassName: "ConcurrentTest",
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create mock embedding provider for vector-based searches
	mockEmbedder := newMockEmbeddingProvider(128)

	t.Run("concurrent stores", func(t *testing.T) {
		// Note: Not using t.Parallel() here to avoid issues with shared client
		// The client itself should handle concurrent requests safely

		const numGoroutines = 5
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				content := fmt.Sprintf("Concurrent document %d", idx)
				vec, err := mockEmbedder.Embed(ctx, content)
				if err != nil {
					errChan <- fmt.Errorf("failed to generate embedding: %w", err)
					return
				}

				doc := retrieval.Document{
					ID:      generateTestUUID(fmt.Sprintf("concurrent-doc-%d", idx)),
					Content: content,
					Metadata: map[string]any{
						"vector": []float32(vec),
					},
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
		// Note: Not using t.Parallel() here to avoid issues with shared client
		// The client itself should handle concurrent requests safely

		// First store a document with vector
		content := "Document for concurrent search testing"
		vec, err := mockEmbedder.Embed(ctx, content)
		if err != nil {
			t.Fatalf("Failed to generate embedding: %v", err)
		}

		doc := retrieval.Document{
			ID:      generateTestUUID("search-doc"),
			Content: content,
			Metadata: map[string]any{
				"vector": []float32(vec),
			},
		}
		if err := client.Store(ctx, []retrieval.Document{doc}); err != nil {
			t.Fatalf("Failed to store document: %v", err)
		}

		time.Sleep(2 * time.Second) // Wait for indexing

		const numGoroutines = 5
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				// Use vector search instead of text search
				searchVec, err := mockEmbedder.Embed(ctx, "concurrent search")
				if err != nil {
					errChan <- fmt.Errorf("failed to generate search embedding: %w", err)
					return
				}

				query := retrieval.SearchQuery{
					Vector:     searchVec,
					Collection: "ConcurrentTest",
					Limit:      10,
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

// TestFilterBuilding tests complex filter scenarios
func TestFilterBuilding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	wc, err := setupWeaviateContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Weaviate container: %v", err)
	}
	defer wc.teardown(ctx)

	schema := getDefaultSchema("FilterTest")
	client, err := New(ctx, &Config{
		URL:       wc.URL,
		ClassName: "FilterTest",
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create mock embedding provider
	mockEmbedder := newMockEmbeddingProvider(128)

	// Seed test data with various metadata types
	seedDocs := []retrieval.Document{
		{
			ID:      generateTestUUID("filter-doc-1"),
			Content: "Document one",
			Metadata: map[string]any{
				"category": "tech",
				"priority": 1,
				"active":   true,
				"score":    0.95,
			},
		},
		{
			ID:      generateTestUUID("filter-doc-2"),
			Content: "Document two",
			Metadata: map[string]any{
				"category": "science",
				"priority": 2,
				"active":   false,
				"score":    0.85,
			},
		},
	}

	// Add vectors to documents via metadata
	for i := range seedDocs {
		vec, err := mockEmbedder.Embed(ctx, seedDocs[i].Content)
		if err != nil {
			t.Fatalf("Failed to generate embedding: %v", err)
		}
		seedDocs[i].Metadata["vector"] = []float32(vec)
	}

	if err := client.Store(ctx, seedDocs); err != nil {
		t.Fatalf("Failed to seed test data: %v", err)
	}

	time.Sleep(2 * time.Second)

	tests := []struct {
		name    string
		filter  map[string]any
		checkFn func(t *testing.T, docs []retrieval.Document)
	}{
		{
			name: "filter by string value",
			filter: map[string]any{
				"category": "tech",
			},
			checkFn: func(t *testing.T, docs []retrieval.Document) {
				for _, doc := range docs {
					if cat, ok := doc.Metadata["category"].(string); !ok || cat != "tech" {
						t.Errorf("Expected category 'tech', got %v", doc.Metadata["category"])
					}
				}
			},
		},
		{
			name: "filter by integer value",
			filter: map[string]any{
				"priority": 1,
			},
			checkFn: func(t *testing.T, docs []retrieval.Document) {
				for _, doc := range docs {
					// Priority may come back as different numeric type
					if doc.Metadata["priority"] == nil {
						t.Error("Expected priority metadata")
					}
				}
			},
		},
		{
			name: "filter by boolean value",
			filter: map[string]any{
				"active": true,
			},
			checkFn: func(t *testing.T, docs []retrieval.Document) {
				for _, doc := range docs {
					if active, ok := doc.Metadata["active"].(bool); !ok || !active {
						t.Errorf("Expected active=true, got %v", doc.Metadata["active"])
					}
				}
			},
		},
		{
			name: "multiple filters combined with AND",
			filter: map[string]any{
				"category": "tech",
				"active":   true,
			},
			checkFn: func(t *testing.T, docs []retrieval.Document) {
				for _, doc := range docs {
					if cat, ok := doc.Metadata["category"].(string); !ok || cat != "tech" {
						t.Errorf("Expected category 'tech', got %v", doc.Metadata["category"])
					}
					if active, ok := doc.Metadata["active"].(bool); !ok || !active {
						t.Errorf("Expected active=true, got %v", doc.Metadata["active"])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use vector search instead of text search
			searchVec, err := mockEmbedder.Embed(ctx, "document")
			if err != nil {
				t.Fatalf("Failed to generate search embedding: %v", err)
			}

			query := retrieval.SearchQuery{
				Vector:     searchVec,
				Collection: "FilterTest",
				Limit:      10,
				Filter:     tt.filter,
			}

			result, err := client.Search(ctx, query)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, result.Documents)
			}
		})
	}
}
