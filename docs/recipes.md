# Recipes & Examples

Practical patterns for production Go-Calque applications.

## HTTP API Integration

### REST Endpoint

```go
http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
    var req ChatRequest
    json.NewDecoder(r.Body).Decode(&req)

    var response string
    if err := flow.Run(r.Context(), req.Message, &response); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(ChatResponse{Message: response})
})
```

### SSE Streaming

```go
http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    userID := r.URL.Query().Get("user_id")
    message := r.URL.Query().Get("message")

    sseConverter := convert.ToSSE(w, userID).
        WithChunkMode(convert.SSEChunkByWord)

    // Tokens stream directly to client
    flow.Run(r.Context(), message, sseConverter)
})
```

## Testing

### Unit Testing Middleware

```go
func TestMyHandler(t *testing.T) {
    handler := MyHandler()

    input := strings.NewReader("test input")
    output := &bytes.Buffer{}

    req := &calque.Request{
        Data:    input,
        Context: context.Background(),
    }
    res := &calque.Response{Data: output}

    err := handler.ServeFlow(req, res)
    require.NoError(t, err)
    assert.Equal(t, "expected output", output.String())
}
```

### Integration Testing

```go
func TestFlow(t *testing.T) {
    flow := calque.NewFlow().
        Use(handler1).
        Use(handler2)

    var result string
    err := flow.Run(context.Background(), "input", &result)
    
    require.NoError(t, err)
    assert.Contains(t, result, "expected")
}
```

### Mocking AI Clients

```go
type MockClient struct {
    Response string
}

func (m *MockClient) Call(ctx context.Context, prompt string) (string, error) {
    return m.Response, nil
}

func TestWithMockAI(t *testing.T) {
    mock := &MockClient{Response: "mocked response"}
    flow := calque.NewFlow().Use(ai.Agent(mock))

    var result string
    flow.Run(context.Background(), "test", &result)
    assert.Equal(t, "mocked response", result)
}
```

## Performance Optimization

### Use ctrl.Chain for Sequential Tasks

```go
// 3.4x faster than concurrent flow for sequential operations
ctrl.Chain(
    handler1,
    handler2,
    handler3,
)
```

### Reuse Flow Instances

```go
// Create once
var chatFlow = calque.NewFlow().
    Use(convMem.Input()).
    Use(ai.Agent(client)).
    Use(convMem.Output())

// Reuse for each request
func handleChat(userID, message string) string {
    var result string
    chatFlow.Run(ctx, message, &result)
    return result
}
```

### Lazy Initialization

```go
var (
    flowOnce sync.Once
    chatFlow *calque.Flow
)

func getFlow() *calque.Flow {
    flowOnce.Do(func() {
        client, _ := openai.New("gpt-4")
        chatFlow = calque.NewFlow().Use(ai.Agent(client))
    })
    return chatFlow
}
```

## Debugging

### Inspect Data Flow

```go
flow := calque.NewFlow().
    Use(inspect.Print("before-ai")).
    Use(ai.Agent(client)).
    Use(inspect.Print("after-ai"))
```

### Timing Analysis

```go
flow := calque.NewFlow().
    Use(inspect.Timing("handler1", handler1)).
    Use(inspect.Timing("ai-agent", ai.Agent(client)))
```

### Structured Logging

```go
func Handler() calque.HandlerFunc {
    return func(req *calque.Request, res *calque.Response) error {
        calque.LogInfo(req.Context, "processing started")

        result, err := process()
        if err != nil {
            calque.LogError(req.Context, "processing failed", "error", err)
            return err
        }

        calque.LogInfo(req.Context, "processing completed",
            "output_size", len(result),
        )
        return calque.Write(res, result)
    }
}
```

---

## Real-World Examples

### Chatbot with Memory and Tools

```go
func main() {
    client, _ := openai.New("gpt-4")
    convMem := memory.NewConversation()

    // Define tools
    searchDocs := tools.Simple("search_docs", "Search documentation", searchHandler)
    createTicket := tools.Simple("create_ticket", "Create support ticket", ticketHandler)

    // Build chatbot flow
    chatbot := calque.NewFlow().
        Use(convMem.Input("session")).
        Use(ctrl.Retry(
            ai.Agent(client, ai.WithTools(searchDocs, createTicket)),
            3,
        )).
        Use(convMem.Output("session"))

    // Handle messages
    http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
        var req ChatRequest
        json.NewDecoder(r.Body).Decode(&req)

        var response string
        if err := chatbot.Run(r.Context(), req.Message, &response); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        json.NewEncoder(w).Encode(ChatResponse{Message: response})
    })
}
```

### RAG Application

```go
func main() {
    client, _ := ollama.New("llama3.2:3b")
    store := qdrant.New("localhost:6334", "knowledge_base")

    // Retrieval configuration
    strategy := retrieval.StrategyRelevant
    searchOpts := &retrieval.SearchOptions{
        Threshold: 0.75,
        Limit:     5,
        Strategy:  &strategy,
        MaxTokens: 3000,
    }

    // RAG pipeline
    ragFlow := calque.NewFlow().
        Use(retrieval.VectorSearch(store, searchOpts)).
        Use(prompt.Template(`You are a helpful assistant.

Context:
{{.Input}}

Question: {{.Query}}

Answer based only on the provided context.`)).
        Use(ai.Agent(client))

    // API endpoint
    http.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
        question := r.URL.Query().Get("q")

        var answer string
        if err := ragFlow.Run(r.Context(), question, &answer); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        fmt.Fprint(w, answer)
    })
}
```

### Multi-Agent Router

```go
// Create specialized agents
mathAgent := multiagent.Route(
    ai.Agent(mathClient),
    "math",
    "Solve mathematical problems and calculations",
    "calculate,solve,math,equation")

codeAgent := multiagent.Route(
    ai.Agent(codeClient),
    "code",
    "Programming, debugging, code review",
    "code,program,debug,function")

// Router automatically selects best agent
flow := calque.NewFlow().
    Use(multiagent.Router(routerClient, mathAgent, codeAgent))

flow.Run(ctx, "What's the factorial of 10?", &result)  // Routes to mathAgent
flow.Run(ctx, "Write a bubble sort in Go", &result)    // Routes to codeAgent
```

---

## Next

**→ [Performance Analysis](BENCHMARK_ANALYSIS_REPORT.md)** - Benchmark data and optimization opportunities

