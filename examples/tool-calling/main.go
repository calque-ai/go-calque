package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/ai"
	"github.com/calque-ai/calque-pipe/middleware/tools"
	"github.com/joho/godotenv"
)

func main() {

	// Load environment variables from .env file
	// Make sure to have GOOGLE_API_KEY set in your .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	fmt.Println("=== Tool Calling Examples ===")
	fmt.Println("Prerequisites:")
	fmt.Println("1. Install Ollama: https://ollama.ai/")
	fmt.Println("2. Pull a model: ollama pull llama3.2:1b")
	fmt.Println("3. Make sure Ollama is running: ollama serve")
	fmt.Println()

	// Example 1: Simple Agent
	fmt.Println("Example 1: Simple Agent with Two Tools")
	runSimpleAgent()
	fmt.Println()

	// Example 2: Agent with Configuration
	fmt.Println("Example 2: Agent with Custom Configuration")
	runConfiguredAgent()
}

// Example 1: Simple agent with basic tools
func runSimpleAgent() {
	// Create a simple demo calculator tool
	calculator := tools.Simple("calculator", "Performs basic math calculations", func(jsonArgs string) string {
		// Parse JSON arguments
		var args struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal([]byte(jsonArgs), &args); err != nil {
			return fmt.Sprintf("Error parsing arguments: %v", err)
		}

		result, err := calculate(args.Input)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("%.2f", result)
	})

	// Create a simple time lookup tool
	currentTime := tools.Simple("current_time", "Gets the current date and time. Call with empty string or '2006-01-02 15:04:05' format to get current time in default format.", func(jsonArgs string) string {
		// Parse JSON arguments
		var args struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal([]byte(jsonArgs), &args); err != nil {
			return fmt.Sprintf("Error parsing arguments: %v", err)
		}

		format := args.Input
		if format == "" {
			format = "2006-01-02 15:04:05"
		}
		return time.Now().Format(format)
	})

	// Create Gemini example client (reads GOOGLE_API_KEY from env)
	client, err := ai.NewGemini("gemini-2.0-flash")
	if err != nil {
		log.Fatal("Failed to create Gemini client:", err)
	}

	// Create simple tool agent
	agent := ai.Agent(client, ai.WithTools(calculator, currentTime))

	// Test the agent
	ctx := context.Background()
	input := "What is 15 * 8? Also, what time is it right now?"

	var result string
	err = core.New().Use(agent).Run(ctx, input, &result)
	if err != nil {
		log.Printf("Agent error: %v", err)
		return
	}

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Result: %s\n", result)
}

// Example 2: Agent with custom configuration
func runConfiguredAgent() {
	// Create tools with more complex logic
	calculator := tools.Simple("calculator", "Advanced calculator", func(expression string) string {
		result, err := calculate(expression)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("Result: %.2f", result)
	})

	textAnalyzer := tools.Simple("text_analyzer", "Analyzes text and provides statistics", func(text string) string {
		words := strings.Fields(text)
		chars := len(text)
		sentences := strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?")
		if sentences == 0 {
			sentences = 1
		}

		return fmt.Sprintf("Text Analysis:\n- Words: %d\n- Characters: %d\n- Sentences: %d\n- Text: %s",
			len(words), chars, sentences, text)
	})

	// Configure agent with custom settings
	toolConfig := tools.Config{
		PassThroughOnError:    true, // Continue even if a tool fails
		MaxConcurrentTools:    2,    // Run up to 2 tools concurrently
		IncludeOriginalOutput: true, // Include LLM output with tool results
	}

	client, err := ai.NewOllama("llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama client:", err)
	}

	// Create configured agent
	agent := ai.Agent(client, ai.WithTools(calculator, textAnalyzer), ai.WithToolsConfig(toolConfig))

	// Test the agent
	ctx := context.Background()
	input := "Please calculate 25 + 17, and analyze this text: 'Hello world! This is a test.'"

	var result string
	err = core.New().Use(agent).Run(ctx, input, &result)
	if err != nil {
		log.Printf("Configured agent error: %v", err)
		return
	}

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Result: %s\n", result)
}

// Helper function for basic math calculations
func calculate(expression string) (float64, error) {
	expr := strings.ReplaceAll(expression, " ", "")

	// Handle addition
	if strings.Contains(expr, "+") {
		parts := strings.Split(expr, "+")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid addition expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers")
		}
		return a + b, nil
	}

	// Handle multiplication
	if strings.Contains(expr, "*") {
		parts := strings.Split(expr, "*")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid multiplication expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers")
		}
		return a * b, nil
	}

	// Handle subtraction
	if strings.Contains(expr, "-") {
		parts := strings.Split(expr, "-")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid subtraction expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers")
		}
		return a - b, nil
	}

	// Handle division
	if strings.Contains(expr, "/") {
		parts := strings.Split(expr, "/")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid division expression")
		}
		a, err1 := strconv.ParseFloat(parts[0], 64)
		b, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid numbers")
		}
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	}

	// Single number
	return strconv.ParseFloat(expr, 64)
}
