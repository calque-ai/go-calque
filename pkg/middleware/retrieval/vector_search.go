package retrieval

import (
	"encoding/json"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// VectorSearch creates a vector similarity search middleware.
//
// Input: string query text
// Output: SearchResult with ranked documents 
// Behavior: BUFFERED - reads entire input to perform search
//
// Performs similarity search against a vector database to find relevant documents.
// Takes input text, converts it to embeddings, searches vector store for similar
// embeddings above the specified threshold, and returns matching documents
// ranked by similarity score.
//
// Example:
//
//	vectorStore := weaviate.New("http://localhost:8080")
//	opts := &retrieval.SearchOptions{Threshold: 0.8}
//	flow := calque.NewFlow().
//	    Use(retrieval.VectorSearch(vectorStore, opts))
func VectorSearch(store VectorStore, opts *SearchOptions) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var queryText string
		err := calque.Read(r, &queryText)
		if err != nil {
			return err
		}

		// Create search query with options
		query := SearchQuery{
			Text:      queryText,
			Threshold: opts.Threshold,
			Limit:     opts.Limit,
			Filter:    opts.Filter,
		}

		// Use custom embedding if provided
		if opts.EmbeddingProvider != nil {
			embedding, err := opts.EmbeddingProvider.Embed(r.Context, queryText)
			if err != nil {
				return err
			}
			query.Vector = embedding
		}

		// Perform vector search
		result, err := store.Search(r.Context, query)
		if err != nil {
			return err
		}

		// Write result as JSON
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return err
		}

		return calque.Write(w, resultJSON)
	})
}