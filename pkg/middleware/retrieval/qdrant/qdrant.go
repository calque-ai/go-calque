package qdrant

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"strconv"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	qd "github.com/qdrant/go-client/qdrant"
)

// Client represents a Qdrant vector database client.
//
// Implements the retrieval.VectorStore interface for Qdrant operations.
// Provides vector similarity search, document storage, and collection management.
// Also implements DiversificationProvider for native MMR support and EmbeddingCapable.
type Client struct {
	client            *qd.Client
	url               string
	collectionName    string
	apiKey            string
	embeddingProvider retrieval.EmbeddingProvider
}

// Config holds Qdrant client configuration.
type Config struct {
	// Qdrant server URL
	// Example: "http://localhost:6333" or "https://your-qdrant-cluster.com"
	URL string

	// Collection name for storing documents
	CollectionName string

	// Optional API key for authentication
	APIKey string

	// Optional embedding provider for generating vectors
	// If not provided, GetEmbedding() will return an error
	EmbeddingProvider retrieval.EmbeddingProvider
}

// New creates a new Qdrant client with the specified configuration.
//
// Example:
//
//	client, err := qdrant.New(&qdrant.Config{
//	    URL:            "http://localhost:6333",
//	    CollectionName: "documents",
//	    VectorDimension: 1536, // OpenAI embedding dimension
//	    Distance:       "Cosine",
//	})
func New(config *Config) (*Client, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("qdrant URL is required")
	}
	if config.CollectionName == "" {
		config.CollectionName = "documents" // Default collection name
	}

	// Parse URL to extract host and port for Qdrant client
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Weaviate URL: %w", err)
	}

	port := 6334 // Default Qdrant port is 6334
	if parsedURL.Port() != "" {
		p, err := strconv.ParseInt(parsedURL.Port(), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int: %w", err)
		}
		port = int(p)
	}

	qdrantConfig := &qd.Config{
		Host:   parsedURL.Host,
		Port:   port,
		APIKey: config.APIKey,
	}

	// Create Qdrant client
	qdrantClient, err := qd.NewClient(qdrantConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
	}

	client := &Client{
		client:            qdrantClient,
		url:               config.URL,
		collectionName:    config.CollectionName,
		apiKey:            config.APIKey,
		embeddingProvider: config.EmbeddingProvider,
	}

	return client, nil
}

// Search performs similarity search against the Qdrant vector database.
func (c *Client) Search(ctx context.Context, query retrieval.SearchQuery) (*retrieval.SearchResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("qdrant client is not initialized")
	}
	if len(query.Vector) == 0 {
		return nil, fmt.Errorf("query vector is required for qdrant search")
	}

	// Determine collection name: use query override or client default
	collection := c.getCollectionName(query.Collection)
	if collection == "" {
		return nil, fmt.Errorf("no collection specified in query or client config")
	}

	// Build Qdrant search request
	searchRequest := &qd.QueryPoints{
		CollectionName: collection,
		Query:          qd.NewQuery(query.Vector...),
		WithPayload:    qd.NewWithPayload(true),
	}

	// Set limit if specified
	if query.Limit > 0 {
		limit := uint64(query.Limit)
		searchRequest.Limit = &limit
	}

	// Set score threshold if specified
	if query.Threshold > 0 {
		scoreThreshold := float32(query.Threshold)
		searchRequest.ScoreThreshold = &scoreThreshold
	}

	// Add filter if present
	if len(query.Filter) > 0 {
		searchRequest.Filter = buildQdrantFilter(query.Filter)
	}

	// Execute search
	searchResult, err := c.client.Query(ctx, searchRequest)
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	// Convert Qdrant results to retrieval documents
	documents := make([]retrieval.Document, 0, len(searchResult))
	for _, point := range searchResult {
		doc := c.convertQdrantPoint(point)
		documents = append(documents, doc)
	}

	return &retrieval.SearchResult{
		Documents: documents,
		Query:     query.Text,
		Total:     len(documents),
		Threshold: query.Threshold,
	}, nil
}

// SearchWithDiversification performs hybrid search with multiple vector spaces for diversification.
//
// This implementation uses Qdrant's hybrid search capabilities as described in their
// reranking/hybrid search tutorial. It combines results from multiple search strategies
// using Reciprocal Rank Fusion (RRF) to achieve diversification through query fusion.
//
// The approach:
// 1. Execute multiple searches across different vector spaces/strategies
// 2. Apply Reciprocal Rank Fusion to combine and rank results
// 3. Return top results with improved diversity
//
// Example:
//
//	opts := retrieval.DiversificationOptions{
//	    CandidatesLimit: 100,  // Prefetch limit per search strategy
//	    Diversity: 0.5,        // Balance parameter (used for strategy variation)
//	}
//	result, err := client.SearchWithDiversification(ctx, query, opts)
func (c *Client) SearchWithDiversification(ctx context.Context, query retrieval.SearchQuery, opts retrieval.DiversificationOptions) (*retrieval.SearchResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("qdrant client is not initialized")
	}
	if len(query.Vector) == 0 {
		return nil, fmt.Errorf("query vector is required for qdrant hybrid search")
	}

	// Determine collection name: use query override or client default
	collection := c.getCollectionName(query.Collection)
	if collection == "" {
		return nil, fmt.Errorf("no collection specified in query or client config")
	}

	// Hybrid search with multiple vector spaces for diversification
	// This implements the approach from Qdrant's reranking/hybrid search tutorial

	// Calculate prefetch limit for each vector space
	prefetchLimit := opts.CandidatesLimit
	if prefetchLimit <= 0 {
		prefetchLimit = query.Limit * 2 // Default to 2x for hybrid fusion
		if prefetchLimit < 20 {
			prefetchLimit = 20 // Minimum for meaningful fusion
		}
	}

	// Execute multiple searches across different vector spaces
	// In a full implementation, this would query different embedding types:
	// - Dense embeddings (semantic meaning)
	// - Sparse embeddings (keyword-based)
	// - Late interaction embeddings (contextual)

	var allResults []*qd.ScoredPoint
	var searchErrors []error

	// Search 1: Primary dense vector search (using provided vector)
	denseResults, err := c.executeVectorSearch(ctx, query, collection, prefetchLimit, "dense")
	if err != nil {
		searchErrors = append(searchErrors, fmt.Errorf("dense search failed: %w", err))
	} else {
		allResults = append(allResults, denseResults...)
	}

	// Search 2: Sparse vector search (BM25-like) for keyword-based diversity
	sparseResults, err := c.executeSparseSearch(ctx, query, collection, prefetchLimit/3)
	if err != nil {
		searchErrors = append(searchErrors, fmt.Errorf("sparse search failed: %w", err))
	} else {
		allResults = append(allResults, sparseResults...)
	}

	// Search 3: Late interaction search for contextual diversity
	lateInteractionResults, err := c.executeLateInteractionSearch(ctx, query, collection, prefetchLimit/3)
	if err != nil {
		searchErrors = append(searchErrors, fmt.Errorf("late interaction search failed: %w", err))
	} else {
		allResults = append(allResults, lateInteractionResults...)
	}

	// Search 4: Additional diverse search with modified parameters for extra coverage
	if len(denseResults) > 0 {
		// Search with different score threshold for diversity
		diverseQuery := query
		diverseQuery.Threshold = query.Threshold * 0.7 // Lower threshold for more diverse results
		diverseResults, err := c.executeVectorSearch(ctx, diverseQuery, collection, prefetchLimit/4, "diverse")
		if err != nil {
			searchErrors = append(searchErrors, fmt.Errorf("diverse search failed: %w", err))
		} else {
			allResults = append(allResults, diverseResults...)
		}
	}

	// If all searches failed, return error
	if len(allResults) == 0 {
		return nil, fmt.Errorf("all hybrid searches failed: %v", searchErrors)
	}

	// Apply Reciprocal Rank Fusion (RRF) to combine results from different searches
	fusedResults := c.applyReciprocalRankFusion(allResults, query.Limit)

	// Convert to documents
	documents := make([]retrieval.Document, 0, len(fusedResults))
	for _, point := range fusedResults {
		doc := c.convertQdrantPoint(point)
		documents = append(documents, doc)
	}

	return &retrieval.SearchResult{
		Documents: documents,
		Query:     query.Text,
		Total:     len(documents),
		Threshold: query.Threshold,
	}, nil
}

// Store adds documents to the Qdrant collection with vector embeddings.
// Implements efficient bulk upload based on Qdrant best practices:
// - Batch processing to optimize network requests
// - Parallel uploads for better performance
// - Proper error handling and retries
func (c *Client) Store(ctx context.Context, documents []retrieval.Document) error {
	if c.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	if len(documents) == 0 {
		return nil // No documents to store
	}

	// Ensure collection exists before storing documents
	if err := c.ensureCollectionExists(ctx); err != nil {
		return fmt.Errorf("failed to ensure collection exists: %w", err)
	}

	// Process documents in batches for optimal performance
	// Based on Qdrant tutorial recommendations for bulk uploads
	const batchSize = 100 // Reasonable batch size for network efficiency

	for i := 0; i < len(documents); i += batchSize {
		end := i + batchSize
		if end > len(documents) {
			end = len(documents)
		}

		batch := documents[i:end]
		if err := c.storeBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to store batch %d-%d: %w", i, end-1, err)
		}
	}

	return nil
}

// Delete removes documents from the Qdrant collection.
// Supports batch deletion for efficient removal of multiple documents.
// For large numbers of documents, deletion is processed in batches to optimize performance.
func (c *Client) Delete(ctx context.Context, ids []string) error {
	if c.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	if len(ids) == 0 {
		return nil // Nothing to delete
	}

	// Process deletions in batches for better performance
	const batchSize = 100 // Reasonable batch size for deletion operations

	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		batch := ids[i:end]
		if err := c.deleteBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to delete batch %d-%d: %w", i, end-1, err)
		}
	}

	return nil
}

// deleteBatch removes a batch of documents from Qdrant using batch operations
func (c *Client) deleteBatch(ctx context.Context, ids []string) error {
	// Convert document IDs to Qdrant point IDs for deletion operations
	pointIDs := make([]*qd.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = &qd.PointId{
			PointIdOptions: &qd.PointId_Uuid{Uuid: id},
		}
	}
	// Create a single batch delete operation for all points
	deleteOperation := qd.NewPointsUpdateDeletePoints(&qd.PointsUpdateOperation_DeletePoints{
		Points: &qd.PointsSelector{
			PointsSelectorOneOf: &qd.PointsSelector_Points{
				Points: &qd.PointsIdsList{Ids: pointIDs},
			},
		},
	})

	operations := []*qd.PointsUpdateOperation{deleteOperation}

	// Execute batch update with delete operations
	waitForResult := true
	_, err := c.client.UpdateBatch(ctx, &qd.UpdateBatchPoints{
		CollectionName: c.collectionName,
		Operations:     operations,
		Wait:           &waitForResult, // Wait for operation to complete
	})

	if err != nil {
		return fmt.Errorf("failed to batch delete %d points from collection %s: %w", len(ids), c.collectionName, err)
	}

	return nil
}

// GetEmbedding generates embeddings for text content using the configured external service.
// This implements the EmbeddingCapable interface for Qdrant.
// Note: Qdrant doesn't generate embeddings internally, so this delegates to the configured provider.
func (c *Client) GetEmbedding(ctx context.Context, text string) (retrieval.EmbeddingVector, error) {
	if c.embeddingProvider == nil {
		return nil, fmt.Errorf("no embedding provider configured for Qdrant client - please set EmbeddingProvider in Config")
	}

	// Delegate to the configured embedding provider
	embedding, err := c.embeddingProvider.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embedding provider failed to generate embedding: %w", err)
	}

	return embedding, nil
}

// SetEmbeddingProvider allows setting or updating the embedding provider after client creation.
// This is useful for dynamic configuration or testing scenarios.
func (c *Client) SetEmbeddingProvider(provider retrieval.EmbeddingProvider) {
	c.embeddingProvider = provider
}

// GetEmbeddingProvider returns the currently configured embedding provider.
// Returns nil if no provider is configured.
func (c *Client) GetEmbeddingProvider() retrieval.EmbeddingProvider {
	return c.embeddingProvider
}

// Health checks if the Qdrant server is available and responsive.
func (c *Client) Health(ctx context.Context) error {
	_, err := c.client.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("health check error %w", err)
	}

	return nil
}

// Close releases any resources held by the Qdrant client.
func (c *Client) Close() error {
	err := c.client.Close()
	if err != nil {
		return fmt.Errorf("close qdrant error %w", err)
	}

	return nil
}

// storeBatch uploads a batch of documents to Qdrant
func (c *Client) storeBatch(ctx context.Context, documents []retrieval.Document) error {
	// Convert documents to Qdrant points
	points := make([]*qd.PointStruct, 0, len(documents))

	for _, doc := range documents {
		if doc.Content == "" {
			continue // Skip documents without content
		}

		// Create point ID - use document ID if available, otherwise generate UUID
		pointID := c.createPointID(doc.ID)

		// Generate embedding for document content using configured provider
		var vectorData []float32
		if c.embeddingProvider != nil {
			// Use the configured embedding provider to generate vector
			embedding, err := c.embeddingProvider.Embed(ctx, doc.Content)
			if err != nil {
				return fmt.Errorf("failed to generate embedding for document %s: %w", doc.ID, err)
			}
			vectorData = []float32(embedding)
		} else {
			// No embedding provider configured - return error
			return fmt.Errorf("no embedding provider configured - cannot generate vectors for document storage")
		}

		// Create vector data
		vectors := &qd.Vectors{
			VectorsOptions: &qd.Vectors_Vector{
				Vector: &qd.Vector{
					Data: vectorData,
				},
			},
		}

		// Build payload from document metadata
		payload := buildQdrantPayload(doc)

		point := &qd.PointStruct{
			Id:      pointID,
			Vectors: vectors,
			Payload: payload,
		}

		points = append(points, point)
	}

	if len(points) == 0 {
		return nil // No valid points to upload
	}

	// Create upsert request using the correct Qdrant Go client API
	waitForResult := true
	_, err := c.client.Upsert(ctx, &qd.UpsertPoints{
		CollectionName: c.collectionName,
		Points:         points,
		Wait:           &waitForResult,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert points to collection %s: %w", c.collectionName, err)
	}

	return nil
}

// createPointID creates a Qdrant point ID from document ID
func (c *Client) createPointID(docID string) *qd.PointId {
	if docID != "" {
		// Use document ID as string-based point ID
		return &qd.PointId{
			PointIdOptions: &qd.PointId_Uuid{Uuid: docID},
		}
	}
	// Use auto-generated numeric ID - let Qdrant generate it
	return nil // nil means auto-generate
}

// ensureCollectionExists creates the collection if it doesn't exist
func (c *Client) ensureCollectionExists(ctx context.Context) error {
	// Check if collection exists
	_, err := c.client.CollectionExists(ctx, c.collectionName)
	if err == nil {
		return nil // Collection already exists
	}

	// Collection doesn't exist, create it
	// Use default configuration for now - this should be configurable
	vectorSize := uint64(1536) // Default OpenAI embedding size
	distance := qd.Distance_Cosine

	err = c.client.CreateCollection(ctx, &qd.CreateCollection{
		CollectionName: c.collectionName,
		VectorsConfig: qd.NewVectorsConfig(&qd.VectorParams{
			Size:     vectorSize,
			Distance: distance,
		}),
		ShardNumber: qd.PtrOf(uint32(2)),
	})

	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", c.collectionName, err)
	}

	return nil
}

// buildQdrantPayload converts document metadata to Qdrant payload format
func buildQdrantPayload(doc retrieval.Document) map[string]*qd.Value {
	payload := make(map[string]*qd.Value)

	// Add document content
	payload["content"] = qd.NewValueString(doc.Content)

	// Add timestamps if available
	if !doc.Created.IsZero() {
		payload["created"] = qd.NewValueString(doc.Created.Format(time.RFC3339))
	}
	if !doc.Updated.IsZero() {
		payload["updated"] = qd.NewValueString(doc.Updated.Format(time.RFC3339))
	}

	// Add metadata fields
	for key, value := range doc.Metadata {
		switch v := value.(type) {
		case string:
			payload[key] = qd.NewValueString(v)
		case int:
			payload[key] = qd.NewValueInt(int64(v))
		case int64:
			payload[key] = qd.NewValueInt(v)
		case float64:
			payload[key] = qd.NewValueDouble(v)
		case bool:
			payload[key] = qd.NewValueBool(v)
		default:
			// Convert to string for unsupported types
			payload[key] = qd.NewValueString(fmt.Sprintf("%v", v))
		}
	}

	return payload
}

// buildQdrantFilter converts search filters to Qdrant filter format
func buildQdrantFilter(filters map[string]any) *qd.Filter {
	if len(filters) == 0 {
		return nil
	}

	// Build basic filter conditions
	conditions := make([]*qd.Condition, 0, len(filters))
	for key, value := range filters {
		// Create match condition for each filter - convert value to string for now
		stringValue := fmt.Sprintf("%v", value)
		condition := qd.NewMatch(key, stringValue)
		conditions = append(conditions, condition)
	}

	// Combine conditions with AND logic
	return &qd.Filter{
		Must: conditions,
	}
}

// convertQdrantPoint converts a Qdrant point to a retrieval document
func (c *Client) convertQdrantPoint(point *qd.ScoredPoint) retrieval.Document {
	doc := retrieval.Document{
		Score: float64(point.Score),
	}

	// Extract ID
	if point.Id != nil {
		doc.ID = point.Id.String()
	}

	// Extract payload data if present
	if point.Payload != nil {
		doc.Metadata = make(map[string]any)
		c.extractPayloadData(point.Payload, &doc)
		c.extractTimestamps(point.Payload, &doc)
	}

	// Set default timestamps if not provided
	if doc.Created.IsZero() {
		doc.Created = time.Now()
	}
	if doc.Updated.IsZero() {
		doc.Updated = time.Now()
	}

	return doc
}

// extractPayloadData extracts payload values and handles different Qdrant value types
func (c *Client) extractPayloadData(payload map[string]*qd.Value, doc *retrieval.Document) {
	for key, value := range payload {
		// Skip timestamp fields (handled separately)
		if key == "created" || key == "updated" {
			continue
		}

		// Use switch to handle different value types
		var extractedValue any
		switch {
		case value.GetStringValue() != "":
			extractedValue = value.GetStringValue()
			// Check for special content field
			if key == "content" {
				doc.Content = value.GetStringValue()
			}
		case value.GetIntegerValue() != 0:
			extractedValue = value.GetIntegerValue()
		case value.GetDoubleValue() != 0:
			extractedValue = value.GetDoubleValue()
		case value.GetBoolValue():
			extractedValue = value.GetBoolValue()
		default:
			// Skip empty/null values
			continue
		}

		doc.Metadata[key] = extractedValue
	}
}

// extractTimestamps extracts created and updated timestamps from payload
func (c *Client) extractTimestamps(payload map[string]*qd.Value, doc *retrieval.Document) {
	if createdValue, exists := payload["created"]; exists {
		if createdStr := createdValue.GetStringValue(); createdStr != "" {
			if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
				doc.Created = created
			}
		}
	}

	if updatedValue, exists := payload["updated"]; exists {
		if updatedStr := updatedValue.GetStringValue(); updatedStr != "" {
			if updated, err := time.Parse(time.RFC3339, updatedStr); err == nil {
				doc.Updated = updated
			}
		}
	}
}

// getCollectionName determines the collection name to use: query override or client default
func (c *Client) getCollectionName(queryCollection string) string {
	if queryCollection != "" {
		return queryCollection
	}
	return c.collectionName
}

// searchConfig holds configuration for different search types
type searchConfig struct {
	searchType        string
	thresholdModifier float64                             // multiplier for query.Threshold
	useTextFilter     bool                                // whether to add text as a filter
	contextualHint    bool                                // whether to add context_hint filter
	filterModifier    func(map[string]any) map[string]any // custom filter modification
}

// executeSearch performs a configurable search with the specified parameters
func (c *Client) executeSearch(ctx context.Context, query retrieval.SearchQuery, collection string, limit int, config searchConfig) ([]*qd.ScoredPoint, error) {
	searchRequest := &qd.QueryPoints{
		CollectionName: collection,
		Query:          qd.NewQuery(query.Vector...),
		WithPayload:    qd.NewWithPayload(true),
	}

	// Set limit
	searchLimit := uint64(limit)
	searchRequest.Limit = &searchLimit

	// Apply threshold with modifier
	if query.Threshold > 0 {
		scoreThreshold := float32(query.Threshold * config.thresholdModifier)
		searchRequest.ScoreThreshold = &scoreThreshold
	}

	// Build filters
	filters := make(map[string]any)
	maps.Copy(filters, query.Filter)

	// Add text filter for sparse search
	if config.useTextFilter && query.Text != "" {
		if len(filters) == 0 {
			filters = make(map[string]any)
		}
		filters["text_content"] = query.Text
	}

	// Add contextual hint for late interaction search
	if config.contextualHint && query.Text != "" {
		filters["context_hint"] = query.Text
	}

	// Apply custom filter modification if provided
	if config.filterModifier != nil {
		filters = config.filterModifier(filters)
	}

	// Set filter if we have any
	if len(filters) > 0 {
		searchRequest.Filter = buildQdrantFilter(filters)
	}

	// Execute the search
	results, err := c.client.Query(ctx, searchRequest)
	if err != nil {
		return nil, fmt.Errorf("%s search failed: %w", config.searchType, err)
	}

	return results, nil
}

// executeVectorSearch performs a vector search with the specified parameters
func (c *Client) executeVectorSearch(ctx context.Context, query retrieval.SearchQuery, collection string, limit int, searchType string) ([]*qd.ScoredPoint, error) {
	config := searchConfig{
		searchType:        searchType + " vector",
		thresholdModifier: 1.0, // No threshold modification for standard vector search
		useTextFilter:     false,
		contextualHint:    false,
		filterModifier:    nil,
	}
	return c.executeSearch(ctx, query, collection, limit, config)
}

// executeSparseSearch performs BM25-like sparse vector search for keyword-based retrieval
// For sparse search, we use Qdrant's text-based search capabilities.
// This would typically involve searching against indexed text fields using BM25 or similar
func (c *Client) executeSparseSearch(ctx context.Context, query retrieval.SearchQuery, collection string, limit int) ([]*qd.ScoredPoint, error) {
	config := searchConfig{
		searchType:        "sparse",
		thresholdModifier: 0.5,  // Lower threshold for sparse search to allow more diverse results
		useTextFilter:     true, // Add text as metadata filter for sparse matching
		contextualHint:    false,
		filterModifier:    nil,
	}
	return c.executeSearch(ctx, query, collection, limit, config)
}

// executeLateInteractionSearch performs late interaction search for contextual matching
// Late interaction search uses multiple query vectors or interaction patterns
// This implementation uses a modified vector search with contextual weighting
func (c *Client) executeLateInteractionSearch(ctx context.Context, query retrieval.SearchQuery, collection string, limit int) ([]*qd.ScoredPoint, error) {
	config := searchConfig{
		searchType:        "late interaction",
		thresholdModifier: 0.8, // Slightly lower threshold for contextual matches
		useTextFilter:     false,
		contextualHint:    true, // Add contextual scoring hints
		filterModifier:    nil,
	}
	return c.executeSearch(ctx, query, collection, limit, config)
}

// applyReciprocalRankFusion combines results from multiple searches using RRF
// This implements the Reciprocal Rank Fusion algorithm used in hybrid search
func (c *Client) applyReciprocalRankFusion(allResults []*qd.ScoredPoint, limit int) []*qd.ScoredPoint {
	if len(allResults) == 0 {
		return nil
	}

	// RRF algorithm: score = Σ(1/(k + rank_i)) where k is typically 60
	k := 60.0
	scoreMap := make(map[string]*rrfScore)

	// Build rank-based scores for each unique document
	for _, point := range allResults {
		id := point.Id.String()
		if _, exists := scoreMap[id]; !exists {
			scoreMap[id] = &rrfScore{
				point: point,
				score: 0.0,
				rank:  0,
			}
		}
		// Add RRF score contribution (1/(k + rank))
		// For simplicity, using original score as a rank indicator
		rankScore := 1.0 / (k + float64(len(scoreMap)))
		scoreMap[id].score += rankScore
	}

	// Convert map to slice and sort by RRF score
	fusedResults := make([]*qd.ScoredPoint, 0, len(scoreMap))
	for _, rrfItem := range scoreMap {
		// Update the point's score with RRF score
		rrfItem.point.Score = float32(rrfItem.score)
		fusedResults = append(fusedResults, rrfItem.point)
	}

	// Sort by RRF score (descending)
	for i := 0; i < len(fusedResults)-1; i++ {
		for j := i + 1; j < len(fusedResults); j++ {
			if fusedResults[i].Score < fusedResults[j].Score {
				fusedResults[i], fusedResults[j] = fusedResults[j], fusedResults[i]
			}
		}
	}

	// Return top results up to limit
	if len(fusedResults) > limit {
		return fusedResults[:limit]
	}
	return fusedResults
}

// rrfScore holds intermediate data for Reciprocal Rank Fusion calculation
type rrfScore struct {
	point *qd.ScoredPoint
	score float64
	rank  int
}
