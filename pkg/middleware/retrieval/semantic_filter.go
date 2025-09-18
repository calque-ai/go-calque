package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// SemanticFilterOptions configures semantic filtering behavior.
type SemanticFilterOptions struct {
	TargetEmbeddings  []EmbeddingVector `json:"target_embeddings"` // Target concept embeddings
	Threshold         float64           `json:"threshold"`         // Similarity threshold (0-1)
	EmbeddingProvider EmbeddingProvider `json:"-"`                 // Provider for generating embeddings
}

// SemanticFilter creates a semantic similarity filtering middleware.
//
// Input: []Document JSON array from document loader
// Output: []Document JSON array with filtered documents
// Behavior: BUFFERED - reads entire document array for filtering
//
// Filters documents based on semantic similarity to target concept embeddings.
// Useful for filtering irrelevant documents from large datasets and
// topic-based content filtering.
//
// Example:
//
//	aiTopics := []string{"machine learning", "artificial intelligence", "neural networks"}
//	aiEmbeddings := retrieval.EmbedTopics(aiTopics) // Convert topics to embeddings
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

// EmbedTopics generates embeddings for topic strings using the provided embedding provider.
//
// This is a utility function for creating target embeddings from human-readable topic
// descriptions, commonly used with SemanticFilter for topic-based document filtering.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - topics: Slice of topic strings to embed (e.g., "machine learning", "AI research")
//   - provider: EmbeddingProvider implementation to generate embeddings
//
// Returns embeddings in the same order as input topics.
//
// Example:
//
//	provider := openai.NewEmbeddingProvider(apiKey)
//	topics := []string{"machine learning", "artificial intelligence", "neural networks"}
//	embeddings, err := retrieval.EmbedTopics(ctx, topics, provider)
//	if err != nil {
//	    return err
//	}
//
//	opts := &retrieval.SemanticFilterOptions{
//	    TargetEmbeddings: embeddings,
//	    Threshold: 0.7,
//	    EmbeddingProvider: provider,
//	}
func EmbedTopics(ctx context.Context, topics []string, provider EmbeddingProvider) ([]EmbeddingVector, error) {
	embeddings := make([]EmbeddingVector, 0, len(topics))

	for _, topic := range topics {
		embedding, err := provider.Embed(ctx, topic)
		if err != nil {
			return nil, fmt.Errorf("failed to embed topic '%s': %w", topic, err)
		}
		embeddings = append(embeddings, embedding)
	}

	return embeddings, nil
}

// filterDocuments applies semantic filtering to the document array
func filterDocuments(ctx context.Context, documents []Document, opts *SemanticFilterOptions) ([]Document, error) {
	// validate inputs
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}
	if len(documents) == 0 {
		return documents, nil
	}
	if len(opts.TargetEmbeddings) == 0 {
		// No target embeddings, return all documents
		return documents, nil
	}
	if opts.EmbeddingProvider == nil {
		return nil, fmt.Errorf("embedding provider is required when target embeddings are specified")
	}

	// pre-allocate with estimated capacity
	filtered := make([]Document, 0, len(documents)/2)

	for i, doc := range documents {
		// check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// validate document content
		if strings.TrimSpace(doc.Content) == "" {
			continue // Skip empty documents
		}

		// Get embedding for document content
		docEmbedding, err := opts.EmbeddingProvider.Embed(ctx, doc.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for document %d: %w", i, err)
		}

		// validate embedding
		if len(docEmbedding) == 0 {
			continue // Skip documents with invalid embeddings
		}

		// Calculate maximum similarity to any target embedding
		maxSimilarity := findMaxSimilarity(docEmbedding, opts.TargetEmbeddings)

		// Include document if it meets the threshold
		if maxSimilarity >= opts.Threshold {
			// Annotate document with similarity score
			doc.Score = maxSimilarity
			filtered = append(filtered, doc)
		}
	}

	return filtered, nil
}

// findMaxSimilarity finds the highest cosine similarity between a document embedding and any target embedding.
func findMaxSimilarity(docEmbedding EmbeddingVector, targetEmbeddings []EmbeddingVector) float64 {
	maxSimilarity := 0.0
	for _, targetEmbedding := range targetEmbeddings {
		// skip invalid target embeddings
		if len(targetEmbedding) == 0 {
			continue
		}

		similarity := cosineSimilarity(docEmbedding, targetEmbedding)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
		}
	}
	return maxSimilarity
}

// cosineSimilarity calculates cosine similarity between two embedding vectors
func cosineSimilarity(a, b EmbeddingVector) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64

	for i := range a {
		af, bf := float64(a[i]), float64(b[i])
		dotProduct += af * bf
		normA += af * af
		normB += bf * bf
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
