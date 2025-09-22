// Package main demonstrates MCP integration with calque flows
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/mcp"
)

func main() {
	runBasicExample()
	runRealisticExample()
	runAdvancedExample()
	runNaturalLanguageExample()
}

func runBasicExample() {
	fmt.Println("=== Example 1: Basic MCP Tool Calling ===")

	client, err := mcp.NewStdio("go", []string{"run", "cmd/server/main.go"})
	if err != nil {
		log.Printf("Failed to create MCP client: %v", err)
		return
	}
	defer client.Close()

	flow := calque.NewFlow()
	flow.Use(client.Tool("multiply"))

	input := `{"a": 6, "b": 7}`
	var output string

	if err := flow.Run(context.Background(), input, &output); err != nil {
		log.Printf("Tool call failed: %v", err)
		return
	}

	fmt.Printf("Result: 6 × 7 = %s\n", output)
}

func runRealisticExample() {
	fmt.Println("\n=== Example 2: Realistic MCP Usage ===")

	client, err := mcp.NewStdio("go", []string{"run", "cmd/server/main.go"})
	if err != nil {
		log.Printf("Failed to create MCP client: %v", err)
		return
	}
	defer client.Close()

	// 1. Use tool for search
	fmt.Println("1. Using search tool...")
	searchFlow := calque.NewFlow()
	searchFlow.Use(client.Tool("search"))

	searchInput := `{"query": "golang", "limit": 3}`
	var searchOutput string

	if err := searchFlow.Run(context.Background(), searchInput, &searchOutput); err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("Search results:\n%s\n", searchOutput)
	}

	// 2. Use resource to get documentation
	fmt.Println("2. Fetching API documentation resource...")
	resourceFlow := calque.NewFlow()
	resourceFlow.Use(client.Resource("file:///api-docs"))

	resourceInput := "I need to understand the API endpoints"
	var resourceOutput string

	if err := resourceFlow.Run(context.Background(), resourceInput, &resourceOutput); err != nil {
		log.Printf("Resource fetch failed: %v", err)
	} else {
		fmt.Printf("Documentation retrieved:\n%s\n", resourceOutput)
	}

	// 3. Use resource template for dynamic config access
	fmt.Println("3. Using resource template for config...")
	templateFlow := calque.NewFlow()
	templateFlow.Use(client.ResourceTemplate("file:///configs/{name}"))

	templateInput := `{"name": "database.json"}`
	var templateOutput string

	if err := templateFlow.Run(context.Background(), templateInput, &templateOutput); err != nil {
		log.Printf("Resource template failed: %v", err)
	} else {
		fmt.Printf("Config retrieved:\n%s\n", templateOutput)
	}
}

func runAdvancedExample() {
	fmt.Println("\n=== Example 3: Advanced MCP Features ===")

	client, err := mcp.NewStdio("go", []string{"run", "cmd/server/main.go"})
	if err != nil {
		log.Printf("Failed to create MCP client: %v", err)
		return
	}
	defer client.Close()

	// 1. Use prompt template
	fmt.Println("1. Using prompt template...")
	promptFlow := calque.NewFlow()
	promptFlow.Use(client.Prompt("code_review"))

	promptInput := `{"language": "go", "style": "security"}`
	var promptOutput string

	if err := promptFlow.Run(context.Background(), promptInput, &promptOutput); err != nil {
		log.Printf("Prompt failed: %v", err)
	} else {
		fmt.Printf("Generated prompt:\n%s\n", promptOutput)
	}

	// 2. Tool with progress tracking
	fmt.Println("2. Tool with progress tracking...")
	progressFlow := calque.NewFlow()

	var progressUpdates []string
	progressCallback := func(params *mcp.ProgressNotificationParams) {
		progressUpdates = append(progressUpdates, fmt.Sprintf("Progress: %.0f%%", params.Progress*100))
	}

	progressFlow.Use(client.Tool("progress_demo", progressCallback))

	progressInput := `{"steps": 5}`
	var progressOutput string

	if err := progressFlow.Run(context.Background(), progressInput, &progressOutput); err != nil {
		log.Printf("Progress tool failed: %v", err)
	} else {
		fmt.Printf("Progress tool result: %s\n", progressOutput)
		for _, update := range progressUpdates {
			fmt.Printf("  %s\n", update)
		}
	}
}

// runNaturalLanguageExample demonstrates natural language MCP tool interaction
func runNaturalLanguageExample() {
	fmt.Println("\n=== Example 4: Natural Language MCP ===")

	// Create MCP client
	mcpClient, err := mcp.NewStdio("go", []string{"run", "cmd/server/main.go"})
	if err != nil {
		log.Printf("Failed to create MCP client: %v", err)
		return
	}
	defer mcpClient.Close()

	// Create AI client for natural language processing
	aiClient, err := ollama.New("llama3.2:1b")
	if err != nil {
		log.Printf("AI client not available: %v", err)
		log.Println("Install and run ollama with llama3.2:1b model to try this feature")
		return
	}

	// Create flow that converts natural language to tool calls
	flow := calque.NewFlow()
	flow.Use(ctrl.Chain(
		mcp.Registry(mcpClient),     // 1. Discover available MCP tools
		mcp.Detect(aiClient),        // 2. Use AI to select appropriate tool
		mcp.ExtractParams(aiClient), // 3. Extract parameters from natural language
		mcp.Execute(),               // 4. Execute the tool with extracted parameters
	))

	// Test natural language inputs
	examples := []string{
		"Search for golang tutorials",
		"What is 6 times 7?",
		"Say hello to Bob",
		"Read the file /etc/hosts",
	}

	fmt.Println("Converting natural language to tool calls...")
	for i, input := range examples {
		fmt.Printf("\n%d. Input: \"%s\"\n", i+1, input)

		var output string
		if err := flow.Run(context.Background(), input, &output); err != nil {
			log.Printf("Error: %v", err)
		} else {
			fmt.Printf("   Output: %s\n", output)
		}
	}
}
