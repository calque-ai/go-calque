package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/memory"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
)

// TestConversationMemory tests conversation memory functionality
func TestConversationMemory(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Go is a statically typed, compiled programming language designed at Google.")
	agent := ai.Agent(mockClient)

	// Create conversation memory
	convMem := memory.NewConversation()

	// Create pipe with conversation memory
	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Head("INPUT", 100)).
		Use(convMem.Input("user123")).
		Use(inspect.Head("WITH_HISTORY", 100)).
		Use(prompt.System("You are a helpful coding assistant. Keep responses brief.")).
		Use(agent).
		Use(convMem.Output("user123"))

	// Simulate a conversation
	inputs := []string{
		"What is Go?",
		"How do I handle errors in Go?",
		"Can you show me an example?",
	}

	for i, input := range inputs {
		t.Run(input, func(t *testing.T) {
			var result string
			err := pipe.Run(context.Background(), input, &result)
			if err != nil {
				t.Fatalf("Conversation step %d failed: %v", i+1, err)
			}

			// Verify AI response
			if !strings.Contains(strings.ToLower(result), "go") {
				t.Errorf("Expected response to mention Go, got: %s", result)
			}
		})
	}

	// Verify conversation memory
	ctx := context.Background()
	msgCount, exists, err := convMem.Info(ctx, "user123")
	if err != nil {
		t.Fatalf("Failed to get conversation info: %v", err)
	}
	if !exists {
		t.Error("Expected conversation to exist")
	}
	if msgCount < 3 {
		t.Errorf("Expected at least 3 messages, got %d", msgCount)
	}

	// List active conversations
	keys := convMem.ListKeys()
	if len(keys) == 0 {
		t.Error("Expected at least one active conversation")
	}

	// Clean up
	convMem.Clear("user123")
}

// TestContextMemory tests context memory functionality
func TestContextMemory(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("Here's a simple Go function: func hello() string { return \"Hello, World!\" }")
	agent := ai.Agent(mockClient)

	// Create context memory
	ctxMem := memory.NewContext()

	// Create pipe with context memory
	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Head("INPUT", 100)).
		Use(ctxMem.Input("session456", 2000)).
		Use(prompt.System("You are a helpful coding assistant.")).
		Use(agent).
		Use(ctxMem.Output("session456", 2000))

	// Test context memory with multiple inputs
	inputs := []string{
		"Write a hello function in Go",
		"Now add error handling to it",
		"What's the difference between this and the previous version?",
	}

	for i, input := range inputs {
		t.Run(input, func(t *testing.T) {
			var result string
			err := pipe.Run(context.Background(), input, &result)
			if err != nil {
				t.Fatalf("Context memory step %d failed: %v", i+1, err)
			}

			// Verify AI response
			if len(result) == 0 {
				t.Error("Expected non-empty AI response")
			}
		})
	}

	// Verify context memory
	ctx2 := context.Background()
	tokenCount, maxTokens, exists, err := ctxMem.Info(ctx2, "session456")
	if err != nil {
		t.Fatalf("Failed to get context info: %v", err)
	}
	if !exists {
		t.Error("Expected context to exist")
	}
	if tokenCount < 10 {
		t.Errorf("Expected at least 10 tokens, got %d", tokenCount)
	}
	if maxTokens != 2000 {
		t.Errorf("Expected max tokens 2000, got %d", maxTokens)
	}

	// Clean up
	ctxMem.Clear("session456")
}

// TestMemoryStore tests memory store functionality
func TestMemoryStore(t *testing.T) {
	t.Parallel()

	// Create in-memory store
	store := memory.NewInMemoryStore()

	// Test storing and retrieving data
	testData := []byte("test message content")
	err := store.Set("key1", testData)
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	// Retrieve data
	result, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Failed to retrieve data: %v", err)
	}

	if string(result) != string(testData) {
		t.Errorf("Expected '%s', got '%s'", string(testData), string(result))
	}

	// Test non-existent key
	result, err = store.Get("nonexistent")
	if err != nil {
		t.Fatalf("Failed to get non-existent key: %v", err)
	}
	if result != nil {
		t.Error("Expected nil result for non-existent key")
	}

	// Test clearing
	err = store.Delete("key1")
	if err != nil {
		t.Fatalf("Failed to clear data: %v", err)
	}

	// Verify cleared
	result, err = store.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get cleared key: %v", err)
	}
	if result != nil {
		t.Error("Expected nil result after clearing data")
	}
}

// TestMemoryPipeline tests memory in a complete pipeline
func TestMemoryPipeline(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("This is a response with context from previous messages.")
	agent := ai.Agent(mockClient)

	// Create conversation memory
	convMem := memory.NewConversation()

	// Create complex pipeline
	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Print("START")).
		Use(convMem.Input("user789")).
		Use(prompt.System("You are a helpful assistant. Use context from previous messages.")).
		Use(agent).
		Use(convMem.Output("user789")).
		Use(inspect.Print("END"))

	// Test multiple conversation turns
	conversation := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello, my name is Alice",
			expected: "response",
		},
		{
			input:    "What's my name?",
			expected: "response",
		},
		{
			input:    "Tell me about yourself",
			expected: "response",
		},
	}

	for i, turn := range conversation {
		t.Run(turn.input, func(t *testing.T) {
			var result string
			err := pipe.Run(context.Background(), turn.input, &result)
			if err != nil {
				t.Fatalf("Conversation turn %d failed: %v", i+1, err)
			}

			// Verify response contains expected content
			if !strings.Contains(strings.ToLower(result), turn.expected) {
				t.Errorf("Expected response to contain '%s', got: %s", turn.expected, result)
			}
		})
	}

	// Verify conversation history
	ctx3 := context.Background()
	msgCount, exists, err := convMem.Info(ctx3, "user789")
	if err != nil {
		t.Fatalf("Failed to get conversation info: %v", err)
	}
	if !exists {
		t.Error("Expected conversation to exist")
	}
	if msgCount < 3 {
		t.Errorf("Expected at least 3 messages, got %d", msgCount)
	}

	// Clean up
	convMem.Clear("user789")
}

// TestMemoryConcurrency tests memory operations under concurrent access
func TestMemoryConcurrency(t *testing.T) {
	t.Parallel()

	// Create conversation memory
	convMem := memory.NewConversation()

	// Test concurrent access to different user IDs
	const numUsers = 5
	const messagesPerUser = 3
	results := make(chan error, numUsers)

	for userID := 0; userID < numUsers; userID++ {
		go func(id int) {
			userKey := fmt.Sprintf("user%d", id)

			// Create simple pipeline for this user
			pipe := calque.NewFlow()
			pipe.Use(convMem.Input(userKey))
			pipe.Use(convMem.Output(userKey))

			for msgID := 0; msgID < messagesPerUser; msgID++ {
				input := fmt.Sprintf("Message %d from user %d", msgID, id)
				var result string
				err := pipe.Run(context.Background(), input, &result)
				if err != nil {
					results <- fmt.Errorf("user %d, message %d failed: %v", id, msgID, err)
					return
				}
			}
			results <- nil
		}(userID)
	}

	// Collect results
	for i := 0; i < numUsers; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent test failed: %v", err)
		}
	}

	// Verify all conversations were created
	ctx4 := context.Background()
	for userID := 0; userID < numUsers; userID++ {
		userKey := fmt.Sprintf("user%d", userID)
		msgCount, exists, err := convMem.Info(ctx4, userKey)
		if err != nil {
			t.Errorf("Failed to get info for %s: %v", userKey, err)
		}
		if !exists {
			t.Errorf("Expected conversation %s to exist", userKey)
		}
		if msgCount < messagesPerUser {
			t.Errorf("Expected at least %d messages for %s, got %d", messagesPerUser, userKey, msgCount)
		}

		// Clean up
		convMem.Clear(userKey)
	}
}
