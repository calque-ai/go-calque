// Package mock provides a simple in-memory vector store for demonstration purposes.
package mock

import (
	"context"
	"strings"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
)

// VectorStore is a simple in-memory vector store for demonstration
type VectorStore struct {
	documents []retrieval.Document
}

// New creates a new mock vector store
func New() *VectorStore {
	return &VectorStore{
		documents: make([]retrieval.Document, 0),
	}
}

func (m *VectorStore) Search(ctx context.Context, query retrieval.SearchQuery) (*retrieval.SearchResult, error) {
	var results []retrieval.Document

	// Simple keyword-based similarity (in production, use actual embeddings)
	queryWords := strings.Fields(strings.ToLower(query.Text))

	for _, doc := range m.documents {
		score := calculateSimpleSimilarity(queryWords, doc.Content)
		if score >= query.Threshold {
			docCopy := doc
			docCopy.Score = score
			results = append(results, docCopy)
		}
	}

	// Sort by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Apply limit
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return &retrieval.SearchResult{
		Documents: results,
		Query:     query.Text,
		Total:     len(results),
		Threshold: query.Threshold,
	}, nil
}

func (m *VectorStore) Store(ctx context.Context, documents []retrieval.Document) error {
	m.documents = append(m.documents, documents...)
	return nil
}

func (m *VectorStore) Delete(ctx context.Context, ids []string) error {
	return nil
}

func (m *VectorStore) Health(ctx context.Context) error {
	return nil
}

func (m *VectorStore) Close() error {
	return nil
}

// calculateSimpleSimilarity computes basic keyword overlap similarity
func calculateSimpleSimilarity(queryWords []string, content string) float64 {
	contentLower := strings.ToLower(content)
	matches := 0

	for _, word := range queryWords {
		if len(word) > 2 && strings.Contains(contentLower, word) {
			matches++
		}
	}

	if len(queryWords) == 0 {
		return 0
	}

	return float64(matches) / float64(len(queryWords))
}
