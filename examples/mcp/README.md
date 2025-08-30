# MCP Examples

This directory contains examples demonstrating Model Context Protocol (MCP) integration with the Calque-Pipe AI Agent Framework.

## What You'll Learn

### Example 1: Basic Tool Calling (`runBasicExample`)

- **Core Concepts**: MCP client setup and simple tool calling
- **Features Covered**:
  - Creating MCP client with stdio transport
  - Calling tools with JSON arguments
  - Basic error handling
  - Tool result processing

### Example 2: Realistic Usage (`runRealisticExample`)

- **Comprehensive MCP Integration**: Demonstrates multiple MCP capabilities
- **Features Covered**:
  - **Tool Calling**: Search operations with parameters
  - **Resource Access**: Static documentation retrieval
  - **Resource Templates**: Dynamic configuration file access
  - RAG pattern for augmenting queries with resource content

### Example 3: Advanced Features (`runAdvancedExample`)

- **Advanced MCP Capabilities**: Progress tracking and prompt templates
- **Features Covered**:
  - **Prompt Templates**: Dynamic prompt generation with arguments
  - **Progress Tracking**: Tool execution progress monitoring
  - **Callback Handling**: Asynchronous progress notifications
  - Advanced error handling and metadata processing

## Key MCP Concepts

- **MCP Protocol**: Standardized way for AI applications to access tools, resources, and prompts
- **Transport Types**:
  - **Stdio**: Communication via stdin/stdout with subprocess
  - **SSE**: HTTP-based transport with Server-Sent Events
- **Tool Calling**: Execute functions on MCP servers with structured arguments
- **Resource Access**: Fetch content from MCP servers (static or template-based)
- **Prompt Templates**: Generate dynamic prompts with variable substitution
- **Progress Tracking**: Monitor long-running operations with callbacks

## Running the Examples

```bash
go run main.go
```

### Prerequisites

- Go 1.19 or later
- MCP Go SDK: `go get github.com/modelcontextprotocol/go-sdk`

## Example Output

The examples demonstrate how MCP enables:

- **Tool Integration**: Calling external tools through standardized protocol
- **Resource Augmentation**: Enhancing queries with contextual data (RAG pattern)
- **Dynamic Prompts**: Generating prompts based on runtime parameters
- **Progress Monitoring**: Tracking long-running operations with real-time updates
