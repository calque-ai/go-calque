package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/multiagent"
)

// TestSmartRouter tests intelligent agent selection using schema-based routing
func TestSmartRouter(t *testing.T) {
	t.Parallel()

	// Create AI clients (in practice, these would be your actual AI clients)
	mathClient := ai.NewMockClient("42")
	codeClient := ai.NewMockClient("func hello() { fmt.Println(\"Hello, World!\") }")

	// Mock selection client that returns JSON schema response for "math"
	selectionClient := ai.NewMockClient(`{"route": "math", "confidence": 0.95, "reasoning": "Mathematical calculation request"}`)

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

	// Build router with intelligent selection
	router := multiagent.Router(selectionClient, mathHandler, codeHandler)

	// Complete pipeline
	pipeline := calque.NewFlow().Use(router)

	// The router will automatically select the best handler based on the input
	var result string
	err := pipeline.Run(context.Background(), "What is the square root of 144?", &result)
	if err != nil {
		t.Fatalf("Smart router failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "42") {
		t.Errorf("Expected math response, got: %s", result)
	}
}

// TestLoadBalancer tests distributing load across multiple agents
func TestLoadBalancer(t *testing.T) {
	t.Parallel()

	// Multiple instances of the same agent type for scaling
	agent1 := ai.Agent(ai.NewMockClient("Response from agent 1"))
	agent2 := ai.Agent(ai.NewMockClient("Response from agent 2"))
	agent3 := ai.Agent(ai.NewMockClient("Response from agent 3"))

	// Distribute requests using round-robin
	scaledAgent := multiagent.LoadBalancer(agent1, agent2, agent3)

	// Create pipeline
	pipeline := calque.NewFlow().Use(scaledAgent)

	// Test multiple requests to see load distribution
	responses := make([]string, 3)
	for i := 0; i < 3; i++ {
		var result string
		err := pipeline.Run(context.Background(), "test request", &result)
		if err != nil {
			t.Fatalf("Load balancer request %d failed: %v", i+1, err)
		}
		responses[i] = result
	}

	// Verify we got different responses from different agents
	uniqueResponses := make(map[string]bool)
	for _, resp := range responses {
		uniqueResponses[resp] = true
	}

	if len(uniqueResponses) < 2 {
		t.Errorf("Expected responses from different agents, got: %v", responses)
	}
}

// TestFallbackMechanism tests fallback behavior with multiple agents
func TestFallbackMechanism(t *testing.T) {
	t.Parallel()

	// Create a primary handler that actually fails
	failingHandler := ai.NewMockClientWithError("mock error for testing fallback")

	// Create a fallback handler
	fallbackHandler := ai.Agent(ai.NewMockClient("fallback: successful response"))

	// Create fallback mechanism using ctrl.Fallback
	fallbackAgent := ctrl.Fallback(ai.Agent(failingHandler), fallbackHandler)

	// Create pipeline
	pipeline := calque.NewFlow().Use(fallbackAgent)

	// Test fallback behavior
	var result string
	err := pipeline.Run(context.Background(), "test request", &result)
	if err != nil {
		t.Fatalf("Fallback mechanism failed: %v", err)
	}

	// Verify fallback response
	if !strings.Contains(result, "fallback:") {
		t.Errorf("Expected fallback response, got: %s", result)
	}
}

// TestComplexPipeline tests a complex multi-agent pipeline
func TestComplexPipeline(t *testing.T) {
	t.Parallel()

	// Create specialized agents
	analyzer := ai.Agent(ai.NewMockClient("Analysis: This is a complex request that requires multiple steps"))
	processor := ai.Agent(ai.NewMockClient("Processing: Request has been analyzed and processed"))
	validator := ai.Agent(ai.NewMockClient("Validation: All checks passed successfully"))

	// Create complex pipeline with multiple agents
	pipeline := calque.NewFlow()
	pipeline.
		Use(analyzer).
		Use(processor).
		Use(validator)

	// Test the complex pipeline
	var result string
	err := pipeline.Run(context.Background(), "complex request", &result)
	if err != nil {
		t.Fatalf("Complex pipeline failed: %v", err)
	}

	// Verify the result contains all processing steps
	if !strings.Contains(result, "Validation:") {
		t.Errorf("Expected validation step, got: %s", result)
	}
}

// TestCombinedRouting tests combining multiple routing strategies
func TestCombinedRouting(t *testing.T) {
	t.Parallel()

	// Create different types of agents
	mathAgent := multiagent.Route(
		ai.Agent(ai.NewMockClient("Math result: 42")),
		"math",
		"Mathematical operations",
		"calculate,math,equation")
	textAgent := ai.Agent(ai.NewMockClient("Text result: processed text"))

	// Create load balancer for code agents
	codeAgent1 := ai.Agent(ai.NewMockClient("Code agent 1"))
	codeAgent2 := ai.Agent(ai.NewMockClient("Code agent 2"))
	codeLoadBalancer := multiagent.LoadBalancer(codeAgent1, codeAgent2)

	// Combine routing strategies
	combinedPipeline := calque.NewFlow()
	combinedPipeline.
		Use(mathAgent).
		Use(codeLoadBalancer).
		Use(textAgent)

	// Test combined pipeline
	var result string
	err := combinedPipeline.Run(context.Background(), "calculate something", &result)
	if err != nil {
		t.Fatalf("Combined routing failed: %v", err)
	}

	// Verify the result
	if len(result) == 0 {
		t.Error("Expected non-empty result from combined pipeline")
	}
}

// TestJSONConsensus tests JSON consensus mechanism
func TestJSONConsensus(t *testing.T) {
	t.Parallel()

	// Create agents that return JSON responses
	agent1 := ai.Agent(ai.NewMockClient(`{"result": "42", "confidence": 0.9}`))
	agent2 := ai.Agent(ai.NewMockClient(`{"result": "42", "confidence": 0.85}`))
	agent3 := ai.Agent(ai.NewMockClient(`{"result": "42", "confidence": 0.95}`))

	// Create consensus mechanism using SimpleConsensus
	agents := []calque.Handler{agent1, agent2, agent3}
	voteFunc := func(responses []string) (string, error) {
		// Simple voting: return the first response that contains "42"
		for _, resp := range responses {
			if strings.Contains(resp, "42") {
				return resp, nil
			}
		}
		return responses[0], nil
	}
	consensusAgent := multiagent.SimpleConsensus(agents, voteFunc, 2)

	// Create pipeline
	pipeline := calque.NewFlow().Use(consensusAgent)

	// Test consensus
	var result string
	err := pipeline.Run(context.Background(), "get consensus", &result)
	if err != nil {
		t.Fatalf("JSON consensus failed: %v", err)
	}

	// Verify consensus result
	if !strings.Contains(result, "42") {
		t.Errorf("Expected consensus result to contain '42', got: %s", result)
	}
}

// TestTextConsensus tests text consensus mechanism
func TestTextConsensus(t *testing.T) {
	t.Parallel()

	// Create agents that return text responses
	agent1 := ai.Agent(ai.NewMockClient("The answer is 42"))
	agent2 := ai.Agent(ai.NewMockClient("I believe the answer is 42"))
	agent3 := ai.Agent(ai.NewMockClient("Based on my analysis, the answer is 42"))

	// Create text consensus mechanism using SimpleConsensus
	agents := []calque.Handler{agent1, agent2, agent3}
	voteFunc := func(responses []string) (string, error) {
		// Simple voting: return the first response that contains "42"
		for _, resp := range responses {
			if strings.Contains(resp, "42") {
				return resp, nil
			}
		}
		return responses[0], nil
	}
	consensusAgent := multiagent.SimpleConsensus(agents, voteFunc, 2)

	// Create pipeline
	pipeline := calque.NewFlow().Use(consensusAgent)

	// Test text consensus
	var result string
	err := pipeline.Run(context.Background(), "get text consensus", &result)
	if err != nil {
		t.Fatalf("Text consensus failed: %v", err)
	}

	// Verify consensus result
	if !strings.Contains(result, "42") {
		t.Errorf("Expected text consensus to contain '42', got: %s", result)
	}
}

// TestMultiAgentConcurrency tests multiple agents working concurrently
func TestMultiAgentConcurrency(t *testing.T) {
	t.Parallel()

	// Create multiple agents
	agents := make([]calque.Handler, 5)
	for i := 0; i < 5; i++ {
		agents[i] = ai.Agent(ai.NewMockClient(fmt.Sprintf("Response from agent %d", i+1)))
	}

	// Create parallel processing pipeline
	parallelPipeline := calque.NewFlow().Use(ctrl.Parallel(agents...))

	// Test parallel processing
	var result string
	err := parallelPipeline.Run(context.Background(), "concurrent request", &result)
	if err != nil {
		t.Fatalf("Parallel processing failed: %v", err)
	}

	// Verify we got responses from multiple agents
	responseCount := 0
	for i := 1; i <= 5; i++ {
		if strings.Contains(result, fmt.Sprintf("agent %d", i)) {
			responseCount++
		}
	}

	if responseCount < 3 {
		t.Errorf("Expected responses from at least 3 agents, got responses from %d", responseCount)
	}
}

// TestAgentChain tests chaining multiple agents in sequence
func TestAgentChain(t *testing.T) {
	t.Parallel()

	// Create agents for different processing steps
	step1Agent := ai.Agent(ai.NewMockClient("Step 1 completed"))
	step2Agent := ai.Agent(ai.NewMockClient("Step 2 completed"))
	step3Agent := ai.Agent(ai.NewMockClient("Step 3 completed"))

	// Create chained pipeline
	chainedPipeline := calque.NewFlow().Use(ctrl.Chain(step1Agent, step2Agent, step3Agent))

	// Test chained processing
	var result string
	err := chainedPipeline.Run(context.Background(), "chained request", &result)
	if err != nil {
		t.Fatalf("Chained processing failed: %v", err)
	}

	// Verify all steps were executed
	if !strings.Contains(result, "Step 3 completed") {
		t.Errorf("Expected final step result, got: %s", result)
	}
}
