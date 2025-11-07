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

// SimilarityAlgorithm defines available similarity algorithms
type SimilarityAlgorithm string

const (
	// CosineSimilarity uses n-gram cosine similarity for document comparison
	CosineSimilarity SimilarityAlgorithm = "cosine"
	// JaccardSimilarity uses Jaccard index for word-level document comparison
	JaccardSimilarity SimilarityAlgorithm = "jaccard"
	// JaroWinklerSimilarity uses Jaro-Winkler distance for string similarity
	JaroWinklerSimilarity SimilarityAlgorithm = "jaro-winkler"
	// SorensenDiceSimilarity uses Sorensen-Dice coefficient for document comparison
	SorensenDiceSimilarity SimilarityAlgorithm = "sorensen-dice"
	// HybridSimilarity combines multiple algorithms for robust similarity calculation
	HybridSimilarity SimilarityAlgorithm = "hybrid"
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

		// Handle embedding generation based on store capabilities
		if err := handleEmbeddingForQuery(r.Context, store, &query, opts); err != nil {
			return err
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

// handleEmbeddingForQuery determines how to handle embedding generation based on store capabilities
func handleEmbeddingForQuery(ctx context.Context, store VectorStore, query *SearchQuery, opts *SearchOptions) error {
	// Check if store supports auto-embedding (like Weaviate)
	if autoEmbedder, ok := store.(AutoEmbeddingCapable); ok && autoEmbedder.SupportsAutoEmbedding() {
		// Store handles embeddings automatically - no vector needed in query
		// The store will generate embeddings from query.Text internally
		return nil
	}

	// Check if store can generate embeddings (like Qdrant with external service)
	if embedder, ok := store.(EmbeddingCapable); ok {
		// Use store's embedding capability
		vector, err := embedder.GetEmbedding(ctx, query.Text)
		if err != nil {
			return fmt.Errorf("failed to generate embedding using store capability: %w", err)
		}
		query.Vector = vector
		return nil
	}

	// Check if user provided custom embedding provider in options
	if opts.EmbeddingProvider != nil {
		// Use custom embedding provider
		vector, err := opts.EmbeddingProvider.Embed(ctx, query.Text)
		if err != nil {
			return fmt.Errorf("failed to generate embedding using custom provider: %w", err)
		}
		query.Vector = vector
		return nil
	}

	// No embedding capability available - let store handle text-based search if supported
	// Some stores might support text search without embeddings
	return nil
}

// strategySearch executes search using native capabilities when available
// Returns: (*SearchResult, nativeProcessed bool, error)
func strategySearch(ctx context.Context, store VectorStore, query SearchQuery, opts *SearchOptions) (*SearchResult, bool, error) {
	// If no strategy specified, use regular search
	if opts.Strategy == nil {
		result, err := store.Search(ctx, query)
		return result, false, err
	}

	processingMode := opts.GetStrategyProcessing()

	// Handle StrategyPost - skip native processing entirely
	if processingMode == StrategyPost {
		result, err := store.Search(ctx, query)
		return result, false, err
	}

	// Handle StrategyNative - only use native, fail if not available
	if processingMode == StrategyNative {
		nativeResult, err := tryNativeStrategy(ctx, store, query, opts)
		if err != nil {
			return nil, false, fmt.Errorf("native strategy processing failed: %w", err)
		}
		if nativeResult != nil {
			return nativeResult, true, nil
		}
		return nil, false, fmt.Errorf("native strategy processing not available for %s", *opts.Strategy)
	}

	// Handle StrategyAuto and StrategyBoth - try native first
	nativeResult, err := tryNativeStrategy(ctx, store, query, opts)
	if err != nil {
		// For StrategyBoth, native failure should not stop the process
		if processingMode != StrategyBoth {
			return nil, false, err
		}
	}

	// For StrategyBoth, we always do post-search processing even if native succeeded
	// For StrategyAuto, we only do post-search if native failed
	if processingMode == StrategyBoth || (processingMode == StrategyAuto && nativeResult == nil) {
		// Use regular search, will apply post-processing in buildContext
		result, err := store.Search(ctx, query)
		if processingMode == StrategyBoth && nativeResult != nil {
			// Combine native and regular results? For now, prefer native results
			// This could be enhanced to merge results intelligently
			return nativeResult, true, err
		}
		return result, false, err
	}

	// StrategyAuto with successful native processing
	if nativeResult != nil {
		return nativeResult, true, nil
	}

	// Fall back to regular search
	result, err := store.Search(ctx, query)
	return result, false, err
}

// tryNativeStrategy attempts to use native database strategy processing
func tryNativeStrategy(ctx context.Context, store VectorStore, query SearchQuery, opts *SearchOptions) (*SearchResult, error) {
	// Try native diversification for StrategyDiverse
	if *opts.Strategy == StrategyDiverse {
		if diversifier, ok := store.(DiversificationProvider); ok {
			diversityOpts := DiversificationOptions{
				Diversity:       opts.GetDiversityLambda(),
				CandidatesLimit: int(float64(query.Limit) * opts.GetCandidatesMultiplier()),
				Strategy:        opts.GetDiversityStrategy(),
			}
			return diversifier.SearchWithDiversification(ctx, query, diversityOpts)
		}
	}

	// Try native reranking for StrategyRelevant
	if *opts.Strategy == StrategyRelevant {
		if reranker, ok := store.(RerankingProvider); ok {
			rerankOpts := RerankingOptions{
				Query: query.Text,
				TopK:  int(float64(query.Limit) * opts.GetRerankMultiplier()),
			}
			return reranker.SearchWithReranking(ctx, query, rerankOpts)
		}
	}

	// No native support available
	return nil, nil
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
	separator := opts.GetSeparator()

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
			docTokens = estimateTokens(doc.Content, opts)
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
		docs = selectDiverse(docs, opts)
	case StrategySummary:
		// For now, just truncate long documents (could integrate with summarization later)
		docs = truncateDocuments(docs, opts.GetSummaryWordLimit())
	default:
		return nil, fmt.Errorf("unknown context strategy: %s", *opts.Strategy)
	}

	return docs, nil
}

// selectDiverse implements Maximum Marginal Relevance (MMR) algorithm for diversity selection
func selectDiverse(documents []Document, opts *SearchOptions) []Document {
	if len(documents) <= 1 {
		return documents
	}

	// Determine similarity algorithm
	algorithm := opts.GetSimilarityAlgorithm()
	if opts.GetAdaptiveAlgorithm() {
		algorithm = selectOptimalSimilarity(documents)
	}

	// MMR algorithm: balance relevance and diversity
	lambda := opts.GetDiversityLambda()
	maxResults := min(len(documents), opts.GetMaxDiverseResults())

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
				similarity := contentSimilarity(candidate.Content, selected.Content, algorithm)
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

// selectOptimalSimilarity chooses best algorithm based on document properties
func selectOptimalSimilarity(docs []Document) SimilarityAlgorithm {
	if len(docs) == 0 {
		return CosineSimilarity
	}

	// Analyze document characteristics
	avgLength := calculateAverageLength(docs)

	switch {
	case avgLength < 100: // Short documents
		return JaccardSimilarity // Better for keyword matching
	case avgLength > 1000: // Long documents
		return HybridSimilarity // Combine approaches
	default: // Medium length (100-1000)
		return CosineSimilarity // Best for semantic similarity
	}
}

// calculateAverageLength computes the average character length of documents
func calculateAverageLength(docs []Document) float64 {
	if len(docs) == 0 {
		return 0
	}

	totalLength := 0
	for _, doc := range docs {
		totalLength += len(doc.Content)
	}

	return float64(totalLength) / float64(len(docs))
}

// contentSimilarity provides accurate content similarity with algorithm selection
func contentSimilarity(a, b string, algorithm SimilarityAlgorithm) float64 {
	switch algorithm {
	case JaccardSimilarity:
		// Use word-level splitting (split=0) for document similarity
		return float64(edlib.JaccardSimilarity(a, b, 0))
	case CosineSimilarity:
		return float64(edlib.CosineSimilarity(a, b, 2)) // 2-grams
	case JaroWinklerSimilarity:
		sim := edlib.JaroWinklerSimilarity(a, b)
		return float64(1.0 - sim) // Convert distance to similarity
	case SorensenDiceSimilarity:
		// Use word-level splitting (split=0) for document similarity
		return float64(edlib.SorensenDiceCoefficient(a, b, 0))
	case HybridSimilarity:
		return hybridSimilarity(a, b)
	default:
		return float64(edlib.CosineSimilarity(a, b, 2))
	}
}

// Hybrid approach combining multiple algorithms
func hybridSimilarity(a, b string) float64 {
	jaccard := edlib.JaccardSimilarity(a, b, 0) // Word-level splitting
	cosine := edlib.CosineSimilarity(a, b, 2)   // 2-grams

	// Weight semantic (cosine) more than lexical (jaccard) for documents
	return float64(0.7*cosine + 0.3*jaccard)
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
func estimateTokens(text string, opts *SearchOptions) int {
	words := strings.Fields(text)
	ratio := opts.GetTokenEstimationRatio()
	return int(float64(len(words)) * ratio)
}
