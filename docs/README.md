# Go-Calque Documentation

Welcome to the Go-Calque documentation.

## Quick Links

| Guide | Description |
|-------|-------------|
| [Getting Started](getting-started.md) | Installation, quickstart, core concepts |
| [Middleware Reference](middleware.md) | All middleware packages and usage |
| [Architecture](architecture.md) | Streaming pipeline deep dive |
| [Advanced Topics](advanced.md) | Custom middleware, concurrency, composition |
| [Performance](BENCHMARK_ANALYSIS_REPORT.md) | Benchmark analysis and optimization |

## Examples

All runnable examples are in the [`examples/`](../examples/) directory:

| Example | Description |
|---------|-------------|
| [basics](../examples/basics/) | Core flow concepts |
| [ai-clients](../examples/ai-clients/) | OpenAI, Ollama, Gemini integration |
| [streaming-chat](../examples/streaming-chat/) | SSE streaming with memory |
| [tool-calling](../examples/tool-calling/) | Function calling with AI |
| [memory](../examples/memory/) | Conversation memory |
| [retrieval](../examples/retrieval/) | RAG/vector search |
| [mcp](../examples/mcp/) | Model Context Protocol |
| [multiagent](../examples/multiagent/) | Multi-agent routing |
| [batch-processing](../examples/batch-processing/) | Batch operations |
| [converters](../examples/converters/) | JSON/YAML/Protobuf conversion |

## Quick Reference

### Performance Highlights

| Metric | Value |
|--------|-------|
| AI Overhead | <0.02% at 100ms latency |
| Streaming | 3x faster than buffered |
| Text Processing | Up to 86% faster than hand-coded |
| Memory | 87% less allocation with streaming |

### When to Use What

| Use Case | Recommendation |
|----------|----------------|
| AI chat with streaming output | Regular flow with `.Use()` |
| Tool calling / context propagation | `ctrl.Chain()` |
| Parallel tool execution | Regular flow with `.Use()` |
| Simple transformations | `ctrl.Chain()` |
| High-throughput HTTP API | Regular flow with concurrency config |

### Running Benchmarks

```bash
# All benchmarks
go test -bench=. -benchmem ./benchmarks/...

# Specific categories
go test -bench=BenchmarkAILatency -benchmem ./benchmarks/...
go test -bench=BenchmarkChainVsFlow -benchmem ./benchmarks/...

# With profiling
go test -bench=BenchmarkRealisticWorkflow -cpuprofile=cpu.prof ./benchmarks/...
go tool pprof -http=:8080 cpu.prof
```

## Reference

- [AGENTS.md](../AGENTS.md) - Development setup and project structure
- [pkg.go.dev](https://pkg.go.dev/github.com/calque-ai/go-calque) - API documentation
- [Main README](../README.md) - Project overview

## Contributing

Documentation improvements are welcome! Please:

1. Keep the main README focused on quickstart
2. Add detailed content to this `docs/` folder
3. Link from README to docs when appropriate
4. Include runnable code examples
