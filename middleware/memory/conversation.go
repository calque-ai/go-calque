package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
)

// Message represents a single conversation message
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

// ConversationMemory provides structured conversation memory using a pluggable store
type ConversationMemory struct {
	store Store
}

// NewConversation creates a conversation memory with default in-memory store
func NewConversation() *ConversationMemory {
	return &ConversationMemory{
		store: NewInMemoryStore(),
	}
}

// NewConversationWithStore creates a conversation memory with custom store
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
func (cm *ConversationMemory) getConversation(key string) ([]Message, error) {
	data, err := cm.store.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return []Message{}, nil // Empty conversation
	}

	var conv conversationData
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
	}

	return conv.Messages, nil
}

// saveConversation stores conversation history to store
func (cm *ConversationMemory) saveConversation(key string, messages []Message) error {
	conv := conversationData{Messages: messages}
	data, err := json.Marshal(conv)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
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
func (cm *ConversationMemory) Input(key string) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		// Read current input
		inputBytes, err := io.ReadAll(r.Data)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		currentInput := strings.TrimSpace(string(inputBytes))
		if currentInput == "" {
			return fmt.Errorf("empty input for conversation")
		}

		// Get conversation history
		history, err := cm.getConversation(key)
		if err != nil {
			return fmt.Errorf("failed to get conversation: %w", err)
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
		updatedHistory := append(history, newMessage)

		if err := cm.saveConversation(key, updatedHistory); err != nil {
			return fmt.Errorf("failed to save conversation: %w", err)
		}

		// Write full context to output
		fullContext := strings.Join(contextParts, "\n")
		_, err = w.Data.Write([]byte(fullContext))
		return err
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
func (cm *ConversationMemory) Output(key string) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		// Create a buffer to capture the response
		var responseBuffer bytes.Buffer

		// Use TeeReader to stream to output while capturing
		teeReader := io.TeeReader(r.Data, &responseBuffer)

		// Stream through to output
		_, err := io.Copy(w.Data, teeReader)
		if err != nil {
			return fmt.Errorf("failed to stream response: %w", err)
		}

		// Store the captured response
		responseBytes := responseBuffer.Bytes()
		if len(responseBytes) > 0 {
			// Get current conversation
			history, err := cm.getConversation(key)
			if err != nil {
				return fmt.Errorf("failed to get conversation: %w", err)
			}

			// Add assistant response
			newMessage := Message{
				Role:    "assistant",
				Content: responseBytes,
			}
			updatedHistory := append(history, newMessage)

			if err := cm.saveConversation(key, updatedHistory); err != nil {
				return fmt.Errorf("failed to save conversation: %w", err)
			}
		}

		return nil
	})
}

// Clear removes all conversation history for a key
func (cm *ConversationMemory) Clear(key string) error {
	return cm.store.Delete(key)
}

// Info returns information about a conversation
func (cm *ConversationMemory) Info(key string) (messageCount int, exists bool, err error) {
	history, err := cm.getConversation(key)
	if err != nil {
		return 0, false, err
	}

	exists = cm.store.Exists(key)
	return len(history), exists, nil
}

// ListKeys returns all active conversation keys
func (cm *ConversationMemory) ListKeys() []string {
	return cm.store.List()
}
