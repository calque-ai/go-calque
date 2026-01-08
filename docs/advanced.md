# Advanced Topics

Advanced usage patterns and techniques for Go-Calque.

## Writing Custom Middleware

### Buffered Middleware

Most middleware reads all input, processes it, then writes output:

```go
func AddTimestamp(prefix string) calque.HandlerFunc {
    return func(req *calque.Request, res *calque.Response) error {
        // Read input using the helper
        var input string
        if err := calque.Read(req, &input); err != nil {
            return err
        }

        // Transform
        timestamp := time.Now().Format("2006-01-02 15:04:05")
        output := fmt.Sprintf("[%s %s] %s", prefix, timestamp, input)

        // Write output using the helper
        return calque.Write(res, output)
    }
}
```

### Streaming Middleware

For large data, process without buffering:

```go
func StreamingProcessor() calque.HandlerFunc {
    return func(req *calque.Request, res *calque.Response) error {
        scanner := bufio.NewScanner(req.Data)
        for scanner.Scan() {
            line := scanner.Text()
            processed := fmt.Sprintf("PROCESSED: %s\n", line)
            if _, err := res.Data.Write([]byte(processed)); err != nil {
                return err
            }
        }
        return scanner.Err()
    }
}
```

### Conditional Middleware

Execute different logic based on input:

```go
func ConditionalHandler(condition func(string) bool, ifTrue, ifFalse calque.Handler) calque.HandlerFunc {
    return func(req *calque.Request, res *calque.Response) error {
        var input string
        if err := calque.Read(req, &input); err != nil {
            return err
        }

        // Create new request with input
        newReq := &calque.Request{
            Data:    strings.NewReader(input),
            Context: req.Context,
        }

        if condition(input) {
            return ifTrue.ServeFlow(newReq, res)
        }
        return ifFalse.ServeFlow(newReq, res)
    }
}
```

## Flow Composition

### Reusable Sub-Flows

```go
// Define reusable components
textPreprocessor := calque.NewFlow().
    Use(text.Transform(strings.TrimSpace)).
    Use(text.Transform(strings.ToLower))

textAnalyzer := calque.NewFlow().
    Use(text.Transform(func(s string) string {
        wordCount := len(strings.Fields(s))
        return fmt.Sprintf("TEXT: %s\nWORDS: %d", s, wordCount)
    }))

// Compose into main flow
mainFlow := calque.NewFlow().
    Use(textPreprocessor).
    Use(textAnalyzer).
    Use(ai.Agent(client))
```

### Branching Logic

```go
flow := calque.NewFlow().
    Use(text.Branch(
        func(s string) bool { return len(s) > 100 },
        longTextHandler,   // If true
        shortTextHandler,  // If false
    ))
```

## Concurrency Control

### High-Throughput Configuration

```go
config := calque.FlowConfig{
    MaxConcurrent: calque.ConcurrencyAuto,
    CPUMultiplier: 10,  // 10x GOMAXPROCS
}

flow := calque.NewFlow(config).
    Use(ai.Agent(client))
```

### Fixed Concurrency

```go
config := calque.FlowConfig{
    MaxConcurrent: 100,  // Max 100 concurrent operations
}
```

### MetadataBus for Concurrent Handlers

```go
func Handler1() calque.HandlerFunc {
    return func(req *calque.Request, res *calque.Response) error {
        // Set metadata for other handlers
        req.MetadataBus.Set("processing_id", uuid.New().String())
        
        // Process...
        return nil
    }
}

func Handler2() calque.HandlerFunc {
    return func(req *calque.Request, res *calque.Response) error {
        // Read metadata from concurrent handler
        id := req.MetadataBus.Get("processing_id")
        
        // Use the shared data
        return nil
    }
}
```

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

## Error Handling Patterns

### Context-Aware Errors

```go
func Handler() calque.HandlerFunc {
    return func(req *calque.Request, res *calque.Response) error {
        result, err := doWork()
        if err != nil {
            // Error includes trace_id and request_id
            return calque.WrapErr(req.Context, err, "operation failed")
        }
        return calque.Write(res, result)
    }
}
```

### Retry with Backoff

```go
flow := calque.NewFlow().
    Use(ctrl.Retry(handler, 3,
        ctrl.WithBackoff(time.Second),
        ctrl.WithMaxBackoff(10*time.Second),
    ))
```

### Fallback Chains

```go
flow := calque.NewFlow().
    Use(ctrl.Fallback(
        ai.Agent(gpt4Client),      // Try GPT-4 first
        ai.Agent(gpt35Client),     // Fall back to GPT-3.5
        ai.Agent(ollamaClient),    // Last resort: local model
    ))
```

## Testing Middleware

### Unit Testing

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

## Next

**→ [Performance Analysis](BENCHMARK_ANALYSIS_REPORT.md)** - Benchmark data and optimization opportunities

