# Memory Examples

This directory demonstrates memory middleware functionality in the Calque-Pipe AI Agent Framework. Memory middleware provides conversation history and context management for AI agents, enabling stateful interactions across multiple pipeline executions.

## What You'll Learn

### Basic Conversation Memory (`conversationExample`)

- **Core Concepts**: Structured conversation history with user/assistant roles
- **Features Covered**:
  - Creating conversation memory with `memory.NewConversation()`
  - Storing user inputs with `convMem.Input(userID)`
  - Capturing assistant responses with `convMem.Output(userID)`
  - Managing conversation state and metadata
  - Retrieving conversation information and statistics

### Persistent Storage with Badger (`badgerConversationExample`)

- **External Storage**: Using BadgerDB for persistent conversation storage
- **Features Covered**:
  - Custom store implementation with `memory.NewConversationWithStore()`
  - Persistent storage that survives application restarts
  - Third-party database integration patterns
  - Storage adapter interface compliance

### Context Memory Management (`contextExample`)

- **Token-Limited Memory**: Sliding window context management
- **Features Covered**:
  - Creating context memory with `memory.NewContext()`
  - Token-limited context windows for efficient memory usage
  - Automatic context pruning and management
  - Context size monitoring and optimization

### Custom Store Isolation (`customStoreExample`)

- **Multi-Store Architecture**: Separate storage backends for different use cases
- **Features Covered**:
  - Multiple isolated memory stores
  - Role-based conversation separation (user vs admin)
  - Independent pipeline configurations
  - Store isolation and data privacy

## Key Memory Concepts

- **Conversation Memory**: Maintains structured chat history with user/assistant message roles
- **Context Memory**: Provides sliding window context management with token limits
- **Pluggable Storage**: Interface-based storage allows any backend implementation
- **State Management**: Persistent state across pipeline executions
- **Session Isolation**: Separate memory spaces for different users/sessions

## When to Use Memory Middleware

- Building chatbots or conversational AI agents
- Maintaining context across multiple user interactions
- Creating stateful AI workflows and decision trees
- Implementing user session management
- Building multi-turn conversation systems
- Managing context windows for large language models

## Storage Adapter Implementation

The `badger/` directory contains a complete example of implementing the `memory.Store` interface for persistent storage:

```go
// Implement all interface methods
func (s *BadgerStore) Get(key string) ([]byte, error) { ... }
func (s *BadgerStore) Set(key string, value []byte) error { ... }
func (s *BadgerStore) Delete(key string) error { ... }
func (s *BadgerStore) List() []string { ... }
func (s *BadgerStore) Exists(key string) bool { ... }

// Verify interface compliance at compile time
var _ memory.Store = (*BadgerStore)(nil)
```

This pattern can be used to implement adapters for:
- Redis for distributed storage
- SQLite for embedded databases  
- LevelDB for high-performance storage
- PostgreSQL for enterprise solutions

## Running the Examples

```bash
go run main.go
```

### Prerequisites for LLM Integration

- Install [Ollama](https://ollama.ai)
- Pull the model: `ollama pull llama3.2:1b`
- Ensure Ollama is running on `localhost:11434`

### Prerequisites for Badger Example

```bash
go mod tidy  # Install BadgerDB dependency
```

## Example Output

The examples demonstrate how memory gets "calqued" (maintained and retrieved) across pipeline executions, showing:

- Conversation history building across multiple interactions
- Persistent storage with external databases
- Context window management and token limiting
- Store isolation for different user sessions
- Memory statistics and conversation metadata

## Next Steps

After understanding memory middleware, explore other examples in the parent directory for advanced features like caching, prompt management, and complex agent workflows.