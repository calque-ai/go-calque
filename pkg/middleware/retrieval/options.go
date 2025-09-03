package retrieval

import "context"

// SearchOptions configures vector search behavior and optional context building.
type SearchOptions struct {
	Threshold         float64           `json:"threshold"`        // Similarity threshold (0-1)
	Limit             int               `json:"limit,omitempty"`  // Maximum results to return
	Filter            map[string]any    `json:"filter,omitempty"` // Metadata filters
	EmbeddingProvider EmbeddingProvider `json:"-"`                // Custom embedding provider

	// Advanced search options - Strategy Processing Control
	StrategyProcessing StrategyProcessingMode `json:"strategy_processing,omitempty"` // How to apply strategies (default: StrategyAuto)

	// Diversification options (for StrategyDiverse)
	DiversityLambda     *float64 `json:"diversity_lambda,omitempty"`      // MMR balance parameter (0-1, default: 0.5)
	CandidatesMultipler *float64 `json:"candidates_multiplier,omitempty"` // Multiplier for candidates limit (default: 2.0)
	MaxDiverseResults   *int     `json:"max_diverse_results,omitempty"`   // Max results for MMR (default: 10)
	DiversityStrategy   *string  `json:"diversity_strategy,omitempty"`    // Strategy for native diversification (default: "mmr")

	// Reranking options (for StrategyRelevant with native support)
	RerankMultiplier *float64 `json:"rerank_multiplier,omitempty"` // Multiplier for rerank candidates (default: 2.0)

	// Context building options (optional)
	Strategy  *ContextStrategy `json:"strategy,omitempty"`   // If set, returns formatted context instead of JSON
	MaxTokens int              `json:"max_tokens,omitempty"` // Token limit for context
	Separator string           `json:"separator,omitempty"`  // Document separator in context

	// Summary strategy options
	SummaryWordLimit *int `json:"summary_word_limit,omitempty"` // Word limit per document for StrategySummary (default: 500)

	// Token estimation options
	TokenEstimationRatio *float64 `json:"token_estimation_ratio,omitempty"` // Ratio for token estimation (default: 1.33)
}

// EmbeddingProvider interface for generating embeddings.
type EmbeddingProvider interface {
	// Embed generates embeddings for text content
	Embed(ctx context.Context, text string) (EmbeddingVector, error)
}

// StrategyProcessingMode defines how strategies are applied
type StrategyProcessingMode string

const (
	// Strategy processing modes
	StrategyAuto   StrategyProcessingMode = "auto"   // Use native if available, otherwise post-search (default)
	StrategyNative StrategyProcessingMode = "native" // Use native DB processing only, fail if not available
	StrategyPost   StrategyProcessingMode = "post"   // Use post-search processing only, skip native
	StrategyBoth   StrategyProcessingMode = "both"   // Use both native AND post-search processing
)

// Default configuration values
const (
	DefaultDiversityLambda      = 0.5           // Equal weight to relevance and diversity
	DefaultCandidatesMultiplier = 2.0           // Consider 2x candidates for native operations
	DefaultMaxDiverseResults    = 10            // Reasonable limit for MMR processing
	DefaultDiversityStrategy    = "mmr"         // Maximum Marginal Relevance strategy
	DefaultRerankMultiplier     = 2.0           // Consider 2x candidates for reranking
	DefaultSummaryWordLimit     = 500           // 500 words max per document
	DefaultTokenEstimationRatio = 1.33          // ~1 token per 0.75 words
	DefaultSeparator            = "\n\n---\n\n" // Document separator
)

// GetDiversityLambda returns the configured diversity lambda or default
func (opts *SearchOptions) GetDiversityLambda() float64 {
	if opts.DiversityLambda != nil {
		return *opts.DiversityLambda
	}
	return DefaultDiversityLambda
}

// GetCandidatesMultiplier returns the configured candidates multiplier or default
func (opts *SearchOptions) GetCandidatesMultiplier() float64 {
	if opts.CandidatesMultipler != nil {
		return *opts.CandidatesMultipler
	}
	return DefaultCandidatesMultiplier
}

// GetMaxDiverseResults returns the configured max diverse results or default
func (opts *SearchOptions) GetMaxDiverseResults() int {
	if opts.MaxDiverseResults != nil {
		return *opts.MaxDiverseResults
	}
	return DefaultMaxDiverseResults
}

// GetDiversityStrategy returns the configured diversity strategy or default
func (opts *SearchOptions) GetDiversityStrategy() string {
	if opts.DiversityStrategy != nil {
		return *opts.DiversityStrategy
	}
	return DefaultDiversityStrategy
}

// GetRerankMultiplier returns the configured rerank multiplier or default
func (opts *SearchOptions) GetRerankMultiplier() float64 {
	if opts.RerankMultiplier != nil {
		return *opts.RerankMultiplier
	}
	return DefaultRerankMultiplier
}

// GetSummaryWordLimit returns the configured summary word limit or default
func (opts *SearchOptions) GetSummaryWordLimit() int {
	if opts.SummaryWordLimit != nil {
		return *opts.SummaryWordLimit
	}
	return DefaultSummaryWordLimit
}

// GetTokenEstimationRatio returns the configured token estimation ratio or default
func (opts *SearchOptions) GetTokenEstimationRatio() float64 {
	if opts.TokenEstimationRatio != nil {
		return *opts.TokenEstimationRatio
	}
	return DefaultTokenEstimationRatio
}

// GetSeparator returns the configured separator or default
func (opts *SearchOptions) GetSeparator() string {
	if opts.Separator != "" {
		return opts.Separator
	}
	return DefaultSeparator
}

// GetStrategyProcessing returns the configured strategy processing mode or default
func (opts *SearchOptions) GetStrategyProcessing() StrategyProcessingMode {
	if opts.StrategyProcessing != "" {
		return opts.StrategyProcessing
	}
	return StrategyAuto
}
