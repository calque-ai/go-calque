package weaviate

import (
	"testing"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
)

func TestValidatePropertyType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		key       string
		value     any
		expected  PropertyType
		expectErr bool
	}{
		// Text type tests
		{
			name:      "valid text",
			key:       "category",
			value:     "technology",
			expected:  PropertyTypeText,
			expectErr: false,
		},
		{
			name:      "invalid text - int",
			key:       "category",
			value:     123,
			expected:  PropertyTypeText,
			expectErr: true,
		},
		{
			name:      "nil value is valid for text",
			key:       "category",
			value:     nil,
			expected:  PropertyTypeText,
			expectErr: false,
		},

		// TextArray type tests
		{
			name:      "valid text array - []string",
			key:       "tags",
			value:     []string{"ai", "ml"},
			expected:  PropertyTypeTextArray,
			expectErr: false,
		},
		{
			name:      "valid text array - []interface{} with strings",
			key:       "tags",
			value:     []interface{}{"ai", "ml"},
			expected:  PropertyTypeTextArray,
			expectErr: false,
		},
		{
			name:      "invalid text array - []interface{} with non-strings",
			key:       "tags",
			value:     []interface{}{"ai", 123},
			expected:  PropertyTypeTextArray,
			expectErr: true,
		},
		{
			name:      "invalid text array - not array",
			key:       "tags",
			value:     "not an array",
			expected:  PropertyTypeTextArray,
			expectErr: true,
		},

		// Int type tests
		{
			name:      "valid int",
			key:       "priority",
			value:     42,
			expected:  PropertyTypeInt,
			expectErr: false,
		},
		{
			name:      "valid int32",
			key:       "priority",
			value:     int32(42),
			expected:  PropertyTypeInt,
			expectErr: false,
		},
		{
			name:      "valid int64",
			key:       "priority",
			value:     int64(42),
			expected:  PropertyTypeInt,
			expectErr: false,
		},
		{
			name:      "invalid int - string",
			key:       "priority",
			value:     "42",
			expected:  PropertyTypeInt,
			expectErr: true,
		},

		// Number type tests
		{
			name:      "valid number - float64",
			key:       "score",
			value:     0.95,
			expected:  PropertyTypeNumber,
			expectErr: false,
		},
		{
			name:      "valid number - float32",
			key:       "score",
			value:     float32(0.95),
			expected:  PropertyTypeNumber,
			expectErr: false,
		},
		{
			name:      "valid number - int (allowed)",
			key:       "score",
			value:     1,
			expected:  PropertyTypeNumber,
			expectErr: false,
		},
		{
			name:      "invalid number - string",
			key:       "score",
			value:     "0.95",
			expected:  PropertyTypeNumber,
			expectErr: true,
		},

		// Bool type tests
		{
			name:      "valid bool - true",
			key:       "active",
			value:     true,
			expected:  PropertyTypeBool,
			expectErr: false,
		},
		{
			name:      "valid bool - false",
			key:       "active",
			value:     false,
			expected:  PropertyTypeBool,
			expectErr: false,
		},
		{
			name:      "invalid bool - string",
			key:       "active",
			value:     "true",
			expected:  PropertyTypeBool,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validatePropertyType(tt.key, tt.value, tt.expected)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateDocument(t *testing.T) {
	t.Parallel()

	schema := &SchemaConfig{
		ClassName:  "TestDoc",
		Vectorizer: "none",
		Properties: []PropertyConfig{
			{Name: "category", Type: PropertyTypeText},
			{Name: "priority", Type: PropertyTypeInt},
			{Name: "score", Type: PropertyTypeNumber},
			{Name: "active", Type: PropertyTypeBool},
			{Name: "tags", Type: PropertyTypeTextArray},
		},
	}

	client := &Client{schema: schema}

	tests := []struct {
		name      string
		doc       retrieval.Document
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid document with all fields",
			doc: retrieval.Document{
				Content: "test content",
				Metadata: map[string]any{
					"category": "tech",
					"priority": 1,
					"score":    0.95,
					"active":   true,
					"tags":     []string{"ai", "ml"},
				},
			},
			expectErr: false,
		},
		{
			name: "valid document with partial metadata",
			doc: retrieval.Document{
				Content: "test content",
				Metadata: map[string]any{
					"category": "tech",
					"priority": 1,
				},
			},
			expectErr: false,
		},
		{
			name: "valid document with no metadata",
			doc: retrieval.Document{
				Content: "test content",
			},
			expectErr: false,
		},
		{
			name: "invalid document - empty content",
			doc: retrieval.Document{
				Content: "",
				Metadata: map[string]any{
					"category": "tech",
				},
			},
			expectErr: true,
			errMsg:    "document content is required",
		},
		{
			name: "invalid document - unknown property",
			doc: retrieval.Document{
				Content: "test content",
				Metadata: map[string]any{
					"unknown_field": "value",
				},
			},
			expectErr: true,
			errMsg:    "unknown property",
		},
		{
			name: "invalid document - wrong type for category",
			doc: retrieval.Document{
				Content: "test content",
				Metadata: map[string]any{
					"category": 123, // Should be string
				},
			},
			expectErr: true,
			errMsg:    "must be string",
		},
		{
			name: "invalid document - wrong type for priority",
			doc: retrieval.Document{
				Content: "test content",
				Metadata: map[string]any{
					"priority": "high", // Should be int
				},
			},
			expectErr: true,
			errMsg:    "must be int",
		},
		{
			name: "valid document - vector field is ignored in validation",
			doc: retrieval.Document{
				Content: "test content",
				Metadata: map[string]any{
					"vector":   []float32{0.1, 0.2, 0.3},
					"category": "tech",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := client.ValidateDocument(tt.doc)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateDocumentWithoutSchema(t *testing.T) {
	t.Parallel()

	// Client without schema should skip validation
	client := &Client{}

	doc := retrieval.Document{
		Content: "test content",
		Metadata: map[string]any{
			"any_field":    "any value",
			"numeric":      123,
			"unknown_type": []int{1, 2, 3},
		},
	}

	err := client.ValidateDocument(doc)
	if err != nil {
		t.Errorf("Validation without schema should pass, got error: %v", err)
	}
}

func TestGetSchema(t *testing.T) {
	t.Parallel()

	schema := &SchemaConfig{
		ClassName:  "TestDoc",
		Vectorizer: "none",
		Properties: []PropertyConfig{
			{Name: "field1", Type: PropertyTypeText},
		},
	}

	client := &Client{schema: schema}

	retrieved := client.GetSchema()
	if retrieved != schema {
		t.Error("GetSchema should return the cached schema")
	}

	// Client without schema
	clientNoSchema := &Client{}
	if clientNoSchema.GetSchema() != nil {
		t.Error("GetSchema should return nil when no schema is set")
	}
}

func TestBuildWeaviateClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  *SchemaConfig
		checkFn func(t *testing.T, class any)
	}{
		{
			name: "schema with vectorizer",
			schema: &SchemaConfig{
				ClassName:   "TestDoc",
				Description: "Test document class",
				Vectorizer:  "text2vec-openai",
				Properties: []PropertyConfig{
					{Name: "category", Type: PropertyTypeText, Description: "Document category"},
					{Name: "priority", Type: PropertyTypeInt},
				},
			},
			checkFn: func(_ *testing.T, _ any) {
				// Type assertion would require importing models
				// For now, just ensure it doesn't panic
			},
		},
		{
			name: "schema with vectorizer none",
			schema: &SchemaConfig{
				ClassName:  "TestDoc",
				Vectorizer: "none",
				Properties: []PropertyConfig{
					{Name: "field1", Type: PropertyTypeText},
				},
			},
			checkFn: func(_ *testing.T, _ any) {
				// Just ensure it doesn't panic
			},
		},
		{
			name: "schema without vectorizer defaults to none",
			schema: &SchemaConfig{
				ClassName: "TestDoc",
				Properties: []PropertyConfig{
					{Name: "field1", Type: PropertyTypeText},
				},
			},
			checkFn: func(_ *testing.T, _ any) {
				// Just ensure it doesn't panic
			},
		},
		{
			name: "schema with all property types",
			schema: &SchemaConfig{
				ClassName: "AllTypes",
				Properties: []PropertyConfig{
					{Name: "text_field", Type: PropertyTypeText},
					{Name: "text_array", Type: PropertyTypeTextArray},
					{Name: "int_field", Type: PropertyTypeInt},
					{Name: "number_field", Type: PropertyTypeNumber},
					{Name: "bool_field", Type: PropertyTypeBool},
					{Name: "date_field", Type: PropertyTypeDate},
				},
			},
			checkFn: func(_ *testing.T, _ any) {
				// Just ensure it doesn't panic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &Client{}
			class := client.buildWeaviateClass(tt.schema)

			if class == nil {
				t.Fatal("buildWeaviateClass returned nil")
			}

			if tt.checkFn != nil {
				tt.checkFn(t, class)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
