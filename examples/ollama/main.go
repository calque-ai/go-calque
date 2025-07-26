package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/examples/providers/ollama"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	str "github.com/calque-ai/calque-pipe/middleware/strings"
)

func main() {
	// Create Ollama provider (connects to localhost:11434 by default)
	provider, err := ollama.NewOllamaProvider("", "llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	// Create flow with LLM integration
	pipe := core.New()

	pipe.
		Use(flow.Logger("INPUT", 100)).           // Log input
		Use(str.Transform(func(s string) string { // Add context
			return "Please provide a concise response to: " + s
		})).
		Use(flow.Logger("PROMPT", 100)).                               // Log formatted prompt
		Use(flow.Timeout[string](llm.Chat(provider), 60*time.Second)). // LLM with timeout (longer for local)
		Use(flow.Logger("RESPONSE", 100))                              // Log response

	// Run the flow
	result, err := pipe.Run(context.Background(), "What is Go programming language?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}
