# Streaming Chat Example

## Key Improvements

### **1. SSE Converter (`convert.ToSSE`)**

- ✅ **No custom OutputConverter needed** - Use built-in SSE converter
- ✅ **Configurable chunking** - Word, char, line, or complete streaming
- ✅ **Automatic event formatting** - Handles SSE protocol details
- ✅ **Error handling** - Built-in error event formatting

### **2. Contextual Memory Middleware**

- ✅ **No manual context extraction** - Automatically uses `user_id` from context
- ✅ **Single middleware call** - Combines input + output memory
- ✅ **Type-safe context keys** - Prevents collisions with proper context types
- ✅ **Flexible key sources** - Supports `user_id`, `session_id`, or custom extractors

## Usage

### Run the Chat Example

```bash
cd examples/streaming-chat-simple
go run main.go
```

### Key Features Demonstrated

- **🔄 SSE Streaming**: Using `convert.ToSSE()` converter
- **💭 Contextual Memory**: Using `ContextualInput()` and `ContextualOutput()`
- **🚦 Rate Limiting**: Same `ctrl.RateLimit()` middleware
- **🛡️ Agent Fallback**: Same `ctrl.Fallback()` middleware
- **📝 Type-Safe Context**: Using `memory.WithUserID()` for context

## Advanced Usage

### Custom Memory Key Extraction

```go
memoryMiddleware := memory.NewMemoryMiddleware(conversationMemory).
    WithKeyFunc(func(ctx context.Context) string {
        // Custom logic: use session_id + tenant_id
        sessionID := memory.GetSessionID(ctx)
        tenantID := getTenantID(ctx)
        return fmt.Sprintf("%s:%s", tenantID, sessionID)
    }).
    WithOptional() // Don't error if key missing

pipe.Use(memoryMiddleware.InputOnly())
```

### Different SSE Chunking Modes

```go
// Character-by-character streaming
sseConverter := convert.ToSSE(w, userID).WithChunkMode(convert.SSEChunkByChar)

// Line-by-line streaming
sseConverter := convert.ToSSE(w, userID).WithChunkMode(convert.SSEChunkByLine)

// Complete response as single event
sseConverter := convert.ToSSE(w, userID).WithChunkMode(convert.SSEChunkNone)
```
