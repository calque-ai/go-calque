package retrieval

import (
	"context"
	"encoding/json"
	"math"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// SemanticFilterOptions configures semantic filtering behavior.
type SemanticFilterOptions struct {
	TargetEmbeddings []EmbeddingVector `json:"target_embeddings"`  // Target concept embeddings
	Threshold        float64           `json:"threshold"`          // Similarity threshold (0-1)
	EmbeddingProvider EmbeddingProvider `json:"-"`                 // Provider for generating embeddings
}

// SemanticFilter creates a semantic similarity filtering middleware.
//
// Input: []Document JSON array from document loader
// Output: []Document JSON array with filtered documents
// Behavior: BUFFERED - reads entire document array for filtering
//
// Filters streaming text based on semantic similarity to target concepts.
// Useful for filtering irrelevant documents from large datasets,
// topic-based content filtering, and semantic deduplication.
//
// Example:
//
//	aiTopics := []string{"machine learning", "artificial intelligence", "neural networks"}
//	aiEmbeddings := embedTopics(aiTopics) // Convert topics to embeddings
//	opts := &retrieval.SemanticFilterOptions{
//	    TargetEmbeddings: aiEmbeddings,
//	    Threshold: 0.6, // 60% similarity to AI topics
//	}
//	flow := calque.NewFlow().
//	    Use(retrieval.DocumentLoader("./docs/*.md")).
//	    Use(retrieval.SemanticFilter(opts))
func SemanticFilter(opts *SemanticFilterOptions) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input []byte
		err := calque.Read(r, &input)
		if err != nil {
			return err
		}

		// Parse documents array
		var documents []Document
		if err := json.Unmarshal(input, &documents); err != nil {
			return err
		}

		// Filter documents based on semantic similarity
		filtered, err := filterDocuments(r.Context, documents, opts)
		if err != nil {
			return err
		}

		// Write filtered documents as JSON
		result, err := json.Marshal(filtered)
		if err != nil {
			return err
		}

		return calque.Write(w, result)
	})
}

// filterDocuments applies semantic filtering to the document array
func filterDocuments(ctx context.Context, documents []Document, opts *SemanticFilterOptions) ([]Document, error) {
	if len(opts.TargetEmbeddings) == 0 {
		// No target embeddings, return all documents
		return documents, nil
	}

	var filtered []Document

	for _, doc := range documents {
		// Get embedding for document content
		var docEmbedding EmbeddingVector
		var err error

		if opts.EmbeddingProvider != nil {
			docEmbedding, err = opts.EmbeddingProvider.Embed(ctx, doc.Content)
			if err != nil {
				return nil, err
			}
		} else {
			// Skip documents if no embedding provider available
			continue
		}

		// Calculate maximum similarity to any target embedding
		maxSimilarity := 0.0
		for _, targetEmbedding := range opts.TargetEmbeddings {
			similarity := cosineSimilarity(docEmbedding, targetEmbedding)
			if similarity > maxSimilarity {
				maxSimilarity = similarity
			}
		}

		// Include document if it meets the threshold
		if maxSimilarity >= opts.Threshold {
			doc.Score = maxSimilarity // Store similarity score
			filtered = append(filtered, doc)
		}
	}

	return filtered, nil
}

// cosineSimilarity calculates cosine similarity between two embedding vectors
func cosineSimilarity(a, b EmbeddingVector) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}