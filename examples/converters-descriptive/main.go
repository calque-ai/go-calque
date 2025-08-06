// Package main demonstrates descriptive converter functionality in the Calque-Pipe AI Agent Framework.
// Descriptive converters enhance structured data with field descriptions for LLM understanding.
// They generate format-specific output with embedded comments explaining each field's purpose.
//
// When to use descriptive converters:
// - Providing structured data to LLMs that need field context
// - Creating self-documenting API inputs/outputs
// - Building prompts that require data schema information
// - Processing complex structured data where field meaning matters
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/calque-ai/calque-pipe/convert"
	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	"github.com/calque-ai/calque-pipe/middleware/str"
	"github.com/joho/godotenv"
)

// ProductInfo represents structured product data with descriptions for LLM understanding
type ProductInfo struct {
	Name        string   `yaml:"name" desc:"The product's commercial name"`
	Category    string   `yaml:"category" desc:"Product classification category"`
	Price       float64  `yaml:"price" desc:"Current market price in USD"`
	Features    []string `yaml:"features" desc:"List of key product features"`
	Description string   `yaml:"description" desc:"Detailed product description"`
}

// ProductAnalysis represents an LLM's analysis of a product with descriptive fields
type ProductAnalysis struct {
	MarketPosition  string   `yaml:"market_position" desc:"Where this product fits in the market"`
	TargetAudience  []string `yaml:"target_audience" desc:"Who should buy this product"`
	Strengths       []string `yaml:"strengths" desc:"Key advantages of this product"`
	Recommendations string   `yaml:"recommendations" desc:"Suggested improvements or marketing strategies"`
	CompetitorPrice float64  `yaml:"competitor_price" desc:"Estimated competitor pricing"`
}

func main() {
	fmt.Println("Calque-Pipe Descriptive Converter Examples")

	runDescriptiveBasics()     // Basic descriptive converter demo
	runLLMIntegrationExample() // LLM + descriptive converter demo
}

// runDescriptiveBasics demonstrates descriptive converter concepts: adding field descriptions to structured output.
// Shows how descriptive converters generate comments that help LLMs understand field meanings.
func runDescriptiveBasics() {
	fmt.Println("\nRunning basic descriptive converter pipeline...")
	// Example: struct -> descriptive YAML with comments

	product := ProductInfo{
		Name:        "Smart Widget",
		Category:    "Electronics",
		Price:       99.99,
		Features:    []string{"WiFi", "Bluetooth", "Voice Control"},
		Description: "A widget that calques ideas from competing products.",
	}

	pipe := core.New()
	pipe.
		Use(flow.Logger("STRUCT_INPUT", 600)).                                   // Log struct input
		Use(str.Transform(func(s string) string { return strings.ToUpper(s) })). // Transform to uppercase
		Use(flow.Logger("DESCRIPTIVE_YAML", 600))                                // Log descriptive YAML output

	var result string
	err := pipe.Run(context.Background(), convert.ToDescYaml(product), &result)
	if err != nil {
		log.Printf("Descriptive conversion error: %v", err)
		return
	}

	fmt.Printf("Generated descriptive YAML:\n%s\n", result)
}

// runLLMIntegrationExample shows round-trip descriptive converter usage with structured output parsing.
// Demonstrates ToDescYaml for input and FromDescYaml for parsing LLM's structured response.
func runLLMIntegrationExample() {
	fmt.Println("\nRunning LLM integration with structured output parsing...")

	product := ProductInfo{
		Name:        "Neural Interface",
		Category:    "AI Hardware",
		Price:       2499.99,
		Features:    []string{"Brain-Computer Interface", "AI Processing", "Neural Mapping"},
		Description: "This device calques neural patterns and transforms them into digital commands.",
	}

	// Load environment variables from .env file
	// Make sure to have GOOGLE_API_KEY set in your .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// // Create Gemini example provider (reads GOOGLE_API_KEY from env)
	// provider, err := llm.NewGeminiProvider("", "gemini-2.0-flash")
	// if err != nil {
	// 	log.Fatal("Failed to create Gemini provider:", err)
	// }

	// Create Ollama provider (connects to localhost:11434 by default)
	provider, err := llm.NewOllamaProvider("", "llama3.2:1b", llm.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	// Pipeline: struct -> descriptive YAML -> mock LLM -> parse structured response
	pipe := core.New()
	pipe.
		Use(flow.Logger("DESCRIPTIVE_INPUT", 600)). // Log descriptive YAML input
		Use(llm.Chat(provider)).                    // use Gemini LLM that returns structured YAML
		Use(flow.Logger("STRUCTURED_OUTPUT", 400))  // Log structured YAML response

	var analysis ProductAnalysis
	err = pipe.Run(context.Background(), convert.ToDescYaml(product), convert.FromDescYaml[ProductAnalysis](&analysis))
	if err != nil {
		log.Printf("LLM integration error: %v", err)
		return
	}

	fmt.Printf("Parsed structured analysis:\n")
	fmt.Printf("  Market Position: %s\n", analysis.MarketPosition)
	fmt.Printf("  Target Audience: %v\n", analysis.TargetAudience)
	fmt.Printf("  Strengths: %v\n", analysis.Strengths)
	fmt.Printf("  Recommendations: %s\n", analysis.Recommendations)
	fmt.Printf("  Competitor Price: $%.2f\n", analysis.CompetitorPrice)
}

// mockLLMAnalyzer simulates an LLM that returns structured YAML output
func mockLLMAnalyzer() core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		var input []byte
		_ = core.Read(r, &input) //Read the input even if we are not using it in this case

		// Simulate LLM generating structured YAML response based on descriptive input
		structuredResponse := `market_position: "Premium AI hardware for enterprise applications"
target_audience: ["AI Researchers", "Tech Companies", "Medical Institutions"]
strengths: ["Cutting-edge technology", "Neural processing capabilities", "Enterprise-grade hardware"]
recommendations: "Partner with research institutions and focus on B2B sales channels"
competitor_price: 2199.99`

		return core.Write(w, structuredResponse)
	})
}
