package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/examples/providers/gemini"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	str "github.com/calque-ai/calque-pipe/middleware/strings"
	"github.com/joho/godotenv"
)

func main() {

	// Load environment variables from .env file
	// Make sure to have GOOGLE_API_KEY set in your .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Create Gemini example provider (reads GOOGLE_API_KEY from env)
	provider, err := gemini.NewGeminiProvider("", "gemini-2.0-flash")
	if err != nil {
		log.Fatal("Failed to create Gemini provider:", err)
	}

	// Create flow with LLM integration
	flow := core.New()

	flow.
		Use(core.Logger("INPUT")).                // Log input
		Use(str.Transform(func(s string) string { // Add context
			return "Please provide a concise response to: " + s
		})).
		Use(core.Logger("PROMPT")).                                    // Log formatted prompt
		Use(core.Timeout[string](llm.Chat(provider), 30*time.Second)). // LLM with timeout
		Use(core.Logger("RESPONSE"))                                   // Log response

	// Run the flow
	result, err := flow.Run(context.Background(), "What is Go programming language?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}
