package memory

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
)

// ContextMemory provides sliding window context memory using a pluggable store
type ContextMemory struct {
	store Store
}

// NewContext creates a context memory with default in-memory store
func NewContext() *ContextMemory {
	return &ContextMemory{
		store: NewInMemoryStore(),
	}
}

// NewContextWithStore creates a context memory with custom store
func NewContextWithStore(store Store) *ContextMemory {
	return &ContextMemory{
		store: store,
	}
}

// contextData holds the sliding window context information
type contextData struct {
	MaxTokens int    `json:"max_tokens"`
	Content   []byte `json:"content"`
}

// approximateTokenCount provides a rough token estimate
// Uses a more sophisticated approach: 1 token ≈ 3.5 chars for English
// Counts words, punctuation separately for better estimates
func approximateTokenCount(data []byte) int {
	text := string(data)

	// Rough heuristic based on OpenAI's guidelines:
	// - Average English word ≈ 1.3 tokens
	// - Punctuation and spaces ≈ additional tokens
	words := strings.Fields(text)

	// Base count from words
	tokenCount := float64(len(words)) * 1.3

	// Add tokens for punctuation and special chars
	for _, char := range text {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == ' ') {
			tokenCount += 0.5
		}
	}

	return int(tokenCount)
}

// trimToTokenLimit trims content to stay within token limit
// Tries to preserve sentence boundaries when possible
func trimToTokenLimit(content []byte, maxTokens int) []byte {
	if approximateTokenCount(content) <= maxTokens {
		return content
	}

	text := string(content)

	// Binary search to find the right cut point
	left, right := 0, len(text)
	bestCut := right

	for left < right {
		mid := (left + right) / 2
		if approximateTokenCount([]byte(text[mid:])) <= maxTokens {
			bestCut = mid
			right = mid
		} else {
			left = mid + 1
		}
	}

	// Try to cut at sentence boundary (. ! ?)
	cutText := text[bestCut:]
	for _, delimiter := range []string{". ", "! ", "? ", "\n\n", "\n"} {
		if idx := strings.Index(cutText, delimiter); idx != -1 {
			return []byte(cutText[idx+len(delimiter):])
		}
	}

	// Fall back to word boundary
	if spaceIdx := strings.Index(cutText, " "); spaceIdx != -1 {
		return []byte(cutText[spaceIdx+1:])
	}

	return []byte(cutText)
}

// getContext retrieves context data from store
func (cm *ContextMemory) getContext(key string) (*contextData, error) {
	data, err := cm.store.Get(key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil // No context found
	}

	var ctx contextData
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context: %w", err)
	}

	return &ctx, nil
}

// saveContext stores context data to store
func (cm *ContextMemory) saveContext(key string, ctx *contextData) error {
	data, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	return cm.store.Set(key, data)
}

// GetContext retrieves current context content for a key
func (cm *ContextMemory) GetContext(key string) ([]byte, error) {
	ctx, err := cm.getContext(key)
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		return nil, nil
	}

	// Return copy to prevent external modification
	result := make([]byte, len(ctx.Content))
	copy(result, ctx.Content)
	return result, nil
}

// AddToContext adds content to the sliding window
func (cm *ContextMemory) AddToContext(key string, content []byte, maxTokens int) error {
	// Get or create context
	ctx, err := cm.getContext(key)
	if err != nil {
		return err
	}

	if ctx == nil {
		ctx = &contextData{
			MaxTokens: maxTokens,
			Content:   make([]byte, 0),
		}
	}

	// Update max tokens if different
	ctx.MaxTokens = maxTokens

	// Append new content
	ctx.Content = append(ctx.Content, content...)

	// Trim to token limit
	ctx.Content = trimToTokenLimit(ctx.Content, maxTokens)

	return cm.saveContext(key, ctx)
}

// Clear removes all context for a key
func (cm *ContextMemory) Clear(key string) error {
	return cm.store.Delete(key)
}

// Info returns information about a context window
func (cm *ContextMemory) Info(key string) (tokenCount, maxTokens int, exists bool, err error) {
	ctx, err := cm.getContext(key)
	if err != nil {
		return 0, 0, false, err
	}

	if ctx == nil {
		exists = cm.store.Exists(key)
		return 0, 0, exists, nil
	}

	return approximateTokenCount(ctx.Content), ctx.MaxTokens, true, nil
}

// ListKeys returns all active context keys
func (cm *ContextMemory) ListKeys() []string {
	return cm.store.List()
}

// Input creates a middleware that maintains a sliding window of recent content
//
// This middleware:
// 1. Prepends recent context (up to maxTokens) to current input
// 2. Stores the current input in the sliding window
// 3. Automatically trims old content when token limit is exceeded
//
// Unlike Conversation which stores structured messages, Context maintains
// raw content flow - useful for maintaining context across multiple interactions.
//
// Example:
//
//	ctxMem := memory.NewContext()
//	flow.Use(ctxMem.Input("session123", 4000))
func (cm *ContextMemory) Input(key string, maxTokens int) core.Handler {
	return core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		// Read current input
		inputBytes, err := io.ReadAll(req.Data)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		currentInput := strings.TrimSpace(string(inputBytes))
		if currentInput == "" {
			return fmt.Errorf("empty input for context")
		}

		// Get existing context
		existingContext, err := cm.GetContext(key)
		if err != nil {
			return fmt.Errorf("failed to get context: %w", err)
		}

		// Build full context
		var fullContext strings.Builder

		// Add existing context if available
		if len(existingContext) > 0 {
			fullContext.Write(existingContext)
			fullContext.WriteString("\n")
		}

		// Add current input
		fullContext.WriteString(currentInput)

		// Store current input in sliding window
		if err := cm.AddToContext(key, []byte(currentInput+"\n"), maxTokens); err != nil {
			return fmt.Errorf("failed to add to context: %w", err)
		}

		// Write full context to output
		_, err = res.Data.Write([]byte(fullContext.String()))
		return err
	})
}

// Output creates a middleware that adds responses to the sliding window
//
// This middleware captures responses and adds them to the context window.
// Use this after your LLM handler to include responses in future context.
//
// Example:
//
//	ctxMem := memory.NewContext()
//	flow.
//		Use(ctxMem.Input("session123", 4000)).
//		Use(llm.Chat(provider)).
//		Use(ctxMem.Output("session123", 4000))
func (cm *ContextMemory) Output(key string, maxTokens int) core.Handler {
	return core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		// Create a buffer to capture the response
		var responseBuffer strings.Builder

		// Use TeeReader to stream to output while capturing
		teeReader := io.TeeReader(req.Data, &responseBuffer)

		// Stream through to output
		_, err := io.Copy(res.Data, teeReader)
		if err != nil {
			return fmt.Errorf("failed to stream response: %w", err)
		}

		// Add response to context window
		response := responseBuffer.String()
		if response != "" {
			if err := cm.AddToContext(key, []byte("Assistant: "+response+"\n"), maxTokens); err != nil {
				return fmt.Errorf("failed to add response to context: %w", err)
			}
		}

		return nil
	})
}
