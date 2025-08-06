package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	"github.com/calque-ai/calque-pipe/middleware/tools"
	"github.com/invopop/jsonschema"
)

func main() {
	ctx := context.Background()

	// Example 1: Basic usage with default configuration
	fmt.Println("=== Example 1: Basic Gemini with defaults ===")
	
	provider, err := llm.NewGeminiProvider("", "gemini-1.5-flash", nil) // Uses DefaultConfig()
	if err != nil {
		log.Fatal(err)
	}

	pipe := core.New()
	pipe.Use(llm.Chat(provider))

	var result string
	err = pipe.Run(ctx, "What is the capital of France?", &result)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result)

	// Example 2: Custom configuration at provider creation
	fmt.Println("=== Example 2: Gemini with custom config ===")
	
	customConfig := &llm.Config{
		Temperature: llm.Float32Ptr(0.2), // More focused responses
		MaxTokens:   llm.IntPtr(100),     // Shorter responses
		Streaming:   llm.BoolPtr(false),  // Non-streaming
	}

	providerCustom, err := llm.NewGeminiProvider("", "gemini-1.5-pro", customConfig)
	if err != nil {
		log.Fatal(err)
	}

	pipe2 := core.New()
	pipe2.Use(llm.Chat(providerCustom))

	var result2 string
	err = pipe2.Run(ctx, "Explain quantum computing in one sentence.", &result2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result2)

	// Example 3: Using tools with provider
	fmt.Println("=== Example 3: Tools with configured provider ===")
	
	calculator := tools.Simple("calculator", "Evaluate math expressions", func(expr string) string {
		// Simple calculator (in practice, you'd use a proper math parser)
		switch expr {
		case "2 + 2":
			return "4"
		case "10 * 5":
			return "50"
		default:
			return "Unable to calculate"
		}
	})

	pipe3 := core.New()
	pipe3.Use(llm.ChatWithTools(provider, calculator))

	var result3 string
	err = pipe3.Run(ctx, "What is 2 + 2?", &result3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n\n", result3)

	// Example 4: Structured output with JSON schema
	fmt.Println("=== Example 4: Structured JSON output ===")

	// Define a schema for structured output
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"city": {
				Type:        "string",
				Description: "The city name",
			},
			"country": {
				Type:        "string", 
				Description: "The country name",
			},
			"population": {
				Type:        "integer",
				Description: "The population of the city",
			},
		},
		Required: []string{"city", "country", "population"},
	}

	responseFormat := &llm.ResponseFormat{
		Type:   "json_schema",
		Schema: schema,
	}

	pipe4 := core.New()
	pipe4.Use(llm.ChatWithSchema(provider, responseFormat))

	var result4 string
	err = pipe4.Run(ctx, "Give me information about Tokyo", &result4)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Structured Result: %s\n\n", result4)

	// Example 5: Ollama with custom config
	fmt.Println("=== Example 5: Ollama with custom config ===")

	ollamaConfig := &llm.Config{
		Temperature: llm.Float32Ptr(0.8),
		MaxTokens:   llm.IntPtr(200),
		Stop:        []string{"\n\n", "Human:", "Assistant:"},
	}

	ollamaProvider, err := llm.NewOllamaProvider("", "llama3.2", ollamaConfig)
	if err != nil {
		log.Printf("Ollama not available: %v\n", err)
	} else {
		pipe5 := core.New()
		pipe5.Use(llm.Chat(ollamaProvider))

		var result5 string
		err = pipe5.Run(ctx, "Write a haiku about programming", &result5)
		if err != nil {
			log.Printf("Ollama error: %v\n", err)
		} else {
			fmt.Printf("Ollama Result: %s\n", result5)
		}
	}

	fmt.Println("=== Summary ===")
	fmt.Printf("Gemini features: %+v\n", provider.SupportedFeatures())
	if ollamaProvider != nil {
		fmt.Printf("Ollama features: %+v\n", ollamaProvider.SupportedFeatures())
	}
}