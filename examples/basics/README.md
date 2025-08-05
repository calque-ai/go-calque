# Basic Examples

This directory contains introductory examples that demonstrate the core concepts of the Calque-Pipe AI Agent Framework.

## What You'll Learn

### Text-Only Pipeline (`runTextOnlyExample`)

- **Core Concepts**: Pipes, handlers, and middleware flow
- **Features Covered**:
  - Creating pipelines with `core.New()`
  - Adding handlers with `pipe.Use()`
  - Text transformations
  - Conditional branching
  - Logging and monitoring
  - Pipeline execution

### AI-Powered Pipeline (`runAIExample`)

- **LLM Integration**: How to connect AI providers to processing pipelines
- **Features Covered**:
  - AI provider setup (Ollama)
  - Prompt template construction
  - AI chat requests with timeouts
  - Response processing and analysis
  - Error handling

## Key Framework Concepts

- **Middleware Pattern**: Similar to HTTP middleware, each `Use()` call adds a handler to the processing flow
- **Streaming Architecture**: Data flows through handlers in sequence, enabling real-time processing
- **Concurrent Processing**: Handlers start processing as soon as they receive data - they don't wait for the previous handler to complete. This creates a pipeline where multiple handlers can be working simultaneously on different parts of the data stream
- **Parallel Execution**: The framework processes handlers in parallel by default for optimal performance, maximizing throughput and minimizing latency
- **Composability**: Build complex AI agents by combining simple, reusable middleware components

## Running the Examples

```bash
go run main.go
```

### Prerequisites for AI Example

- Install [Ollama](https://ollama.ai)
- Pull the model: `ollama pull llama3.2:1b`
- Ensure Ollama is running on `localhost:11434`

## Example Output

The examples demonstrate how text gets "calqued" (copied and transformed) through the processing pipeline, showing:

- Input logging and preprocessing
- Conditional routing based on content
- AI prompt construction and execution
- Response analysis and post-processing
- Final result output

## Next Steps

After understanding these basics, explore other examples in the parent directory for more advanced features like memory management, tool calling, and structured data processing.
