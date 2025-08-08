package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/ai"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/prompt"
	"github.com/calque-ai/calque-pipe/middleware/str"
	"github.com/joho/godotenv"
)

func main() {

	ollamaExample()
	geminiExample()
}

func ollamaExample() {

	// Create Ollama client (connects to localhost:11434 by default)
	client, err := ai.NewOllama("llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	// Create flow with LLM integration
	pipe := core.New()

	pipe.
		Use(flow.Logger("INPUT", 100)).           // Log first 100 bytes of input
		Use(str.Transform(func(s string) string { // Transform input by adding context
			return "Please provide a concise response to: " + s
		})).
		Use(flow.Logger("PROMPT", 100)).                     // Log finalized input
		Use(flow.Timeout(ai.Agent(client), 60*time.Second)). // Send the input to the agent and wrap it with a timeout
		Use(flow.Logger("RESPONSE", 100))                    // Log agents response

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

	// Create an optional custom gemini configuration
	config := &ai.GeminiConfig{
		Temperature: ai.Float32Ptr(1.1),
	}

	// Create Gemini example client (reads GOOGLE_API_KEY from env unless set in the config)
	client, err := ai.NewGemini("gemini-2.0-flash", ai.WithGeminiConfig(config))
	if err != nil {
		log.Fatal("Failed to create Gemini client:", err)
	}

	// Create pipe with llm agent integration
	pipe := core.New()

	pipe.
		Use(flow.Logger("INPUT", 100)).                                                  // Log input, first 100 bytes
		Use(prompt.Template("Please provide a concise response. Question: {{.Input}}")). // Setup a prompt template
		Use(flow.Logger("PROMPT", 100)).                                                 // Log the finalized prompt
		Use(ai.Agent(client)).                                                           // Send prompt to llm agent
		Use(flow.Logger("RESPONSE", 200))                                                // Log the agent response

	// Run the pipe
	var result string
	err = pipe.Run(context.Background(), "What is the Go programming language?", &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}
