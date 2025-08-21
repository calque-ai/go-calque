package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/gemini"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/openai"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/logger"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

func main() {

	ollamaExample()
	geminiExample()
	openaiExample()
}

func ollamaExample() {

	// Create Ollama client (connects to localhost:11434 by default)
	client, err := ollama.New("llama3.2:3b")
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	// Create flow with LLM integration
	flow := calque.NewFlow()

	flow.
		Use(logger.Print("INPUT")).                // Log input
		Use(text.Transform(func(s string) string { // Transform input by adding context
			return "Please provide a concise response to: " + s
		})).
		Use(logger.Print("PROMPT")).                         // Log finalized input
		Use(ctrl.Timeout(ai.Agent(client), 60*time.Second)). // Send the input to the agent and wrap it with a timeout
		Use(logger.Head("RESPONSE", 100))                    // Log first 100 bytes of the agents response

	// Run the flow
	var result string
	err = flow.Run(context.Background(), "What is Go programming language?", &result)
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
	config := &gemini.Config{
		Temperature: ai.Float32Ptr(1.1),
	}

	// Create Gemini example client (reads GOOGLE_API_KEY from env unless set in the config)
	client, err := gemini.New("gemini-2.0-flash", gemini.WithConfig(config))
	if err != nil {
		log.Fatal("Failed to create Gemini client:", err)
	}

	// Create flow with llm agent integration
	flow := calque.NewFlow()

	flow.
		Use(logger.Print("INPUT")).                                                      // Log input
		Use(prompt.Template("Please provide a concise response. Question: {{.Input}}")). // Setup a prompt template
		Use(logger.Print("PROMPT")).                                                     // Log the finalized prompt
		Use(ai.Agent(client)).                                                           // Send prompt to llm agent
		Use(logger.Head("RESPONSE", 200))                                                // Log the agent response using logger.head for streaming

	// Run the flow
	var result string
	err = flow.Run(context.Background(), "What is the Go programming language?", &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}

func openaiExample() {

	// Load environment variables from .env file
	// Make sure to have OPENAI_API_KEY set in your .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
		log.Println("To run OpenAI example:")
		log.Println("  1. Get API key from: https://platform.openai.com/api-keys")
		log.Println("  2. Create .env file with: OPENAI_API_KEY=your_api_key")
		return
	}

	// Create an optional custom OpenAI configuration
	config := &openai.Config{
		Temperature: ai.Float32Ptr(0.8),
		MaxTokens:   ai.IntPtr(150),
	}

	// Create OpenAI client (reads OPENAI_API_KEY from env unless set in the config)
	client, err := openai.New("gpt-5", openai.WithConfig(config))
	if err != nil {
		log.Printf("Warning: Could not connect to OpenAI: %v", err)
		log.Println("To run OpenAI example:")
		log.Println("  1. Get API key from: https://platform.openai.com/api-keys")
		log.Println("  2. Set: export OPENAI_API_KEY=your_api_key")
		return
	}

	// Create flow with LLM integration
	flow := calque.NewFlow()

	flow.
		Use(logger.Print("INPUT")).                                                      // Log input
		Use(prompt.Template("Please provide a concise response. Question: {{.Input}}")). // Setup a prompt template
		Use(logger.Print("PROMPT")).                                                     // Log the finalized prompt
		Use(ctrl.Timeout(ai.Agent(client), 30*time.Second)).                             // Send prompt to LLM agent with timeout
		Use(logger.Head("RESPONSE", 200))                                                // Log the agent response

	// Run the flow
	var result string
	err = flow.Run(context.Background(), "What is the Go programming language?", &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFinal result:")
	fmt.Println(result)
}
