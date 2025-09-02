package weaviate

import (
	"context"
	"fmt"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	weav "github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/auth"
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

	// Configure Weaviate client
	conf := weav.Config{
		Host:   config.URL,
		Scheme: "http", // Default to http, can be overridden
	}

	// Add API key authentication if provided
	if config.APIKey != "" {
		conf.AuthConfig = auth.ApiKey{Value: config.APIKey}
	}

	// Detect if URL uses https
	if len(config.URL) > 5 && config.URL[:5] == "https" {
		conf.Scheme = "https"
		// Remove scheme from host
		if len(config.URL) > 8 {
			conf.Host = config.URL[8:] // Remove "https://"
		}
	} else if len(config.URL) > 4 && config.URL[:4] == "http" {
		conf.Scheme = "http"
		// Remove scheme from host
		if len(config.URL) > 7 {
			conf.Host = config.URL[7:] // Remove "http://"
		}
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
	return c.performSearch(ctx, query, query.Limit, "weaviate search failed")
}


// Store adds documents to the Weaviate vector database with embeddings.
func (c *Client) Store(ctx context.Context, documents []retrieval.Document) error {
	// TODO: Implement actual Weaviate storage
	return fmt.Errorf("weaviate store not yet implemented - add weaviate-go-client dependency")
}

// Delete removes documents from the Weaviate vector database.
func (c *Client) Delete(ctx context.Context, ids []string) error {
	// TODO: Implement actual Weaviate deletion
	return fmt.Errorf("weaviate delete not yet implemented - add weaviate-go-client dependency")
}

// GetEmbedding generates embeddings for text content using Weaviate's embedding modules.
func (c *Client) GetEmbedding(ctx context.Context, text string) (retrieval.EmbeddingVector, error) {
	// TODO: Implement actual embedding generation
	return nil, fmt.Errorf("weaviate embedding not yet implemented - add weaviate-go-client dependency")
}

// Health checks if the Weaviate instance is available and responsive.
func (c *Client) Health(ctx context.Context) error {
	// TODO: Implement actual health check
	return fmt.Errorf("weaviate health check not yet implemented - add weaviate-go-client dependency")
}

// Close releases any resources held by the Weaviate client.
func (c *Client) Close() error {
	// TODO: Implement cleanup if needed
	return nil
}

// parseWeaviateDocument converts Weaviate response to retrieval.Document
// performSearch executes a Weaviate search with the given parameters.
func (c *Client) performSearch(ctx context.Context, query retrieval.SearchQuery, limit int, errorMsg string) (*retrieval.SearchResult, error) {
	// Build nearText search query
	nearText := c.client.GraphQL().NearTextArgBuilder().
		WithConcepts([]string{query.Text})

	if query.Threshold > 0 {
		nearText = nearText.WithDistance(float32(1.0 - query.Threshold)) // Convert similarity to distance
	}

	// Build GraphQL query
	builder := c.client.GraphQL().Get().
		WithClassName(c.className).
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

	// Parse results
	documents := c.parseSearchResults(result)

	return &retrieval.SearchResult{
		Documents: documents,
		Query:     query.Text,
		Total:     len(documents),
		Threshold: query.Threshold,
	}, nil
}

// parseSearchResults extracts documents from Weaviate GraphQL response.
func (c *Client) parseSearchResults(result *models.GraphQLResponse) []retrieval.Document {
	var documents []retrieval.Document
	if result.Data != nil {
		if get, ok := result.Data["Get"].(map[string]any); ok {
			if classData, ok := get[c.className].([]any); ok {
				for _, item := range classData {
					if doc, ok := item.(map[string]any); ok {
						document := parseWeaviateDocument(doc)
						documents = append(documents, document)
					}
				}
			}
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
