// Package main demonstrates multi-agent coordination patterns with the calque framework.
// It showcases intelligent routing, load balancing, consensus mechanisms,
// and complex agent orchestration for building sophisticated AI workflows.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/multiagent"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
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

	fmt.Println("\n5. Combined Routing Example:")
	combinedRouting()

	fmt.Println("\n6. JSON Consensus Example:")
	jsonConsensusExample()

	fmt.Println("\n7. Text Consensus Example:")
	textConsensusExample()
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

// jsonConsensusExample demonstrates consensus with structured JSON data using numeric scoring
func jsonConsensusExample() {
	// Create mock agents that return JSON with scores
	agent1 := ai.Agent(ai.NewMockClient(`{"answer": "positive", "confidence": 0.85, "reasoning": "Strong positive indicators"}`))
	agent2 := ai.Agent(ai.NewMockClient(`{"answer": "positive", "confidence": 0.90, "reasoning": "Clear positive sentiment"}`))
	agent3 := ai.Agent(ai.NewMockClient(`{"answer": "negative", "confidence": 0.60, "reasoning": "Some negative elements"}`))

	// Custom voting function that extracts JSON data and uses confidence scoring
	confidenceVote := func(responses []string) (string, error) {
		type SentimentResponse struct {
			Answer     string  `json:"answer"`
			Confidence float64 `json:"confidence"`
			Reasoning  string  `json:"reasoning"`
		}

		var weightedVotes = make(map[string]float64)
		var responseMap = make(map[string]string)

		for _, resp := range responses {
			var sentiment SentimentResponse
			if err := json.Unmarshal([]byte(resp), &sentiment); err != nil {
				continue // Skip malformed responses
			}

			// Weight votes by confidence score
			weightedVotes[sentiment.Answer] += sentiment.Confidence
			responseMap[sentiment.Answer] = resp
		}

		// Find answer with highest weighted confidence
		var maxWeight float64
		var winner string
		for answer, weight := range weightedVotes {
			if weight > maxWeight {
				maxWeight = weight
				winner = answer
			}
		}

		if winner == "" {
			return "", fmt.Errorf("no valid responses found")
		}

		return responseMap[winner], nil
	}

	// Create consensus with confidence-weighted voting
	consensus := multiagent.SimpleConsensus([]calque.Handler{agent1, agent2, agent3}, confidenceVote, 2)

	pipeline := calque.NewFlow().Use(consensus)

	var result string
	err := pipeline.Run(context.Background(), "Analyze sentiment: 'I love this product!'", &result)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Input: Analyze sentiment: 'I love this product!'\n")
	fmt.Printf("   Consensus Result: %s\n", result)
	fmt.Printf("   Strategy: Confidence-weighted voting (positive: 1.75, negative: 0.60)\n")
}

// textConsensusExample demonstrates simple voting consensus for text responses
func textConsensusExample() {
	// Create mock agents with different viewpoints
	agent1 := ai.Agent(ai.NewMockClient("The answer is A because it has strong evidence and data support."))
	agent2 := ai.Agent(ai.NewMockClient("I believe the answer is B based on historical patterns and research."))
	agent3 := ai.Agent(ai.NewMockClient("The evidence points to A, with multiple studies confirming this conclusion."))
	agent4 := ai.Agent(ai.NewMockClient("Answer is B, as it aligns with recent findings and expert opinions."))

	// Createa a voting function that extracts A or B answers and counts votes
	simpleVote := func(responses []string) (string, error) {
		if len(responses) == 0 {
			return "", fmt.Errorf("no responses available")
		}

		// Count votes for each answer
		votes := make(map[string]int)
		responseMap := make(map[string]string)

		for _, resp := range responses {
			text := strings.ToLower(resp)

			// Extract the answer (A or B)
			var answer string
			switch {
			case strings.Contains(text, "answer is a"):
				answer = "A"
			case strings.Contains(text, "answer is b"):
				answer = "B"
			default:
				answer = "unknown"
			}

			votes[answer]++
			responseMap[answer] = resp // Keep one response for each answer
			fmt.Printf("   Agent voted for: %s\n", answer)
		}

		// Find answer with most votes
		var winner string
		var maxVotes int
		for answer, count := range votes {
			fmt.Printf("   '%s' received %d votes\n", answer, count)
			if count > maxVotes {
				maxVotes = count
				winner = answer
			}
		}

		if winner == "" || winner == "unknown" {
			return "", fmt.Errorf("no valid answers found")
		}

		return responseMap[winner], nil
	}

	// Create consensus with simple voting strategy
	consensus := multiagent.SimpleConsensus([]calque.Handler{agent1, agent2, agent3, agent4}, simpleVote, 2)

	pipeline := calque.NewFlow().Use(consensus)

	var result string
	err := pipeline.Run(context.Background(), "What is the correct answer and why?", &result)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Input: What is the correct answer and why?\n")
	fmt.Printf("   Winning Response: %s\n", result)
	fmt.Printf("   Strategy: Simple majority voting (extracts A or B answers and counts votes)\n")
}
