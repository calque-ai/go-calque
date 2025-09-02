package retrieval

import (
	"context"
)

// VectorStore interface for basic vector database operations.
//
// Defines the core interface that all vector database implementations must satisfy.
// Supports similarity search, document storage, and embedding operations.
//
// Example:
//
//	store := weaviate.New("http://localhost:8080")
//	flow := calque.NewFlow().Use(retrieval.VectorSearch(store, 0.8))
type VectorStore interface {
	// Search performs similarity search against the vector database
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
	
	// Store adds documents to the vector database with embeddings
	Store(ctx context.Context, documents []Document) error
	
	// Delete removes documents from the vector database
	Delete(ctx context.Context, ids []string) error
	
	// GetEmbedding generates embeddings for text content
	GetEmbedding(ctx context.Context, text string) (EmbeddingVector, error)
	
	// Health checks if the vector store is available
	Health(ctx context.Context) error
	
	// Close releases any resources held by the client
	Close() error
}

// DiversificationProvider interface for native diversification capabilities.
//
// Implement this interface if your vector store supports native diversification
// algorithms like MMR (Maximum Marginal Relevance) or clustering-based selection.
//
// Example:
//
//	if diversifier, ok := store.(retrieval.DiversificationProvider); ok {
//	    // Use native diversification
//	    result, err := diversifier.SearchWithDiversification(ctx, query, opts)
//	}
type DiversificationProvider interface {
	// SearchWithDiversification performs search with native diversification
	SearchWithDiversification(ctx context.Context, query SearchQuery, opts DiversificationOptions) (*SearchResult, error)
}

// RerankingProvider interface for native reranking capabilities.
//
// Implement this interface if your vector store supports native reranking
// like cross-encoder models or other relevance scoring methods.
//
// Example:
//
//	if reranker, ok := store.(retrieval.RerankingProvider); ok {
//	    // Use native reranking
//	    result, err := reranker.SearchWithReranking(ctx, query, opts)
//	}
type RerankingProvider interface {
	// SearchWithReranking performs search with native reranking
	SearchWithReranking(ctx context.Context, query SearchQuery, opts RerankingOptions) (*SearchResult, error)
}

// TokenEstimator interface for native token counting capabilities.
//
// Implement this interface if your vector store can provide accurate
// token counts for context building and text processing.
//
// Example:
//
//	if estimator, ok := store.(retrieval.TokenEstimator); ok {
//	    tokens := estimator.EstimateTokens(text)
//	}
type TokenEstimator interface {
	// EstimateTokens returns the estimated token count for the given text
	EstimateTokens(text string) int
	
	// EstimateTokensBatch returns token counts for multiple texts efficiently
	EstimateTokensBatch(texts []string) []int
}

// DiversificationOptions configures native diversification (e.g., MMR in Qdrant)
type DiversificationOptions struct {
	// Diversity controls the relevance vs diversity tradeoff
	// 0.0 = pure relevance, 1.0 = pure diversity
	Diversity float64 `json:"diversity"`
	
	// CandidatesLimit is the number of candidates to consider for diversification
	CandidatesLimit int `json:"candidates_limit,omitempty"`
	
	// Strategy specifies the diversification algorithm (e.g., "mmr", "clustering")
	Strategy string `json:"strategy,omitempty"`
}

// RerankingOptions configures native reranking (e.g., cross-encoder in Weaviate)
type RerankingOptions struct {
	// Model specifies the reranking model to use
	Model string `json:"model,omitempty"`
	
	// Query is the text query for relevance scoring
	Query string `json:"query,omitempty"`
	
	// TopK limits reranking to top K candidates for performance
	TopK int `json:"top_k,omitempty"`
}