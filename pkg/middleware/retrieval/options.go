package retrieval

import "context"

// SearchOptions configures vector search behavior and optional context building.
type SearchOptions struct {
	Threshold         float64            `json:"threshold"`          // Similarity threshold (0-1)
	Limit            int                `json:"limit,omitempty"`    // Maximum results to return
	Filter           map[string]any     `json:"filter,omitempty"`   // Metadata filters
	EmbeddingProvider EmbeddingProvider  `json:"-"`                 // Custom embedding provider
	
	// Context building options (optional)
	Strategy          *ContextStrategy   `json:"strategy,omitempty"` // If set, returns formatted context instead of JSON
	MaxTokens         int                `json:"max_tokens,omitempty"` // Token limit for context
	Separator         string             `json:"separator,omitempty"`  // Document separator in context
}

// EmbeddingProvider interface for generating embeddings.
type EmbeddingProvider interface {
	// Embed generates embeddings for text content
	Embed(ctx context.Context, text string) (EmbeddingVector, error)
}