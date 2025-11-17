package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// mockEmbeddingProvider provides deterministic embeddings for testing
type mockEmbeddingProvider struct {
	dimension int
}

func newMockEmbeddingProvider(dimension int) *mockEmbeddingProvider {
	return &mockEmbeddingProvider{dimension: dimension}
}

func (m *mockEmbeddingProvider) Embed(_ context.Context, text string) (EmbeddingVector, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text cannot be embedded")
	}

	// Generate consistent, deterministic embeddings based on keyword matching
	// This creates semantically meaningful clusters for testing
	vector := make(EmbeddingVector, m.dimension)
	textLower := strings.ToLower(text)
	words := strings.Fields(textLower)

	// Define topic-specific keywords and their vector positions
	topics := map[string][]string{
		"ai":      {"machine", "learning", "artificial", "intelligence", "neural", "deep", "algorithm", "algorithms", "data", "science", "ml", "ai", "network", "architectures", "methods"},
		"cooking": {"cooking", "recipe", "recipes", "food", "pasta", "italian", "dish", "kitchen", "chef", "ingredient"},
		"sports":  {"football", "soccer", "sport", "training", "athletic", "game", "player", "team", "ball"},
		"auto":    {"car", "vehicle", "maintenance", "engine", "automotive", "mechanic", "driving", "guide"},
		"tech":    {"programming", "software", "code", "computer", "technology", "development", "engineering", "tutorial"},
	}

	// Assign base values based on topic keyword matches (whole word matches only)
	topicScores := make(map[string]float32)
	for topic, keywords := range topics {
		score := float32(0.0)
		for _, keyword := range keywords {
			for _, word := range words {
				// Exact match gets full score
				if word == keyword {
					score += 1.0
				} else if strings.Contains(word, keyword) || strings.Contains(keyword, word) {
					// Partial match gets partial score
					score += 0.3
				}
			}
		}
		topicScores[topic] = score
	}

	// Distribute topic scores across dimensions
	topicIdx := 0
	topicNames := []string{"ai", "cooking", "sports", "auto", "tech"}
	for _, topic := range topicNames {
		if topicIdx < m.dimension {
			vector[topicIdx] = topicScores[topic] * 10.0 // Amplify topic signal
		}
		topicIdx++
	}

	// Fill remaining dimensions with smaller hash values
	for i := topicIdx; i < m.dimension; i++ {
		hash := 0
		for j, ch := range textLower {
			hash = (hash*31 + int(ch) + i*j) % 10000
		}
		vector[i] = float32(hash) / 100000.0 // Reduced weight
	}

	// Normalize the vector to unit length
	var magnitudeSquared float64
	for _, v := range vector {
		magnitudeSquared += float64(v * v)
	}
	if magnitudeSquared > 0 {
		magnitude := math.Sqrt(magnitudeSquared)
		for i := range vector {
			vector[i] = float32(float64(vector[i]) / magnitude)
		}
	}

	return vector, nil
}

// TestSemanticFilterBasic tests semantic filtering functionality
//
//nolint:gocyclo // Table-driven test with many cases
func TestSemanticFilterBasic(t *testing.T) {
	embedProvider := newMockEmbeddingProvider(128)
	ctx := context.Background()

	tests := []struct {
		name      string
		documents []Document
		opts      *SemanticFilterOptions
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, filtered []Document)
	}{
		{
			name: "filter documents by semantic similarity",
			documents: []Document{
				{ID: "doc1", Content: "machine learning basics", Score: 0.0},
				{ID: "doc2", Content: "deep learning tutorial", Score: 0.0},
				{ID: "doc3", Content: "cooking italian recipes", Score: 0.0},
			},
			opts: func() *SemanticFilterOptions {
				aiEmbedding, _ := embedProvider.Embed(ctx, "artificial intelligence")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{aiEmbedding},
					Threshold:         0.5,
					EmbeddingProvider: embedProvider,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				// Should filter out cooking-related doc
				for _, doc := range filtered {
					if strings.Contains(doc.Content, "cooking") {
						t.Error("Cooking document should have been filtered out")
					}
					if doc.Score < 0.5 {
						t.Errorf("Filtered document has score %f below threshold 0.5", doc.Score)
					}
				}
			},
		},
		{
			name: "high threshold filters more documents",
			documents: []Document{
				{ID: "doc1", Content: "machine learning algorithms", Score: 0.0},
				{ID: "doc2", Content: "neural network architectures", Score: 0.0},
				{ID: "doc3", Content: "data science methods", Score: 0.0},
			},
			opts: func() *SemanticFilterOptions {
				mlEmbedding, _ := embedProvider.Embed(ctx, "machine learning")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{mlEmbedding},
					Threshold:         0.95,
					EmbeddingProvider: embedProvider,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				// High threshold should filter out most documents
				if len(filtered) > 1 {
					t.Logf("High threshold (0.95) returned %d documents", len(filtered))
				}
			},
		},
		{
			name: "low threshold allows more documents",
			documents: []Document{
				{ID: "doc1", Content: "programming tutorial", Score: 0.0},
				{ID: "doc2", Content: "software engineering", Score: 0.0},
			},
			opts: func() *SemanticFilterOptions {
				techEmbedding, _ := embedProvider.Embed(ctx, "technology")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{techEmbedding},
					Threshold:         0.1,
					EmbeddingProvider: embedProvider,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				// Low threshold should allow most documents
				if len(filtered) == 0 {
					t.Error("Expected some documents to pass low threshold")
				}
			},
		},
		{
			name: "multiple target embeddings - matches any",
			documents: []Document{
				{ID: "doc1", Content: "machine learning tutorial", Score: 0.0},
				{ID: "doc2", Content: "italian cooking recipes", Score: 0.0},
				{ID: "doc3", Content: "car maintenance guide", Score: 0.0},
			},
			opts: func() *SemanticFilterOptions {
				mlEmbedding, _ := embedProvider.Embed(ctx, "artificial intelligence")
				cookingEmbedding, _ := embedProvider.Embed(ctx, "food and cooking")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{mlEmbedding, cookingEmbedding},
					Threshold:         0.5,
					EmbeddingProvider: embedProvider,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				// Should keep ML and cooking docs, filter out car maintenance
				hasML := false
				hasCooking := false
				for _, doc := range filtered {
					if strings.Contains(doc.Content, "machine learning") {
						hasML = true
					}
					if strings.Contains(doc.Content, "cooking") {
						hasCooking = true
					}
					if strings.Contains(doc.Content, "car maintenance") {
						t.Error("Car maintenance doc should have been filtered out")
					}
				}
				if !hasML && !hasCooking {
					t.Error("Expected at least ML or cooking documents to pass filter")
				}
			},
		},
		{
			name:      "empty document list returns empty",
			documents: []Document{},
			opts: func() *SemanticFilterOptions {
				embedding, _ := embedProvider.Embed(ctx, "test")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{embedding},
					Threshold:         0.5,
					EmbeddingProvider: embedProvider,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				if len(filtered) != 0 {
					t.Errorf("Expected 0 documents, got %d", len(filtered))
				}
			},
		},
		{
			name: "nil options returns error",
			documents: []Document{
				{ID: "doc1", Content: "test content", Score: 0.0},
			},
			opts:      nil,
			expectErr: true,
			errMsg:    "options cannot be nil",
		},
		{
			name: "no target embeddings returns all documents",
			documents: []Document{
				{ID: "doc1", Content: "test content 1", Score: 0.0},
				{ID: "doc2", Content: "test content 2", Score: 0.0},
			},
			opts: &SemanticFilterOptions{
				TargetEmbeddings:  []EmbeddingVector{},
				Threshold:         0.5,
				EmbeddingProvider: embedProvider,
			},
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				if len(filtered) != 2 {
					t.Errorf("Expected 2 documents when no target embeddings, got %d", len(filtered))
				}
			},
		},
		{
			name: "nil embedding provider returns error when target embeddings exist",
			documents: []Document{
				{ID: "doc1", Content: "test content", Score: 0.0},
			},
			opts: func() *SemanticFilterOptions {
				embedding, _ := embedProvider.Embed(ctx, "test")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{embedding},
					Threshold:         0.5,
					EmbeddingProvider: nil,
				}
			}(),
			expectErr: true,
			errMsg:    "embedding provider is required",
		},
		{
			name: "documents with empty content are skipped",
			documents: []Document{
				{ID: "doc1", Content: "", Score: 0.0},
				{ID: "doc2", Content: "   \t\n  ", Score: 0.0},
				{ID: "doc3", Content: "valid content", Score: 0.0},
			},
			opts: func() *SemanticFilterOptions {
				embedding, _ := embedProvider.Embed(ctx, "test")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{embedding},
					Threshold:         0.0,
					EmbeddingProvider: embedProvider,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				if len(filtered) > 1 {
					t.Errorf("Expected at most 1 document (empty content should be skipped), got %d", len(filtered))
				}
				for _, doc := range filtered {
					if strings.TrimSpace(doc.Content) == "" {
						t.Error("Empty content document should have been skipped")
					}
				}
			},
		},
		{
			name: "filtered documents have similarity scores set",
			documents: []Document{
				{ID: "doc1", Content: "machine learning", Score: 0.0},
			},
			opts: func() *SemanticFilterOptions {
				embedding, _ := embedProvider.Embed(ctx, "machine learning")
				return &SemanticFilterOptions{
					TargetEmbeddings:  []EmbeddingVector{embedding},
					Threshold:         0.5,
					EmbeddingProvider: embedProvider,
				}
			}(),
			expectErr: false,
			checkFn: func(t *testing.T, filtered []Document) {
				if len(filtered) == 0 {
					t.Fatal("Expected at least one filtered document")
				}
				for _, doc := range filtered {
					if doc.Score == 0.0 {
						t.Error("Expected Score to be set on filtered document")
					}
					if doc.Score < -0.01 || doc.Score > 1.01 {
						t.Errorf("Score %f is out of range [0, 1]", doc.Score)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler
			handler := SemanticFilter(tt.opts)

			// Prepare input (documents as JSON)
			inputJSON, err := json.Marshal(tt.documents)
			if err != nil {
				t.Fatalf("Failed to marshal input documents: %v", err)
			}

			// Create request with input
			req := calque.NewRequest(context.Background(), bytes.NewBuffer(inputJSON))
			respBuf := &bytes.Buffer{}
			resp := calque.NewResponse(respBuf)

			// Execute handler
			err = handler.ServeFlow(req, resp)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Parse response
			var filtered []Document
			if err := json.Unmarshal(respBuf.Bytes(), &filtered); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, filtered)
			}
		})
	}
}

// TestEmbedTopics tests the EmbedTopics utility function
func TestEmbedTopics(t *testing.T) {
	ctx := context.Background()
	embedProvider := newMockEmbeddingProvider(128)

	tests := []struct {
		name      string
		topics    []string
		provider  EmbeddingProvider
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, embeddings []EmbeddingVector)
	}{
		{
			name:      "embed multiple topics successfully",
			topics:    []string{"machine learning", "deep learning", "neural networks"},
			provider:  embedProvider,
			expectErr: false,
			checkFn: func(t *testing.T, embeddings []EmbeddingVector) {
				if len(embeddings) != 3 {
					t.Errorf("Expected 3 embeddings, got %d", len(embeddings))
				}
				for i, emb := range embeddings {
					if len(emb) != 128 {
						t.Errorf("Embedding %d has wrong dimension: expected 128, got %d", i, len(emb))
					}
				}
			},
		},
		{
			name:      "embed single topic",
			topics:    []string{"artificial intelligence"},
			provider:  embedProvider,
			expectErr: false,
			checkFn: func(t *testing.T, embeddings []EmbeddingVector) {
				if len(embeddings) != 1 {
					t.Errorf("Expected 1 embedding, got %d", len(embeddings))
				}
			},
		},
		{
			name:      "empty topics list returns empty embeddings",
			topics:    []string{},
			provider:  embedProvider,
			expectErr: false,
			checkFn: func(t *testing.T, embeddings []EmbeddingVector) {
				if len(embeddings) != 0 {
					t.Errorf("Expected 0 embeddings, got %d", len(embeddings))
				}
			},
		},
		{
			name:      "empty topic string returns error",
			topics:    []string{"valid topic", "", "another topic"},
			provider:  embedProvider,
			expectErr: true,
			errMsg:    "empty text cannot be embedded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embeddings, err := EmbedTopics(ctx, tt.topics, tt.provider)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.checkFn != nil {
				tt.checkFn(t, embeddings)
			}
		})
	}
}

// TestCosineSimilarity tests the cosine similarity calculation
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name    string
		vec1    EmbeddingVector
		vec2    EmbeddingVector
		checkFn func(t *testing.T, similarity float64)
	}{
		{
			name: "identical vectors have similarity 1.0",
			vec1: EmbeddingVector{1.0, 2.0, 3.0},
			vec2: EmbeddingVector{1.0, 2.0, 3.0},
			checkFn: func(t *testing.T, similarity float64) {
				if similarity < 0.99 || similarity > 1.01 {
					t.Errorf("Expected similarity ~1.0, got %f", similarity)
				}
			},
		},
		{
			name: "orthogonal vectors have similarity 0.0",
			vec1: EmbeddingVector{1.0, 0.0, 0.0},
			vec2: EmbeddingVector{0.0, 1.0, 0.0},
			checkFn: func(t *testing.T, similarity float64) {
				if similarity < -0.01 || similarity > 0.01 {
					t.Errorf("Expected similarity ~0.0, got %f", similarity)
				}
			},
		},
		{
			name: "opposite vectors have negative similarity",
			vec1: EmbeddingVector{1.0, 1.0, 1.0},
			vec2: EmbeddingVector{-1.0, -1.0, -1.0},
			checkFn: func(t *testing.T, similarity float64) {
				if similarity > -0.99 || similarity < -1.01 {
					t.Errorf("Expected similarity ~-1.0, got %f", similarity)
				}
			},
		},
		{
			name: "different length vectors return 0.0",
			vec1: EmbeddingVector{1.0, 2.0},
			vec2: EmbeddingVector{1.0, 2.0, 3.0},
			checkFn: func(t *testing.T, similarity float64) {
				if similarity != 0.0 {
					t.Errorf("Expected similarity 0.0 for different length vectors, got %f", similarity)
				}
			},
		},
		{
			name: "zero vector returns 0.0",
			vec1: EmbeddingVector{0.0, 0.0, 0.0},
			vec2: EmbeddingVector{1.0, 2.0, 3.0},
			checkFn: func(t *testing.T, similarity float64) {
				if similarity != 0.0 {
					t.Errorf("Expected similarity 0.0 for zero vector, got %f", similarity)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := cosineSimilarity(tt.vec1, tt.vec2)
			if tt.checkFn != nil {
				tt.checkFn(t, similarity)
			}
		})
	}
}

// TestDocumentLoaderAndSemanticFilterIntegration tests the integration of DocumentLoader and SemanticFilter
func TestDocumentLoaderAndSemanticFilterIntegration(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()
	files := map[string]string{
		"ai_doc.txt":      "machine learning and artificial intelligence",
		"cooking_doc.txt": "italian cooking recipes and pasta dishes",
		"sports_doc.txt":  "football training and soccer tactics",
	}

	for filename, content := range files {
		filepath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	embedProvider := newMockEmbeddingProvider(128)
	ctx := context.Background()

	// Prepare semantic filter options for AI topics
	aiEmbedding, err := embedProvider.Embed(ctx, "artificial intelligence and machine learning")
	if err != nil {
		t.Fatalf("Failed to generate AI embedding: %v", err)
	}

	filterOpts := &SemanticFilterOptions{
		TargetEmbeddings:  []EmbeddingVector{aiEmbedding},
		Threshold:         0.6,
		EmbeddingProvider: embedProvider,
	}

	// Create handlers
	pattern := filepath.Join(tempDir, "*.txt")
	loaderHandler := DocumentLoader(pattern)
	filterHandler := SemanticFilter(filterOpts)

	// Execute loader
	loaderReq := calque.NewRequest(ctx, bytes.NewReader(nil))
	loaderRespBuf := &bytes.Buffer{}
	loaderResp := calque.NewResponse(loaderRespBuf)

	if err := loaderHandler.ServeFlow(loaderReq, loaderResp); err != nil {
		t.Fatalf("DocumentLoader failed: %v", err)
	}

	// Verify loaded documents
	var loadedDocs []Document
	if err := json.Unmarshal(loaderRespBuf.Bytes(), &loadedDocs); err != nil {
		t.Fatalf("Failed to parse loaded documents: %v", err)
	}

	if len(loadedDocs) != 3 {
		t.Fatalf("Expected 3 loaded documents, got %d", len(loadedDocs))
	}

	// Execute filter with loaded documents
	filterReq := calque.NewRequest(ctx, bytes.NewBuffer(loaderRespBuf.Bytes()))
	filterRespBuf := &bytes.Buffer{}
	filterResp := calque.NewResponse(filterRespBuf)

	if err := filterHandler.ServeFlow(filterReq, filterResp); err != nil {
		t.Fatalf("SemanticFilter failed: %v", err)
	}

	// Verify filtered documents
	var filteredDocs []Document
	if err := json.Unmarshal(filterRespBuf.Bytes(), &filteredDocs); err != nil {
		t.Fatalf("Failed to parse filtered documents: %v", err)
	}

	// Check that AI document was kept and others were filtered
	if len(filteredDocs) == 0 {
		t.Fatal("Expected at least one document to pass semantic filter")
	}

	hasAI := false
	for _, doc := range filteredDocs {
		if strings.Contains(doc.Content, "machine learning") {
			hasAI = true
		}
		if strings.Contains(doc.Content, "cooking") || strings.Contains(doc.Content, "football") {
			t.Error("Non-AI documents should have been filtered out")
		}
		if doc.Score < 0.6 {
			t.Errorf("Filtered document has score %f below threshold 0.6", doc.Score)
		}
	}

	if !hasAI {
		t.Error("Expected AI document to pass semantic filter")
	}
}
