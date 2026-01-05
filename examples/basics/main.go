// Package main demonstrates basic usage of the calque framework.
// It showcases core concepts like flows, middleware, text processing,
// streaming, and AI integration through simple, practical examples.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

func main() {

	runTextOnlyExample() // Basic text transforming demo

	runStreamingExample() // Streaming capabilities demo

	runComposedPipelineExample() // Pipeline composition demo

	// Initialize AI client (using Ollama as a free, local option)
	client, err := ollama.New("llama3.2:1b")
	if err != nil {
		log.Printf("Warning: Could not connect to Ollama: %v", err)
		return
	}

	runAIExample(client) // Basic AI demo

}

// runTextOnlyExample demonstrates calque framework concepts: flows, middleware handlers.
// Shows how to build processing flows using transformations, logging, and branching.
func runTextOnlyExample() {
	fmt.Println("\nRunning text-only flow (no AI)")

	flow := calque.NewFlow() // Create new flow

	// Build flow using middleware pattern - each Use() adds a handler to the flow
	flow.
		Use(inspect.Print("INPUT")).          // Log original input, with a prefix
		Use(text.Transform(strings.ToUpper)). // Transform input to uppercase
		Use(inspect.Print("TRANSFORMED")).    // Log transformed result
		Use(text.Branch(                      // Branch based on content
			func(s string) bool { return strings.Contains(s, "HELLO") },                 // Condition
			text.Transform(func(s string) string { return s + " [GREETING DETECTED]" }), // If true
			text.Transform(func(s string) string { return s + " [GENERAL TEXT]" }),      // If false
		)).
		Use(text.Transform(func(s string) string { return fmt.Sprintf("%s\nLength: %d characters", s, len(s)) })). // Add stats
		Use(inspect.Print("FINAL"))                                                                                // Log final result

	inputText := "Hello world! This text flows through pipes, getting calqued and transformed along the way."
	fmt.Printf("\nProcessing: %q\n\n", inputText)

	var result string                                         // Output placeholder
	err := flow.Run(context.Background(), inputText, &result) // Execute flow
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Print("RESULT: ")
	fmt.Println(result)
}

// runAIExample shows how to integrate ai clients into processing flows.
// Demonstrates prompt templates, AI chat requests, timeouts, and response handling.
func runAIExample(client ai.Client) {
	fmt.Println("\nRunning AI-powered flow...")

	flow := calque.NewFlow()

	flow.
		Use(inspect.Print("INPUT")).                                                       // Log original input
		Use(text.Transform(preprocessText)).                                               // Clean text
		Use(inspect.Print("PREPROCESSED")).                                                // Log cleaned text
		Use(prompt.Template("Analyze this text and provide a brief summary: {{.Input}}")). // Build AI prompt using a go template
		Use(inspect.Print("PROMPT")).                                                      // Log prompt
		Use(ctrl.Timeout(ai.Agent(client), 30*time.Second)).                               // Send to AI with timeout
		Use(inspect.Head("AI_RESPONSE", 300)).                                             // Log first 300 bytes of AI response
		Use(text.Branch(                                                                   // Branch on response type
			func(response string) bool {
				return strings.Contains(strings.ToLower(response), "summary")
			},
			text.Transform(func(s string) string { return s + "\n\n[Analysis completed successfully]" }), // If summary found
			text.Transform(func(s string) string { return s + "\n\n[General response provided]" }),       // If no summary
		)).
		Use(inspect.Head("FINAL", 400)) // Log first 400 bytes result

	inputText := "This AI framework calques ideas from text processing flows - copying and transforming data patterns."
	fmt.Printf("\nProcessing: %q\n\n", inputText)

	var result string
	err := flow.Run(context.Background(), inputText, &result)
	if err != nil {
		log.Printf("Flow error: %v", err)
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

	// Demo 1: Pure streaming flow
	fmt.Println("\n=== STREAMING Flow ===")
	runStreamingPipeline()

	// Demo 2: Mixed streaming/buffered pipeline
	fmt.Println("\n=== MIXED STREAMING/BUFFERED PIPELINE ===")
	runMixedPipeline()
}

// Pure streaming pipeline - processes data as it flows
func runStreamingPipeline() {
	var logBuffer bytes.Buffer
	var errorBuffer bytes.Buffer

	flow := calque.NewFlow()
	flow.
		Use(inspect.Head("INPUT", 100)).                  // Log first 100 bytes of original input
		Use(ctrl.TeeReader(&logBuffer, &errorBuffer)).    // STREAMING: Tee to multiple destinations
		Use(text.LineProcessor(func(line string) string { // STREAMING: Process line-by-line
			return fmt.Sprintf("[STREAM-%d] %s", len(line), strings.TrimSpace(line))
		})).
		Use(ctrl.Timeout( // STREAMING: Timeout protection
			text.LineProcessor(func(line string) string { // STREAMING: Another line processor
				return fmt.Sprintf("FINAL: %s", strings.ToUpper(line))
			}),
			2*time.Second,
		)).
		Use(inspect.Chunks("FINAL", 32)) // Log final results in chunks of 32 bytes as it streams in

	inputText := `Streaming processes each line individually
Data flows through without full buffering
Memory efficient for large inputs`

	var result string
	err := flow.Run(context.Background(), inputText, &result)
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
	flow := calque.NewFlow()

	// Create streaming handler for comparison
	streamingHandler := text.LineProcessor(func(line string) string {
		return fmt.Sprintf("STREAMING: %s [processed line-by-line]", line)
	})

	// Create buffered handler chain for comparison
	bufferedHandler := ctrl.Chain(
		text.Transform(func(s string) string {
			return fmt.Sprintf("BUFFERED STEP 1: %s [read all %d chars]", s, len(s))
		}),
		text.Transform(func(s string) string {
			wordCount := len(strings.Fields(s))
			return fmt.Sprintf("BUFFERED STEP 2: %s [analyzed %d words]", s, wordCount)
		}),
	)

	flow.
		Use(inspect.Head("INPUT", 100)). // Log original input
		Use(ctrl.Parallel(               // Split stream for comparison
			streamingHandler, // STREAMING: Line-by-line processing
			bufferedHandler,  // BUFFERED: Sequential chain processing
		)).
		Use(inspect.Head("COMPARISON_RESULTS", 500)) // Show both results

	inputText := `Line 1: Compare streaming vs buffered
Line 2: Streaming processes incrementally  
Line 3: Buffered reads everything first`

	fmt.Printf("Processing (streaming vs buffered comparison):\n%s\n\n", inputText)

	var result string
	err := flow.Run(context.Background(), inputText, &result)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("COMPARISON RESULT:\n%s\n", result)
}

// runComposedPipelineExample demonstrates pipeline composition by building reusable
// sub-pipelines and composing them into larger main pipelines.
func runComposedPipelineExample() {
	fmt.Println("\nRunning composed pipeline example...")

	// Build reusable sub-pipeline for text preprocessing
	// Important: Don't Run this sub-pipeline directly, it will be composed into the main pipeline
	textPreprocessor := calque.NewFlow()
	textPreprocessor.
		Use(inspect.Print("PREPROCESS_INPUT")).
		Use(text.Transform(strings.TrimSpace)).
		Use(text.Transform(func(s string) string {
			// Normalize whitespace
			return strings.Join(strings.Fields(s), " ")
		})).
		Use(text.Transform(strings.ToLower)).
		Use(inspect.Print("PREPROCESS_OUTPUT"))

	// Build reusable sub-pipeline for text analysis
	textAnalyzer := calque.NewFlow()
	textAnalyzer.
		Use(inspect.Print("ANALYZE_INPUT")).
		Use(text.Transform(func(s string) string {
			wordCount := len(strings.Fields(s))
			charCount := len(s)
			return fmt.Sprintf("TEXT: %s\nSTATS: %d words, %d characters", s, wordCount, charCount)
		})).
		Use(inspect.Print("ANALYZE_OUTPUT"))

	// Build main pipeline that composes the sub-pipelines
	mainFlow := calque.NewFlow()
	mainFlow.
		Use(inspect.Print("MAIN_START")).
		Use(textPreprocessor). // Use preprocessing sub-pipeline
		Use(text.Branch(       // Branch based on content length
			func(s string) bool { return len(s) > 50 },
			ctrl.Chain( // Long text path
				text.Transform(func(s string) string { return s + " [LONG TEXT DETECTED]" }),
				textAnalyzer, // Use analysis sub-pipeline
			),
			text.Transform(func(s string) string { return s + " [SHORT TEXT - BASIC PROCESSING]" }), // Short text path
		)).
		Use(text.Transform(func(s string) string {
			return fmt.Sprintf("FINAL RESULT:\n%s\n[Pipeline composition complete]", s)
		})).
		Use(inspect.Print("MAIN_END"))

	// Test with different inputs
	inputs := []string{
		"  Hello WORLD!  This is a    Test   ",
		"Short text",
		"This is a much longer piece of text that will trigger the analysis sub-pipeline because it exceeds the 50 character threshold",
	}

	for i, input := range inputs {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %q\n\n", input)

		var result string
		err := mainFlow.Run(context.Background(), input, &result)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("OUTPUT:\n%s\n", result)
	}
}

// preprocessText cleans and normalizes input text
func preprocessText(input string) string {
	// Trim whitespace and normalize spacing
	cleaned := strings.TrimSpace(input)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned
}
