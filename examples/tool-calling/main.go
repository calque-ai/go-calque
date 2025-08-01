package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

func main() {
	fmt.Println("=== Tool Calling Example ===")
	fmt.Println("Prerequisites:")
	fmt.Println("1. Install Ollama: https://ollama.ai/")
	fmt.Println("2. Pull a model: ollama pull llama3.2:1b")
	fmt.Println("3. Make sure Ollama is running: ollama serve")
	fmt.Println()

	// Example 1: Simple Agent with Quick Tools
	fmt.Println("1. Simple Agent Example:")
	runSimpleAgentExample()
	fmt.Println()

	// Example 2: Flexible Pipeline Composition
	fmt.Println("2. Flexible Pipeline Example:")
	runFlexiblePipelineExample()
	fmt.Println()

	// Example 3: Advanced Agent with Configuration
	fmt.Println("3. Advanced Agent Example:")
	runAdvancedAgentExample()
	fmt.Println()
}

// runSimpleAgentExample demonstrates the easiest way to create a tool-enabled agent
func runSimpleAgentExample() {
	// Create tools using the Quick constructor for simple string-to-string functions
	calculator := tools.Simple("calculator", "Math Calculator", func(expr string) string {
		result, err := evaluateExpression(expr)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("%.2f", result)
	})

	dateTime := tools.Simple("datetime", "Get current date and time information", func(format string) string {
		if format == "" {
			format = "2006-01-02 15:04:05"
		}
		return time.Now().Format(format)
	})

	fileInfo := tools.HandlerFunc("file_info", "Get information about a file",
		func(ctx context.Context, r io.Reader, w io.Writer) error {
			var filename string
			if err := core.Read(r, &filename); err != nil {
				return err
			}

			info, err := os.Stat(filename)
			if err != nil {
				return core.Write(w, fmt.Sprintf("Error: %v", err))
			}

			result := fmt.Sprintf("File: %s, Size: %d bytes, Modified: %s",
				filename, info.Size(), info.ModTime().Format("2006-01-02 15:04:05"))
			return core.Write(w, result)
		})

	// Create Ollama provider (make sure Ollama is running with a model)
	provider, err := llm.NewOllamaProvider("", "llama3.2:1b")
	if err != nil {
		log.Printf("Failed to create Ollama provider: %v", err)
		log.Println("Make sure Ollama is running and you have a model installed (e.g., 'ollama pull llama3.2:1b')")
		return
	}

	// Create agent with tools - this handles everything automatically
	agent := llm.Agent(provider, calculator, dateTime, fileInfo)

	// Run the agent
	ctx := context.Background()
	input := "Please calculate 25 times 4, and also tell me what time it is now. Use the available tools to help with this."

	var result string
	err = core.New().Use(agent).Run(ctx, input, &result)
	if err != nil {
		log.Printf("Simple agent error: %v", err)
		return
	}

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Result: %s\n", result)
}

// runFlexiblePipelineExample demonstrates manual pipeline composition for maximum control
func runFlexiblePipelineExample() {
	// Create tools with more complex logic
	textProcessor := tools.New("text_processor", "Process and analyze text",
		createTextProcessorHandler())

	wordCount := tools.Simple("word_count", "Word counter", func(text string) string {
		words := strings.Fields(text)
		return fmt.Sprintf("Word count: %d", len(words))
	})

	// Create Ollama provider
	provider, err := llm.NewOllamaProvider("", "llama3.2:1b")
	if err != nil {
		log.Printf("Failed to create Ollama provider: %v", err)
		return
	}

	// Build pipeline manually for full control
	pipeline := core.New().
		Use(flow.Logger("INPUT", 200)).                // Log input
		Use(tools.Registry(textProcessor, wordCount)). // Register tools
		Use(tools.Format(tools.FormatStyleDetailed)).  // Format tool info for LLM
		Use(flow.Logger("PRE-LLM", 300)).              // Log formatted input
		Use(llm.Chat(provider)).                       // Send to LLM
		Use(flow.Logger("LLM-RESPONSE", 300)).         // Log LLM response
		Use(tools.Execute()).                          // Execute any tool calls
		Use(flow.Logger("TOOL-RESULTS", 300)).         // Log tool results
		Use(llm.Chat(provider)).                       // Send results back to LLM
		Use(flow.Logger("FINAL", 200))                 // Log final output

	ctx := context.Background()
	input := "Please analyze this text: 'Hello world this is a test'. Use the text processor tool to provide detailed analysis."

	var result string
	err = pipeline.Run(ctx, input, &result)
	if err != nil {
		log.Printf("Flexible pipeline error: %v", err)
		return
	}

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Result: %s\n", result)
}

// runAdvancedAgentExample demonstrates advanced configuration and error handling
func runAdvancedAgentExample() {
	// Create tools with error handling
	calculator := tools.Simple("calculator", "Math Calculator", func(expr string) string {
		result, err := evaluateExpression(expr)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("%.2f", result)
	})

	// Tool that sometimes fails
	unstableTool := tools.HandlerFunc("unstable_tool", "A tool that sometimes fails",
		func(ctx context.Context, r io.Reader, w io.Writer) error {
			var input string
			if err := core.Read(r, &input); err != nil {
				return err
			}

			// Simulate occasional failure
			if strings.Contains(input, "fail") {
				return fmt.Errorf("tool intentionally failed")
			}

			return core.Write(w, "Tool executed successfully with input: "+input)
		})

	// Configure robust agent with error handling
	config := llm.AgentConfig{
		MaxIterations: 3,
		ExecuteConfig: tools.ExecuteConfig{
			PassThroughOnError:    true, // Continue on tool errors
			MaxConcurrentTools:    2,    // Limit concurrent execution
			IncludeOriginalOutput: true, // Include LLM output in results
		},
		Timeout: 30 * time.Second, // Safety timeout
	}

	// Create Ollama provider
	provider, err := llm.NewOllamaProvider("", "llama3.2:1b")
	if err != nil {
		log.Printf("Failed to create Ollama provider: %v", err)
		return
	}

	agent := llm.AgentWithConfig(provider, config, calculator, unstableTool)

	ctx := context.Background()
	input := "Please calculate 10 + 5 using the calculator tool."

	var result string
	err = core.New().Use(agent).Run(ctx, input, &result)
	if err != nil {
		log.Printf("Advanced agent error: %v", err)
		return
	}

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Result: %s\n", result)
}

// Helper Functions

// evaluateExpression performs simple math expression evaluation
func evaluateExpression(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	// Simple calculator - supports +, -, *, /
	if strings.Contains(expr, "+") {
		parts := strings.Split(expr, "+")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid addition expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers in expression")
		}
		return a + b, nil
	}

	if strings.Contains(expr, "*") {
		parts := strings.Split(expr, "*")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid multiplication expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers in expression")
		}
		return a * b, nil
	}

	if strings.Contains(expr, "-") {
		parts := strings.Split(expr, "-")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid subtraction expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers in expression")
		}
		return a - b, nil
	}

	if strings.Contains(expr, "/") {
		parts := strings.Split(expr, "/")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid division expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers in expression")
		}
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	}

	// Single number
	return strconv.ParseFloat(expr, 64)
}

// createTextProcessorHandler creates a handler for text processing
func createTextProcessorHandler() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		var text string
		if err := core.Read(r, &text); err != nil {
			return err
		}

		words := strings.Fields(text)
		chars := len(text)
		sentences := strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?")
		if sentences == 0 {
			sentences = 1 // At least one sentence if no punctuation
		}

		avgWordLength := 0.0
		if len(words) > 0 {
			totalChars := 0
			for _, word := range words {
				totalChars += len(word)
			}
			avgWordLength = float64(totalChars) / float64(len(words))
		}

		analysis := fmt.Sprintf("Text Analysis:\n- Words: %d\n- Characters: %d\n- Sentences: %d\n- Average word length: %.1f\n- Text: %s",
			len(words), chars, sentences, avgWordLength, text)

		return core.Write(w, analysis)
	})
}
