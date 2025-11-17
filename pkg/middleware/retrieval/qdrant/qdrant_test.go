package qdrant

import (
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	qd "github.com/qdrant/go-client/qdrant"
)

func TestBuildQdrantPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   retrieval.Document
		checkFn func(t *testing.T, result map[string]*qd.Value)
	}{
		{
			name: "document with content only",
			input: retrieval.Document{
				Content: "This is test content",
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if result == nil {
					t.Fatal("Expected non-nil payload")
				}
				if content := result["content"].GetStringValue(); content != "This is test content" {
					t.Errorf("Expected content 'This is test content', got %q", content)
				}
			},
		},
		{
			name: "document with empty content",
			input: retrieval.Document{
				Content: "",
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if result == nil {
					t.Fatal("Expected non-nil payload")
				}
				if content := result["content"].GetStringValue(); content != "" {
					t.Errorf("Expected empty content, got %q", content)
				}
			},
		},
		{
			name: "document with string metadata",
			input: retrieval.Document{
				Content: "Test",
				Metadata: map[string]any{
					"author":   "John Doe",
					"category": "tech",
				},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if author := result["author"].GetStringValue(); author != "John Doe" {
					t.Errorf("Expected author 'John Doe', got %q", author)
				}
				if category := result["category"].GetStringValue(); category != "tech" {
					t.Errorf("Expected category 'tech', got %q", category)
				}
			},
		},
		{
			name: "document with int metadata",
			input: retrieval.Document{
				Content: "Test",
				Metadata: map[string]any{
					"year":  2024,
					"count": 42,
				},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if year := result["year"].GetIntegerValue(); year != 2024 {
					t.Errorf("Expected year 2024, got %d", year)
				}
				if count := result["count"].GetIntegerValue(); count != 42 {
					t.Errorf("Expected count 42, got %d", count)
				}
			},
		},
		{
			name: "document with int64 metadata",
			input: retrieval.Document{
				Content: "Test",
				Metadata: map[string]any{
					"large_number": int64(9223372036854775807),
				},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if val := result["large_number"].GetIntegerValue(); val != 9223372036854775807 {
					t.Errorf("Expected large_number 9223372036854775807, got %d", val)
				}
			},
		},
		{
			name: "document with float64 metadata",
			input: retrieval.Document{
				Content: "Test",
				Metadata: map[string]any{
					"score":  0.95,
					"rating": 4.7,
				},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if score := result["score"].GetDoubleValue(); score != 0.95 {
					t.Errorf("Expected score 0.95, got %f", score)
				}
				if rating := result["rating"].GetDoubleValue(); rating != 4.7 {
					t.Errorf("Expected rating 4.7, got %f", rating)
				}
			},
		},
		{
			name: "document with bool metadata",
			input: retrieval.Document{
				Content: "Test",
				Metadata: map[string]any{
					"active":   true,
					"archived": false,
				},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if active := result["active"].GetBoolValue(); !active {
					t.Errorf("Expected active to be true")
				}
				if archived := result["archived"].GetBoolValue(); archived {
					t.Errorf("Expected archived to be false")
				}
			},
		},
		{
			name: "document with mixed metadata types",
			input: retrieval.Document{
				Content: "Test",
				Metadata: map[string]any{
					"title":     "My Document",
					"year":      2024,
					"score":     0.88,
					"published": true,
				},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if title := result["title"].GetStringValue(); title != "My Document" {
					t.Errorf("Expected title 'My Document', got %q", title)
				}
				if year := result["year"].GetIntegerValue(); year != 2024 {
					t.Errorf("Expected year 2024, got %d", year)
				}
				if score := result["score"].GetDoubleValue(); score != 0.88 {
					t.Errorf("Expected score 0.88, got %f", score)
				}
				if published := result["published"].GetBoolValue(); !published {
					t.Errorf("Expected published to be true")
				}
			},
		},
		{
			name: "document with unsupported metadata type",
			input: retrieval.Document{
				Content: "Test",
				Metadata: map[string]any{
					"tags": []string{"tag1", "tag2"},
				},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				// Unsupported types should be converted to string
				if tags := result["tags"].GetStringValue(); tags == "" {
					t.Error("Expected tags to be converted to string")
				}
			},
		},
		{
			name: "document with timestamps",
			input: retrieval.Document{
				Content: "Test",
				Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Updated: time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC),
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				if created := result["created"].GetStringValue(); created == "" {
					t.Error("Expected created timestamp to be set")
				}
				if updated := result["updated"].GetStringValue(); updated == "" {
					t.Error("Expected updated timestamp to be set")
				}
			},
		},
		{
			name: "document with zero timestamps",
			input: retrieval.Document{
				Content: "Test",
				Created: time.Time{},
				Updated: time.Time{},
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				// Zero timestamps should not be added to payload
				if _, exists := result["created"]; exists {
					t.Error("Expected zero created timestamp to not be in payload")
				}
				if _, exists := result["updated"]; exists {
					t.Error("Expected zero updated timestamp to not be in payload")
				}
			},
		},
		{
			name: "document with nil metadata",
			input: retrieval.Document{
				Content:  "Test",
				Metadata: nil,
			},
			checkFn: func(t *testing.T, result map[string]*qd.Value) {
				// Should still have content
				if content := result["content"].GetStringValue(); content != "Test" {
					t.Errorf("Expected content 'Test', got %q", content)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildQdrantPayload(tt.input)

			if result == nil {
				t.Fatal("Expected non-nil payload")
			}

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestBuildQdrantFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     map[string]any
		expectNil bool
		checkFn   func(t *testing.T, result *qd.Filter)
	}{
		{
			name:      "empty filter map returns nil",
			input:     map[string]any{},
			expectNil: true,
		},
		{
			name:      "nil filter map returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name: "single string filter",
			input: map[string]any{
				"category": "tech",
			},
			expectNil: false,
			checkFn: func(t *testing.T, result *qd.Filter) {
				if result == nil {
					t.Fatal("Expected non-nil filter")
				}
				if result.Must == nil || len(result.Must) != 1 {
					t.Errorf("Expected 1 condition in Must, got %d", len(result.Must))
				}
			},
		},
		{
			name: "multiple filters",
			input: map[string]any{
				"category": "tech",
				"year":     2024,
				"active":   true,
			},
			expectNil: false,
			checkFn: func(t *testing.T, result *qd.Filter) {
				if result == nil {
					t.Fatal("Expected non-nil filter")
				}
				if result.Must == nil || len(result.Must) != 3 {
					t.Errorf("Expected 3 conditions in Must, got %d", len(result.Must))
				}
			},
		},
		{
			name: "filter with various types",
			input: map[string]any{
				"string_field": "value",
				"int_field":    42,
				"float_field":  3.14,
				"bool_field":   false,
			},
			expectNil: false,
			checkFn: func(t *testing.T, result *qd.Filter) {
				if result == nil {
					t.Fatal("Expected non-nil filter")
				}
				if result.Must == nil || len(result.Must) != 4 {
					t.Errorf("Expected 4 conditions in Must, got %d", len(result.Must))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildQdrantFilter(tt.input)

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil filter, got non-nil")
				}
				return
			}

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestExtractPayloadData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload map[string]*qd.Value
		checkFn func(t *testing.T, doc *retrieval.Document)
	}{
		{
			name:    "empty payload",
			payload: map[string]*qd.Value{},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if doc.Content != "" {
					t.Errorf("Expected empty content, got %q", doc.Content)
				}
				if doc.Metadata == nil {
					t.Error("Expected non-nil metadata map")
				}
			},
		},
		{
			name: "payload with content",
			payload: map[string]*qd.Value{
				"content": qd.NewValueString("Test content"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if doc.Content != "Test content" {
					t.Errorf("Expected content 'Test content', got %q", doc.Content)
				}
			},
		},
		{
			name: "payload with string values",
			payload: map[string]*qd.Value{
				"title":  qd.NewValueString("My Document"),
				"author": qd.NewValueString("John Doe"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if title, ok := doc.Metadata["title"].(string); !ok || title != "My Document" {
					t.Errorf("Expected title 'My Document', got %v", doc.Metadata["title"])
				}
				if author, ok := doc.Metadata["author"].(string); !ok || author != "John Doe" {
					t.Errorf("Expected author 'John Doe', got %v", doc.Metadata["author"])
				}
			},
		},
		{
			name: "payload with integer values",
			payload: map[string]*qd.Value{
				"year":  qd.NewValueInt(2024),
				"count": qd.NewValueInt(100),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if year, ok := doc.Metadata["year"].(int64); !ok || year != 2024 {
					t.Errorf("Expected year 2024, got %v", doc.Metadata["year"])
				}
				if count, ok := doc.Metadata["count"].(int64); !ok || count != 100 {
					t.Errorf("Expected count 100, got %v", doc.Metadata["count"])
				}
			},
		},
		{
			name: "payload with double values",
			payload: map[string]*qd.Value{
				"score":  qd.NewValueDouble(0.95),
				"rating": qd.NewValueDouble(4.5),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if score, ok := doc.Metadata["score"].(float64); !ok || score != 0.95 {
					t.Errorf("Expected score 0.95, got %v", doc.Metadata["score"])
				}
				if rating, ok := doc.Metadata["rating"].(float64); !ok || rating != 4.5 {
					t.Errorf("Expected rating 4.5, got %v", doc.Metadata["rating"])
				}
			},
		},
		{
			name: "payload with bool values",
			payload: map[string]*qd.Value{
				"active":   qd.NewValueBool(true),
				"archived": qd.NewValueBool(false),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if active, ok := doc.Metadata["active"].(bool); !ok || !active {
					t.Errorf("Expected active true, got %v", doc.Metadata["active"])
				}
				if archived, ok := doc.Metadata["archived"].(bool); !ok || archived {
					t.Errorf("Expected archived false, got %v", doc.Metadata["archived"])
				}
			},
		},
		{
			name: "payload skips timestamp fields",
			payload: map[string]*qd.Value{
				"content": qd.NewValueString("Test"),
				"created": qd.NewValueString("2024-01-15T10:30:00Z"),
				"updated": qd.NewValueString("2024-01-16T14:45:00Z"),
				"other":   qd.NewValueString("value"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				// created and updated should not be in metadata
				if _, exists := doc.Metadata["created"]; exists {
					t.Error("Expected 'created' to be skipped in metadata")
				}
				if _, exists := doc.Metadata["updated"]; exists {
					t.Error("Expected 'updated' to be skipped in metadata")
				}
				// other fields should be present
				if other, ok := doc.Metadata["other"].(string); !ok || other != "value" {
					t.Errorf("Expected other 'value', got %v", doc.Metadata["other"])
				}
			},
		},
		{
			name: "payload with mixed types",
			payload: map[string]*qd.Value{
				"content": qd.NewValueString("Test"),
				"title":   qd.NewValueString("Doc"),
				"year":    qd.NewValueInt(2024),
				"score":   qd.NewValueDouble(0.88),
				"active":  qd.NewValueBool(true),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if doc.Content != "Test" {
					t.Errorf("Expected content 'Test', got %q", doc.Content)
				}
				// Metadata should have: content, title, year, score, active = 5 fields
				if len(doc.Metadata) != 5 {
					t.Errorf("Expected 5 metadata fields, got %d", len(doc.Metadata))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &Client{}
			doc := &retrieval.Document{
				Metadata: make(map[string]any),
			}

			client.extractPayloadData(tt.payload, doc)

			if tt.checkFn != nil {
				tt.checkFn(t, doc)
			}
		})
	}
}

func TestExtractTimestamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload map[string]*qd.Value
		checkFn func(t *testing.T, doc *retrieval.Document)
	}{
		{
			name:    "empty payload",
			payload: map[string]*qd.Value{},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if !doc.Created.IsZero() {
					t.Error("Expected zero Created timestamp")
				}
				if !doc.Updated.IsZero() {
					t.Error("Expected zero Updated timestamp")
				}
			},
		},
		{
			name: "payload with valid created timestamp",
			payload: map[string]*qd.Value{
				"created": qd.NewValueString("2024-01-15T10:30:00Z"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				if !doc.Created.Equal(expected) {
					t.Errorf("Expected created %v, got %v", expected, doc.Created)
				}
			},
		},
		{
			name: "payload with valid updated timestamp",
			payload: map[string]*qd.Value{
				"updated": qd.NewValueString("2024-01-16T14:45:00Z"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				expected := time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC)
				if !doc.Updated.Equal(expected) {
					t.Errorf("Expected updated %v, got %v", expected, doc.Updated)
				}
			},
		},
		{
			name: "payload with both timestamps",
			payload: map[string]*qd.Value{
				"created": qd.NewValueString("2024-01-15T10:30:00Z"),
				"updated": qd.NewValueString("2024-01-16T14:45:00Z"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				expectedCreated := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				expectedUpdated := time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC)
				if !doc.Created.Equal(expectedCreated) {
					t.Errorf("Expected created %v, got %v", expectedCreated, doc.Created)
				}
				if !doc.Updated.Equal(expectedUpdated) {
					t.Errorf("Expected updated %v, got %v", expectedUpdated, doc.Updated)
				}
			},
		},
		{
			name: "payload with invalid created timestamp format",
			payload: map[string]*qd.Value{
				"created": qd.NewValueString("not-a-timestamp"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if !doc.Created.IsZero() {
					t.Error("Expected zero Created timestamp for invalid format")
				}
			},
		},
		{
			name: "payload with invalid updated timestamp format",
			payload: map[string]*qd.Value{
				"updated": qd.NewValueString("invalid-date"),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if !doc.Updated.IsZero() {
					t.Error("Expected zero Updated timestamp for invalid format")
				}
			},
		},
		{
			name: "payload with empty string timestamps",
			payload: map[string]*qd.Value{
				"created": qd.NewValueString(""),
				"updated": qd.NewValueString(""),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				if !doc.Created.IsZero() {
					t.Error("Expected zero Created timestamp for empty string")
				}
				if !doc.Updated.IsZero() {
					t.Error("Expected zero Updated timestamp for empty string")
				}
			},
		},
		{
			name: "payload with non-string timestamp values",
			payload: map[string]*qd.Value{
				"created": qd.NewValueInt(123456),
				"updated": qd.NewValueBool(true),
			},
			checkFn: func(t *testing.T, doc *retrieval.Document) {
				// Should handle gracefully by not setting timestamps
				if !doc.Created.IsZero() {
					t.Error("Expected zero Created timestamp for non-string value")
				}
				if !doc.Updated.IsZero() {
					t.Error("Expected zero Updated timestamp for non-string value")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &Client{}
			doc := &retrieval.Document{}

			client.extractTimestamps(tt.payload, doc)

			if tt.checkFn != nil {
				tt.checkFn(t, doc)
			}
		})
	}
}

func TestConvertQdrantPoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		point   *qd.ScoredPoint
		checkFn func(t *testing.T, doc retrieval.Document)
	}{
		{
			name: "point with score only",
			point: &qd.ScoredPoint{
				Score: 0.95,
			},
			checkFn: func(t *testing.T, doc retrieval.Document) {
				// Use approximate comparison for float32->float64 conversion
				if doc.Score < 0.94999 || doc.Score > 0.95001 {
					t.Errorf("Expected score ~0.95, got %f", doc.Score)
				}
			},
		},
		{
			name: "point with ID and score",
			point: &qd.ScoredPoint{
				Id: &qd.PointId{
					PointIdOptions: &qd.PointId_Uuid{Uuid: "test-id-123"},
				},
				Score: 0.88,
			},
			checkFn: func(t *testing.T, doc retrieval.Document) {
				// Use approximate comparison for float32->float64 conversion
				if doc.Score < 0.87999 || doc.Score > 0.88001 {
					t.Errorf("Expected score ~0.88, got %f", doc.Score)
				}
				// ID extraction depends on PointId.String() implementation
			},
		},
		{
			name: "point with payload",
			point: &qd.ScoredPoint{
				Score: 0.92,
				Payload: map[string]*qd.Value{
					"content": qd.NewValueString("Test content"),
					"title":   qd.NewValueString("Test Title"),
				},
			},
			checkFn: func(t *testing.T, doc retrieval.Document) {
				if doc.Content != "Test content" {
					t.Errorf("Expected content 'Test content', got %q", doc.Content)
				}
				if doc.Metadata == nil {
					t.Fatal("Expected non-nil metadata")
				}
			},
		},
		{
			name: "point with nil payload",
			point: &qd.ScoredPoint{
				Score:   0.85,
				Payload: nil,
			},
			checkFn: func(t *testing.T, doc retrieval.Document) {
				// Use approximate comparison for float32->float64 conversion
				if doc.Score < 0.84999 || doc.Score > 0.85001 {
					t.Errorf("Expected score ~0.85, got %f", doc.Score)
				}
				// Should have default timestamps
				if doc.Created.IsZero() {
					t.Error("Expected non-zero Created timestamp")
				}
				if doc.Updated.IsZero() {
					t.Error("Expected non-zero Updated timestamp")
				}
			},
		},
		{
			name: "point with timestamps in payload",
			point: &qd.ScoredPoint{
				Score: 0.90,
				Payload: map[string]*qd.Value{
					"content": qd.NewValueString("Test"),
					"created": qd.NewValueString("2024-01-15T10:30:00Z"),
					"updated": qd.NewValueString("2024-01-16T14:45:00Z"),
				},
			},
			checkFn: func(t *testing.T, doc retrieval.Document) {
				expectedCreated := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				expectedUpdated := time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC)
				if !doc.Created.Equal(expectedCreated) {
					t.Errorf("Expected created %v, got %v", expectedCreated, doc.Created)
				}
				if !doc.Updated.Equal(expectedUpdated) {
					t.Errorf("Expected updated %v, got %v", expectedUpdated, doc.Updated)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &Client{}
			result := client.convertQdrantPoint(tt.point)

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestQdrantConfigValidation(t *testing.T) {
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
			name: "valid URL with default collection name",
			config: &Config{
				URL: "http://localhost:6333",
			},
			expectErr: false,
		},
		{
			name: "valid URL with custom collection name",
			config: &Config{
				URL:            "http://localhost:6333",
				CollectionName: "my_documents",
			},
			expectErr: false,
		},
		{
			name: "URL with https scheme",
			config: &Config{
				URL: "https://my-qdrant-cluster.com:6333",
			},
			expectErr: false,
		},
		{
			name: "URL without scheme",
			config: &Config{
				URL: "localhost:6333",
			},
			expectErr: false,
		},
		{
			name: "URL with non-standard port",
			config: &Config{
				URL: "http://localhost:8080",
			},
			expectErr: false,
		},
		{
			name: "URL with API key",
			config: &Config{
				URL:    "http://localhost:6333",
				APIKey: "my-api-key",
			},
			expectErr: false,
		},
		{
			name: "invalid URL format",
			config: &Config{
				URL: "ht!tp://invalid url",
			},
			expectErr: true,
		},
		{
			name: "URL with invalid port",
			config: &Config{
				URL: "http://localhost:invalid-port",
			},
			expectErr: true,
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
					t.Logf("Expected error message %q, got %q", tt.errMsg, err.Error())
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

			// Verify default collection name is set
			if tt.config.CollectionName == "" && client.collectionName != "documents" {
				t.Errorf("Expected default collectionName 'documents', got %q", client.collectionName)
			}

			// Verify custom collection name is preserved
			if tt.config.CollectionName != "" && client.collectionName != tt.config.CollectionName {
				t.Errorf("Expected collectionName %q, got %q", tt.config.CollectionName, client.collectionName)
			}

			// Clean up
			if client != nil {
				_ = client.Close()
			}
		})
	}
}
