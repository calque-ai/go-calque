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

---

## Next

**→ [Recipes & Examples](recipes.md)** - HTTP integration, testing, debugging, real-world examples
