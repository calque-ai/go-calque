package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/hbollon/go-edlib"
)

// VectorSearch creates a vector similarity search middleware with optional context building.
//
// Input: string query text
// Output: SearchResult JSON or formatted context string (based on options)
// Behavior: BUFFERED - reads entire input to perform search
//
// Performs similarity search against a vector database to find relevant documents.
// When Strategy is specified in SearchOptions, automatically builds formatted context
// using native database capabilities when available. Otherwise returns SearchResult JSON.
//
// Examples:
//
//	// Search only - returns SearchResult JSON
//	opts := &retrieval.SearchOptions{Threshold: 0.8}
//	flow := calque.NewFlow().Use(retrieval.VectorSearch(store, opts))
//
//	// Search + context building - returns formatted string
//	strategy := retrieval.StrategyDiverse
//	opts := &retrieval.SearchOptions{
//	    Threshold: 0.8,
//	    Strategy:  &strategy,
//	    MaxTokens: 4000,
//	}
//	flow := calque.NewFlow().Use(retrieval.VectorSearch(store, opts))
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

		// Perform search using db native capabilities when strategy is specified
		result, isNative, err := strategySearch(r.Context, store, query, opts)
		if err != nil {
			return err
		}

		// If no strategy specified, return SearchResult JSON
		if opts.Strategy == nil {
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return err
			}
			return calque.Write(w, resultJSON)
		}

		// Strategy specified - build formatted context
		context, err := buildContext(result.Documents, opts, store, isNative)
		if err != nil {
			return err
		}

		return calque.Write(w, context)
	})
}

// strategySearch executes search using native capabilities when available
// Returns: (*SearchResult, nativeProcessed bool, error)
func strategySearch(ctx context.Context, store VectorStore, query SearchQuery, opts *SearchOptions) (*SearchResult, bool, error) {
	// If no strategy specified, use regular search
	if opts.Strategy == nil {
		result, err := store.Search(ctx, query)
		return result, false, err
	}

	// Check if we can use native diversification during search
	if *opts.Strategy == StrategyDiverse {
		if diversifier, ok := store.(DiversificationProvider); ok {
			diversityOpts := DiversificationOptions{
				Diversity:       0.5,             // Balance relevance and diversity
				CandidatesLimit: query.Limit * 2, // Consider 2x candidates
				Strategy:        "mmr",
			}
			result, err := diversifier.SearchWithDiversification(ctx, query, diversityOpts)
			return result, true, err // Native diversification was used
		}
	}

	// Check if we can use native reranking during search (only for StrategyRelevant)
	if *opts.Strategy == StrategyRelevant {
		if reranker, ok := store.(RerankingProvider); ok {
			rerankOpts := RerankingOptions{
				Query: query.Text,
				TopK:  query.Limit * 2, // Rerank 2x candidates
			}
			result, err := reranker.SearchWithReranking(ctx, query, rerankOpts)
			return result, true, err // Native reranking was used
		}
	}

	// Fall back to regular search
	result, err := store.Search(ctx, query)
	return result, false, err
}

// buildContext assembles documents using native store capabilities when available
func buildContext(documents []Document, opts *SearchOptions, store VectorStore, isNative bool) (string, error) {
	if len(documents) == 0 {
		return "", nil
	}

	var selectedDocs []Document
	var err error

	if isNative {
		// Native processing already applied strategy, use documents as-is
		selectedDocs = documents
	} else {
		// Apply strategy-based sorting/filtering post-search
		selectedDocs, err = applyStrategy(documents, opts)
		if err != nil {
			return "", err
		}
	}

	// Build final context string using native token estimation if available
	separator := opts.Separator
	if separator == "" {
		separator = "\n\n---\n\n" // Default separator
	}

	contextParts := make([]string, 0, len(selectedDocs))
	currentTokens := 0

	// Check if store provides native token estimation
	tokenEstimator, hasNativeTokens := store.(TokenEstimator)

	for _, doc := range selectedDocs {
		var docTokens int
		if hasNativeTokens {
			// Use native token estimation for accuracy
			docTokens = tokenEstimator.EstimateTokens(doc.Content)
		} else {
			// Fall back to rough estimation
			docTokens = estimateTokens(doc.Content)
		}

		if opts.MaxTokens > 0 && currentTokens+docTokens > opts.MaxTokens {
			break
		}

		contextParts = append(contextParts, doc.Content)
		currentTokens += docTokens
	}

	return strings.Join(contextParts, separator), nil
}

// applyStrategy applies the specified strategy post search to select and order documents
func applyStrategy(documents []Document, opts *SearchOptions) ([]Document, error) {
	if opts.Strategy == nil {
		return documents, nil
	}

	docs := make([]Document, len(documents))
	copy(docs, documents)

	switch *opts.Strategy {
	case StrategyRelevant:
		// Sort by score (descending)
		slices.SortFunc(docs, func(a, b Document) int {
			if a.Score > b.Score {
				return -1 // a comes before b (descending)
			}
			if a.Score < b.Score {
				return 1 // a comes after b
			}
			return 0 // equal
		})
	case StrategyRecent:
		// Sort by created timestamp (descending)
		slices.SortFunc(docs, func(a, b Document) int {
			if a.Created.After(b.Created) {
				return -1 // a comes before b (descending)
			}
			if b.Created.After(a.Created) {
				return 1 // a comes after b
			}
			return 0 // equal
		})
	case StrategyDiverse:
		// If native diversification wasn't used during search, apply local implementation
		docs = selectDiverse(docs)
	case StrategySummary:
		// For now, just truncate long documents (could integrate with summarization later)
		docs = truncateDocuments(docs, 500) // 500 words max per doc
	default:
		return nil, fmt.Errorf("unknown context strategy: %s", *opts.Strategy)
	}

	return docs, nil
}

// selectDiverse implements Maximum Marginal Relevance (MMR) algorithm for diversity selection
func selectDiverse(documents []Document) []Document {
	if len(documents) <= 1 {
		return documents
	}

	// MMR algorithm: balance relevance and diversity
	// λ = 0.5 gives equal weight to relevance and diversity
	lambda := 0.5
	maxResults := min(len(documents), 10) // Limit to reasonable number
	
	var selected []Document
	remaining := make([]Document, len(documents))
	copy(remaining, documents)
	
	// Start with highest scoring document
	selected = append(selected, remaining[0])
	remaining = remaining[1:]
	
	for len(selected) < maxResults && len(remaining) > 0 {
		var bestIdx int
		var bestScore float64 = -1
		
		for i, candidate := range remaining {
			// Calculate MMR score: λ * relevance - (1-λ) * max_similarity_to_selected
			relevanceScore := candidate.Score
			
			// Find maximum similarity to any selected document
			maxSimilarity := 0.0
			for _, selected := range selected {
				similarity := contentSimilarity(candidate.Content, selected.Content)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
			
			// MMR formula
			mmrScore := lambda*relevanceScore - (1-lambda)*maxSimilarity
			
			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}
		
		// Add best candidate to selected and remove from remaining
		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}
	
	return selected
}

// contentSimilarity provides accurate content similarity using cosine similarity
func contentSimilarity(a, b string) float64 {
	// Use cosine similarity with 2-grams for better semantic comparison
	// 2-grams work well for document-level text similarity
	similarity := edlib.CosineSimilarity(a, b, 2)
	return float64(similarity)
}

// truncateDocuments truncates document content to specified word limit (fallback)
func truncateDocuments(documents []Document, maxWords int) []Document {
	truncated := make([]Document, len(documents))
	copy(truncated, documents)

	for i, doc := range truncated {
		words := strings.Fields(doc.Content)
		if len(words) > maxWords {
			truncated[i].Content = strings.Join(words[:maxWords], " ") + "..."
		}
	}

	return truncated
}

// estimateTokens provides a rough token count estimation (fallback)
func estimateTokens(text string) int {
	words := strings.Fields(text)
	// Rough approximation: 1 token per 0.75 words
	return int(float64(len(words)) * 1.33)
}

