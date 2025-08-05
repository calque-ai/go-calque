package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	"github.com/calque-ai/calque-pipe/middleware/prompt"
	str "github.com/calque-ai/calque-pipe/middleware/strings"
)

func main() {

	runTextOnlyExample() //Basic text transforming demo

	// Initialize AI provider (using Ollama as a free, local option)
	provider, err := llm.NewOllamaProvider("http://localhost:11434", "llama3.2:1b")
	if err != nil {
		log.Printf("Warning: Could not connect to Ollama: %v", err)
		return
	}

	runAIExample(provider) //Basic AI demo

}

// runTextOnlyExample demonstrates core framework concepts: pipes, handlers, and middleware flow.
// Shows how to build processing pipelines using transformations, logging, and branching.
func runTextOnlyExample() {
	fmt.Println("\nRunning text-only pipeline (no AI)...")

	pipe := core.New() // Create new pipeline

	// Build pipeline using middleware pattern - each Use() adds a handler to the flow
	pipe.
		Use(flow.Logger("INPUT", 100)).       // Log original input, with prefix and number of bytes to log
		Use(str.Transform(strings.ToUpper)).  // Transform input to uppercase
		Use(flow.Logger("TRANSFORMED", 100)). // Log transformed result
		Use(str.Branch(                       // Branch based on content
			func(s string) bool { return strings.Contains(s, "HELLO") },                // Condition
			str.Transform(func(s string) string { return s + " [GREETING DETECTED]" }), // If true
			str.Transform(func(s string) string { return s + " [GENERAL TEXT]" }),      // If false
		)).
		Use(str.Transform(func(s string) string { return fmt.Sprintf("%s\nLength: %d characters", s, len(s)) })). // Add stats
		Use(flow.Logger("FINAL", 200))                                                                            // Log final result

	inputText := "Hello world! This text flows through pipes, getting calqued and transformed along the way."
	fmt.Printf("\nProcessing: %q\n\n", inputText)

	var result string                                         // Output placeholder
	err := pipe.Run(context.Background(), inputText, &result) // Execute pipeline
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Print("RESULT: ")
	fmt.Println(result)
}

// runAIExample shows how to integrate LLM providers into processing pipelines.
// Demonstrates prompt templates, AI chat requests, timeouts, and response handling.
func runAIExample(provider llm.LLMProvider) {
	fmt.Println("\nRunning AI-powered pipeline...")

	pipe := core.New()

	pipe.
		Use(flow.Logger("INPUT", 100)).                                                    // Log original input
		Use(str.Transform(preprocessText)).                                                // Clean text
		Use(flow.Logger("PREPROCESSED", 80)).                                              // Log cleaned text
		Use(prompt.Template("Analyze this text and provide a brief summary: {{.Input}}")). // Build AI prompt
		Use(flow.Logger("PROMPT", 200)).                                                   // Log prompt
		Use(flow.Timeout[string](llm.Chat(provider), 30*time.Second)).                     // Send to AI with timeout
		Use(flow.Logger("AI_RESPONSE", 300)).                                              // Log AI response
		Use(str.Branch(                                                                    // Branch on response type
			func(response string) bool {
				return strings.Contains(strings.ToLower(response), "summary")
			},
			str.Transform(func(s string) string { return s + "\n\n[Analysis completed successfully]" }), // If summary found
			str.Transform(func(s string) string { return s + "\n\n[General response provided]" }),       // If no summary
		)).
		Use(flow.Logger("FINAL", 400)) // Log final result

	inputText := "This AI framework calques ideas from text processing pipelines - copying and transforming data patterns."
	fmt.Printf("\nProcessing: %q\n\n", inputText)

	var result string
	err := pipe.Run(context.Background(), inputText, &result)
	if err != nil {
		log.Printf("Pipeline error: %v", err)
		return
	}

	fmt.Print("FINAL RESULT: ")
	fmt.Println(result)
}

// preprocessText cleans and normalizes input text
func preprocessText(input string) string {
	// Trim whitespace and normalize spacing
	cleaned := strings.TrimSpace(input)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned
}
