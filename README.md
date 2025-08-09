# Calque-Pipe

A **streaming middleware framework** for building AI agents and data processing pipelines in Go.

## What is Calque-Pipe?

Calque-Pipe brings HTTP middleware patterns to AI and data processing. Instead of handling HTTP requests, you compose pipelines where each middleware processes streaming data through `io.Pipe` connections.

**Key benefits:**
- **Streaming first**: Process data as it flows, not after it's fully loaded
- **True concurrency**: Each middleware runs in its own goroutine  
- **Memory efficient**: Constant memory usage regardless of input size
- **Real-time processing**: Responses begin immediately, no waiting for full datasets
- **Composable**: Chain middleware just like HTTP handlers

```go
flow := calque.Flow().
    Use(logger.Print("INPUT")).
    Use(ai.Agent(geminiClient)).
    Use(logger.Head("RESPONSE", 200))

var result string
err := flow.Run(ctx, "What's the capital of France?", &result)
```

Built on Go's `io.Reader`/`io.Writer` interfaces, every middleware runs concurrently through `io.Pipe` connections for maximum performance.

## How It Works

**The Pattern**: Chain middleware like HTTP handlers, but for streaming data:

```go
flow := calque.Flow().
    Use(middleware1).
    Use(middleware2).
    Use(middleware3)

err := flow.Run(ctx, input, &output)
```

**The Architecture**: Each middleware runs concurrently, connected by `io.Pipe`:

```
Input → Middleware1 → Middleware2 → Middleware3 → Output
         ↓ (pipe)     ↓ (pipe)     ↓ (pipe)
      goroutine    goroutine    goroutine
```

**Key Patterns:**

1. **Streaming Processing**: Uses `io.Reader`/`io.Writer` under the hood via `Request`/`Response` wrappers
2. **Concurrent Execution**: Each middleware runs in its own goroutine  
3. **Immediate Processing**: No buffering - processing starts as data arrives
4. **Backpressure Handling**: Pipes automatically handle flow control
5. **Context Propagation**: Cancellation and timeouts flow through the entire chain

## Middleware Packages

Calque-Pipe includes batteries-included middleware for common AI and data processing patterns:

### AI & LLM (`ai/`, `prompt/`)
- **AI Agents**: `ai.Agent(client)` - Connect to Gemini, Ollama, or custom providers
- **Prompt Templates**: `prompt.Template("Question: {{.Input}}")` - Dynamic prompt formatting  
- **Streaming Support**: Real-time response processing as tokens arrive
- **Context Management**: Automatic conversation and context handling

### Flow Control (`ctrl/`, `logger/`)
- **Timeouts**: `ctrl.Timeout(handler, duration)` - Prevent hanging operations
- **Retries**: `ctrl.Retry(handler, attempts)` - Handle transient failures
- **Parallel Processing**: `ctrl.Parallel(handlers...)` - Concurrent execution 
- **Logging**: `logger.Print(label)` - Non-intrusive request/response logging
- **Conditional Logic**: `ctrl.If(condition, handler)` - Dynamic routing

### Data Processing (`text/`, `convert/`)
- **Text Transform**: `text.Transform(func)` - Simple string transformations
- **JSON/YAML/XML**: `convert.JSON()`, `convert.YAML()` - Structured data conversion
- **Schema Validation**: Ensure data conforms to expected formats
- **Streaming Parsers**: Process large files without loading into memory

### Memory & State (`memory/`)
- **Conversation Memory**: Track chat history with configurable limits
- **Context Windows**: Sliding window memory management for long conversations
- **Storage Backends**: In-memory, Redis, or custom storage adapters
- **Token Counting**: Automatic token limit management for LLMs

## Converters

Transform structured data at pipeline boundaries:

**Input Converters** (prepare data for processing):
```go
convert.ToJson(struct)      // Struct → JSON stream
convert.ToYaml(struct)      // Struct → YAML stream  
convert.ToJsonSchema(struct) // Struct + schema → stream (for AI context)
```

**Output Converters** (parse results):
```go
convert.FromJson(&result)           // JSON stream → struct
convert.FromJsonSchema(&result)     // Validates output against schema
```

**Usage:**
```go
// JSON processing pipeline
err := flow.Run(ctx, convert.ToJson(data), convert.FromJson(&result))

// AI with schema validation  
err := flow.Run(ctx, convert.ToJsonSchema(input), convert.FromJsonSchema[Output](&result))
```

## Roadmap

### Priority Middleware

**✅ Tool Calling** - Function execution for AI agents (completed)  
**RAG Components** - Vector search, document chunking, context building  
**Guardrails & Safety** - Input filtering, output validation, schema compliance  
**Multi-Agent Routing** - Agent selection, load balancing, conditional routing  
**HTTP/API Integration** - Web handlers, streaming responses, WebSocket support  

### Framework Improvements

**Enhanced Memory** - Vector-based semantic memory retrieval  
**Advanced Agents** - Planning, reflection, and self-evaluation capabilities  
**Developer Experience** - Better type inference, pipeline helpers, function shortcuts  

### Essential Examples

**Core Framework**: ✅ basics, ✅ converters, ✅ converters-jsonschema, 🔲 streaming-chats  
**Data Processing**: ✅ memory, 🔲 batch-processing, 🔲 pipeline-composition  
**AI Agents**: ✅ tool-calling, 🔲 rag-pipeline, 🔲 multi-agent-workflow, 🔲 guardrails-validation  
**Advanced**: 🔲 web-api-agent, 🔲 human-in-the-loop  

### Nice-to-Have

**Batch Processing** - Splitters, aggregators, parallel processors  
**State Management** - State machines, checkpoints, conditional flows
