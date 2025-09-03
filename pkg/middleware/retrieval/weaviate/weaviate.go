package weaviate

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	"github.com/go-openapi/strfmt"
	weav "github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

// Client represents a Weaviate vector database client.
//
// Implements the retrieval.VectorStore interface for Weaviate operations.
// Provides vector similarity search, document storage, and embedding capabilities.
//
// NOTE: This client does not implement retrieval.RerankingProvider because the
// Weaviate Go client does not yet support native reranking modules (rerank-cohere,
// rerank-transformers, etc.). Framework-level reranking should be used instead.
type Client struct {
	url       string
	className string
	apiKey    string
	client    *weav.Client
}

// Config holds Weaviate client configuration.
type Config struct {
	URL       string // Weaviate instance URL
	ClassName string // Weaviate class name for documents
	APIKey    string // Optional API key for authentication
}

// New creates a new Weaviate client with the specified configuration.
//
// Example:
//
//	client := weaviate.New(&weaviate.Config{
//	    URL:       "http://localhost:8080",
//	    ClassName: "Document",
//	    APIKey:    "your-api-key",
//	})
func New(config *Config) (*Client, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("weaviate URL is required")
	}
	if config.ClassName == "" {
		config.ClassName = "Document" // Default class name
	}

	// Parse the URL to extract scheme and host
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Weaviate URL: %w", err)
	}

	// Configure Weaviate client
	conf := weav.Config{
		Host:   parsedURL.Host,
		Scheme: parsedURL.Scheme,
	}

	// Default to http if no scheme provided
	if conf.Scheme == "" {
		conf.Scheme = "http"
		conf.Host = config.URL
	}

	// Add API key authentication if provided
	if config.APIKey != "" {
		conf.AuthConfig = auth.ApiKey{Value: config.APIKey}
	}

	// Create Weaviate client
	weaviateClient, err := weav.NewClient(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create Weaviate client: %w", err)
	}

	client := &Client{
		url:       config.URL,
		className: config.ClassName,
		apiKey:    config.APIKey,
		client:    weaviateClient,
	}

	return client, nil
}

// Search performs similarity search against the Weaviate vector database.
func (c *Client) Search(ctx context.Context, query retrieval.SearchQuery) (*retrieval.SearchResult, error) {
	// Determine class name: use query override or client default
	className := c.getClassName(query.Collection)
	if className == "" {
		return nil, fmt.Errorf("no class specified in query or client config")
	}

	return c.performSearch(ctx, query, className, query.Limit, "weaviate search failed")
}

// Store adds documents to the Weaviate vector database using batch operations.
func (c *Client) Store(ctx context.Context, documents []retrieval.Document) error {
	if len(documents) == 0 {
		return nil // Nothing to store
	}

	if c.client == nil {
		return fmt.Errorf("weaviate client is not initialized")
	}
	if c.className == "" {
		return fmt.Errorf("weaviate class name is not configured")
	}

	objects := make([]*models.Object, 0, len(documents))

	// Convert documents to Weaviate objects
	for i, doc := range documents {
		if doc.Content == "" {
			return fmt.Errorf("document %d has empty content", i)
		}

		properties := make(map[string]any, 2) // content + metadata
		properties["content"] = doc.Content

		// Add metadata if present
		if doc.Metadata != nil {
			properties["metadata"] = doc.Metadata
		}

		// Create Weaviate object
		obj := &models.Object{
			Class:      c.className,
			Properties: properties,
		}

		// Set ID if provided, otherwise let Weaviate generate one
		if doc.ID != "" {
			obj.ID = strfmt.UUID(doc.ID)
		}

		objects = append(objects, obj)
	}

	// Execute batch create
	result, err := c.client.Batch().ObjectsBatcher().
		WithObjects(objects...).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("batch store failed: %w", err)
	}

	// Defensive check for result
	if result == nil {
		return fmt.Errorf("batch store returned nil result")
	}

	// Check for any batch errors
	if len(result) > 0 {
		var errors []error
		for i, res := range result {
			// Defensive checks for result structure
			if res.Result == nil {
				errors = append(errors, fmt.Errorf("document %d: nil result", i))
				continue
			}
			if res.Result.Errors != nil && len(res.Result.Errors.Error) > 0 {
				errors = append(errors, fmt.Errorf("document %d failed: %v", i, res.Result.Errors.Error))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("batch store partially failed with %d errors: %v", len(errors), errors)
		}
	}

	return nil
}

// Delete removes documents from the Weaviate vector database using batch operations.
func (c *Client) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil // Nothing to delete
	}

	// For single ID, use individual delete for simplicity
	if len(ids) == 1 {
		err := c.client.Data().Deleter().
			WithClassName(c.className).
			WithID(ids[0]).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete document %s: %w", ids[0], err)
		}
		return nil
	}

	// For multiple IDs, use batch delete with OR filter
	// Build a WHERE filter that matches any of the provided IDs
	var whereFilters []*filters.WhereBuilder
	for _, id := range ids {
		whereFilter := filters.Where().
			WithOperator(filters.Equal).
			WithPath([]string{"id"}).
			WithValueString(id)
		whereFilters = append(whereFilters, whereFilter)
	}

	// Combine all ID filters with OR operator
	var combinedWhere *filters.WhereBuilder
	if len(whereFilters) == 1 {
		combinedWhere = whereFilters[0]
	} else {
		// Use OR operator with all filters as operands
		combinedWhere = filters.Where().
			WithOperator(filters.Or).
			WithOperands(whereFilters)
	}

	// Execute batch delete
	result, err := c.client.Batch().ObjectsBatchDeleter().
		WithClassName(c.className).
		WithWhere(combinedWhere).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("batch delete failed: %w", err)
	}

	// Check if we deleted the expected number of documents
	if result != nil && result.Results != nil {
		if result.Results.Failed > 0 {
			return fmt.Errorf("batch delete partially failed: %d succeeded, %d failed",
				result.Results.Successful, result.Results.Failed)
		}
	}

	return nil
}

// SupportsAutoEmbedding returns true indicating Weaviate handles embeddings automatically
func (c *Client) SupportsAutoEmbedding() bool {
	return true
}

// GetEmbeddingConfig returns information about Weaviate's auto-embedding configuration
func (c *Client) GetEmbeddingConfig() retrieval.EmbeddingConfig {
	// In a real implementation, this could query Weaviate's schema to get actual config
	// For now, return default configuration
	return retrieval.EmbeddingConfig{
		Model:      "text2vec-openai",  // Default model - could be detected from schema
		Dimensions: 1536,               // Default OpenAI dimensions - could be detected
		Provider:   "weaviate",         // Provider is Weaviate itself
	}
}

// Health checks if the Weaviate instance is available and responsive.
func (c *Client) Health(ctx context.Context) error {
	// Use the cluster health endpoint
	healthy, err := c.client.Cluster().NodesStatusGetter().Do(ctx)
	if err != nil {
		return fmt.Errorf("weaviate health check failed: %w", err)
	}

	// Check if any nodes are healthy
	if healthy == nil || len(healthy.Nodes) == 0 {
		return fmt.Errorf("weaviate cluster has no nodes")
	}

	// Check if at least one node is healthy
	for _, node := range healthy.Nodes {
		if node.Status != nil && *node.Status == "HEALTHY" {
			return nil
		}
	}

	return fmt.Errorf("weaviate cluster has no healthy nodes")
}

// Close releases any resources held by the Weaviate client.
func (c *Client) Close() error {
	// Weaviate Go client doesn't require explicit cleanup
	c.client = nil
	return nil
}

// parseWeaviateDocument converts Weaviate response to retrieval.Document
// getClassName determines the class name to use: query override or client default
func (c *Client) getClassName(queryCollection string) string {
	if queryCollection != "" {
		return queryCollection
	}
	return c.className
}

// performSearch executes a Weaviate search with the given parameters.
func (c *Client) performSearch(ctx context.Context, query retrieval.SearchQuery, className string, limit int, errorMsg string) (*retrieval.SearchResult, error) {
	// Build nearText search query
	nearText := c.client.GraphQL().NearTextArgBuilder().
		WithConcepts([]string{query.Text})

	if query.Threshold > 0 {
		nearText = nearText.WithDistance(float32(1.0 - query.Threshold)) // Convert similarity to distance
	}

	// Build GraphQL query
	builder := c.client.GraphQL().Get().
		WithClassName(className).
		WithNearText(nearText).
		WithFields(
			graphql.Field{Name: "content"},
			graphql.Field{Name: "metadata", Fields: []graphql.Field{}},
			graphql.Field{Name: "_additional", Fields: []graphql.Field{
				{Name: "id"},
				{Name: "score"},
			}},
		)

	if limit > 0 {
		builder = builder.WithLimit(limit)
	}

	// Execute query
	result, err := builder.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errorMsg, err)
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("%s: GraphQL errors: %v", errorMsg, result.Errors)
	}

	// Parse results
	documents := c.parseSearchResults(result, className)

	return &retrieval.SearchResult{
		Documents: documents,
		Query:     query.Text,
		Total:     len(documents),
		Threshold: query.Threshold,
	}, nil
}

// parseSearchResults extracts documents from Weaviate GraphQL response.
func (c *Client) parseSearchResults(result *models.GraphQLResponse, className string) []retrieval.Document {
	var documents []retrieval.Document

	if result.Data == nil {
		return documents
	}

	// Extract the class data using type assertions
	get, ok := result.Data["Get"].(map[string]any)
	if !ok {
		return documents
	}

	classData, ok := get[className].([]any)
	if !ok {
		return documents
	}

	// Parse each document
	for _, item := range classData {
		if doc, ok := item.(map[string]any); ok {
			document := parseWeaviateDocument(doc)
			documents = append(documents, document)
		}
	}

	return documents
}

func parseWeaviateDocument(doc map[string]any) retrieval.Document {
	document := retrieval.Document{}

	// Extract content
	if content, ok := doc["content"].(string); ok {
		document.Content = content
	}

	// Extract metadata
	if metadata, ok := doc["metadata"].(map[string]any); ok {
		document.Metadata = metadata
	}

	// Extract additional fields
	if additional, ok := doc["_additional"].(map[string]any); ok {
		if id, ok := additional["id"].(string); ok {
			document.ID = id
		}
		if score, ok := additional["score"].(float64); ok {
			document.Score = score
		}
	}

	// Set timestamps (Weaviate doesn't provide these by default, use current time)
	now := time.Now()
	document.Created = now
	document.Updated = now

	return document
}
