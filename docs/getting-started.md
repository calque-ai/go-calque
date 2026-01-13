# Getting Started with Go-Calque

This guide will help you get up and running with Go-Calque quickly.

## Installation

```bash
go get github.com/calque-ai/go-calque
```

**Requirements:** Go 1.24+

## Core Concepts

Go-Calque uses a **middleware pattern** similar to HTTP frameworks. You chain handlers together to process data through a flow.

```
Input → Handler1 → Handler2 → Handler3 → Output
```

Each handler:
- Receives data from the previous handler
- Processes it
- Passes it to the next handler

## Your First Flow

### Simple AI Chat

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/calque-ai/go-calque/pkg/calque"
    "github.com/calque-ai/go-calque/pkg/middleware/ai"
    "github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
)

func main() {
    // 1. Create an AI client
    client, err := ollama.New("llama3.2:3b")
    if err != nil {
        log.Fatal(err)
    }

    // 2. Create a flow with the AI agent
    flow := calque.NewFlow().Use(ai.Agent(client))

    // 3. Run the flow
    var result string
    err = flow.Run(context.Background(), "What's the capital of France?", &result)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result)
}
```

## Progressive Complexity

Go-Calque grows with your needs. Start simple, add capabilities as required.

### Adding Conversation Memory

```go
import "github.com/calque-ai/go-calque/pkg/middleware/memory"

client, _ := ollama.New("llama3.2:1b")
convMem := memory.NewConversation()

flow := calque.NewFlow().
    Use(convMem.Input("user123")).    // Store user input
    Use(ai.Agent(client)).
    Use(convMem.Output("user123"))    // Store AI response

// First message
flow.Run(ctx, "My name is Alice", &response)

// Second message - AI remembers the conversation
flow.Run(ctx, "What's my name?", &response)
// Response: "Your name is Alice"
```

### Adding Tool Calling

```go
import "github.com/calque-ai/go-calque/pkg/middleware/tools"

// Define tools
calculator := tools.Simple("calculator", "Performs math calculations",
    func(jsonArgs string) string {
        var args struct{ Expression string `json:"expression"` }
        json.Unmarshal([]byte(jsonArgs), &args)
        return evaluate(args.Expression)
    })

weather := tools.Simple("get_weather", "Gets current weather",
    func(jsonArgs string) string {
        var args struct{ City string `json:"city"` }
        json.Unmarshal([]byte(jsonArgs), &args)
        return fetchWeather(args.City)
    })

// Agent automatically calls tools when needed
flow := calque.NewFlow().
    Use(ai.Agent(client, ai.WithTools(calculator, weather)))

flow.Run(ctx, "What's the weather in Tokyo and what's 15% of 340?", &result)
// AI calls both tools and synthesizes the response
```

### Getting Structured Output

```go
import "github.com/calque-ai/go-calque/pkg/convert"

type TaskAnalysis struct {
    TaskType string `json:"task_type" jsonschema:"required,description=Type of task"`
    Priority string `json:"priority" jsonschema:"required,enum=low;medium;high"`
    Hours    int    `json:"hours" jsonschema:"description=Estimated hours"`
}

flow := calque.NewFlow().
    Use(ai.Agent(client, ai.WithSchema(&TaskAnalysis{})))

var analysis TaskAnalysis
flow.Run(ctx, "Build a user authentication system",
    convert.FromJSONSchema[TaskAnalysis](&analysis))

fmt.Printf("Type: %s, Priority: %s, Hours: %d\n",
    analysis.TaskType, analysis.Priority, analysis.Hours)
```

### Building a RAG Pipeline

```go
import (
    "github.com/calque-ai/go-calque/pkg/middleware/retrieval"
    "github.com/calque-ai/go-calque/pkg/middleware/prompt"
)

// Initialize vector store
store := weaviate.New("http://localhost:8080", "Documents")

// Configure retrieval
strategy := retrieval.StrategyDiverse
searchOpts := &retrieval.SearchOptions{
    Threshold: 0.7,
    Limit:     5,
    Strategy:  &strategy,
    MaxTokens: 2000,
}

// Build RAG pipeline
flow := calque.NewFlow().
    Use(retrieval.VectorSearch(store, searchOpts)).
    Use(prompt.Template(`Answer based on this context:
{{.Input}}

Question: {{.Query}}`)).
    Use(ai.Agent(client))

flow.Run(ctx, "How do I configure authentication?", &result)
```

## Supported AI Providers

### OpenAI

```go
import "github.com/calque-ai/go-calque/pkg/middleware/ai/openai"

client, err := openai.New("gpt-4")
// Uses OPENAI_API_KEY environment variable
```

### Ollama (Local)

```go
import "github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"

client, err := ollama.New("llama3.2:3b")
// Connects to localhost:11434 by default
```

### Google Gemini

```go
import "github.com/calque-ai/go-calque/pkg/middleware/ai/gemini"

client, err := gemini.New(context.Background(), "gemini-pro")
// Uses GOOGLE_API_KEY environment variable
```

## Error Handling & Retries

```go
import "github.com/calque-ai/go-calque/pkg/middleware/ctrl"

flow := calque.NewFlow().
    Use(ctrl.Retry(ai.Agent(client), 3)).  // Retry up to 3 times
    Use(ctrl.Fallback(
        ai.Agent(primaryClient),   // Try primary first
        ai.Agent(backupClient),    // Fall back to backup
    ))
```

## Next

**→ [Middleware Reference](middleware.md)** - Learn about all available middleware packages

