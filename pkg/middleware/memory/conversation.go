package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Message represents a single conversation message.
//
// Contains role ("user", "assistant", "system") and raw content bytes.
// Supports any content type - text, JSON, binary data.
//
// Example:
//
//	msg := Message{Role: "user", Content: []byte("Hello")}
//	fmt.Println(msg.Text()) // "Hello"
type Message struct {
	Role    string // "user", "assistant", "system"
	Content []byte // Raw content - can be text, JSON, binary, etc.
}

// Text returns the content as a string
func (m Message) Text() string {
	return string(m.Content)
}

// String implements the Stringer interface
func (m Message) String() string {
	return fmt.Sprintf("%s: %s", m.Role, m.Text())
}

// ConversationMemory provides structured conversation memory using a pluggable store.
//
// Manages conversation history with configurable storage backends.
// Supports multiple concurrent conversations identified by keys.
//
// Example:
//
//	mem := memory.NewConversation()
//	flow.Use(mem.Input("user123")).Use(llm).Use(mem.Output("user123"))
type ConversationMemory struct {
	store Store
}

// NewConversation creates a conversation memory with default in-memory store.
//
// Input: none
// Output: *ConversationMemory with in-memory storage
// Behavior: Creates fresh conversation manager
//
// Uses built-in memory store that persists for application lifetime.
// For persistent storage, use NewConversationWithStore.
//
// Example:
//
//	mem := memory.NewConversation()
//	flow.Use(mem.Input("session1"))
func NewConversation() *ConversationMemory {
	return &ConversationMemory{
		store: NewInMemoryStore(),
	}
}

// NewConversationWithStore creates a conversation memory with custom store.
//
// Input: Store implementation
// Output: *ConversationMemory with custom storage
// Behavior: Creates conversation manager with provided storage
//
// Allows pluggable storage backends for persistence, Redis, databases, etc.
//
// Example:
//
//	redisStore := memory.NewRedisStore("localhost:6379")
//	mem := memory.NewConversationWithStore(redisStore)
func NewConversationWithStore(store Store) *ConversationMemory {
	return &ConversationMemory{
		store: store,
	}
}

// conversationData holds the structured conversation history
type conversationData struct {
	Messages []Message `json:"messages"`
}

// getConversation retrieves conversation history from store
func (cm *ConversationMemory) getConversation(ctx context.Context, key string) ([]Message, error) {
	data, err := cm.store.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return []Message{}, nil // Empty conversation
	}

	var conv conversationData
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, calque.WrapErr(ctx, err, "failed to unmarshal conversation")
	}

	return conv.Messages, nil
}

// saveConversation stores conversation history to store
func (cm *ConversationMemory) saveConversation(ctx context.Context, key string, messages []Message) error {
	conv := conversationData{Messages: messages}
	data, err := json.Marshal(conv)
	if err != nil {
		return calque.WrapErr(ctx, err, "failed to marshal conversation")
	}

	return cm.store.Set(key, data)
}

// Input creates a middleware that prepends conversation history and stores user input
//
// This middleware:
// 1. Retrieves conversation history for the key
// 2. Prepends it to current input in formatted style
// 3. Stores current input as a "user" message
//
// Example:
//
//	convMem := memory.NewConversation()
//	flow.Use(convMem.Input("user123"))
func (cm *ConversationMemory) Input(key string) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Read current input
		var input string
		err := calque.Read(r, &input)
		if err != nil {
			return calque.WrapErr(r.Context, err, "failed to read input")
		}

		currentInput := strings.TrimSpace(input)
		if currentInput == "" {
			return calque.NewErr(r.Context, "empty input for conversation")
		}

		// Get conversation history
		history, err := cm.getConversation(r.Context, key)
		if err != nil {
			return calque.WrapErr(r.Context, err, "failed to get conversation")
		}

		// Build conversation context
		var contextParts []string

		// Add previous messages
		for _, msg := range history {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", msg.Role, msg.Text()))
		}

		// Add current user message
		contextParts = append(contextParts, fmt.Sprintf("user: %s", currentInput))

		// Store current input as user message
		newMessage := Message{
			Role:    "user",
			Content: []byte(currentInput),
		}
		updatedHistory := make([]Message, len(history), len(history)+1)
		copy(updatedHistory, history)
		updatedHistory = append(updatedHistory, newMessage)

		if err := cm.saveConversation(r.Context, key, updatedHistory); err != nil {
			return calque.WrapErr(r.Context, err, "failed to save conversation")
		}

		// Write full context to output
		fullContext := strings.Join(contextParts, "\n")
		return calque.Write(w, fullContext)
	})
}

// Output creates a middleware that captures and stores assistant responses
//
// This middleware captures responses and stores them as "assistant" messages.
// Uses TeeReader to stream through while capturing.
//
// Example:
//
//	convMem := memory.NewConversation()
//	flow.
//		Use(convMem.Input("user123")).
//		Use(llm.Chat(provider)).
//		Use(convMem.Output("user123"))
func (cm *ConversationMemory) Output(key string) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Create a buffer to capture the response
		var responseBuffer bytes.Buffer

		// Use TeeReader to stream to output while capturing
		teeReader := io.TeeReader(r.Data, &responseBuffer)

		// Stream through to output
		_, err := io.Copy(w.Data, teeReader)
		if err != nil {
			return calque.WrapErr(r.Context, err, "failed to stream response")
		}

		// Store the captured response
		responseBytes := responseBuffer.Bytes()
		if len(responseBytes) > 0 {
			// Get current conversation
			history, err := cm.getConversation(r.Context, key)
			if err != nil {
				return calque.WrapErr(r.Context, err, "failed to get conversation")
			}

			// Add assistant response
			newMessage := Message{
				Role:    "assistant",
				Content: responseBytes,
			}
			updatedHistory := make([]Message, len(history), len(history)+1)
			copy(updatedHistory, history)
			updatedHistory = append(updatedHistory, newMessage)

			if err := cm.saveConversation(r.Context, key, updatedHistory); err != nil {
				return calque.WrapErr(r.Context, err, "failed to save conversation")
			}
		}

		return nil
	})
}

// Clear removes all conversation history for a key.
//
// Input: conversation key string
// Output: error if deletion fails
// Behavior: Permanently deletes conversation data
//
// Example:
//
//	err := mem.Clear("user123")
func (cm *ConversationMemory) Clear(key string) error {
	return cm.store.Delete(key)
}

// Info returns information about a conversation.
//
// Input: conversation key string
// Output: message count, existence flag, error
// Behavior: Non-destructive inspection of conversation state
//
// Example:
//
//	count, exists, err := mem.Info("user123")
//	if exists { fmt.Printf("%d messages", count) }
func (cm *ConversationMemory) Info(ctx context.Context, key string) (messageCount int, exists bool, err error) {
	history, err := cm.getConversation(ctx, key)
	if err != nil {
		return 0, false, err
	}

	exists = cm.store.Exists(key)
	return len(history), exists, nil
}

// ListKeys returns all active conversation keys.
//
// Input: none
// Output: slice of conversation key strings
// Behavior: Lists all stored conversation identifiers
//
// Example:
//
//	keys := mem.ListKeys()
//	for _, key := range keys { fmt.Println(key) }
func (cm *ConversationMemory) ListKeys() []string {
	return cm.store.List()
}

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

const (
	// MemoryKey is the context key for memory identification
	MemoryKey ContextKey = "memory_key"
)

// WithKey adds a memory key to the context.
//
// Input: context and key string
// Output: context with embedded key
// Behavior: Embeds conversation key for downstream handlers
//
// Example:
//
//	ctx := memory.WithKey(context.Background(), "user123")
//	req := calque.NewRequest(ctx, input)
func WithKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, MemoryKey, key)
}

// GetKey extracts memory key from context.
//
// Input: context
// Output: key string (empty if not found)
// Behavior: Retrieves conversation key from context
//
// Example:
//
//	key := memory.GetKey(req.Context)
//	if key != "" { /* use key */ }
func GetKey(ctx context.Context) string {
	if key, ok := ctx.Value(MemoryKey).(string); ok {
		return key
	}
	return ""
}

// InputFromContext creates input middleware that uses key from context.
//
// Input: user message (requires memory key in context)
// Output: formatted conversation with history
// Behavior: BUFFERED - loads and prepends conversation history
//
// Automatically extracts conversation key from request context.
//
// Example:
//
//	flow.Use(memory.WithKeyMiddleware("user123"))
//	flow.Use(mem.InputFromContext())
func (cm *ConversationMemory) InputFromContext() calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		key := GetKey(req.Context)
		if key == "" {
			return calque.NewErr(req.Context, "no memory key found in context for memory input")
		}

		return cm.Input(key).ServeFlow(req, res)
	})
}

// OutputFromContext creates output middleware that uses key from context.
//
// Input: assistant response (requires memory key in context)
// Output: same response (pass-through)
// Behavior: STREAMING - captures and stores while streaming
//
// Automatically extracts conversation key from request context.
//
// Example:
//
//	flow.Use(llm).Use(mem.OutputFromContext())
func (cm *ConversationMemory) OutputFromContext() calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		key := GetKey(req.Context)
		if key == "" {
			return calque.NewErr(req.Context, "no memory key found in context for memory output")
		}

		return cm.Output(key).ServeFlow(req, res)
	})
}

// WrapFromContext wraps a handler with both input and output memory using key from context.
//
// Input: user message (requires memory key in context)
// Output: assistant response with memory captured
// Behavior: BUFFERED input, STREAMING output
//
// Convenience wrapper that applies Input → Handler → Output flow.
//
// Example:
//
//	wrapped := mem.WrapFromContext(llm.Chat(client))
//	flow.Use(wrapped)
func (cm *ConversationMemory) WrapFromContext(handler calque.Handler) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		key := GetKey(req.Context)
		if key == "" {
			return calque.NewErr(req.Context, "no memory key found in context for memory")
		}

		// Create a flow: Input → Handler → Output
		memoryFlow := calque.NewFlow().
			Use(cm.Input(key)).
			Use(handler).
			Use(cm.Output(key))

		return memoryFlow.Run(req.Context, req.Data, res.Data)
	})
}
