# Protocol Buffers (protobuf) with buf

This directory contains Protocol Buffer definitions and configuration for the go-calque project.

## What is buf?

`buf` is a modern tool for working with Protocol Buffers. It makes it easier to:
- Generate Go code from `.proto` files
- Validate protobuf definitions
- Manage protobuf dependencies

## Files in this directory

- `calque.proto` - Main protobuf definitions for gRPC services
- `buf.yaml` - Configuration for buf linting and breaking change detection
- `buf.gen.yaml` - Configuration for code generation
- `calque.pb.go` - Generated Go structs for protobuf messages
- `calque_grpc.pb.go` - Generated Go gRPC client and server code

## Prerequisites

Install buf:
```bash
# macOS
brew install bufbuild/buf/buf

# Linux
curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-$(uname -s)-$(uname -m)" -o "/usr/local/bin/buf"
chmod +x "/usr/local/bin/buf"

# Or use Go
go install github.com/bufbuild/buf/cmd/buf@latest
```

## Basic Commands

### 1. Update dependencies (first time setup)
Download required dependencies:
```bash
cd proto
buf dep update
```

### 2. Lint protobuf files
Check if your `.proto` files are valid:
```bash
cd proto
buf lint
```

### 3. Generate Go code
Generate Go code from protobuf definitions:
```bash
cd proto
buf generate
```

This creates Go files in the same `proto/` directory that you can import in your Go code.

### 4. Check for breaking changes
Make sure your changes don't break existing code:
```bash
cd proto
buf breaking --against '.git#branch=main'
```

## What happens when you run `buf generate`?

1. buf reads `buf.gen.yaml` configuration
2. It finds all `.proto` files in the directory
3. It generates Go code using the protoc-gen-go and protoc-gen-go-grpc plugins
4. Generated files are placed in the `proto/` directory

## Generated Files

After running `buf generate`, you'll see:
- `calque.pb.go` - Go structs for your protobuf messages
- `calque_grpc.pb.go` - Go gRPC client and server code

## Using Generated Code

Import the generated code in your Go files:
```go
import calquepb "github.com/calque-ai/go-calque/proto"
```

## Common Workflow

1. Edit `calque.proto` to add/modify services or messages, If any depencies changed: Run `buf dep update` to download dependencies
2. Run `buf lint` to check for errors
3. Run `buf generate` to create Go code
4. Use the generated code in your Go application


## More Information

- [buf Documentation](https://docs.buf.build/)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers)
- [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
