package retrieval

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// ContextBuilder creates a context assembly middleware.
//
// Input: SearchResult JSON from vector search
// Output: string assembled context
// Behavior: BUFFERED - reads entire SearchResult to build context
//
// Assembles retrieved documents into optimized context for AI models.
// Supports multiple strategies for document selection and ordering,
// with token limits and custom formatting options.
//
// Example:
//
//	config := &retrieval.ContextConfig{
//	    MaxTokens: 4000,
//	    Strategy: retrieval.StrategyRelevant,
//	}
//	flow := calque.NewFlow().
//	    Use(retrieval.VectorSearch(store, searchOpts)).
//	    Use(retrieval.ContextBuilder(config))
func ContextBuilder(config *ContextConfig) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input []byte
		err := calque.Read(r, &input)
		if err != nil {
			return err
		}

		// Parse SearchResult
		var result SearchResult
		if err := json.Unmarshal(input, &result); err != nil {
			return err
		}

		// Build context based on strategy
		context, err := buildContext(result.Documents, config)
		if err != nil {
			return err
		}

		return calque.Write(w, context)
	})
}

// buildContext assembles documents into context based on the specified strategy
func buildContext(documents []Document, config *ContextConfig) (string, error) {
	if len(documents) == 0 {
		return "", nil
	}

	// Apply strategy-based sorting/filtering
	selectedDocs, err := applyStrategy(documents, config)
	if err != nil {
		return "", err
	}

	// Build final context string
	separator := config.Separator
	if separator == "" {
		separator = "\n\n---\n\n" // Default separator
	}

	var contextParts []string
	currentTokens := 0

	for _, doc := range selectedDocs {
		// Rough token estimation (words * 1.3)
		docTokens := estimateTokens(doc.Content)
		
		if config.MaxTokens > 0 && currentTokens+docTokens > config.MaxTokens {
			break
		}

		contextParts = append(contextParts, doc.Content)
		currentTokens += docTokens
	}

	return strings.Join(contextParts, separator), nil
}

// applyStrategy applies the specified strategy to select and order documents
func applyStrategy(documents []Document, config *ContextConfig) ([]Document, error) {
	docs := make([]Document, len(documents))
	copy(docs, documents)

	switch config.Strategy {
	case StrategyRelevant:
		// Sort by score (descending)
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].Score > docs[j].Score
		})
	case StrategyRecent:
		// Sort by created timestamp (descending)
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].Created.After(docs[j].Created)
		})
	case StrategyDiverse:
		// Implement diversity selection (simplified version)
		docs = selectDiverse(docs)
	case StrategySummary:
		// For now, just truncate long documents (could integrate with summarization later)
		docs = truncateDocuments(docs, 500) // 500 words max per doc
	default:
		return nil, fmt.Errorf("unknown context strategy: %s", config.Strategy)
	}

	return docs, nil
}

// selectDiverse implements a simple diversity selection algorithm
func selectDiverse(documents []Document) []Document {
	if len(documents) <= 1 {
		return documents
	}

	var selected []Document
	selected = append(selected, documents[0]) // Start with highest scoring

	for _, doc := range documents[1:] {
		// Simple diversity check: ensure content isn't too similar to already selected
		isDiverse := true
		for _, sel := range selected {
			if contentSimilarity(doc.Content, sel.Content) > 0.8 {
				isDiverse = false
				break
			}
		}
		if isDiverse {
			selected = append(selected, doc)
		}
	}

	return selected
}

// contentSimilarity provides a simple content similarity measure
func contentSimilarity(a, b string) float64 {
	// Simple word overlap similarity (could be enhanced with proper algorithms)
	wordsA := strings.Fields(strings.ToLower(a))
	wordsB := strings.Fields(strings.ToLower(b))
	
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	wordSetA := make(map[string]bool)
	for _, word := range wordsA {
		wordSetA[word] = true
	}

	overlap := 0
	for _, word := range wordsB {
		if wordSetA[word] {
			overlap++
		}
	}

	return float64(overlap) / float64(max(len(wordsA), len(wordsB)))
}

// truncateDocuments truncates document content to specified word limit
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

// estimateTokens provides a rough token count estimation
func estimateTokens(text string) int {
	words := strings.Fields(text)
	// Rough approximation: 1 token per 0.75 words
	return int(float64(len(words)) * 1.33)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}