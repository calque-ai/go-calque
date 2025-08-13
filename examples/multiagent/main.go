package main

import (
	"context"
	"fmt"

	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ai"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ctrl"
	"github.com/calque-ai/calque-pipe/pkg/middleware/multiagent"
	"github.com/calque-ai/calque-pipe/pkg/middleware/tools"
)

func main() {
	fmt.Println("Multi-Agent Pipeline Examples")
	fmt.Println("=================================")

	fmt.Println("\n1. Smart Router Example:")
	smartRouter()

	fmt.Println("\n2. Load Balancer Example:")
	loadBalancer()

	fmt.Println("\n3. Fallback Example:")
	fallbackExample()

	fmt.Println("\n4. Complex Pipeline Example:")
	complexPipeline()

	fmt.Println("\n4. Combined Routing Example:")
	combinedRouting()
}

// smartRouter demonstrates intelligent agent selection using schema-based routing
func smartRouter() {
	// Create AI clients (in practice, these would be your actual AI clients)
	mathClient := ai.NewMockClient("42")
	codeClient := ai.NewMockClient("func hello() { fmt.Println(\"Hello, World!\") }")

	// Mock selection client that returns JSON schema response for "math"
	selectionClient := ai.NewMockClient(`{"route": "math", "confidence": 0.95, "reasoning": "Mathematical calculation request"}`)

	// Uncomment to use Ollama or other AI client
	// selectionClient, err := ollama.New("llama3.2:1b")
	// if err != nil {
	// 	fmt.Printf("Failed to create Ollama provider: %v", err)
	// }

	// Create routed handlers with metadata
	mathHandler := multiagent.Route(
		ai.Agent(mathClient),
		"math",
		"Solve mathematical problems, calculations, equations, and numerical analysis",
		"calculate,solve,math,equation")

	codeHandler := multiagent.Route(
		ai.Agent(codeClient),
		"code",
		"Programming, debugging, code review, and software development tasks",
		"code,program,debug,function")

	// Build router with intelligent selection - no prompts needed!
	// Router automatically creates structured input with route metadata and uses JSON Schema
	router := multiagent.Router(selectionClient, mathHandler, codeHandler)

	// Complete pipeline
	pipeline := calque.NewFlow().Use(router)

	// The router will automatically select the best handler based on the input
	var result string
	err := pipeline.Run(context.Background(), "What is the square root of 144?", &result)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Input: What is the square root of 144?\n")
	fmt.Printf("   Output: %s\n", result)
}

// loadBalancer demonstrates distributing load across multiple agents
func loadBalancer() {
	// Multiple instances of the same agent type for scaling
	agent1 := ai.Agent(ai.NewMockClient("Response from agent 1"))
	agent2 := ai.Agent(ai.NewMockClient("Response from agent 2"))
	agent3 := ai.Agent(ai.NewMockClient("Response from agent 3"))

	// Distribute requests using round-robin
	scaledAgent := multiagent.LoadBalancer(agent1, agent2, agent3)

	pipeline := calque.NewFlow().Use(scaledAgent)

	// Each request goes to a different agent instance
	for i := range 3 {
		var result string
		err := pipeline.Run(context.Background(), fmt.Sprintf("Request %d", i), &result)
		if err != nil {
			fmt.Printf("   Error on request %d: %v\n", i, err)
			continue
		}
		fmt.Printf("   Request %d: %s\n", i, result)
	}
}

// fallbackExample demonstrates agent reliability with automatic failover
func fallbackExample() {
	// Create agents with different reliability levels
	primaryAgent := ai.NewMockClientWithError("primary agent failed")  // Will fail
	backupAgent := ai.Agent(ai.NewMockClient("Backup agent response")) // Will succeed
	localAgent := ai.Agent(ai.NewMockClient("Local agent response"))   // Fallback

	// Set up fallback chain - tries agents in order until one succeeds
	reliableAgent := ctrl.Fallback(
		ai.Agent(primaryAgent),
		backupAgent,
		localAgent)

	pipeline := calque.NewFlow().Use(reliableAgent)

	var result string
	err := pipeline.Run(context.Background(), "Explain quantum computing", &result)
	if err != nil {
		fmt.Printf("   All agents failed: %v\n", err)
		return
	}

	fmt.Printf("   Input: Explain quantum computing\n")
	fmt.Printf("   Output: %s\n", result)
}

// complexPipeline demonstrates a complex multi-agent pipeline with composable sub-pipelines
func complexPipeline() {
	// Create different types of agents
	mathAgent := multiagent.Route(
		ai.Agent(ai.NewMockClient("The answer is 42")),
		"math", "Mathematical problem solver", "calculate,solve,math")

	// Code agent with tools pipeline
	codeTools := []tools.Tool{
		tools.Simple("format_code", "Format and beautify code", func(code string) string {
			return fmt.Sprintf("// Formatted code:\n%s", code)
		}),
	}

	codeWithTools := calque.NewFlow().
		Use(ai.Agent(ai.NewMockClient("func hello() { fmt.Println(\"Hello!\") }"), ai.WithTools(codeTools...)))

	codeAgent := multiagent.Route(
		codeWithTools,
		"code", "Programming assistant with code tools", "code,program,function")

	// Research agent (mock web search)
	researchAgent := multiagent.Route(
		ai.Agent(ai.NewMockClient("Based on my research, the answer is...")),
		"research", "Information gathering and web search", "search,find,research,information")

	// Mock selection client that returns structured JSON response for "math"
	selectionClient := ai.NewMockClient(`{"route": "math", "confidence": 0.9, "reasoning": "Philosophical question - math can provide logical analysis"}`)

	// Main router - automatically handles schema-based routing
	router := multiagent.Router(selectionClient, mathAgent, codeAgent, researchAgent)

	// Complete pipeline
	pipeline := calque.NewFlow().Use(router)

	var result string
	err := pipeline.Run(context.Background(), "What's the meaning of life?", &result)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Input: What's the meaning of life?\n")
	fmt.Printf("   Output: %s\n", result)
}

// combinedRouting demonstrates combining multiple routing strategies
func combinedRouting() {
	// Create specialized agent pools
	mathAgent1 := ai.Agent(ai.NewMockClient("Math result from agent 1"))
	mathAgent2 := ai.Agent(ai.NewMockClient("Math result from agent 2"))
	mathAgents := multiagent.LoadBalancer(mathAgent1, mathAgent2)

	// Code agents with fallback
	primaryCodeAgent := ai.NewMockClientWithError("code agent failed")
	backupCodeAgent := ai.Agent(ai.NewMockClient("Code result from backup"))
	codeAgents := ctrl.Fallback(
		ai.Agent(primaryCodeAgent),
		backupCodeAgent)

	// Create routed handler pools
	mathHandler := multiagent.Route(mathAgents, "math", "Mathematical calculations", "math,calculate")
	codeHandler := multiagent.Route(codeAgents, "code", "Programming tasks", "code,program")

	// Mock selection client that chooses "code" for demo
	selectionClient := ai.NewMockClient(`{"route": "code", "confidence": 0.8, "reasoning": "Code-related request detected"}`)

	// High-level routing to agent pools
	router := multiagent.Router(selectionClient, mathHandler, codeHandler)

	pipeline := calque.NewFlow().Use(router)

	var result string
	err := pipeline.Run(context.Background(), "code: write a hello world function", &result)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Combined routing result: %s\n", result)
}
