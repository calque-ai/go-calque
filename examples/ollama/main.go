package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/examples/providers/ollama"
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
	flow := core.New()

	flow.
		Use(core.Logger("INPUT")).                // Log input
		Use(str.Transform(func(s string) string { // Add context
			return "Please provide a concise response to: " + s
		})).
		Use(core.Logger("PROMPT")).                                    // Log formatted prompt
		Use(core.Timeout[string](llm.Chat(provider), 60*time.Second)). // LLM with timeout (longer for local)
		Use(core.Logger("RESPONSE"))                                   // Log response

	// Run the flow
	result, err := flow.Run(context.Background(), "What is Go programming language?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}
