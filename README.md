# Go-Calque

<div align="center">
  <img src=".github/images/go-calque.webp" alt="Go-Calque" width="400">

  <p>
    <a href="https://github.com/calque-ai/go-calque/releases"><img src="https://img.shields.io/badge/Pre--release-v0.5.0-orange?style=flat" alt="Pre-release"></a>
    <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.24+-blue.svg?style=flat" alt="Go Version"></a>
    <a href="https://goreportcard.com/report/github.com/calque-ai/go-calque"><img src="https://goreportcard.com/badge/github.com/calque-ai/go-calque?style=flat" alt="Go Report Card"></a>
    <a href="https://pkg.go.dev/github.com/calque-ai/go-calque"><img src="https://pkg.go.dev/badge/github.com/calque-ai/go-calque.svg" alt="Go Reference"></a>
    <a href="https://github.com/calque-ai/go-calque/actions/workflows/ci.yml"><img src="https://github.com/calque-ai/go-calque/workflows/CI/badge.svg" alt="Build Status" height="20"></a>
    <a href="https://codecov.io/gh/calque-ai/go-calque"><img src="https://codecov.io/gh/calque-ai/go-calque/branch/main/graph/badge.svg" alt="Code Coverage"></a>
    <a href="https://opensource.org/licenses/MPL-2.0"><img src="https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg?style=flat" alt="License: MPL 2.0"></a>
    <a href="https://discord.gg/sga8uzDbth"><img src="https://img.shields.io/badge/Discord-Join%20Community-6D28D9?style=flat&logo=discord" alt="Discord"></a>
  </p>
</div>

A **composable AI agent framework** for Go that makes it easy to build production-ready AI applications.

_Developed by [Calque AI](https://calque.ai)_

## The Problem

Building AI apps in Go means wrestling with:

- **Provider lock-in** - Switching between OpenAI, Gemini, or local models requires rewriting code
- **Conversation state** - Managing chat history and context windows across requests
- **Tool calling** - Connecting AI to your Go functions with proper error handling
- **Structured outputs** - Getting reliable JSON responses that match your types
- **RAG pipelines** - Coordinating document retrieval, embedding, and generation

Go-Calque solves these with a simple, composable middleware pattern that feels native to Go.

## Installation

```bash
go get github.com/calque-ai/go-calque
```

## Quickstart

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
    client, err := ollama.New("llama3.2:3b")
    if err != nil {
        log.Fatal(err)
    }

    flow := calque.NewFlow().Use(ai.Agent(client))

    var result string
    err = flow.Run(context.Background(), "What's the capital of France?", &result)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result)
}
```

Three lines to set up, one line to run.

## What You Can Build

<table>
<tr>
<td width="50%">

### Chatbot with Memory

```go
convMem := memory.NewConversation()

flow := calque.NewFlow().
    Use(convMem.Input(userID)).
    Use(ai.Agent(client)).
    Use(convMem.Output(userID))
```

</td>
<td width="50%">

### AI with Tool Calling

```go
calculator := tools.Simple("calc", "Math", calcFn)
weather := tools.Simple("weather", "Weather", weatherFn)

flow := calque.NewFlow().
    Use(ai.Agent(client, ai.WithTools(calculator, weather)))
```

</td>
</tr>
<tr>
<td width="50%">

### Structured Output

```go
flow := calque.NewFlow().
    Use(ai.Agent(client, ai.WithSchema(&MyType{})))

var result MyType
flow.Run(ctx, "Analyze this", convert.FromJSONSchema(&result))
```

</td>
<td width="50%">

### RAG Pipeline

```go
flow := calque.NewFlow().
    Use(retrieval.VectorSearch(store, opts)).
    Use(prompt.Template(ragTemplate)).
    Use(ai.Agent(client))
```

</td>
</tr>
</table>

📖 **[See Getting Started Guide →](docs/getting-started.md)**

## Why Go-Calque?

| Challenge               | Raw SDK                    | Go-Calque                                |
| ----------------------- | -------------------------- | ---------------------------------------- |
| **Provider switching**  | Rewrite API calls          | Change one line: `ollama.New()` → `openai.New()` |
| **Conversation memory** | Manual state management    | `convMem.Input()` / `convMem.Output()`   |
| **Tool calling**        | Parse, match, handle errors| `ai.WithTools(...)` - automatic          |
| **Structured output**   | Hope AI follows instructions| `ai.WithSchema()` - guaranteed types    |
| **Retries & fallbacks** | Custom logic               | `ctrl.Retry()`, `ctrl.Fallback()`        |

## Features

### Core
- **[AI Agents](docs/middleware.md#ai-agents)** - OpenAI, Gemini, Ollama with unified interface
- **[Tool Calling](docs/middleware.md#tool-integration)** - Auto-discovery and execution of Go functions
- **[Memory](docs/middleware.md#memory)** - Conversation history with configurable limits

### Data Processing
- **[RAG & Retrieval](docs/middleware.md#retrieval)** - Vector search, context building, semantic filtering
- **[Converters](docs/middleware.md#converters)** - JSON, YAML, Protobuf, JSONSchema, SSE
- **[Flow Control](docs/middleware.md#flow-control)** - Retry, timeout, fallback, parallel, chain

### Production
- **[Observability](docs/middleware.md#observability)** - Metrics, tracing, health checks, structured logging
- **[MCP Support](docs/middleware.md#mcp)** - Model Context Protocol client
- **[Multi-Agent](docs/middleware.md#multi-agent)** - Agent routing and load balancing

📖 **[See Full Middleware Reference →](docs/middleware.md)**

## Performance

Go-Calque is built for **production AI workloads** where LLM latency dominates.

| Metric | Value |
|--------|-------|
| **AI Overhead** | <0.02% at 100ms latency |
| **Streaming** | 3x faster than buffered |
| **Text Processing** | Up to 86% faster than hand-coded |
| **Memory** | 87% less allocation with streaming |

📊 **[See Benchmark Analysis →](docs/BENCHMARK_ANALYSIS_REPORT.md)**

## Documentation

| Guide | Description |
|-------|-------------|
| **[Getting Started](docs/getting-started.md)** | Installation, quickstart, core concepts |
| **[Middleware Reference](docs/middleware.md)** | All middleware packages and usage |
| **[Architecture](docs/architecture.md)** | Streaming pipeline deep dive |
| **[Advanced Topics](docs/advanced.md)** | Custom middleware, concurrency, composition |
| **[Performance](docs/BENCHMARK_ANALYSIS_REPORT.md)** | Benchmark analysis and optimization |
| **[Examples](examples/)** | Runnable code examples |
| **[API Reference](https://pkg.go.dev/github.com/calque-ai/go-calque)** | pkg.go.dev documentation |

## Examples

| Example | Description |
|---------|-------------|
| [basics](examples/basics/) | Core flow concepts |
| [ai-clients](examples/ai-clients/) | OpenAI, Ollama, Gemini |
| [streaming-chat](examples/streaming-chat/) | SSE streaming with memory |
| [tool-calling](examples/tool-calling/) | Function calling with AI |
| [memory](examples/memory/) | Conversation memory |
| [retrieval](examples/retrieval/) | RAG/vector search |
| [mcp](examples/mcp/) | Model Context Protocol |

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new middleware
4. Submit a pull request

See [AGENTS.md](AGENTS.md) for development setup.

## Contributors

<a href="https://github.com/calque-ai/go-calque/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=calque-ai/go-calque" />
</a>

## License

Mozilla Public License 2.0 - see [LICENSE](LICENSE) file for details.
