# Distributed gRPC Example

This directory demonstrates comprehensive gRPC integration with the Calque-Pipe AI Agent Framework using the registry pattern for distributed service communication.

## What You'll Learn

### Registry Pattern Demo (`demonstrateRegistryPattern`)

- **Core Concepts**: Service discovery and connection management
- **Features Covered**:
  - Creating service registries with `grpcmw.NewRegistryHandler()`
  - Sequential execution with `ctrl.Chain()` for context propagation
  - Service chaining with `grpcmw.Call()`
  - Distributed flow orchestration across multiple services

### Type-Safe Calls Demo (`demonstrateTypeSafeCalls`)

- **Type Safety**: Compile-time type checking for gRPC calls
- **Features Covered**:
  - Type-safe service configuration with `grpcmw.ServiceWithTypes()`
  - Generic gRPC calls with `grpcmw.CallWithTypes()`
  - Protobuf message handling and validation
  - Error handling for type mismatches

### Streaming Services Demo (`demonstrateStreamingServices`)

- **Streaming Architecture**: Bidirectional streaming for real-time communication
- **Features Covered**:
  - Streaming service configuration with `grpcmw.StreamingService()`
  - Real-time data flow with `grpcmw.Stream()`
  - Chunked response processing
  - Streaming error handling

### Full Distributed Flow (`demonstrateFullDistributedFlow`)

- **Complete Integration**: End-to-end distributed processing
- **Features Covered**:
  - Service timeout configuration
  - Comprehensive error handling and retry logic
  - Multi-service orchestration
  - Performance optimization with gRPC

## Key gRPC Integration Concepts

- **Registry Pattern**: Services are discovered and connected dynamically through a centralized registry
- **Context Propagation**: Sequential execution ensures proper context flow between handlers
- **Type Safety**: Generic types provide compile-time validation for gRPC calls
- **Streaming Support**: Bidirectional streaming enables real-time communication
- **Error Handling**: Centralized gRPC error management with retry logic
- **Performance**: Binary protobuf serialization provides 30-50% smaller payloads than JSON

## Service Architecture

The example implements three distributed services:

1. **AI Service** (`:8080`): Handles AI-related requests and streaming chat
2. **Memory Service** (`:8081`): Manages memory operations and data storage  
3. **Tools Service** (`:8082`): Executes external tools and functions

## Running the Example

```bash
go run main.go
```

## Example Output

The examples demonstrate how data gets "calqued" (copied and transformed) through distributed gRPC services:

- **Registry Pattern**: Service chaining with proper context propagation
- **Type-Safe Calls**: Compile-time validated gRPC communication
- **Streaming Services**: Real-time bidirectional data flow
- **Full Distributed Flow**: Complete end-to-end distributed processing

Example output shows the chained execution:
```
Registry Pattern Result: Tool executed: Memory processed: AI Response: Hello from registry pattern!
```

## Key Benefits

- **Performance**: 30-50% smaller payloads compared to JSON
- **Type Safety**: Compile-time type checking for gRPC calls
- **Scalability**: Easy to add new services to the registry
- **Reliability**: Built-in retry logic and error handling
- **Flexibility**: Support for both unary and streaming calls

## Next Steps

After understanding distributed gRPC integration, explore other examples in the parent directory for more advanced features like:

- **AI Client Examples**: gRPC services with AI processing
- **Memory Management**: Distributed state management
- **Tool Calling**: gRPC-based function execution
- **Multi-Agent Systems**: Coordinated distributed agents
