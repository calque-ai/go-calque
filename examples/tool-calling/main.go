// Package main demonstrates tool calling capabilities with the calque framework.
// It showcases how to create AI agents that can use external tools and functions,
// enabling complex workflows that combine AI reasoning with practical actions.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/gemini"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/openai"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
	"github.com/invopop/jsonschema"
	"github.com/joho/godotenv"
	orderedmap "github.com/wk8/go-ordered-map/v2"
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
	fmt.Println("4. Set GOOGLE_API_KEY in your .env file for Gemini examples")
	fmt.Println("5. Set OPENAI_API_KEY in your .env file for OpenAI examples")
	fmt.Println()

	// Example 1: Simple Agent
	fmt.Println("Example 1: Simple Agent with Two Tools")
	runSimpleAgent()
	fmt.Println()

	// Example 2: Agent with Configuration
	fmt.Println("Example 2: Agent with Custom Configuration")
	runConfiguredAgent()
	fmt.Println()

	// Example 3: OpenAI Agent
	fmt.Println("Example 3: OpenAI Agent with Tools")
	runOpenAIAgent()
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
	client, err := gemini.New("gemini-2.5-flash")
	if err != nil {
		log.Fatal("Failed to create Gemini client:", err)
	}

	// Create simple tool agent
	agent := ai.Agent(client, ai.WithTools(calculator, currentTime))

	// Test the agent
	ctx := context.Background()
	input := "What is 15 * 8? Also, what time is it right now?"

	var result string
	err = calque.NewFlow().Use(agent).Run(ctx, input, &result)
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
		MaxConcurrentTools:    2,    // Run up to 2 tools concurrently
		IncludeOriginalOutput: true, // Include LLM output with tool results
	}

	client, err := ollama.New("llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama client:", err)
	}

	// Create configured agent
	agent := ai.Agent(client, ai.WithTools(calculator, textAnalyzer), ai.WithToolsConfig(toolConfig))

	// Test the agent
	ctx := context.Background()
	input := "Please calculate 25 + 17, and analyze this text: 'Hello world! This is a test.'"

	var result string
	err = calque.NewFlow().Use(agent).Run(ctx, input, &result)
	if err != nil {
		log.Printf("Configured agent error: %v", err)
		return
	}

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Result: %s\n", result)
}

// Example 3: OpenAI agent with tools
func runOpenAIAgent() {
	// Create a weather lookup tool with proper schema
	weatherSchema := &jsonschema.Schema{
		Type:       "object",
		Properties: orderedmap.New[string, *jsonschema.Schema](),
		Required:   []string{"city"},
	}
	weatherSchema.Properties.Set("city", &jsonschema.Schema{
		Type:        "string",
		Description: "The city to get weather for",
	})

	// Create the weather tool using tools.New to access full request/response
	weatherTool := tools.New("weather", "Gets current weather for a city", weatherSchema,
		calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
			fmt.Println("[DEBUG] Weather tool called!")
			var input string
			if err := calque.Read(r, &input); err != nil {
				return err
			}
			fmt.Printf("[DEBUG] Weather tool input: %s\n", input)

			var args struct {
				City string `json:"city"`
			}
			if err := json.Unmarshal([]byte(input), &args); err != nil {
				return err
			}

			// Simulate weather lookup
			weather := []string{"sunny", "cloudy", "rainy", "snowy"}
			temp := []int{72, 68, 45, 32}
			idx := len(args.City) % len(weather)

			result := fmt.Sprintf("Weather in %s: %s, %d°F", args.City, weather[idx], temp[idx])
			fmt.Printf("[DEBUG] Weather tool result: %s\n", result)
			return calque.Write(w, result)
		}))

	// Create a simple unit converter tool
	converter := tools.Simple("unit_converter", "Converts temperatures between fahrenheit and celsius. Input should be like '100 fahrenheit to celsius'", func(jsonArgs string) string {
		fmt.Println("[DEBUG] Converter tool called!")
		fmt.Printf("[DEBUG] Converter tool input: %s\n", jsonArgs)

		// Parse JSON arguments
		var args struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal([]byte(jsonArgs), &args); err != nil {
			return fmt.Sprintf("Error parsing arguments: %v", err)
		}

		result, err := convertTemperature(args.Input)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}

		fmt.Printf("[DEBUG] Converter tool result: %s\n", result)
		return result
	})

	// Create OpenAI client (reads OPENAI_API_KEY from env)
	client, err := openai.New("gpt-4o-mini")
	if err != nil {
		log.Fatal("Failed to create OpenAI client:", err)
	}

	// Create OpenAI agent with tools
	agent := ai.Agent(client, ai.WithTools(weatherTool, converter))

	// Test the agent with logging
	ctx := context.Background()
	input := "What's the weather like in New York? Also, convert 100 fahrenheit to celsius."
	fmt.Printf("[DEBUG] Number of tools registered: %d\n", 2)

	var result string
	flow := calque.NewFlow()
	flow.Use(inspect.Head("REQUEST", 500))
	flow.Use(agent)
	flow.Use(inspect.Head("RESPONSE", 500))
	err = flow.Run(ctx, input, &result)
	if err != nil {
		log.Printf("OpenAI agent error: %v", err)
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

// Helper function for temperature conversions
func convertTemperature(input string) (string, error) {
	// Parse input like "100 fahrenheit to celsius"
	parts := strings.Fields(strings.ToLower(input))
	if len(parts) < 4 {
		return "", fmt.Errorf("invalid format, use: '100 fahrenheit to celsius'")
	}

	// Extract value
	value, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return "", fmt.Errorf("invalid temperature value: %s", parts[0])
	}

	from := parts[1]
	to := parts[3] // Skip "to"

	switch {
	case from == "fahrenheit" && to == "celsius":
		converted := (value - 32) * 5 / 9
		return fmt.Sprintf("%.2f°F = %.2f°C", value, converted), nil
	case from == "celsius" && to == "fahrenheit":
		converted := (value * 9 / 5) + 32
		return fmt.Sprintf("%.2f°C = %.2f°F", value, converted), nil
	default:
		return "", fmt.Errorf("conversion from %s to %s not supported", from, to)
	}
}
