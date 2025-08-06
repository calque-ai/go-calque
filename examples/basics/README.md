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

### Streaming vs Buffered Pipeline (`runStreamingExample`)

- **Streaming Architecture**: Demonstrates the difference between streaming and buffered middleware
- **Features Covered**:
  - **Pure Streaming Pipeline**: TeeReader, LineProcessor, Timeout with streaming handlers
  - **Streaming vs Buffered Comparison**: Side-by-side demonstration using Parallel to split input
  - Memory-efficient line-by-line processing vs full-input buffering
  - Multiple destination teeing and timeout protection
  - Real-time data flow vs sequential batch processing

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
- **Streaming vs Buffered Processing**: 
  - **Streaming**: Processes data as it flows (LineProcessor, TeeReader, PassThrough) - memory efficient
  - **Buffered**: Reads entire input before processing (Transform, Chain, Branch) - enables complex analysis
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

- **Text-Only**: Input logging, preprocessing, conditional routing, and transformations
- **Streaming**: Real-time line-by-line processing, multi-destination teeing, and streaming vs buffered comparisons
- **AI-Powered**: Prompt construction, LLM execution, response analysis, and post-processing

## Next Steps

After understanding these basics, explore other examples in the parent directory for more advanced features like memory management, tool calling, and structured data processing.
