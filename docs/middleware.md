# Middleware Reference

Complete reference for all Go-Calque middleware packages.

## AI Agents

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/ai`

### Basic Usage

```go
import (
    "github.com/calque-ai/go-calque/pkg/middleware/ai"
    "github.com/calque-ai/go-calque/pkg/middleware/ai/openai"
)

client, _ := openai.New("gpt-4")
flow := calque.NewFlow().Use(ai.Agent(client))
```

### Options

```go
ai.Agent(client,
    ai.WithSystemPrompt("You are a helpful assistant"),
    ai.WithTools(tool1, tool2),           // Enable tool calling
    ai.WithSchema(&MyType{}),             // Structured output
    ai.WithMaxTokens(1000),
    ai.WithTemperature(0.7),
)
```

### Providers

| Provider | Import | Constructor |
|----------|--------|-------------|
| OpenAI | `ai/openai` | `openai.New("gpt-4")` |
| Ollama | `ai/ollama` | `ollama.New("llama3.2:3b")` |
| Gemini | `ai/gemini` | `gemini.New(ctx, "gemini-pro")` |

---

## Memory

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/memory`

### Conversation Memory

```go
convMem := memory.NewConversation()

flow := calque.NewFlow().
    Use(convMem.Input(userID)).   // Store user message
    Use(ai.Agent(client)).
    Use(convMem.Output(userID))   // Store AI response
```

### Options

```go
memory.NewConversation(
    memory.WithMaxMessages(100),      // Limit history size
    memory.WithMaxTokens(4000),       // Token-based limit
    memory.WithStore(customStore),    // Custom backend
)
```

### Storage Backends

- **In-memory** (default) - For development
- **Badger** - Persistent local storage
- **Custom** - Implement `memory.Store` interface

---

## Tool Integration

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/tools`

### Simple Tools

```go
calculator := tools.Simple("calculator", "Performs math",
    func(jsonArgs string) string {
        var args struct{ Expression string `json:"expression"` }
        json.Unmarshal([]byte(jsonArgs), &args)
        return evaluate(args.Expression)
    })

flow := calque.NewFlow().
    Use(ai.Agent(client, ai.WithTools(calculator)))
```

### Typed Tools

```go
type WeatherArgs struct {
    City    string `json:"city" jsonschema:"required"`
    Unit    string `json:"unit" jsonschema:"enum=celsius;fahrenheit"`
}

weather := tools.Typed("weather", "Get weather", 
    func(args WeatherArgs) (string, error) {
        return fetchWeather(args.City, args.Unit), nil
    })
```

### Tool Registry

```go
registry := tools.NewRegistry()
registry.Register(calculator)
registry.Register(weather)

flow := calque.NewFlow().
    Use(ai.Agent(client, ai.WithToolRegistry(registry)))
```

---

## Retrieval (RAG)

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/retrieval`

### Vector Search

```go
store := weaviate.New("http://localhost:8080", "Documents")

strategy := retrieval.StrategyDiverse
opts := &retrieval.SearchOptions{
    Threshold: 0.7,
    Limit:     5,
    Strategy:  &strategy,
    MaxTokens: 2000,
}

flow := calque.NewFlow().
    Use(retrieval.VectorSearch(store, opts))
```

### Context Strategies

| Strategy | Description |
|----------|-------------|
| `StrategyRelevant` | Most similar documents |
| `StrategyRecent` | Most recent documents |
| `StrategyDiverse` | MMR for diversity |
| `StrategySummary` | Summarized context |

### Document Loading

```go
loader := retrieval.DocumentLoader(
    retrieval.FromFile("docs/*.md"),
    retrieval.FromURL("https://example.com/docs"),
)
```

### Vector Stores

- **Weaviate** - `retrieval/weaviate`
- **Qdrant** - `retrieval/qdrant`
- **PGVector** - `retrieval/pgvector`

---

## Flow Control

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/ctrl`

### Chain (Sequential)

```go
// Handlers run sequentially, buffered data transfer
ctrl.Chain(handler1, handler2, handler3)
```

### Retry

```go
ctrl.Retry(handler, 3)                    // Retry 3 times
ctrl.Retry(handler, 3, ctrl.WithBackoff(time.Second))
```

### Timeout

```go
ctrl.Timeout(handler, 30*time.Second)
```

### Fallback

```go
ctrl.Fallback(
    ai.Agent(primaryClient),
    ai.Agent(backupClient),
)
```

### Parallel

```go
ctrl.Parallel(handler1, handler2, handler3)
```

### Batch

```go
ctrl.Batch(handler, ctrl.BatchSize(10))
```

---

## Prompt Templates

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/prompt`

```go
prompt.Template(`You are a helpful assistant.

Context:
{{.Input}}

Question: {{.Query}}

Answer based only on the provided context.`)
```

### Available Variables

- `{{.Input}}` - Data from previous handler
- `{{.Query}}` - Original user query
- Custom variables via context

---

## Converters

**Package:** `github.com/calque-ai/go-calque/pkg/convert`

### Input Converters

```go
convert.ToJSON(struct)         // Struct → JSON stream
convert.ToYAML(struct)         // Struct → YAML stream
convert.ToJSONSchema(struct)   // Struct + schema → stream
convert.ToProtobuf(msg)        // Proto message → binary
convert.ToSSE(data)            // Data → Server-Sent Events
```

### Output Converters

```go
convert.FromJSON(&result)          // JSON → struct
convert.FromYAML(&result)          // YAML → struct
convert.FromJSONSchema(&result)    // JSON → struct (validated)
convert.FromProtobuf(&result)      // Binary → proto message
```

---

## Multi-Agent

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/multiagent`

### Agent Routing

```go
mathAgent := multiagent.Route(
    ai.Agent(mathClient),
    "math",
    "Solve mathematical problems",
    "calculate,solve,math")

codeAgent := multiagent.Route(
    ai.Agent(codeClient),
    "code",
    "Programming tasks",
    "code,program,debug")

flow := calque.NewFlow().
    Use(multiagent.Router(routerClient, mathAgent, codeAgent))
```

---

## MCP (Model Context Protocol)

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/mcp`

### Client Setup

```go
client, _ := mcp.NewClient(mcp.StdioTransport("./mcp-server"))

// Register tools with AI agent
flow := calque.NewFlow().
    Use(mcp.RegisterTools(client)).
    Use(ai.Agent(llmClient))
```

### Natural Language Tools

```go
flow := calque.NewFlow().
    Use(mcp.DetectTools(client, llmClient)).
    Use(mcp.ExtractToolParams(client, llmClient)).
    Use(mcp.ExecuteTools(client))
```

---

## Observability

### Context & Errors

**Package:** `github.com/calque-ai/go-calque/pkg/calque`

```go
// Context tracking
ctx = calque.WithTraceID(ctx, "trace-123")
ctx = calque.WithRequestID(ctx, "req-456")

// Context-aware errors
err := calque.WrapErr(ctx, err, "operation failed")
err := calque.NewErr(ctx, "something went wrong")

// Logging with context
calque.LogInfo(ctx, "processing request", "user", userID)
calque.LogError(ctx, "failed", "error", err)
```

### Metrics

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/observability`

```go
provider := observability.NewPrometheusProvider()

flow := calque.NewFlow().
    Use(observability.Metrics(provider, 
        observability.WithLabels("service", "chat")))

// Expose /metrics endpoint
http.Handle("/metrics", provider.Handler())
```

### Distributed Tracing

```go
provider := observability.NewOTLPTracerProvider("http://jaeger:4318")

flow := calque.NewFlow().
    Use(observability.Tracing(provider, "chat-flow"))
```

### Health Checks

```go
flow := calque.NewFlow().
    Use(observability.HealthCheck(
        observability.TCPHealthCheck("db", "localhost:5432"),
        observability.HTTPHealthCheck("api", "http://api/health"),
    ))
```

---

## Inspection & Debugging

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/inspect`

```go
flow := calque.NewFlow().
    Use(inspect.Print("input")).       // Log all input
    Use(handler).
    Use(inspect.Head("output", 100))   // Log first 100 bytes

// Other options
inspect.Chunks("data", 1024)           // Log in chunks
inspect.HeadTail("stream", 50, 50)     // Log start and end
inspect.Timing("handler", handler)     // Measure execution time
```

---

## Cache

**Package:** `github.com/calque-ai/go-calque/pkg/middleware/cache`

```go
flow := calque.NewFlow().
    Use(cache.Cache(expensiveHandler, 5*time.Minute))
```

### Custom Store

```go
store := cache.NewStore(
    cache.WithTTL(10*time.Minute),
    cache.WithMaxSize(1000),
)

cache.Cache(handler, 0, cache.WithStore(store))
```

---

## Next

**→ [Architecture](architecture.md)** - Understand how streaming pipelines work under the hood

