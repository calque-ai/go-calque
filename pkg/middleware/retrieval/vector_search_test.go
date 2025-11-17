package retrieval

import (
	"testing"
	"time"
)

func TestSelectDiverse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		docs    []Document
		opts    *SearchOptions
		checkFn func(t *testing.T, result []Document)
	}{
		{
			name: "empty document list returns empty",
			docs: []Document{},
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 0 {
					t.Errorf("Expected empty result, got %d documents", len(result))
				}
			},
		},
		{
			name: "single document returns same document",
			docs: []Document{
				{Content: "machine learning", Score: 0.9},
			},
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 1 {
					t.Errorf("Expected 1 document, got %d", len(result))
				}
				if result[0].Content != "machine learning" {
					t.Errorf("Expected content 'machine learning', got %q", result[0].Content)
				}
			},
		},
		{
			name: "MMR selects diverse documents",
			docs: []Document{
				{Content: "machine learning basics", Score: 0.9},
				{Content: "machine learning tutorial", Score: 0.85}, // Very similar to first
				{Content: "deep learning fundamentals", Score: 0.8},
				{Content: "neural networks explained", Score: 0.75},
			},
			opts: &SearchOptions{
				DiversityLambda:   ptr(0.5),
				MaxDiverseResults: ptr(3),
			},
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 3 {
					t.Errorf("Expected 3 documents, got %d", len(result))
				}
				// First should be highest score
				if result[0].Content != "machine learning basics" {
					t.Errorf("Expected first doc to be 'machine learning basics', got %q", result[0].Content)
				}
				// Should skip very similar "machine learning tutorial"
				// and include more diverse documents
				containsSimilar := false
				for _, doc := range result {
					if doc.Content == "machine learning tutorial" {
						containsSimilar = true
						break
					}
				}
				if containsSimilar {
					t.Error("Expected to skip similar 'machine learning tutorial' in favor of diverse results")
				}
			},
		},
		{
			name: "high lambda (0.9) favors relevance over diversity",
			docs: []Document{
				{Content: "cat", Score: 0.95},
				{Content: "cat animal", Score: 0.90}, // Similar but high score
				{Content: "dog", Score: 0.50},        // Diverse but low score
			},
			opts: &SearchOptions{
				DiversityLambda:   ptr(0.9), // High lambda = favor relevance
				MaxDiverseResults: ptr(2),
			},
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 2 {
					t.Errorf("Expected 2 documents, got %d", len(result))
				}
				// With high lambda, should prefer high-scoring similar docs
				if result[0].Score < 0.9 {
					t.Errorf("Expected first doc to have high score, got %f", result[0].Score)
				}
			},
		},
		{
			name: "low lambda (0.1) favors diversity over relevance",
			docs: []Document{
				{Content: "apple fruit red", Score: 0.95},
				{Content: "apple fruit green", Score: 0.90}, // Similar
				{Content: "banana yellow", Score: 0.60},     // Diverse
			},
			opts: &SearchOptions{
				DiversityLambda:   ptr(0.1), // Low lambda = favor diversity
				MaxDiverseResults: ptr(2),
			},
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 2 {
					t.Errorf("Expected 2 documents, got %d", len(result))
				}
				// First should still be highest score
				if result[0].Content != "apple fruit red" {
					t.Errorf("Expected first to be 'apple fruit red', got %q", result[0].Content)
				}
			},
		},
		{
			name: "MaxDiverseResults limits output",
			docs: []Document{
				{Content: "doc1", Score: 0.9},
				{Content: "doc2", Score: 0.8},
				{Content: "doc3", Score: 0.7},
				{Content: "doc4", Score: 0.6},
				{Content: "doc5", Score: 0.5},
			},
			opts: &SearchOptions{
				MaxDiverseResults: ptr(2),
			},
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 2 {
					t.Errorf("Expected 2 documents due to MaxDiverseResults, got %d", len(result))
				}
			},
		},
		{
			name: "default options with multiple docs",
			docs: []Document{
				{Content: "first document about AI", Score: 0.95},
				{Content: "second document about ML", Score: 0.90},
				{Content: "third document about DL", Score: 0.85},
			},
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result []Document) {
				// Should use default lambda and max results
				if len(result) == 0 {
					t.Error("Expected non-empty result")
				}
				// First should be highest score
				if result[0].Score != 0.95 {
					t.Errorf("Expected first score 0.95, got %f", result[0].Score)
				}
			},
		},
		{
			name: "identical documents with different scores",
			docs: []Document{
				{Content: "same content", Score: 0.9},
				{Content: "same content", Score: 0.8},
				{Content: "different content", Score: 0.7},
			},
			opts: &SearchOptions{
				MaxDiverseResults: ptr(2),
			},
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 2 {
					t.Errorf("Expected 2 documents, got %d", len(result))
				}
				// Should pick first identical doc and the different one
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := selectDiverse(tt.docs, tt.opts)

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestContentSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		a         string
		b         string
		algorithm SimilarityAlgorithm
		minScore  float64 // Minimum expected similarity
		maxScore  float64 // Maximum expected similarity
	}{
		{
			name:      "identical strings - cosine",
			a:         "machine learning",
			b:         "machine learning",
			algorithm: CosineSimilarity,
			minScore:  0.99,
			maxScore:  1.0,
		},
		{
			name:      "identical strings - jaccard",
			a:         "machine learning",
			b:         "machine learning",
			algorithm: JaccardSimilarity,
			minScore:  0.99,
			maxScore:  1.0,
		},
		{
			name:      "completely different strings - cosine",
			a:         "apple",
			b:         "xyz",
			algorithm: CosineSimilarity,
			minScore:  0.0,
			maxScore:  0.1,
		},
		{
			name:      "similar strings - cosine",
			a:         "machine learning tutorial",
			b:         "machine learning guide",
			algorithm: CosineSimilarity,
			minScore:  0.5,
			maxScore:  1.0,
		},
		{
			name:      "similar strings - jaccard",
			a:         "the quick brown fox",
			b:         "the fast brown fox",
			algorithm: JaccardSimilarity,
			minScore:  0.5,
			maxScore:  1.0,
		},
		{
			name:      "empty strings - cosine",
			a:         "",
			b:         "",
			algorithm: CosineSimilarity,
			minScore:  0.0,
			maxScore:  1.0,
		},
		{
			name:      "one empty string - cosine",
			a:         "content",
			b:         "",
			algorithm: CosineSimilarity,
			minScore:  0.0,
			maxScore:  0.0,
		},
		{
			name:      "jaro-winkler for short strings",
			a:         "abc",
			b:         "abd",
			algorithm: JaroWinklerSimilarity,
			minScore:  0.0,
			maxScore:  1.0,
		},
		{
			name:      "sorensen-dice similarity",
			a:         "night",
			b:         "nacht",
			algorithm: SorensenDiceSimilarity,
			minScore:  0.0,
			maxScore:  1.0,
		},
		{
			name:      "hybrid similarity combines approaches",
			a:         "machine learning is great",
			b:         "deep learning is amazing",
			algorithm: HybridSimilarity,
			minScore:  0.0,
			maxScore:  1.0,
		},
		{
			name:      "default algorithm fallback",
			a:         "test content",
			b:         "test data",
			algorithm: SimilarityAlgorithm("unknown"),
			minScore:  0.0,
			maxScore:  1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := contentSimilarity(tt.a, tt.b, tt.algorithm)

			if result < tt.minScore || result > tt.maxScore {
				t.Errorf("Expected similarity between %f and %f, got %f",
					tt.minScore, tt.maxScore, result)
			}

			// Verify result is a valid probability
			if result < 0.0 || result > 1.0 {
				t.Errorf("Similarity score should be between 0 and 1, got %f", result)
			}
		})
	}
}

func TestHybridSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
	}{
		{
			name: "identical strings",
			a:    "machine learning",
			b:    "machine learning",
		},
		{
			name: "similar strings",
			a:    "artificial intelligence",
			b:    "artificial intelligence systems",
		},
		{
			name: "different strings",
			a:    "cat",
			b:    "dog",
		},
		{
			name: "empty strings",
			a:    "",
			b:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := hybridSimilarity(tt.a, tt.b)

			// Verify result is valid
			if result < 0.0 || result > 1.0 {
				t.Errorf("Hybrid similarity should be between 0 and 1, got %f", result)
			}

			// Hybrid should combine jaccard and cosine
			jaccard := contentSimilarity(tt.a, tt.b, JaccardSimilarity)
			cosine := contentSimilarity(tt.a, tt.b, CosineSimilarity)

			// Result should be weighted combination (0.7*cosine + 0.3*jaccard)
			expected := 0.7*cosine + 0.3*jaccard
			tolerance := 0.01
			if result < expected-tolerance || result > expected+tolerance {
				t.Errorf("Expected hybrid ~%f (0.7*%f + 0.3*%f), got %f",
					expected, cosine, jaccard, result)
			}
		})
	}
}

func TestCalculateAverageLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		docs     []Document
		expected float64
	}{
		{
			name:     "empty document list",
			docs:     []Document{},
			expected: 0,
		},
		{
			name: "single document",
			docs: []Document{
				{Content: "hello"},
			},
			expected: 5,
		},
		{
			name: "multiple documents same length",
			docs: []Document{
				{Content: "abcd"},
				{Content: "efgh"},
				{Content: "ijkl"},
			},
			expected: 4,
		},
		{
			name: "multiple documents different lengths",
			docs: []Document{
				{Content: "a"},
				{Content: "abc"},
				{Content: "abcde"},
			},
			expected: 3, // (1+3+5)/3
		},
		{
			name: "documents with empty content",
			docs: []Document{
				{Content: ""},
				{Content: "test"},
			},
			expected: 2, // (0+4)/2
		},
		{
			name: "long documents",
			docs: []Document{
				{Content: string(make([]byte, 1000))},
				{Content: string(make([]byte, 2000))},
			},
			expected: 1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := calculateAverageLength(tt.docs)

			if result != tt.expected {
				t.Errorf("Expected average length %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestSelectOptimalSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		docs     []Document
		expected SimilarityAlgorithm
	}{
		{
			name:     "empty document list defaults to cosine",
			docs:     []Document{},
			expected: CosineSimilarity,
		},
		{
			name: "short documents use jaccard",
			docs: []Document{
				{Content: "short"},
				{Content: "text"},
			},
			expected: JaccardSimilarity,
		},
		{
			name: "very short documents use jaccard",
			docs: []Document{
				{Content: "cat dog bird"},
				{Content: "cat fish"},
			},
			expected: JaccardSimilarity,
		},
		{
			name: "exactly 100 chars defaults to cosine",
			docs: []Document{
				{Content: string(make([]byte, 100))},
				{Content: string(make([]byte, 100))},
			},
			expected: CosineSimilarity,
		},
		{
			name: "medium length documents use cosine",
			docs: []Document{
				{Content: "this is a medium length document with enough content to trigger cosine similarity algorithm for testing purposes"},
				{Content: "another medium sized passage with different words but similar length for testing purposes here today"},
			},
			expected: CosineSimilarity,
		},
		{
			name: "medium length with high vocabulary overlap uses cosine",
			docs: []Document{
				{Content: "the quick brown fox jumps over the lazy dog in the garden with the quick brown cat and the lazy fox today"},
				{Content: "the quick brown dog jumps over the lazy fox in the garden with the quick brown cat and the lazy dog today"},
			},
			expected: CosineSimilarity,
		},
		{
			name: "medium length with low vocabulary overlap uses cosine",
			docs: []Document{
				{Content: "this is a medium length document with some unique words that are different from the other text"},
				{Content: "completely separate vocabulary appears in another medium passage having distinct terms unlike previous content"},
			},
			expected: CosineSimilarity,
		},
		{
			name: "long documents use hybrid",
			docs: []Document{
				{Content: string(make([]byte, 1500))},
				{Content: string(make([]byte, 1500))},
			},
			expected: HybridSimilarity,
		},
		{
			name: "exactly 1000 chars defaults to cosine",
			docs: []Document{
				{Content: string(make([]byte, 1000))},
				{Content: string(make([]byte, 1000))},
			},
			expected: CosineSimilarity,
		},
		{
			name: "just over 1000 chars uses hybrid",
			docs: []Document{
				{Content: string(make([]byte, 1001))},
				{Content: string(make([]byte, 1001))},
			},
			expected: HybridSimilarity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := selectOptimalSimilarity(tt.docs)

			if result != tt.expected {
				t.Errorf("Expected algorithm %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestTruncateDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		docs     []Document
		maxWords int
		checkFn  func(t *testing.T, result []Document)
	}{
		{
			name:     "empty document list",
			docs:     []Document{},
			maxWords: 10,
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 0 {
					t.Errorf("Expected empty result, got %d documents", len(result))
				}
			},
		},
		{
			name: "document shorter than max words unchanged",
			docs: []Document{
				{Content: "short text"},
			},
			maxWords: 10,
			checkFn: func(t *testing.T, result []Document) {
				if result[0].Content != "short text" {
					t.Errorf("Expected 'short text', got %q", result[0].Content)
				}
			},
		},
		{
			name: "document longer than max words is truncated",
			docs: []Document{
				{Content: "one two three four five six seven eight nine ten eleven twelve"},
			},
			maxWords: 5,
			checkFn: func(t *testing.T, result []Document) {
				expected := "one two three four five..."
				if result[0].Content != expected {
					t.Errorf("Expected %q, got %q", expected, result[0].Content)
				}
			},
		},
		{
			name: "multiple documents with mixed lengths",
			docs: []Document{
				{Content: "short"},
				{Content: "this is a very long document that should be truncated"},
			},
			maxWords: 3,
			checkFn: func(t *testing.T, result []Document) {
				if len(result) != 2 {
					t.Errorf("Expected 2 documents, got %d", len(result))
				}
				// First is unchanged
				if result[0].Content != "short" {
					t.Errorf("Expected 'short', got %q", result[0].Content)
				}
				// Second is truncated
				expected := "this is a..."
				if result[1].Content != expected {
					t.Errorf("Expected %q, got %q", expected, result[1].Content)
				}
			},
		},
		{
			name: "zero max words truncates to ellipsis",
			docs: []Document{
				{Content: "some content"},
			},
			maxWords: 0,
			checkFn: func(t *testing.T, result []Document) {
				if result[0].Content != "..." {
					t.Errorf("Expected '...', got %q", result[0].Content)
				}
			},
		},
		{
			name: "preserves document metadata",
			docs: []Document{
				{
					Content:  "one two three four five six",
					ID:       "doc1",
					Score:    0.95,
					Metadata: map[string]any{"key": "value"},
				},
			},
			maxWords: 3,
			checkFn: func(t *testing.T, result []Document) {
				if result[0].ID != "doc1" {
					t.Errorf("Expected ID 'doc1', got %q", result[0].ID)
				}
				if result[0].Score != 0.95 {
					t.Errorf("Expected score 0.95, got %f", result[0].Score)
				}
				if result[0].Metadata["key"] != "value" {
					t.Errorf("Expected metadata preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := truncateDocuments(tt.docs, tt.maxWords)

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		opts    *SearchOptions
		checkFn func(t *testing.T, result int)
	}{
		{
			name: "empty text",
			text: "",
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result int) {
				if result != 0 {
					t.Errorf("Expected 0 tokens for empty text, got %d", result)
				}
			},
		},
		{
			name: "single word",
			text: "hello",
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result int) {
				if result <= 0 {
					t.Errorf("Expected positive token count, got %d", result)
				}
			},
		},
		{
			name: "multiple words with default ratio",
			text: "one two three four five",
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result int) {
				// 5 words * default ratio (1.3) = ~6-7 tokens
				if result < 5 || result > 10 {
					t.Errorf("Expected ~6-7 tokens, got %d", result)
				}
			},
		},
		{
			name: "custom token estimation ratio",
			text: "word1 word2 word3",
			opts: &SearchOptions{
				TokenEstimationRatio: ptr(2.0),
			},
			checkFn: func(t *testing.T, result int) {
				// 3 words * 2.0 = 6 tokens
				if result != 6 {
					t.Errorf("Expected 6 tokens (3 words * 2.0), got %d", result)
				}
			},
		},
		{
			name: "long text",
			text: string(make([]byte, 1000)),
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result int) {
				if result <= 0 {
					t.Error("Expected positive token count for long text")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := estimateTokens(tt.text, tt.opts)

			if result < 0 {
				t.Errorf("Token count should not be negative, got %d", result)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestApplyStrategy(t *testing.T) {
	t.Parallel()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	tests := []struct {
		name    string
		docs    []Document
		opts    *SearchOptions
		checkFn func(t *testing.T, result []Document, err error)
	}{
		{
			name: "no strategy returns documents as-is",
			docs: []Document{
				{Content: "doc1", Score: 0.5},
				{Content: "doc2", Score: 0.9},
			},
			opts: &SearchOptions{},
			checkFn: func(t *testing.T, result []Document, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != 2 {
					t.Errorf("Expected 2 documents, got %d", len(result))
				}
			},
		},
		{
			name: "StrategyRelevant sorts by score descending",
			docs: []Document{
				{Content: "low", Score: 0.3},
				{Content: "high", Score: 0.9},
				{Content: "medium", Score: 0.6},
			},
			opts: &SearchOptions{
				Strategy: ptr(StrategyRelevant),
			},
			checkFn: func(t *testing.T, result []Document, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != 3 {
					t.Errorf("Expected 3 documents, got %d", len(result))
				}
				// Should be sorted by score descending
				if result[0].Content != "high" {
					t.Errorf("Expected first doc 'high', got %q", result[0].Content)
				}
				if result[1].Content != "medium" {
					t.Errorf("Expected second doc 'medium', got %q", result[1].Content)
				}
				if result[2].Content != "low" {
					t.Errorf("Expected third doc 'low', got %q", result[2].Content)
				}
			},
		},
		{
			name: "StrategyRecent sorts by timestamp descending",
			docs: []Document{
				{Content: "oldest", Created: lastWeek},
				{Content: "newest", Created: now},
				{Content: "middle", Created: yesterday},
			},
			opts: &SearchOptions{
				Strategy: ptr(StrategyRecent),
			},
			checkFn: func(t *testing.T, result []Document, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Should be sorted by created timestamp descending
				if result[0].Content != "newest" {
					t.Errorf("Expected first doc 'newest', got %q", result[0].Content)
				}
				if result[1].Content != "middle" {
					t.Errorf("Expected second doc 'middle', got %q", result[1].Content)
				}
				if result[2].Content != "oldest" {
					t.Errorf("Expected third doc 'oldest', got %q", result[2].Content)
				}
			},
		},
		{
			name: "StrategyDiverse applies MMR",
			docs: []Document{
				{Content: "machine learning", Score: 0.9},
				{Content: "deep learning", Score: 0.8},
			},
			opts: &SearchOptions{
				Strategy: ptr(StrategyDiverse),
			},
			checkFn: func(t *testing.T, result []Document, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) == 0 {
					t.Error("Expected non-empty result from diverse strategy")
				}
			},
		},
		{
			name: "StrategySummary truncates documents",
			docs: []Document{
				{Content: "one two three four five six seven eight nine ten"},
			},
			opts: &SearchOptions{
				Strategy: ptr(StrategySummary),
			},
			checkFn: func(t *testing.T, result []Document, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Should truncate long documents
				if len(result) == 0 {
					t.Error("Expected non-empty result")
				}
			},
		},
		{
			name: "unknown strategy returns error",
			docs: []Document{
				{Content: "doc"},
			},
			opts: &SearchOptions{
				Strategy: ptr(ContextStrategy("invalid")),
			},
			checkFn: func(t *testing.T, _ []Document, err error) {
				if err == nil {
					t.Error("Expected error for unknown strategy")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := applyStrategy(tt.docs, tt.opts)

			if tt.checkFn != nil {
				tt.checkFn(t, result, err)
			}
		})
	}
}

// Helper function to create pointer to value
func ptr[T any](v T) *T {
	return &v
}
