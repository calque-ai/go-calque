package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	"github.com/calque-ai/calque-pipe/middleware/prompt"
	"github.com/calque-ai/calque-pipe/middleware/str"
	"github.com/joho/godotenv"
)

func main() {

	ollamaExample()
	geminiExample()
}

func ollamaExample() {

	// Create Ollama provider (connects to localhost:11434 by default)
	provider, err := llm.NewOllamaProvider("", "llama3.2:1b", llm.DefaultConfig())
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
	var result string
	err = pipe.Run(context.Background(), "What is Go programming language?", &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}

func geminiExample() {

	// Load environment variables from .env file
	// Make sure to have GOOGLE_API_KEY set in your .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Create Gemini example provider (reads GOOGLE_API_KEY from env)
	provider, err := llm.NewGeminiProvider("", "gemini-2.0-flash", llm.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to create Gemini provider:", err)
	}

	// Create pipe with LLM integration
	pipe := core.New()

	pipe.
		Use(flow.Logger("INPUT", 100)).                                                  // Log input
		Use(prompt.Template("Please provide a concise response. Question: {{.Input}}")). // Setup a prompt
		Use(flow.Logger("PROMPT", 100)).                                                 // Log formatted prompt
		Use(flow.Timeout[string](llm.Chat(provider), 30*time.Second)).                   // LLM with timeout
		Use(flow.Logger("RESPONSE", 200))                                                // Log response

	// Run the pipe
	var result string
	err = pipe.Run(context.Background(), "What is the Go programming language?", &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}
