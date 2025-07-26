package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/examples/providers/ollama"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	"github.com/calque-ai/calque-pipe/middleware/memory"
)

func main() {
	// Create a mock provider for demonstration
	// provider := mock.NewMockProvider("").WithStreamDelay(100) // Slower for demo

	provider, err := ollama.NewOllamaProvider("", "llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	fmt.Println("Memory Middleware Examples")

	conversationExample(provider)
	contextExample(provider)
	customStoreExample(provider)
}

// Example 1: Conversation memory - maintains structured chat history
func conversationExample(provider llm.LLMProvider) {
	fmt.Println("\n=== Conversation Memory Example ===")
	fmt.Println("Maintains structured chat history with user/assistant roles")

	convMem := memory.NewConversation() // Create conversation memory with default in-memory store

	// Create pipe with conversation memory
	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 100)).
		Use(convMem.Input("user123")). // Store input with user ID
		Use(flow.Logger("WITH_HISTORY", 100)).
		Use(llm.SystemPrompt("You are a helpful coding assistant. Keep responses brief.")).
		Use(llm.Chat(provider)).       // Get LLM response
		Use(convMem.Output("user123")) // Store response with user ID

	// Simulate a conversation
	inputs := []string{
		"What is Go?",
		"How do I handle errors in Go?",
		"Can you show me an example?",
	}

	for i, input := range inputs {
		fmt.Printf("\n--- Message %d ---\n", i+1)
		result, err := pipe.Run(context.Background(), input)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("User: %s\n", input)
		fmt.Printf("Assistant: %s\n", result)
	}

	fmt.Println("\n--- Conversation Summary ---")
	msgCount, exists, err := convMem.Info("user123")
	if err == nil {
		fmt.Printf("Total messages: %d, exists: %v\n", msgCount, exists)
	}

	// List all active conversations
	keys := convMem.ListKeys()
	fmt.Printf("Active conversation keys: %v\n", keys)

	// Clean up
	convMem.Clear("user123")
	fmt.Println("Conversation cleared")
}

// Example 2: Custom store (multiple conversation memories with different stores)
func customStoreExample(provider llm.LLMProvider) {
	fmt.Println("\n=== Custom Store Example ===")

	// Create separate stores for different use cases
	userStore := memory.NewInMemoryStore()
	adminStore := memory.NewInMemoryStore()

	// Create conversation memories with custom stores
	userConvMem := memory.NewConversationWithStore(userStore)
	adminConvMem := memory.NewConversationWithStore(adminStore)

	// User pipe
	userPipe := core.New()
	userPipe.
		Use(userConvMem.Input("session1")).
		Use(llm.SystemPrompt("You are a helpful user assistant.")).
		Use(llm.Chat(provider)).
		Use(userConvMem.Output("session1"))

	// Admin pipe
	adminPipe := core.New()
	adminPipe.
		Use(adminConvMem.Input("admin1")).
		Use(llm.SystemPrompt("You are a technical admin assistant.")).
		Use(llm.Chat(provider)).
		Use(adminConvMem.Output("admin1"))

	// Test user conversation
	fmt.Println("\n--- User Conversation ---")
	result, _ := userPipe.Run(context.Background(), "How do I reset my password?")
	fmt.Printf("User: How do I reset my password?\n")
	fmt.Printf("Assistant: %s\n", result)

	// Test admin conversation
	fmt.Println("\n--- Admin Conversation ---")
	result, _ = adminPipe.Run(context.Background(), "Show me server logs")
	fmt.Printf("Admin: Show me server logs\n")
	fmt.Printf("Assistant: %s\n", result)

	// Show isolated stores
	userKeys := userConvMem.ListKeys()
	adminKeys := adminConvMem.ListKeys()

	fmt.Printf("\nUser store keys: %v\n", userKeys)
	fmt.Printf("Admin store keys: %v\n", adminKeys)
	fmt.Println("Stores are completely isolated!")
}

// Example 3: Context memory - maintains sliding window of recent content
func contextExample(provider llm.LLMProvider) {
	fmt.Println("\n=== Context Memory Example ===")
	fmt.Println("Maintains sliding window of recent content (token-limited)")

	contextMem := memory.NewContext() // Create context memory with default in-memory store

	// Create pipe with context memory (small limit for demo)
	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 100)).
		Use(contextMem.Input("session456", 200)). // Keep last 200 tokens
		Use(flow.Logger("WITH_CONTEXT", 100)).
		Use(llm.SystemPrompt("You are a helpful assistant. Be concise.")).
		Use(llm.Chat(provider)).                  // Get LLM response
		Use(contextMem.Output("session456", 200)) // Store response in context

	// Simulate multiple interactions
	inputs := []string{
		"I'm working on a Go project.",
		"It's a web API for user management.",
		"What database should I use?",
		"How do I handle authentication?",
	}

	for i, input := range inputs {
		fmt.Printf("\n--- Interaction %d ---\n", i+1)

		// Show context info before processing
		tokenCount, maxTokens, exists, err := contextMem.Info("session456")
		if err != nil {
			fmt.Printf("Context: %d/%d tokens\n exists: %v", tokenCount, maxTokens, exists)
		}

		result, err := pipe.Run(context.Background(), input)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("User: %s\n", input)
		fmt.Printf("Assistant: %s\n", result)
	}

	// Show final context info
	fmt.Println("\n--- Context Summary ---")
	tokenCount, maxTokens, exists, err := contextMem.Info("session456")
	if err != nil {
		fmt.Printf("Context: %d/%d tokens\n exists: %v", tokenCount, maxTokens, exists)
	}

	// List all active contexts
	keys := contextMem.ListKeys()
	fmt.Printf("Active context keys: %v\n", keys)

	// Clean up
	contextMem.Clear("session456")
	fmt.Println("Context cleared")
}
