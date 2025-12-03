package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/gemini"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/openai"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Example 1: Simple usage tracking with logging
	example1SimpleLogging()

	// Example 2: Track cumulative usage across multiple requests
	example2CumulativeTracking()

	// Example 3: Cost estimation
	example3CostEstimation()
}

// Example 1: Simple logging of token usage
func example1SimpleLogging() {
	fmt.Println("=== Example 1: Simple Usage Logging (OpenAI) ===")

	client, err := openai.New("gpt-5-mini", openai.WithConfig(&openai.Config{
		Temperature: helpers.PtrOf(float32(1.0)),
	}))
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}

	// Create agent with usage handler that logs each request
	agent := ai.Agent(client,
		ai.WithUsageHandler(func(usage *ai.UsageMetadata) {
			fmt.Printf("Token Usage: %d prompt + %d completion = %d total\n",
				usage.PromptTokens,
				usage.CompletionTokens,
				usage.TotalTokens,
			)
		}),
	)

	flow := calque.NewFlow().Use(agent)

	fmt.Println("Sending request: 'Say hello in 3 words'")
	var output string
	err = flow.Run(context.Background(), "Say hello in 3 words", &output)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	fmt.Printf("Response: %s\n\n", output)
}

// Example 2: Track cumulative usage across requests
func example2CumulativeTracking() {
	fmt.Println("=== Example 2: Cumulative Usage Tracking (Gemini) ===")

	client, err := gemini.New("gemini-2.5-flash", gemini.WithConfig(&gemini.Config{
		Temperature: helpers.PtrOf(float32(0.7)),
	}))
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}

	// Track total usage across multiple requests
	var totalTokens int
	var requestCount int
	var mu sync.Mutex

	agent := ai.Agent(client,
		ai.WithUsageHandler(func(usage *ai.UsageMetadata) {
			mu.Lock()
			defer mu.Unlock()
			requestCount++
			totalTokens += usage.TotalTokens
			fmt.Printf("  Request #%d: %d tokens (prompt: %d, completion: %d)\n",
				requestCount, usage.TotalTokens, usage.PromptTokens, usage.CompletionTokens)
		}),
	)

	flow := calque.NewFlow().Use(agent)

	// Make multiple requests
	questions := []string{
		"What is 2+2?",
		"Name a color",
		"Say 'hello' in Spanish",
	}

	fmt.Println("Making multiple requests...")
	for i, question := range questions {
		fmt.Printf("  Sending Q%d: %s\n", i+1, question)
		var output string
		if err := flow.Run(context.Background(), question, &output); err != nil {
			log.Printf("Request failed: %v", err)
			continue
		}
		fmt.Printf("  Response: %s\n", output)
	}

	fmt.Printf("\nTotal Usage Summary:\n")
	fmt.Printf("  Total Requests: %d\n", requestCount)
	fmt.Printf("  Total Tokens: %d\n", totalTokens)
	if requestCount > 0 {
		fmt.Printf("  Average Tokens/Request: %.1f\n", float64(totalTokens)/float64(requestCount))
	}
	fmt.Println()
}

// Example 3: Cost estimation with pricing
func example3CostEstimation() {
	fmt.Println("=== Example 3: Cost Estimation ===")

	client, err := openai.New("gpt-5-mini", openai.WithConfig(&openai.Config{
		Temperature: helpers.PtrOf(float32(1.0)),
	}))
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}

	// GPT-5-mini pricing (as of late 2025)
	const (
		inputCostPer1M  = 0.25 // $0.25 per 1M input tokens
		outputCostPer1M = 2.00 // $2.00 per 1M output tokens
	)

	var totalCost float64
	var mu sync.Mutex

	agent := ai.Agent(client,
		ai.WithUsageHandler(func(usage *ai.UsageMetadata) {
			mu.Lock()
			defer mu.Unlock()

			inputCost := (float64(usage.PromptTokens) / 1_000_000.0) * inputCostPer1M
			outputCost := (float64(usage.CompletionTokens) / 1_000_000.0) * outputCostPer1M
			requestCost := inputCost + outputCost
			totalCost += requestCost

			fmt.Printf("Request cost: $%.6f (%d input + %d output tokens)\n",
				requestCost, usage.PromptTokens, usage.CompletionTokens)
		}),
	)

	flow := calque.NewFlow().Use(agent)

	// Make a few requests
	questions := []string{
		"What is the capital of France?",
		"Write a haiku about coding",
	}

	fmt.Println("Processing requests...")
	for _, question := range questions {
		fmt.Printf("\nQ: %s\n", question)
		var output string
		if err := flow.Run(context.Background(), question, &output); err != nil {
			log.Printf("Request failed: %v", err)
			continue
		}
		fmt.Printf("A: %s\n", output)
	}

	fmt.Printf("\nTotal estimated cost: $%.6f\n\n", totalCost)
}
