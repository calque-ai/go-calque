package retrieval

import (
	"context"
)

// VectorStore interface for vector database operations.
//
// Defines the standard interface that all vector database implementations must satisfy.
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