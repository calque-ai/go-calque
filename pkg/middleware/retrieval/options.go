package retrieval

import "context"

// SearchOptions configures vector search behavior.
type SearchOptions struct {
	Threshold         float64            `json:"threshold"`          // Similarity threshold (0-1)
	Limit            int                `json:"limit,omitempty"`    // Maximum results to return
	Filter           map[string]any     `json:"filter,omitempty"`   // Metadata filters
	EmbeddingProvider EmbeddingProvider  `json:"-"`                 // Custom embedding provider
}

// EmbeddingProvider interface for generating embeddings.
type EmbeddingProvider interface {
	// Embed generates embeddings for text content
	Embed(ctx context.Context, text string) (EmbeddingVector, error)
}