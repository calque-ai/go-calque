# Calque-Pipe

calque-pipe is a **streaming middleware framework** for building AI agents and data processing pipelines. The core innovation is true concurrent execution where handlers process data as it flows through connected `io.Pipe` instances. The framework excels at memory-efficient processing of large datasets and real-time streaming scenarios, making it particularly well-suited for AI applications where latency and resource usage matter.

### Core Components

**Flow Orchestration (`core/`):**

- `Flow`: Main pipeline orchestrator that manages handler chains
- `Handler`: Interface with `ServeFlow(ctx, reader, writer) error` method
- `Pipe`: Wrapper around `io.Pipe` for handler connections
- `Converter`: Input/output type conversion system

**Key Architectural Patterns:**

1. **Streaming-First**: Built on `io.Reader`/`io.Writer` for memory efficiency
2. **Concurrent Execution**: Each handler runs in its own goroutine
3. **Composable Middleware**: Chain handlers with fluent API
4. **Type-Safe Conversions**: Generic functions for common data types

### Middleware Categories

**Flow Control (`middleware/flow/`):**

- `Parallel()`: Concurrent processing of same input
- `Branch()`: Conditional routing based on content
- `Batch()`: Request batching with configurable thresholds
- `Timeout()`/`Retry()`: Resilience patterns
- `Logger()`: Non-intrusive logging with content preview

**LLM Integration (`middleware/llm/`):**

- Provider abstraction for Gemini, Ollama, and mock providers
- Streaming response support for real-time processing
- Context and conversation management

**Memory Management (`middleware/memory/`):**

- Context memory with sliding window and token limits
- Conversation memory for structured message history
- Pluggable storage backends (in-memory, custom)

**Data Processing (`middleware/strings/`, `convert/`):**

- Text transformation and filtering
- Structured data conversion (JSON, YAML, XML) with schema support
- Line-by-line streaming processors

### Handler Development Patterns

**Generic I/O Utilities:**

```go
// In handlers, use these instead of manual io.ReadAll/Write
var input string
core.Read(r, &input)  // Reads and converts to string

return core.Write(w, result)  // Writes string/[]byte to output
```

**Handler Creation:**

```go
func myMiddleware() core.Handler {
    return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
        var input string
        if err := core.Read(r, &input); err != nil {
            return err
        }

        // Process input...
        processed := transform(input)

        return core.Write(w, processed)
    })
}
```

**Pipeline Composition:**

```go
pipeline := core.New().
    Use(flow.Logger("input", 100)).
    Use(myMiddleware()).
    Use(flow.Timeout(anotherHandler(), 30*time.Second)).
    Use(flow.Logger("output", 200))
```

## Data Flow Architecture

**Streaming Execution Model:**

```
Input → Handler1 → Handler2 → Handler3 → Output
         ↓         ↓         ↓
       Pipe1     Pipe2     Pipe3
      (goroutine)(goroutine)(goroutine)
```

- Each handler runs concurrently in its own goroutine
- Connected via `io.Pipe` for true streaming data flow
- Processing begins immediately as data arrives (no buffering)
- Context cancellation propagates through entire chain
- Backpressure handled automatically by pipe blocking behavior

## Key Design Decisions

**Streaming vs Buffered Operations:**

- Use `io.Copy` for streaming operations (memory efficient, real-time)
- Use `io.ReadAll` only when you need the complete input (transformations, parsing)
- Most flow middleware is streaming, most processing middleware is buffered

**Error Handling:**

- Context-aware error propagation throughout pipelines
- Graceful cleanup with defer statements
- Timeout and cancellation support at every level

**Type Safety:**

- Generic `Read[T]` and `Write[T]` functions for common types
- Converter interface for complex data transformations
- Compile-time type checking where possible

**Concurrency Model:**

- True parallelism via goroutines and pipes
- Thread-safe memory and context management
- Concurrent execution overlaps processing for better performance

## Development Roadmap

### Critical Missing Middleware

#### 1. ✅ Tool Calling & Function Execution (`middleware/tools/`)

**Priority: HIGH** - Essential for AI agents - **COMPLETED**

- ✅ `tools.Registry()` - Register available functions
- ✅ `tools.Execute()` - Parse LLM tool calls and execute functions  
- ✅ `tools.Format()` - Format tool results back to LLM
- ✅ `tools.Agent()` - Complete tool-enabled agent
- ✅ Multiple tool constructors: `Quick()`, `New()`, `HandlerFunc()`
- ✅ Flexible parsing: JSON, XML, and simple formats
- ✅ Comprehensive example in `examples/tool-calling/`

```go
// Simple usage:
calc := tools.Quick("calculator", func(expr string) string { return evaluate(expr) })
agent := tools.Agent(llmProvider, calc)

// Advanced usage:
pipeline := core.New().
    Use(tools.Registry(webSearchTool, calculatorTool)).
    Use(tools.Format(tools.FormatStyleDetailed)).
    Use(llm.Chat(provider)).
    Use(tools.Execute()).
    Use(llm.Chat(provider))
```

#### 2. RAG Components (`middleware/retrieval/`)

**Priority: HIGH** - Core AI agent capability

- `retrieval.VectorSearch()` - Search embeddings database
- `retrieval.DocumentChunking()` - Split documents into chunks
- `retrieval.ContextBuilder()` - Combine retrieved docs with query

#### 3. Guardrails & Safety (`middleware/validation/`)

**Priority: HIGH** - Production safety requirements

- `validation.InputFilter()` - Block harmful inputs
- `validation.OutputValidator()` - Check LLM responses
- `validation.SchemaValidation()` - Ensure structured output compliance

#### 4. Multi-Agent Coordination (`middleware/routing/`)

**Priority: MEDIUM** - Advanced agent workflows

- `routing.AgentSelector()` - Choose which agent handles request
- `routing.LoadBalancer()` - Distribute work across agents
- `routing.ConditionalRouter()` - Route based on content/rules

#### 5. HTTP/API Integration (`middleware/web/`)

**Priority: MEDIUM** - Web deployment capabilities

- `web.HTTPHandler()` - Convert HTTP requests to streams
- `web.StreamingResponse()` - Stream responses back to clients
- `web.WebSocketHandler()` - Real-time bidirectional communication

### Framework Enhancements

#### Enhanced Memory Middleware

- `memory.Semantic(embeddings)` - Vector-based memory retrieval for relevant past conversations

#### Advanced Agent Behaviors

- `agent.Tool(name, handler)` - Function calling integration
- `agent.Planning(steps)` - Multi-step reasoning capabilities
- `agent.Reflection()` - Self-evaluation of responses

#### Developer Experience Improvements

**Sub-Pipeline Helpers:**

```go
// Instead of manual sub-pipeline creation:
Use(flow.SubPipeline(convert.StructuredYAML(data), &result))
Use(flow.Convert(convert.StructuredYAML)) // Auto-conversion middleware
```

**HandlerFunc Shortcuts:**

```go
// Instead of verbose HandlerFunc:
Use(flow.Process(func(data Resume) (Evaluation, error) {...}))
Use(flow.Transform(reduceResults)) // Auto-wrap pure functions
```

**Better Type Inference:**

```go
// Remove need for explicit types:
Use(flow.Batch(handler, 2, 1*time.Second)) // Auto-infer T from handler
```

### Essential Examples Development

#### Core Framework Examples (3)

1. ✅ **basic** - Basic pipeline with string middleware
2. ✅ **structured-converter** - JSON/YAML processing
3. 🔲 **streaming-chat** - Real-time LLM streaming with memory

#### Data Processing Patterns (3)

4. ✅ **map-reduce** - Parallel data processing
5. 🔲 **batch-processing** - Handle large datasets efficiently
6. 🔲 **pipeline-composition** - Complex multi-stage data transformation

#### AI Agent Essentials (4)

7. 🔲 **rag-pipeline** - Document retrieval + LLM generation
8. ✅ **tool-calling** - LLM function calling with multiple tools
9. 🔲 **multi-agent-workflow** - Agents collaborating via pipelines
10. 🔲 **guardrails-validation** - Input/output safety checks

#### Advanced Examples (2)

11. 🔲 **web-api-agent** - HTTP integration with streaming responses
12. 🔲 **human-in-the-loop** - Interactive agent with approval workflows

### Nice-to-Have Middleware

#### Batch Processing Utilities (`middleware/batch/`)

- `batch.Splitter()` - Split large inputs into batches
- `batch.Aggregator()` - Combine batch results
- `batch.ParallelProcessor()` - Process batches concurrently

#### Workflow State Management (`middleware/state/`)

- `state.StateMachine()` - Manage agent workflows
- `state.Checkpoint()` - Save/restore pipeline state
- `state.ConditionalFlow()` - Branch based on state

---
