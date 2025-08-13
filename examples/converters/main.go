// Package main demonstrates converter functionality in the Calque-Pipe AI Agent Framework.
// Converters handle data format transformations at pipeline boundaries - converting input data
// to streams and output streams back to structured data. They're essential for working with
// structured formats like JSON, YAML, XML while maintaining the streaming architecture.
//
// When to use converters:
// - Working with structured data (JSON, YAML, XML)
// - Converting between different data formats
// - Integrating with APIs that expect specific formats
// - Processing structured LLM inputs/outputs
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/calque-ai/calque-pipe/pkg/convert"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ai"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ai/ollama"
	"github.com/calque-ai/calque-pipe/pkg/middleware/logger"
	"github.com/calque-ai/calque-pipe/pkg/middleware/prompt"
	"github.com/calque-ai/calque-pipe/pkg/middleware/text"
)

// ProductInfo represents structured product data
type ProductInfo struct {
	Name        string   `json:"name" yaml:"name"`
	Category    string   `json:"category" yaml:"category"`
	Price       float64  `json:"price" yaml:"price"`
	Features    []string `json:"features" yaml:"features"`
	Description string   `json:"description" yaml:"description"`
}

func main() {
	fmt.Println("Calque-Pipe Converter Examples")

	runConverterBasics() // Basic converter demo

	// Initialize AI client
	client, err := ollama.New("llama3.2:1b")
	if err != nil {
		log.Printf("Warning: Could not connect to Ollama: %v", err)
		log.Println("To use AI features, install Ollama and run: ollama pull llama3.2:1b")
		return
	}

	runAIConverterExample(client) // AI + converter demo
}

// runConverterBasics demonstrates basic converter concepts: parsing structured data formats.
// Shows how input converters parse format strings and output converters create structured data.
func runConverterBasics() {
	fmt.Println("\nRunning basic converter pipeline...")
	// Example: JSON string -> struct

	jsonString := `{
		"name": "Smart Widget",
		"category": "Electronics", 
		"price": 99.99,
		"features": ["WiFi", "Bluetooth", "Voice Control"],
		"description": "A widget that calques ideas from competing products."
	}`

	// Pipeline: Json String -> Json -> Uppercase -> Struct
	pipe := calque.NewFlow()
	pipe.
		Use(logger.Print("JSON_INPUT")).                                          // Log original JSON
		Use(text.Transform(func(s string) string { return strings.ToUpper(s) })). // Convert to uppercase for processing
		Use(logger.Print("UPPERCASE_JSON"))                                       // Log transformed JSON

	// Execute with json string input
	// convert.ToJson parses the string to make sure its valid json
	// convert.FromJson converts the uppercase json string back to a ProductInfo struct
	var jsonResult ProductInfo
	err := pipe.Run(context.Background(), convert.ToJson(jsonString), convert.FromJson(&jsonResult))
	if err != nil {
		log.Printf("JSON conversion error: %v", err)
		return
	}

	fmt.Printf("Parsed JSON: %s - $%.2f\n", jsonResult.Name, jsonResult.Price)

}

// runAIConverterExample shows converter usage with AI integration.
// Pipeline: YAML struct input -> AI prompt -> result string.
func runAIConverterExample(client ai.Client) {
	fmt.Println("\nRunning AI-powered converter pipeline...")

	// Input product data
	product := ProductInfo{
		Name:        "Neural Interface",
		Category:    "AI Hardware",
		Price:       2499.99,
		Features:    []string{"Brain-Computer Interface", "AI Processing", "Neural Mapping"},
		Description: "This device calques neural patterns and transforms them into digital commands.",
	}

	// Pipeline: struct -> YAML -> AI analysis -> result string
	pipe := calque.NewFlow()
	pipe.
		Use(logger.Print("STRUCT_INPUT")).
		Use(prompt.Template("Analyze this product data and suggest improvements:\n\n{{.Input}}\n\nProvide a brief analysis.")).
		Use(logger.Print("AI_PROMPT")).
		Use(ai.Agent(client))

	// Execute with struct input, get string result
	// Run handles input or output of strings, bytes and readers/writers automatically without converters
	var result string
	err := pipe.Run(context.Background(), convert.ToYaml(product), &result)
	if err != nil {
		log.Printf("Pipeline error: %v", err)
		return
	}

	fmt.Print("AI ANALYSIS RESULT:\n")
	fmt.Println(result)
}
