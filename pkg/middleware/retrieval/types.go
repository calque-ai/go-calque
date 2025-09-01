package retrieval

import "time"

// Document represents a document with metadata for retrieval operations.
//
// Contains the document content, metadata for filtering and ranking,
// and similarity scores from vector search operations.
type Document struct {
	ID       string            `json:"id"`                 // Unique document identifier
	Content  string            `json:"content"`            // Document text content
	Metadata map[string]any    `json:"metadata,omitempty"` // Additional document metadata
	Score    float64           `json:"score,omitempty"`    // Similarity score (0-1, higher is more similar)
	Created  time.Time         `json:"created,omitempty"`  // Document creation timestamp
	Updated  time.Time         `json:"updated,omitempty"`  // Last update timestamp
}

// SearchResult represents the result of a vector search operation.
//
// Contains the matching documents ranked by similarity score
// along with search metadata.
type SearchResult struct {
	Documents []Document `json:"documents"`          // Matching documents ranked by score
	Query     string     `json:"query"`              // Original search query
	Total     int        `json:"total"`              // Total number of matches found
	Threshold float64    `json:"threshold"`          // Similarity threshold used
}

// ContextStrategy defines how retrieved documents should be assembled into context.
type ContextStrategy string

const (
	// StrategyRelevant ranks documents by similarity score (default)
	StrategyRelevant ContextStrategy = "relevant"
	// StrategyRecent prioritizes newest documents by timestamp
	StrategyRecent ContextStrategy = "recent"
	// StrategyDiverse ensures topic diversity across selected documents
	StrategyDiverse ContextStrategy = "diverse"
	// StrategySummary summarizes long documents to fit more context
	StrategySummary ContextStrategy = "summary"
)

// ContextConfig configures context building behavior.
type ContextConfig struct {
	MaxTokens int             `json:"max_tokens"`        // Maximum tokens in final context
	Strategy  ContextStrategy `json:"strategy"`          // Assembly strategy
	Separator string          `json:"separator"`         // Document separator in context
}

// EmbeddingVector represents a vector embedding.
type EmbeddingVector []float32

// SearchQuery represents a vector search query.
type SearchQuery struct {
	Text      string                 `json:"text"`                // Query text
	Vector    EmbeddingVector        `json:"vector,omitempty"`    // Pre-computed query vector
	Threshold float64                `json:"threshold"`           // Similarity threshold (0-1)
	Limit     int                    `json:"limit,omitempty"`     // Maximum results to return
	Filter    map[string]any         `json:"filter,omitempty"`    // Metadata filters
}