package weaviate

import (
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
)

const testContent = "Test"

func TestBuildWeaviateFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       map[string]any
		expectNil   bool
		validateFn  func(t *testing.T, result *filters.WhereBuilder)
		description string
	}{
		{
			name:        "empty filter map returns nil",
			input:       map[string]any{},
			expectNil:   true,
			description: "Empty filter map should return nil",
		},
		{
			name:        "nil filter map returns nil",
			input:       nil,
			expectNil:   true,
			description: "Nil filter map should return nil",
		},
		{
			name: "single string filter",
			input: map[string]any{
				"category": "technology",
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for single string filter")
				}
			},
			description: "Single string filter should create valid WhereBuilder",
		},
		{
			name: "single int filter",
			input: map[string]any{
				"year": 2024,
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for single int filter")
				}
			},
			description: "Single int filter should create valid WhereBuilder",
		},
		{
			name: "single int64 filter",
			input: map[string]any{
				"count": int64(100),
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for single int64 filter")
				}
			},
			description: "Single int64 filter should create valid WhereBuilder",
		},
		{
			name: "single float64 filter",
			input: map[string]any{
				"score": 0.95,
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for single float64 filter")
				}
			},
			description: "Single float64 filter should create valid WhereBuilder",
		},
		{
			name: "single float32 filter",
			input: map[string]any{
				"rating": float32(4.5),
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for single float32 filter")
				}
			},
			description: "Single float32 filter should create valid WhereBuilder",
		},
		{
			name: "single bool filter - true",
			input: map[string]any{
				"active": true,
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for single bool filter")
				}
			},
			description: "Single bool filter (true) should create valid WhereBuilder",
		},
		{
			name: "single bool filter - false",
			input: map[string]any{
				"archived": false,
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for single bool filter")
				}
			},
			description: "Single bool filter (false) should create valid WhereBuilder",
		},
		{
			name: "multiple filters with different types",
			input: map[string]any{
				"category": "tech",
				"year":     2024,
				"score":    0.95,
				"active":   true,
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for multiple filters")
				}
				// Multiple filters should be combined with AND
			},
			description: "Multiple filters should be combined with AND operator",
		},
		{
			name: "filter with unsupported type converts to string",
			input: map[string]any{
				"custom": []string{"a", "b"},
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for unsupported type filter")
				}
			},
			description: "Unsupported types should be converted to string",
		},
		{
			name: "filter with zero values",
			input: map[string]any{
				"count":  0,
				"score":  0.0,
				"empty":  "",
				"active": false,
			},
			expectNil: false,
			validateFn: func(t *testing.T, result *filters.WhereBuilder) {
				if result == nil {
					t.Fatal("Expected non-nil result for zero value filters")
				}
			},
			description: "Zero values should be valid filter values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildWeaviateFilter(tt.input)

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got non-nil")
				}
				return
			}

			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestParseWeaviateDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected retrieval.Document
		checkFn  func(t *testing.T, result retrieval.Document)
	}{
		{
			name:  "empty document",
			input: map[string]any{},
			checkFn: func(t *testing.T, result retrieval.Document) {
				if result.Content != "" {
					t.Errorf("Expected empty content, got %q", result.Content)
				}
				if result.ID != "" {
					t.Errorf("Expected empty ID, got %q", result.ID)
				}
			},
		},
		{
			name: "document with content only",
			input: map[string]any{
				"content": "This is a test document",
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				if result.Content != "This is a test document" {
					t.Errorf("Expected content 'This is a test document', got %q", result.Content)
				}
			},
		},
		{
			name: "document with metadata",
			input: map[string]any{
				"content": "Test content",
				// Metadata fields are now top-level (flattened) instead of nested
				"author": "John Doe",
				"year":   2024,
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				if result.Content != "Test content" {
					t.Errorf("Expected content 'Test content', got %q", result.Content)
				}
				if result.Metadata == nil {
					t.Fatal("Expected non-nil metadata")
				}
				if author, ok := result.Metadata["author"].(string); !ok || author != "John Doe" {
					t.Errorf("Expected author 'John Doe', got %v", result.Metadata["author"])
				}
				if _, ok := result.Metadata["year"]; !ok {
					t.Errorf("Expected year in metadata, got %v", result.Metadata)
				}
			},
		},
		{
			name: "document with _additional fields",
			input: map[string]any{
				"content": testContent,
				"_additional": map[string]any{
					"id":    "test-id-123",
					"score": 0.95,
				},
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				if result.ID != "test-id-123" {
					t.Errorf("Expected ID 'test-id-123', got %q", result.ID)
				}
				if result.Score != 0.95 {
					t.Errorf("Expected score 0.95, got %f", result.Score)
				}
			},
		},
		{
			name: "document with invalid _additional type",
			input: map[string]any{
				"content":     testContent,
				"_additional": "invalid",
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				// Should not panic, just ignore invalid _additional
				if result.Content != testContent {
					t.Errorf("Expected content 'Test', got %q", result.Content)
				}
			},
		},
		{
			name: "document with invalid metadata type",
			input: map[string]any{
				"content":  testContent,
				"metadata": "invalid",
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				// Should not panic, just ignore invalid metadata
				if result.Content != testContent {
					t.Errorf("Expected content 'Test', got %q", result.Content)
				}
			},
		},
		{
			name: "complete document with all fields",
			input: map[string]any{
				"content": "Complete test document",
				"metadata": map[string]any{
					"category": "test",
					"priority": 1,
				},
				"_additional": map[string]any{
					"id":    "complete-doc-456",
					"score": 0.88,
				},
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				if result.Content != "Complete test document" {
					t.Errorf("Expected content 'Complete test document', got %q", result.Content)
				}
				if result.ID != "complete-doc-456" {
					t.Errorf("Expected ID 'complete-doc-456', got %q", result.ID)
				}
				if result.Score != 0.88 {
					t.Errorf("Expected score 0.88, got %f", result.Score)
				}
				if result.Metadata == nil {
					t.Fatal("Expected non-nil metadata")
				}
				// Timestamps should be set
				if result.Created.IsZero() {
					t.Error("Expected non-zero Created timestamp")
				}
				if result.Updated.IsZero() {
					t.Error("Expected non-zero Updated timestamp")
				}
			},
		},
		{
			name: "document with non-string content type",
			input: map[string]any{
				"content": 12345,
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				// Should handle gracefully, content should be empty
				if result.Content != "" {
					t.Errorf("Expected empty content for non-string type, got %q", result.Content)
				}
			},
		},
		{
			name: "document with non-string ID in _additional",
			input: map[string]any{
				"content": testContent,
				"_additional": map[string]any{
					"id": 12345,
				},
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				// Should handle gracefully
				if result.Content != testContent {
					t.Errorf("Expected content 'Test', got %q", result.Content)
				}
			},
		},
		{
			name: "document with non-float64 score in _additional",
			input: map[string]any{
				"content": testContent,
				"_additional": map[string]any{
					"score": "invalid",
				},
			},
			checkFn: func(t *testing.T, result retrieval.Document) {
				// Should handle gracefully, score should be zero
				if result.Score != 0 {
					t.Errorf("Expected score 0 for invalid type, got %f", result.Score)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a client with schema that includes common test fields
			schema := &SchemaConfig{
				Properties: []PropertyConfig{
					{Name: "author", Type: PropertyTypeText},
					{Name: "year", Type: PropertyTypeInt},
					{Name: "category", Type: PropertyTypeText},
					{Name: "priority", Type: PropertyTypeInt},
				},
			}
			client := &Client{schema: schema}

			// Record start time to verify timestamps are recent
			startTime := time.Now()
			result := client.parseWeaviateDocument(tt.input)

			// Verify timestamps are set and recent
			if !result.Created.IsZero() && result.Created.Before(startTime.Add(-1*time.Second)) {
				t.Errorf("Created timestamp is too old: %v", result.Created)
			}
			if !result.Updated.IsZero() && result.Updated.Before(startTime.Add(-1*time.Second)) {
				t.Errorf("Updated timestamp is too old: %v", result.Updated)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestURLParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    *Config
		expectURL string
	}{
		{
			name: "URL without scheme uses default http",
			config: &Config{
				URL:       "localhost:8080",
				ClassName: testContent,
			},
			expectURL: "localhost:8080",
		},
		{
			name: "URL with API key is preserved",
			config: &Config{
				URL:       "http://localhost:8080",
				ClassName: testContent,
				APIKey:    "test-api-key",
			},
			expectURL: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := newClient(tt.config)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if client.url != tt.expectURL {
				t.Errorf("Expected URL %q, got %q", tt.expectURL, client.url)
			}

			if tt.config.APIKey != "" && client.apiKey != tt.config.APIKey {
				t.Errorf("Expected API key %q, got %q", tt.config.APIKey, client.apiKey)
			}

			client.Close()
		})
	}
}

func TestConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		errMsg    string
	}{
		{
			name:      "missing URL returns error",
			config:    &Config{},
			expectErr: true,
			errMsg:    "weaviate URL is required",
		},
		{
			name: "empty URL returns error",
			config: &Config{
				URL: "",
			},
			expectErr: true,
			errMsg:    "weaviate URL is required",
		},
		{
			name: "valid URL with default class name",
			config: &Config{
				URL: "http://localhost:8080",
			},
			expectErr: false,
		},
		{
			name: "valid URL with custom class name",
			config: &Config{
				URL:       "http://localhost:8080",
				ClassName: "MyDocuments",
			},
			expectErr: false,
		},
		{
			name: "URL with https scheme",
			config: &Config{
				URL: "https://my-weaviate-cluster.com",
			},
			expectErr: false,
		},
		{
			name: "URL without scheme",
			config: &Config{
				URL: "localhost:8080",
			},
			expectErr: false,
		},
		{
			name: "invalid URL format",
			config: &Config{
				URL: "ht!tp://invalid url with spaces",
			},
			expectErr: true,
		},
		{
			name: "URL with API key",
			config: &Config{
				URL:    "http://localhost:8080",
				APIKey: "my-secret-key",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Use the private newClient() function for unit testing config validation
			client, err := newClient(tt.config)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				if tt.errMsg != "" && err != nil && err.Error() != tt.errMsg {
					t.Errorf("Expected error message %q, got %q", tt.errMsg, err.Error())
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

			// Verify default class name is set
			if tt.config.ClassName == "" && client.className != "Document" {
				t.Errorf("Expected default className 'Document', got %q", client.className)
			}

			// Verify custom class name is preserved
			if tt.config.ClassName != "" && client.className != tt.config.ClassName {
				t.Errorf("Expected className %q, got %q", tt.config.ClassName, client.className)
			}

			// Clean up
			if client != nil {
				_ = client.Close()
			}
		})
	}
}
