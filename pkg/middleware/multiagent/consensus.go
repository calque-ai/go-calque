// Package multiagent provides consensus mechanisms for coordinating multiple AI agents.
// It implements voting and consensus algorithms that allow multiple agents to work
// together and reach agreement on responses, enabling more robust and reliable
// AI-powered workflows.
package multiagent

import (
	"bytes"
	"fmt"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
)

// VoteFunc defines how to vote/merge responses from multiple agents
type VoteFunc func(responses []string) (string, error)

// SimpleConsensus executes multiple agents in parallel and applies a voting function to merge their responses.
// This is a single-round consensus mechanism where agents run independently and their outputs are combined.
//
// Input: any data type (passes same input to all agents)
// Output: result of voting function applied to agent responses
// Behavior: BUFFERED - collects all agent responses before voting
//
// Runs all agents concurrently with the same input, then applies the voting function
// to determine the final result. Agents do not interact with each other - they operate
// independently and the voting happens after all responses are collected.
//
// The minResponses parameter ensures a minimum number of valid responses before
// applying the voting function. If fewer valid responses are received, an error is returned.
//
// Example:
//
//	agents := []calque.Handler{agent1, agent2, agent3}
//	consensus := multiagent.SimpleConsensus(agents, votingFunc, 2)
//	flow.Use(consensus)
func SimpleConsensus(agents []calque.Handler, voteFunc VoteFunc, minResponses int) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		if len(agents) == 0 {
			return fmt.Errorf("no agents provided for consensus")
		}

		if voteFunc == nil {
			return fmt.Errorf("vote function cannot be nil")
		}

		// Execute all agents in parallel
		parallel := ctrl.Parallel(agents...)

		var combinedOutput bytes.Buffer
		parallelRes := &calque.Response{Data: &combinedOutput}

		if err := parallel.ServeFlow(req, parallelRes); err != nil {
			return fmt.Errorf("agent execution failed: %w", err)
		}

		// Split responses (parallel uses "\n---\n" separator)
		responses := bytes.Split(combinedOutput.Bytes(), []byte("\n---\n"))

		// Filter empty responses and convert to strings
		var validResponses []string
		for _, resp := range responses {
			if len(bytes.TrimSpace(resp)) > 0 {
				validResponses = append(validResponses, string(resp))
			}
		}

		if len(validResponses) < minResponses {
			return fmt.Errorf("insufficient responses: got %d, need %d",
				len(validResponses), minResponses)
		}

		// Apply voting function
		result, err := voteFunc(validResponses)
		if err != nil {
			return fmt.Errorf("voting failed: %w", err)
		}

		return calque.Write(res, []byte(result))
	})
}
