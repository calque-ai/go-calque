package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/examples/providers/ollama"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
)

func main() {
	// Create provider for all examples
	provider, err := ollama.NewOllamaProvider("", "llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	// Run all examples
	basicTemplateExample(provider)
	templateWithDataExample(provider)
	systemPromptExample(provider)
	chatPromptExample(provider)
}

// Example 1: Basic template usage
func basicTemplateExample(provider llm.LLMProvider) {
	fmt.Println("=== Basic Template Example ===")

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT")).
		Use(llm.Prompt("You are a helpful assistant. {{.Input}}")).
		Use(flow.Logger("PROMPT")).
		Use(llm.Chat(provider))

	result, err := pipe.Run(context.Background(), "What is Golang?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}

// Example 2: Template with additional data
func templateWithDataExample(provider llm.LLMProvider) {
	fmt.Println("=== Template with Data Example ===")

	params := map[string]any{
		"Role":     "Senior Software Engineer",
		"Language": "Go",
	}

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT")).
		Use(llm.Prompt("You are a {{.Role}} specializing in {{.Language}}. {{.Input}}", params)).
		Use(flow.Logger("PROMPT")).
		Use(llm.Chat(provider))

	result, err := pipe.Run(context.Background(), "How do I handle errors?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}

// Example 3: SystemPrompt convenience function
func systemPromptExample(provider llm.LLMProvider) {
	fmt.Println("=== SystemPrompt Example ===")

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT")).
		Use(llm.SystemPrompt("You are a concise coding expert. Always provide practical examples.")).
		Use(flow.Logger("PROMPT")).
		Use(llm.Chat(provider))

	result, err := pipe.Run(context.Background(), "Show me a Go struct example")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}

// Example 4: ChatPrompt convenience function
func chatPromptExample(provider llm.LLMProvider) {
	fmt.Println("=== ChatPrompt Example ===")

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT")).
		Use(llm.ChatPrompt("assistant", "I'm an AI assistant specialized in programming.")).
		Use(flow.Logger("PROMPT")).
		Use(llm.Chat(provider))

	result, err := pipe.Run(context.Background(), "Hello!")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}
