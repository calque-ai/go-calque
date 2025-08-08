package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/pkg/core"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ai"
	"github.com/calque-ai/calque-pipe/pkg/middleware/flow"
	"github.com/calque-ai/calque-pipe/pkg/middleware/prompt"
	"github.com/calque-ai/calque-pipe/pkg/middleware/str"
)

func main() {

	runTextOnlyExample() //Basic text transforming demo

	runStreamingExample() //Streaming capabilities demo

	// Initialize AI client (using Ollama as a free, local option)
	client, err := ai.NewOllama("llama3.2:1b")
	if err != nil {
		log.Printf("Warning: Could not connect to Ollama: %v", err)
		return
	}

	runAIExample(client) //Basic AI demo

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

// runAIExample shows how to integrate ai clients into processing pipelines.
// Demonstrates prompt templates, AI chat requests, timeouts, and response handling.
func runAIExample(client ai.Client) {
	fmt.Println("\nRunning AI-powered pipeline...")

	pipe := core.New()

	pipe.
		Use(flow.Logger("INPUT", 100)).                                                    // Log original input
		Use(str.Transform(preprocessText)).                                                // Clean text
		Use(flow.Logger("PREPROCESSED", 80)).                                              // Log cleaned text
		Use(prompt.Template("Analyze this text and provide a brief summary: {{.Input}}")). // Build AI prompt using a go template
		Use(flow.Logger("PROMPT", 200)).                                                   // Log prompt
		Use(flow.Timeout(ai.Agent(client), 30*time.Second)).                               // Send to AI with timeout
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

// runStreamingExample demonstrates STREAMING vs BUFFERED middleware capabilities.
// Shows the difference between streaming (TeeReader, LineProcessor, PassThrough)
// and buffered (Transform, Chain, Branch) middleware.
func runStreamingExample() {
	fmt.Println("\nRunning streaming vs buffered middleware demo...")

	// Demo 1: Pure streaming pipeline
	fmt.Println("\n=== STREAMING PIPELINE ===")
	runStreamingPipeline()

	// Demo 2: Mixed streaming/buffered pipeline
	fmt.Println("\n=== MIXED STREAMING/BUFFERED PIPELINE ===")
	runMixedPipeline()
}

// Pure streaming pipeline - processes data as it flows
func runStreamingPipeline() {
	var logBuffer bytes.Buffer
	var errorBuffer bytes.Buffer

	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 100)).                  // Log original input
		Use(flow.TeeReader(&logBuffer, &errorBuffer)).   // STREAMING: Tee to multiple destinations
		Use(str.LineProcessor(func(line string) string { // STREAMING: Process line-by-line
			return fmt.Sprintf("[STREAM-%d] %s", len(line), strings.TrimSpace(line))
		})).
		Use(flow.Timeout( // STREAMING: Timeout protection
			str.LineProcessor(func(line string) string { // STREAMING: Another line processor
				return fmt.Sprintf("FINAL: %s", strings.ToUpper(line))
			}),
			2*time.Second,
		)).
		Use(flow.Logger("FINAL", 200)) // Log final result

	inputText := `Streaming processes each line individually
Data flows through without full buffering
Memory efficient for large inputs`

	var result string
	err := pipe.Run(context.Background(), inputText, &result)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("STREAMING RESULT:\n%s\n", result)
	fmt.Printf("Tee buffer 1: %s\n", logBuffer.String()[:50])
	fmt.Printf("Tee buffer 2: %s\n", errorBuffer.String()[:50])
}

// Mixed pipeline showing STREAMING vs BUFFERED side-by-side comparison
func runMixedPipeline() {
	pipe := core.New()

	// Create streaming handler for comparison
	streamingHandler := str.LineProcessor(func(line string) string {
		return fmt.Sprintf("STREAMING: %s [processed line-by-line]", line)
	})

	// Create buffered handler chain for comparison
	bufferedHandler := flow.Chain(
		str.Transform(func(s string) string {
			return fmt.Sprintf("BUFFERED STEP 1: %s [read all %d chars]", s, len(s))
		}),
		str.Transform(func(s string) string {
			wordCount := len(strings.Fields(s))
			return fmt.Sprintf("BUFFERED STEP 2: %s [analyzed %d words]", s, wordCount)
		}),
	)

	pipe.
		Use(flow.Logger("INPUT", 100)). // Log original input
		Use(flow.Parallel(              // Split stream for comparison
			streamingHandler, // STREAMING: Line-by-line processing
			bufferedHandler,  // BUFFERED: Sequential chain processing
		)).
		Use(flow.Logger("COMPARISON_RESULTS", 500)) // Show both results

	inputText := `Line 1: Compare streaming vs buffered
Line 2: Streaming processes incrementally  
Line 3: Buffered reads everything first`

	fmt.Printf("Processing (streaming vs buffered comparison):\n%s\n\n", inputText)

	var result string
	err := pipe.Run(context.Background(), inputText, &result)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("COMPARISON RESULT:\n%s\n", result)
}

// preprocessText cleans and normalizes input text
func preprocessText(input string) string {
	// Trim whitespace and normalize spacing
	cleaned := strings.TrimSpace(input)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned
}
