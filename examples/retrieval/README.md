# Retrieval Examples

This directory demonstrates vector search and RAG (Retrieval-Augmented Generation) patterns using the Calque retrieval package.

## What You'll Learn

### Basic Vector Search (`runBasicSearchExample`)

- **Core Concepts**: Simple semantic search with JSON results
- **Features Covered**:
  - Creating a vector store and loading documents
  - Configuring search options (threshold, limit)
  - Using `retrieval.VectorSearch()` middleware
  - Parsing SearchResult JSON output
  - Document scoring and ranking

### Search with Context Strategy (`runContextStrategyExample`)

- **Advanced Retrieval**: Strategy-based context building
- **Features Covered**:
  - Using `StrategyDiverse` for MMR-based diversity selection
  - Returning formatted context instead of JSON
  - Token-limited context building
  - Custom document separators
  - Balancing relevance and diversity in results

### RAG Pipeline (`runRAGExample`)

- **AI Integration**: Complete retrieval-augmented generation workflow
- **Features Covered**:
  - Combining vector search with AI generation
  - Building prompts with retrieved context
  - Using `StrategyRelevant` for score-based ranking
  - Timeout protection for AI calls
  - End-to-end question answering pipeline

## Key Retrieval Concepts

- **VectorStore Interface**: Abstraction for vector database operations (Search, Store, Delete)
- **SearchOptions**: Configure threshold, limit, filters, and context building behavior
- **Context Strategies**:
  - `StrategyRelevant`: Rank by similarity score (default)
  - `StrategyRecent`: Prioritize newest documents
  - `StrategyDiverse`: MMR algorithm for topic diversity
  - `StrategySummary`: Truncate long documents
- **Document Model**: Content, metadata, scores, and timestamps
- **Embedding Support**: Auto-embedding, external providers, or custom implementations

## Project Structure

```
retrieval/
├── main.go           # Example implementations
├── mock/
│   └── store.go      # In-memory vector store for demonstration
└── README.md
```

## Running the Examples

```bash
go run main.go
```

### Prerequisites for RAG Example

- Install [Ollama](https://ollama.ai)
- Pull the model: `ollama pull llama3.2:1b`
- Ensure Ollama is running: `ollama serve`

## Example Output

The examples demonstrate progressive retrieval complexity:

- **Basic Search**: Query execution, similarity scoring, JSON result parsing
- **Context Strategy**: Diverse document selection, formatted context output
- **RAG Pipeline**: Context retrieval, prompt construction, AI generation

Each example shows:
- Query logging and search configuration
- Vector search execution with different strategies
- Result processing (JSON or formatted context)
- Document scoring and selection
- AI augmentation (RAG example)

## Retrieval Best Practices

1. **Threshold Tuning**: Start with lower thresholds (0.2-0.3) and adjust based on result quality
2. **Limit Management**: Balance between context richness and token limits
3. **Strategy Selection**: Use `StrategyDiverse` for broad topics, `StrategyRelevant` for precise answers
4. **Token Budgeting**: Set `MaxTokens` to fit within AI context windows
5. **Metadata Filtering**: Use filters for category/topic-based narrowing
6. **Document Quality**: Ensure documents have meaningful content and metadata

## Production Considerations

The mock vector store uses simple keyword matching for demonstration. For production use:

- **Weaviate**: Auto-embedding with text2vec modules
- **Qdrant**: High-performance vector search with MMR support
- **PGVector**: PostgreSQL extension for vector similarity

## Next Steps

After understanding retrieval patterns, explore:
- **Memory Management**: Stateful conversations with retrieval context
- **Tool Calling**: AI-triggered document searches
- **Multi-stage Pipelines**: Chaining retrieval with other middleware
- **Advanced Strategies**: Custom embedding providers and reranking
