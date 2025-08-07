package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/ai"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/prompt"
)

func main() {
	// Create client for all examples
	client, err := ai.NewOllama("llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama client:", err)
	}

	// Run all examples
	basicTemplateExample(client)
	templateWithDataExample(client)
	systemPromptExample(client)
	chatPromptExample(client)
}

// Example 1: Basic template usage
func basicTemplateExample(client ai.Client) {
	fmt.Println("=== Basic Template Example ===")

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 100)).
		Use(prompt.Template("You are a helpful assistant. {{.Input}}")).
		Use(flow.Logger("PROMPT", 100)).
		Use(ai.Agent(client))

	var result string
	err := pipe.Run(context.Background(), "What is Golang?", &result)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}

// Example 2: Template with additional data
func templateWithDataExample(client ai.Client) {
	fmt.Println("=== Template with Data Example ===")

	params := map[string]any{
		"Role":     "Senior Software Engineer",
		"Language": "Go",
	}

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 100)).
		Use(prompt.Template("You are a {{.Role}} specializing in {{.Language}}. {{.Input}}", params)).
		Use(flow.Logger("PROMPT", 200)).
		Use(ai.Agent(client))

	var result string
	err := pipe.Run(context.Background(), "How do I handle errors?", &result)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}

// Example 3: SystemPrompt convenience function
func systemPromptExample(client ai.Client) {
	fmt.Println("=== SystemPrompt Example ===")

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 100)).
		Use(prompt.System("You are a concise coding expert. Always provide practical examples.")).
		Use(flow.Logger("PROMPT", 100)).
		Use(ai.Agent(client))

	var result string
	err := pipe.Run(context.Background(), "Show me a Go struct example", &result)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}

// Example 4: ChatPrompt convenience function
func chatPromptExample(client ai.Client) {
	fmt.Println("=== ChatPrompt Example ===")

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 100)).
		Use(prompt.Chat("assistant", "I'm an AI assistant specialized in programming.")).
		Use(flow.Logger("PROMPT", 100)).
		Use(ai.Agent(client))

	var result string
	err := pipe.Run(context.Background(), "Hello!", &result)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)
}
