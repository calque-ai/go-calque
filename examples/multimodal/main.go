// Package main demonstrates multimodal AI capabilities with the calque framework.
// It showcases how to process images and text together using different AI models,
// including both serialized and streaming approaches for handling binary data.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/convert"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/gemini"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/openai"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
)

func main() {
	// Load environment variables
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <image_path>")
		fmt.Println("  go run main.go ./image.jpg")
		os.Exit(1)
	}

	imagePath := os.Args[1]

	fmt.Println("=== Simple Approach (ai.ImageData with []byte) ===")
	analyzeImageSimple(imagePath)

	fmt.Println("\n=== Streaming Approach (ai.Image with Reader) ===")
	analyzeImageStreaming(imagePath)

	fmt.Println("\n=== Ollama Multimodal (granite3.2 Vision Model) ===")
	analyzeImageOllama(imagePath)

	fmt.Println("\n=== OpenAI Multimodal (GPT-4o Vision) ===")
	analyzeImageOpenAI(imagePath)

}

// analyzeImageSimple processes image input with AI using simple multimodal approach
func analyzeImageSimple(imagePath string) {
	fmt.Printf("Processing image: %s\n\n", imagePath)

	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatal("Failed to read image:", err)
	}

	// Create multimodal input with simple approach (ai.ImageData with []byte)
	multimodalInput := ai.Multimodal(
		ai.Text("Please analyze this image and describe what you see in detail. Include information about objects, people, colors, composition, and any text visible."),
		ai.ImageData(imageData, "image/jpeg"), // Serialized approach - data embedded in JSON
	)

	// Create Gemini client
	client, err := gemini.New("gemini-2.0-flash")
	if err != nil {
		log.Fatal("Failed to create Gemini client:", err)
	}

	flow := calque.NewFlow()

	flow.
		Use(inspect.Head("INPUT", 500)). // Will show JSON with base64 data
		Use(ai.Agent(client)).
		Use(inspect.Head("RESPONSE", 200))

	// Run the flow - all data is serialized to json in the multimodal input
	var result string
	err = flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nSimple Image Analysis Result:")
	fmt.Println(result)
}

// analyzeImageStreaming processes image input with AI using streaming multimodal approach
// This allows the AI to handle large binary data without loading it fully into memory
// Note: This requires the llm client to support streaming data input.
// If streaming is not supported the data will be buffered by the client,
// but not serialized as in ImageData().
func analyzeImageStreaming(imagePath string) {
	fmt.Printf("Processing image: %s\n\n", imagePath)

	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatal("Failed to read image:", err)
	}

	// Create multimodal input with streaming approach (ai.Image with Reader)
	multimodalInput := ai.Multimodal(
		ai.Text("Please analyze this image and describe what you see in detail. Include information about objects, people, colors, composition, and any text visible."),
		ai.Image(bytes.NewReader(imageData), "image/jpeg"), // Streaming approach - Reader allows incremental reading
	)

	// Create Gemini client
	client, err := gemini.New("gemini-2.0-flash")
	if err != nil {
		log.Fatal("Failed to create Gemini client:", err)
	}

	// Create flow with multimodal support
	flow := calque.NewFlow()

	flow.
		Use(inspect.Print("INPUT")).                                    // Will show JSON metadata only
		Use(ai.Agent(client, ai.WithMultimodalData(&multimodalInput))). // Multimodal data via option
		Use(inspect.Head("RESPONSE", 200))

	// Run the flow - input is JSON metadata, actual data is in WithMultimodalData
	var result string
	err = flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nStreaming Image Analysis Result:")
	fmt.Println(result)
}

// analyzeImageOllama processes image input with Ollama's vision models (LLaVA)
func analyzeImageOllama(imagePath string) {
	fmt.Printf("Processing image with Ollama: %s\n\n", imagePath)

	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatal("Failed to read image:", err)
	}

	// Create multimodal input - works exactly the same as Gemini!
	multimodalInput := ai.Multimodal(
		ai.Text("Please analyze this image and describe what you see in detail. Include information about objects, people, colors, composition, and any text visible."),
		ai.ImageData(imageData, "image/jpeg"), // Simple approach for Ollama
	)

	// Create Ollama client with vision model (granite3.2-vision)
	client, err := ollama.New("granite3.2-vision") // granite3.2-vision
	if err != nil {
		log.Printf("Failed to create Ollama client (is Ollama running with a vision model?): %v", err)
		log.Println("To use Ollama with vision:")
		log.Println("  1. Install Ollama: https://ollama.ai")
		log.Println("  2. Run: ollama pull granite3.2-vision")
		log.Println("  3. Ensure Ollama is running (ollama serve)")
		return
	}

	// Create flow - same API as Gemini!
	flow := calque.NewFlow()

	flow.
		Use(inspect.Head("INPUT", 200)). // Show some of the JSON
		Use(ai.Agent(client)).           // No WithMultimodalData needed for simple approach
		Use(inspect.Head("RESPONSE", 200))

	// Run the flow - identical to Gemini usage
	var result string
	err = flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		log.Printf("Ollama analysis failed: %v", err)
		return
	}

	fmt.Println("\nOllama Image Analysis Result:")
	fmt.Println(result)
}

// analyzeImageOpenAI processes image input with OpenAI's GPT-4o Vision model
func analyzeImageOpenAI(imagePath string) {
	fmt.Printf("Processing image with OpenAI: %s\n\n", imagePath)

	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatal("Failed to read image:", err)
	}

	// Create multimodal input
	multimodalInput := ai.Multimodal(
		ai.Text("Please analyze this image and describe what you see in detail. Include information about objects, people, colors, composition, and any text visible."),
		ai.ImageData(imageData, "image/jpeg"), // Simple approach for OpenAI
	)

	// Create OpenAI client with GPT-4o Vision
	client, err := openai.New("gpt-4o") // gpt-4o supports vision
	if err != nil {
		log.Printf("Failed to create OpenAI client: %v", err)
		log.Println("To use OpenAI with vision:")
		log.Println("  1. Get API key from: https://platform.openai.com/api-keys")
		log.Println("  2. Create .env file with: OPENAI_API_KEY=your_api_key")
		log.Println("  3. Use gpt-4o or gpt-4o-mini model for vision support")
		return
	}

	// Create flow
	flow := calque.NewFlow()

	flow.
		Use(inspect.Head("INPUT", 200)). // Show some of the JSON
		Use(ai.Agent(client)).           // No WithMultimodalData needed for simple approach
		Use(inspect.Head("RESPONSE", 200))

	// Run the flow
	var result string
	err = flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		log.Printf("OpenAI analysis failed: %v", err)
		return
	}

	fmt.Println("\nOpenAI Image Analysis Result:")
	fmt.Println(result)
}
