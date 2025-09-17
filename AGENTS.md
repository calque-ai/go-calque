# AGENTS.md

Go-Calque is a streaming multi-agent AI framework implementing HTTP middleware patterns for AI data processing. The core pattern chains middleware through `io.Pipe` connections where each middleware runs in its own goroutine.

## Setup commands

### Development Environment

```bash
go mod download                      # Download dependencies
go build -v ./...                   # Build all packages
go build -v -o /dev/null ./...      # Build without output (validation)
```

### Running Tests

```bash
go test -v ./...                                          # Unit tests
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...  # With coverage
go test -bench=. -benchmem ./...                         # Benchmarks
examples/run_integration_tests.sh                        # Integration tests
```

### Code Quality

```bash
go vet ./...                        # Vet analysis
golangci-lint run                   # Linter (uses .golangci.yml config)
gofmt -s -w .                      # Format code
govulncheck ./...                  # Security scan
```

### Complete Development Workflow

```bash
scripts/local.sh                   # Run build, lint, test, benchmarks
```

## Project structure

**Core Framework**:

- `pkg/calque/flow.go` - Main orchestration engine using streaming pipelines
- `pkg/convert/` - Transform structured data at flow boundaries (JSON, YAML, JSONSchema, Protobuf)
- `pkg/calque/` - HTTP-like request/response abstraction for middleware

**Middleware Packages**:

- `pkg/middleware/ai/` - AI client implementations (OpenAI, Ollama, Gemini)
- `pkg/middleware/ctrl/` - Flow control (chain, batch, fallback, ratelimit)
- `pkg/middleware/memory/` - Memory management (conversation, context, store)
- `pkg/middleware/tools/` - Tool integration (registry, execute, detect)
- `pkg/middleware/text/` - Text processing and transformations
- `pkg/middleware/logger/` - Observability with multiple adapter support
- `pkg/middleware/multiagent/` - Multi-agent routing and consensus
- `pkg/middleware/mcp/` - Model Context Protocol implementation
- `pkg/middleware/remote/grpc` - Remote gRPC Transport implementation

**Usage examples**

- `examples/` - Directory for framework usage patterns

## Code style

- **Go Version**: 1.24+ required
- **Module**: `github.com/calque-ai/go-calque`
- **License**: Mozilla Public License 2.0
- Follow standard Go conventions and `gofmt` formatting
- Use `golangci-lint` configuration in `.golangci.yml`
- All code must pass `go vet` analysis
- Maintain race-free code (tested with `-race` flag)

## Architecture patterns

**Streaming Architecture**: Each middleware processes data through `io.Reader`/`io.Writer` interfaces with automatic backpressure handling via `io.Pipe`

**Concurrency Control**: Flow supports `ConcurrencyUnlimited`, `ConcurrencyAuto` (CPU-based), or fixed limits

**Converter Pattern**: Input/Output converters handle structured data transformation at flow boundaries

**Context Propagation**: Cancellation and timeouts flow through entire middleware chain

**Tool Calling**: AI agents can execute Go functions with automatic schema generation and response parsing

## Testing guidelines

- **Unit Tests**: Place `*_test.go` files alongside source code
- **Integration Tests**: Use `examples/*/integration_test.go` for end-to-end scenarios
- **Benchmarks**: Include performance tests in example directories
- **Coverage**: Use atomic coverage mode with race detection
- All tests must pass before committing
- Add tests for new features and bug fixes

## Key dependencies

- OpenAI SDK for GPT models
- Ollama for local model serving
- Google Gemini SDK
- YAML/JSON processing libraries

## Common gotchas

- Context cancellation must be properly propagated through middleware chains
- Memory limits are configurable - adjust based on your use case
- Concurrency settings affect performance - test with your specific workload
