// Package main demonstrates memory management capabilities with the calque framework.
// It showcases conversation memory, context memory, and custom storage backends
// to maintain state and history across multiple AI interactions.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/go-calque/examples/memory/badger"
	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/memory"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
)

func main() {
	// Create a mock provider for demonstration
	// provider := mock.NewMockProvider("").WithStreamDelay(100) // Slower for demo

	client, err := ollama.New("llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	fmt.Println("Memory Middleware Examples")

	conversationExample(client)
	badgerConversationExample(client)
	contextExample(client)
	customStoreExample(client)
}

// Example 1: Conversation memory - maintains structured chat history
// Maintains structured chat history with user/assistant roles
func conversationExample(client ai.Client) {
	fmt.Println("\n=== Conversation Memory Example ===")

	convMem := memory.NewConversation() // Create conversation memory with simple in-memory store

	// Create pipe with conversation memory
	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Head("INPUT", 100)).
		Use(convMem.Input("user123")). // Store input with user ID
		Use(inspect.Head("WITH_HISTORY", 100)).
		Use(prompt.System("You are a helpful coding assistant. Keep responses brief.")).
		Use(ai.Agent(client)).         // Get LLM response
		Use(convMem.Output("user123")) // Store response with user ID

	// Simulate a conversation
	inputs := []string{
		"What is Go?",
		"How do I handle errors in Go?",
		"Can you show me an example?",
	}

	// run the pipeline on each simulated conversation input.
	for i, input := range inputs {
		fmt.Printf("\n--- Message %d ---\n", i+1)
		var result string
		err := pipe.Run(context.Background(), input, result)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("User: %s\n", input)
		fmt.Printf("Assistant: %s\n", result)
	}

	fmt.Println("\n--- Conversation Summary ---")
	ctx := context.Background()
	msgCount, exists, err := convMem.Info(ctx, "user123")
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

// Example 2: Using a 3rd party (badgerDB) database for storage.
func badgerConversationExample(client ai.Client) {
	// Create Badger store
	badgerStore, err := badger.NewStore("./conversations.db")
	if err != nil {
		log.Fatal(err)
	}

	// Use with conversation memory
	convMem := memory.NewConversationWithStore(badgerStore)

	// Use in pipeline
	pipe := calque.NewFlow()
	pipe.
		Use(convMem.Input("user123")).
		Use(inspect.Head("WITH_HISTORY", 100)).
		Use(prompt.System("You are a helpful coding assistant. Keep responses brief.")).
		Use(ai.Agent(client)).
		Use(convMem.Output("user123"))

	// Simulate a conversation
	inputs := []string{
		"What is Go?",
		"How do I handle errors in Go?",
		"Can you show me an example?",
	}

	for i, input := range inputs {
		fmt.Printf("\n--- Message %d ---\n", i+1)
		var result string
		err := pipe.Run(context.Background(), input, result)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("User: %s\n", input)
		fmt.Printf("Assistant: %s\n", result)
	}

	fmt.Println("\n--- Conversation Summary ---")
	ctx2 := context.Background()
	msgCount, exists, err := convMem.Info(ctx2, "user123")
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

// Example 3: Custom store (multiple conversation memories with different stores)
func customStoreExample(client ai.Client) {
	fmt.Println("\n=== Custom Store Example ===")

	// Create separate stores for different use cases
	userStore := memory.NewInMemoryStore()
	adminStore := memory.NewInMemoryStore()

	// Create conversation memories with custom stores
	userConvMem := memory.NewConversationWithStore(userStore)
	adminConvMem := memory.NewConversationWithStore(adminStore)

	// User pipe
	userPipe := calque.NewFlow()
	userPipe.
		Use(userConvMem.Input("session1")).
		Use(prompt.System("You are a helpful user assistant.")).
		Use(ai.Agent(client)).
		Use(userConvMem.Output("session1"))

	// Admin pipe
	adminPipe := calque.NewFlow()
	adminPipe.
		Use(adminConvMem.Input("admin1")).
		Use(prompt.System("You are a technical admin assistant.")).
		Use(ai.Agent(client)).
		Use(adminConvMem.Output("admin1"))

	// Test user conversation
	fmt.Println("\n--- User Conversation ---")
	var result string
	_ = userPipe.Run(context.Background(), "How do I reset my password?", &result)
	fmt.Printf("User: How do I reset my password?\n")
	fmt.Printf("Assistant: %s\n", result)

	// Test admin conversation
	fmt.Println("\n--- Admin Conversation ---")
	_ = adminPipe.Run(context.Background(), "Show me server logs", &result)
	fmt.Printf("Admin: Show me server logs\n")
	fmt.Printf("Assistant: %s\n", result)

	// Show isolated stores
	userKeys := userConvMem.ListKeys()
	adminKeys := adminConvMem.ListKeys()

	fmt.Printf("\nUser store keys: %v\n", userKeys)
	fmt.Printf("Admin store keys: %v\n", adminKeys)
	fmt.Println("Stores are completely isolated!")
}

// Example 4: Context memory - maintains sliding window of recent content
// Maintains sliding window of recent content (token-limited)
func contextExample(client ai.Client) {
	fmt.Println("\n=== Context Memory Example ===")

	contextMem := memory.NewContext() // Create context memory with default in-memory store

	// Create pipe with context memory (small limit for demo)
	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Head("INPUT", 100)).
		Use(contextMem.Input("session456", 200)). // Keep last 200 tokens
		Use(inspect.Head("WITH_CONTEXT", 100)).
		Use(prompt.System("You are a helpful assistant. Be concise.")).
		Use(ai.Agent(client)).                    // Get agent response
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
		ctx3 := context.Background()
		tokenCount, maxTokens, exists, err := contextMem.Info(ctx3, "session456")
		if err != nil {
			fmt.Printf("Context: %d/%d tokens\n exists: %v", tokenCount, maxTokens, exists)
		}

		var result string
		err = pipe.Run(context.Background(), input, &result)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("User: %s\n", input)
		fmt.Printf("Assistant: %s\n", result)
	}

	// Show final context info
	fmt.Println("\n--- Context Summary ---")
	ctx4 := context.Background()
	tokenCount, maxTokens, exists, err := contextMem.Info(ctx4, "session456")
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
