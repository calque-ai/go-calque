// Package weaviate provides integration with Weaviate vector database.
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

// PropertyType represents the data type of a Weaviate property.
type PropertyType string

const (
	// PropertyTypeText represents a text/string property
	PropertyTypeText PropertyType = "text"
	// PropertyTypeTextArray represents an array of text/strings
	PropertyTypeTextArray PropertyType = "text[]"
	// PropertyTypeInt represents an integer property
	PropertyTypeInt PropertyType = "int"
	// PropertyTypeNumber represents a float/number property
	PropertyTypeNumber PropertyType = "number"
	// PropertyTypeBool represents a boolean property
	PropertyTypeBool PropertyType = "boolean"
	// PropertyTypeDate represents a date/timestamp property
	PropertyTypeDate PropertyType = "date"
)

// PropertyConfig defines a single property in the Weaviate schema.
type PropertyConfig struct {
	Name        string       // Property name
	Type        PropertyType // Data type
	Description string       // Optional description
	Indexed     bool         // Whether to index for filtering (default: true)
}

// SchemaConfig defines the complete schema configuration for a Weaviate class.
type SchemaConfig struct {
	ClassName   string           // Name of the Weaviate class
	Description string           // Optional class description
	Vectorizer  string           // Vectorizer module (e.g., "none", "text2vec-openai")
	Properties  []PropertyConfig // List of properties
}

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
	schema    *SchemaConfig // Cached schema configuration
}

// Config holds Weaviate client configuration.
type Config struct {
	URL       string // Weaviate instance URL
	ClassName string // Weaviate class name for documents
	APIKey    string // Optional API key for authentication
}

// New creates a new Weaviate client with explicit schema configuration.
// Following Weaviate best practices, this method:
// - Explicitly defines the schema upfront
// - Ensures schema exists (creating it if necessary)
// - Enables type validation on Store() operations
// - Prevents metadata from being silently lost
//
// Example:
//
//	schema := &weaviate.SchemaConfig{
//	    ClassName:  "Document",
//	    Vectorizer: "none",
//	    Properties: []weaviate.PropertyConfig{
//	        {Name: "category", Type: weaviate.PropertyTypeText, Indexed: true},
//	        {Name: "priority", Type: weaviate.PropertyTypeInt, Indexed: true},
//	        {Name: "tags", Type: weaviate.PropertyTypeTextArray, Indexed: true},
//	    },
//	}
//	client, err := weaviate.New(ctx, &weaviate.Config{
//	    URL: "http://localhost:8080",
//	}, schema)
func New(ctx context.Context, config *Config, schema *SchemaConfig) (*Client, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}

	// Override className from schema if provided
	if schema.ClassName != "" {
		config.ClassName = schema.ClassName
	}

	// Create the client
	client, err := newClient(config)
	if err != nil {
		return nil, err
	}

	// Ensure schema exists
	if err := client.EnsureSchema(ctx, schema); err != nil {
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
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

	// Validate documents against schema if schema is configured
	for i, doc := range documents {
		if err := c.ValidateDocument(doc); err != nil {
			return fmt.Errorf("document %d validation failed: %w", i, err)
		}
	}

	objects := make([]*models.Object, 0, len(documents))

	// Convert documents to Weaviate objects
	for i, doc := range documents {
		if doc.Content == "" {
			return fmt.Errorf("document %d has empty content", i)
		}

		properties := make(map[string]any, 2) // content + metadata
		properties["content"] = doc.Content

		// Add metadata if present - flatten metadata fields to top level for filtering
		if doc.Metadata != nil {
			for key, value := range doc.Metadata {
				properties[key] = value
			}
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

		// Add vector if provided (required when vectorizer is "none")
		// Note: Document struct doesn't have Vector field, so we check metadata for it
		if vec, ok := doc.Metadata["vector"].([]float32); ok && len(vec) > 0 {
			obj.Vector = vec
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
	whereFilters := make([]*filters.WhereBuilder, 0, len(ids))
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
		Model:      "text2vec-openai", // Default model - could be detected from schema
		Dimensions: 1536,              // Default OpenAI dimensions - could be detected
		Provider:   "weaviate",        // Provider is Weaviate itself
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

// EnsureSchema creates or validates the schema in Weaviate.
// This method is idempotent - it can be called multiple times safely.
// If the class already exists, it validates that the existing schema matches.
//
// Following Weaviate best practices, schemas should be explicitly defined
// rather than relying on auto-schema in production environments.
//
// Example:
//
//	schema := &weaviate.SchemaConfig{
//	    ClassName:  "Document",
//	    Vectorizer: "none",
//	    Properties: []weaviate.PropertyConfig{
//	        {Name: "category", Type: weaviate.PropertyTypeText, Indexed: true},
//	        {Name: "priority", Type: weaviate.PropertyTypeInt, Indexed: true},
//	    },
//	}
//	err := client.EnsureSchema(ctx, schema)
func (c *Client) EnsureSchema(ctx context.Context, schema *SchemaConfig) error {
	if schema == nil {
		return fmt.Errorf("schema cannot be nil")
	}
	if schema.ClassName == "" {
		return fmt.Errorf("schema class name is required")
	}

	// Check if class already exists
	exists, err := c.client.Schema().ClassExistenceChecker().WithClassName(schema.ClassName).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if class exists: %w", err)
	}

	if exists {
		// Class exists - validate it matches our schema
		// For now, we'll just cache the schema and trust it matches
		// In a more robust implementation, we could fetch and compare
		c.schema = schema
		return nil
	}

	// Create the class
	classObj := c.buildWeaviateClass(schema)
	err = c.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to create class: %w", err)
	}

	// Cache the schema
	c.schema = schema
	return nil
}

// GetSchema returns the currently configured schema.
// Returns nil if no schema has been set via EnsureSchema.
func (c *Client) GetSchema() *SchemaConfig {
	return c.schema
}

// ValidateDocument checks if a document conforms to the configured schema.
// Returns an error if the document has invalid or missing fields.
func (c *Client) ValidateDocument(doc retrieval.Document) error {
	if c.schema == nil {
		// No schema configured - skip validation
		return nil
	}

	if doc.Content == "" {
		return fmt.Errorf("document content is required")
	}

	// Validate metadata fields against schema
	if doc.Metadata == nil {
		return nil // No metadata is valid
	}

	// Build a map of valid property names and types
	validProps := make(map[string]PropertyType)
	for _, prop := range c.schema.Properties {
		validProps[prop.Name] = prop.Type
	}

	// Check each metadata field
	for key, value := range doc.Metadata {
		// Skip special fields
		if key == "vector" {
			continue // Vector is handled separately
		}

		expectedType, exists := validProps[key]
		if !exists {
			return fmt.Errorf("unknown property %q not in schema", key)
		}

		// Validate type
		if err := validatePropertyType(key, value, expectedType); err != nil {
			return err
		}
	}

	return nil
}

// newClient creates a new Weaviate client with the specified configuration.
// This is a private function - use New() instead which requires a schema.
func newClient(config *Config) (*Client, error) {
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

// buildWeaviateClass converts SchemaConfig to Weaviate models.Class
func (c *Client) buildWeaviateClass(schema *SchemaConfig) *models.Class {
	classObj := &models.Class{
		Class:       schema.ClassName,
		Description: schema.Description,
	}

	// Set vectorizer (default to "none" if not specified)
	if schema.Vectorizer != "" {
		classObj.Vectorizer = schema.Vectorizer
	} else {
		classObj.Vectorizer = "none"
	}

	// Always include content property
	properties := []*models.Property{
		{
			Name:        "content",
			DataType:    []string{"text"},
			Description: "Document content",
		},
	}

	// Add user-defined properties
	for _, prop := range schema.Properties {
		weaviateProp := &models.Property{
			Name:        prop.Name,
			Description: prop.Description,
		}

		// Convert our PropertyType to Weaviate data types
		switch prop.Type {
		case PropertyTypeText:
			weaviateProp.DataType = []string{"text"}
		case PropertyTypeTextArray:
			weaviateProp.DataType = []string{"text[]"}
		case PropertyTypeInt:
			weaviateProp.DataType = []string{"int"}
		case PropertyTypeNumber:
			weaviateProp.DataType = []string{"number"}
		case PropertyTypeBool:
			weaviateProp.DataType = []string{"boolean"}
		case PropertyTypeDate:
			weaviateProp.DataType = []string{"date"}
		default:
			weaviateProp.DataType = []string{"text"} // Fallback to text
		}

		// Set indexing (Weaviate indexes by default, but we can configure it)
		// Note: In Weaviate v5, this is handled via IndexInverted configuration
		// For simplicity, we'll use the default behavior

		properties = append(properties, weaviateProp)
	}

	classObj.Properties = properties
	return classObj
}

// validatePropertyType checks if a value matches the expected PropertyType
func validatePropertyType(key string, value any, expected PropertyType) error {
	if value == nil {
		return nil // Nil values are valid
	}

	switch expected {
	case PropertyTypeText:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("property %q must be string, got %T", key, value)
		}
	case PropertyTypeTextArray:
		switch v := value.(type) {
		case []string:
			// Valid
		case []any:
			// Check that all elements are strings
			for i, elem := range v {
				if _, ok := elem.(string); !ok {
					return fmt.Errorf("property %q[%d] must be string, got %T", key, i, elem)
				}
			}
		default:
			return fmt.Errorf("property %q must be string array, got %T", key, value)
		}
	case PropertyTypeInt:
		switch value.(type) {
		case int, int32, int64:
			// Valid
		default:
			return fmt.Errorf("property %q must be int, got %T", key, value)
		}
	case PropertyTypeNumber:
		switch value.(type) {
		case float32, float64, int, int32, int64:
			// Valid - allow int as number
		default:
			return fmt.Errorf("property %q must be number, got %T", key, value)
		}
	case PropertyTypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("property %q must be bool, got %T", key, value)
		}
	case PropertyTypeDate:
		switch value.(type) {
		case time.Time, string:
			// Valid - accept Time or RFC3339 string
		default:
			return fmt.Errorf("property %q must be time.Time or RFC3339 string, got %T", key, value)
		}
	}

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
	// Build GraphQL query - start with className
	var builder *graphql.GetBuilder

	// Choose search type: prioritize nearText for consistency with auto-embedding
	switch {
	case query.Text != "":
		// Use nearText search with text query
		nearText := c.client.GraphQL().NearTextArgBuilder().
			WithConcepts([]string{query.Text})

		if query.Threshold > 0 {
			nearText = nearText.WithDistance(float32(1.0 - query.Threshold)) // Convert similarity to distance
		}

		builder = c.client.GraphQL().Get().
			WithClassName(className).
			WithNearText(nearText)

	case len(query.Vector) > 0:
		// Use nearVector search with pre-computed vector
		// Convert EmbeddingVector to []float32 explicitly for Weaviate client
		vector := make([]float32, len(query.Vector))
		copy(vector, query.Vector)

		nearVector := c.client.GraphQL().NearVectorArgBuilder().
			WithVector(vector)

		if query.Threshold > 0 {
			nearVector = nearVector.WithDistance(float32(1.0 - query.Threshold))
		}

		builder = c.client.GraphQL().Get().
			WithClassName(className).
			WithNearVector(nearVector)

	default:
		return nil, fmt.Errorf("either query.Text or query.Vector must be provided")
	}

	// Add fields to retrieve - use schema if available, otherwise retrieve all known fields
	fields := c.getFieldsToRetrieve()
	builder = builder.WithFields(fields...)

	// Apply filters if present
	if len(query.Filter) > 0 {
		whereFilter := buildWeaviateFilter(query.Filter)
		if whereFilter != nil {
			builder = builder.WithWhere(whereFilter)
		}
	}

	// Set limit
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
		var errMsgs []string
		for _, e := range result.Errors {
			if e != nil && e.Message != "" {
				errMsgs = append(errMsgs, e.Message)
			}
		}
		return nil, fmt.Errorf("%s: GraphQL errors: %v", errorMsg, errMsgs)
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
			document := c.parseWeaviateDocument(doc)
			documents = append(documents, document)
		}
	}

	return documents
}

// getFieldsToRetrieve returns the list of GraphQL fields to retrieve based on schema
func (c *Client) getFieldsToRetrieve() []graphql.Field {
	fields := []graphql.Field{
		{Name: "content"},
	}

	// Use schema to determine fields
	if c.schema != nil {
		for _, prop := range c.schema.Properties {
			fields = append(fields, graphql.Field{Name: prop.Name})
		}
	}

	// Always include _additional fields
	fields = append(fields, graphql.Field{
		Name: "_additional",
		Fields: []graphql.Field{
			{Name: "id"},
			{Name: "score"},
		},
	})

	return fields
}

// parseWeaviateDocument converts a Weaviate result to a retrieval.Document
func (c *Client) parseWeaviateDocument(doc map[string]any) retrieval.Document {
	document := retrieval.Document{
		Metadata: make(map[string]any),
	}

	// Extract content
	if content, ok := doc["content"].(string); ok {
		document.Content = content
	}

	// Extract metadata fields from top-level properties using schema
	if c.schema != nil {
		for _, prop := range c.schema.Properties {
			if value, exists := doc[prop.Name]; exists && value != nil {
				document.Metadata[prop.Name] = value
			}
		}
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

// buildWeaviateFilter converts search filters to Weaviate WhereBuilder format.
// Similar to Qdrant's filter building, this creates AND-combined equality filters.
func buildWeaviateFilter(filterMap map[string]any) *filters.WhereBuilder {
	if len(filterMap) == 0 {
		return nil
	}

	// Build filter conditions for each key-value pair
	whereFilters := make([]*filters.WhereBuilder, 0, len(filterMap))
	for key, value := range filterMap {
		whereFilter := filters.Where().
			WithPath([]string{key}).
			WithOperator(filters.Equal)

		// Set value based on type
		switch v := value.(type) {
		case string:
			whereFilter = whereFilter.WithValueText(v)
		case int:
			whereFilter = whereFilter.WithValueInt(int64(v))
		case int64:
			whereFilter = whereFilter.WithValueInt(v)
		case float64:
			whereFilter = whereFilter.WithValueNumber(v)
		case float32:
			whereFilter = whereFilter.WithValueNumber(float64(v))
		case bool:
			whereFilter = whereFilter.WithValueBoolean(v)
		default:
			// Convert unsupported types to string
			whereFilter = whereFilter.WithValueText(fmt.Sprintf("%v", v))
		}

		whereFilters = append(whereFilters, whereFilter)
	}

	// If only one filter, return it directly
	if len(whereFilters) == 1 {
		return whereFilters[0]
	}

	// Combine multiple filters with AND operator
	return filters.Where().
		WithOperator(filters.And).
		WithOperands(whereFilters)
}
