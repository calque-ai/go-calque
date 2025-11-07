package pgvector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

// Client represents a PostgreSQL + pgvector database client using pgx.
//
// Implements the retrieval.VectorStore interface for pgvector operations.
// Provides vector similarity search using PostgreSQL with pgvector extension.
type Client struct {
	conn              *pgxpool.Pool
	tableName         string
	vectorDimension   int
	embeddingProvider retrieval.EmbeddingProvider
	schemaEnsured     bool // Track if table exists/was created
}

// Config holds pgvector client configuration.
type Config struct {
	// Database connection string (PostgreSQL format)
	// Example: "postgres://user:password@localhost/dbname?sslmode=disable"
	ConnectionString string

	// Table name for storing documents and vectors
	TableName string

	// Vector dimension (must match embedding model output)
	VectorDimension int

	// Optional embedding provider for generating vectors
	EmbeddingProvider retrieval.EmbeddingProvider
}

// New creates a new pgvector client with the specified configuration.
//
// Checks that pgvector extension is installed but does not create tables.
// Tables are created lazily when Store() is first called if needed.
//
// Example:
//
//	client, err := pgvector.New(&pgvector.Config{
//	    ConnectionString:  "postgres://user:pass@localhost/vectordb",
//	    TableName:         "documents",
//	    VectorDimension:   1536,
//	    EmbeddingProvider: openaiProvider,
//	})
func New(config *Config) (*Client, error) {
	if config.ConnectionString == "" {
		return nil, fmt.Errorf("PostgreSQL connection string is required")
	}
	if config.TableName == "" {
		config.TableName = "documents" // Default table name
	}
	if config.VectorDimension <= 0 {
		config.VectorDimension = 1536 // Default OpenAI embedding dimension
	}

	// Parse pgxpool config
	poolConfig, err := pgxpool.ParseConfig(config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Register pgvector types for each connection
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Check that pgvector extension is installed (fail fast)
	var extExists bool
	err = pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')",
	).Scan(&extExists)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to check pgvector extension: %w", err)
	}
	if !extExists {
		pool.Close()
		return nil, fmt.Errorf("pgvector extension not installed - run: CREATE EXTENSION vector")
	}

	client := &Client{
		conn:              pool,
		tableName:         config.TableName,
		vectorDimension:   config.VectorDimension,
		embeddingProvider: config.EmbeddingProvider,
		schemaEnsured:     false,
	}

	return client, nil
}

// Search performs similarity search using pgvector cosine similarity.
func (c *Client) Search(ctx context.Context, query retrieval.SearchQuery) (*retrieval.SearchResult, error) {
	if len(query.Vector) == 0 {
		return nil, fmt.Errorf("query.Vector is required for pgvector search")
	}

	// Build SQL query with cosine similarity
	// Use <=> for cosine distance, convert to similarity with 1 - distance
	querySQL := fmt.Sprintf(`
		SELECT id, content, metadata, created_at, updated_at,
		       1 - (embedding <=> $1) AS similarity
		FROM %s
		WHERE 1 - (embedding <=> $1) > $2
		ORDER BY embedding <=> $1
		LIMIT $3`,
		c.tableName)

	// Execute query with pgvector types
	rows, err := c.conn.Query(ctx, querySQL,
		pgvector.NewVector(query.Vector), // $1
		query.Threshold,                   // $2
		query.Limit,                       // $3
	)
	if err != nil {
		return nil, fmt.Errorf("pgvector search failed: %w", err)
	}
	defer rows.Close()

	// Scan results into documents
	documents := make([]retrieval.Document, 0, query.Limit)
	for rows.Next() {
		var doc retrieval.Document
		var metadataJSON []byte
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&doc.ID,
			&doc.Content,
			&metadataJSON,
			&createdAt,
			&updatedAt,
			&doc.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse JSONB metadata
		if len(metadataJSON) > 0 {
			var metadata map[string]any
			if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
				return nil, fmt.Errorf("failed to parse metadata: %w", err)
			}
			doc.Metadata = metadata
		}

		doc.Created = createdAt
		doc.Updated = updatedAt

		documents = append(documents, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &retrieval.SearchResult{
		Documents: documents,
		Query:     query.Text,
		Total:     len(documents),
		Threshold: query.Threshold,
	}, nil
}

// Store adds documents to the PostgreSQL table with vector embeddings.
func (c *Client) Store(ctx context.Context, documents []retrieval.Document) error {
	if len(documents) == 0 {
		return nil // Nothing to store
	}

	// Lazy table creation - ensures table exists before storing
	if err := c.ensureTableExists(ctx); err != nil {
		return err
	}

	// Check embedding provider is configured
	if c.embeddingProvider == nil {
		return fmt.Errorf("no embedding provider configured - cannot generate vectors for document storage")
	}

	// Use batch for efficient bulk inserts
	batch := &pgx.Batch{}

	for _, doc := range documents {
		if doc.Content == "" {
			continue // Skip documents without content
		}

		// Generate embedding for document content
		embedding, err := c.embeddingProvider.Embed(ctx, doc.Content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for document %s: %w", doc.ID, err)
		}

		// Marshal metadata to JSONB
		var metadataJSON []byte
		if doc.Metadata != nil {
			metadataJSON, err = json.Marshal(doc.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata for document %s: %w", doc.ID, err)
			}
		}

		// Set timestamps if not provided
		if doc.Created.IsZero() {
			doc.Created = time.Now()
		}
		if doc.Updated.IsZero() {
			doc.Updated = time.Now()
		}

		// Upsert query - insert or update on conflict
		upsertSQL := fmt.Sprintf(`
			INSERT INTO %s (id, content, metadata, embedding, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				content = EXCLUDED.content,
				metadata = EXCLUDED.metadata,
				embedding = EXCLUDED.embedding,
				updated_at = EXCLUDED.updated_at`,
			c.tableName)

		batch.Queue(upsertSQL,
			doc.ID,
			doc.Content,
			metadataJSON,
			pgvector.NewVector(embedding),
			doc.Created,
			doc.Updated,
		)
	}

	// Execute batch
	results := c.conn.SendBatch(ctx, batch)
	defer results.Close()

	// Check results for errors
	for i := 0; i < batch.Len(); i++ {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("failed to store document %d: %w", i, err)
		}
	}

	return nil
}

// Delete removes documents from the PostgreSQL table.
func (c *Client) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil // Nothing to delete
	}

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE id = ANY($1)", c.tableName)
	_, err := c.conn.Exec(ctx, deleteSQL, ids)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	return nil
}

// GetEmbedding generates embeddings for text content using the configured external service.
// This implements the EmbeddingCapable interface for PGVector.
// Note: PGVector doesn't generate embeddings internally, so this delegates to the configured provider.
func (c *Client) GetEmbedding(ctx context.Context, text string) (retrieval.EmbeddingVector, error) {
	if c.embeddingProvider == nil {
		return nil, fmt.Errorf("no embedding provider configured - please set EmbeddingProvider in Config")
	}

	return c.embeddingProvider.Embed(ctx, text)
}

// SetEmbeddingProvider allows setting or updating the embedding provider after client creation.
func (c *Client) SetEmbeddingProvider(provider retrieval.EmbeddingProvider) {
	c.embeddingProvider = provider
}

// GetEmbeddingProvider returns the currently configured embedding provider.
func (c *Client) GetEmbeddingProvider() retrieval.EmbeddingProvider {
	return c.embeddingProvider
}

// Health checks if the PostgreSQL database is available and pgvector extension is loaded.
func (c *Client) Health(ctx context.Context) error {
	// Test database connectivity
	var result int
	err := c.conn.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database connectivity check failed: %w", err)
	}

	// Verify pgvector extension exists
	var extExists bool
	err = c.conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')",
	).Scan(&extExists)
	if err != nil {
		return fmt.Errorf("extension check failed: %w", err)
	}
	if !extExists {
		return fmt.Errorf("pgvector extension not installed - run: CREATE EXTENSION vector")
	}

	return nil
}

// Close closes the pgx connection pool.
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	return nil
}

// ensureTableExists checks if table exists and creates it if needed.
// Called lazily from Store() to support both read-only and write use cases.
func (c *Client) ensureTableExists(ctx context.Context) error {
	if c.schemaEnsured {
		return nil // Already checked/created
	}

	// Check if table exists
	var exists bool
	err := c.conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
		c.tableName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if exists {
		c.schemaEnsured = true
		return nil // Table exists
	}

	// Table doesn't exist - create it
	if c.vectorDimension <= 0 {
		return fmt.Errorf("vectorDimension must be set to create table %s", c.tableName)
	}

	// Create table with vector column
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			metadata JSONB,
			embedding vector(%d),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`, c.tableName, c.vectorDimension)

	_, err = c.conn.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", c.tableName, err)
	}

	// Create IVFFlat index for cosine similarity
	createIndexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_embedding_idx
		ON %s
		USING ivfflat (embedding vector_cosine_ops)
		WITH (lists = 100)`,
		c.tableName, c.tableName)

	_, err = c.conn.Exec(ctx, createIndexSQL)
	if err != nil {
		return fmt.Errorf("failed to create vector index: %w", err)
	}

	c.schemaEnsured = true
	return nil
}
