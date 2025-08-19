package multiagent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Mock agent that returns a fixed response
func mockAgent(response string) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		return calque.Write(res, []byte(response))
	})
}

// Simple test vote function that returns first response
func firstVote(responses []string) (string, error) {
	if len(responses) == 0 {
		return "", fmt.Errorf("no responses to vote on")
	}
	return responses[0], nil
}

func TestConsensus_BasicOperation(t *testing.T) {
	agents := []calque.Handler{
		mockAgent("response1"),
		mockAgent("response2"),
		mockAgent("response3"),
	}

	consensus := SimpleConsensus(agents, firstVote, 1)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := consensus.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if result != "response1" {
		t.Errorf("Expected 'response1', got: %s", result)
	}
}

func TestConsensus_MinResponsesValidation(t *testing.T) {
	agents := []calque.Handler{
		mockAgent("response1"),
		mockAgent("response2"),
	}

	// Require 3 responses but only have 2 agents
	consensus := SimpleConsensus(agents, firstVote, 3)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := consensus.ServeFlow(req, res)
	if err == nil {
		t.Fatal("Expected error for insufficient responses")
	}

	expectedMsg := "insufficient responses: got 2, need 3"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, err)
	}
}

func TestConsensus_EmptyAgents(t *testing.T) {
	var agents []calque.Handler

	consensus := SimpleConsensus(agents, firstVote, 1)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := consensus.ServeFlow(req, res)
	if err == nil {
		t.Fatal("Expected error for no agents")
	}

	if !strings.Contains(err.Error(), "no agents provided") {
		t.Errorf("Expected 'no agents provided' error, got: %v", err)
	}
}

func TestConsensus_NilVoteFunc(t *testing.T) {
	agents := []calque.Handler{
		mockAgent("response1"),
	}

	consensus := SimpleConsensus(agents, nil, 1)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := consensus.ServeFlow(req, res)
	if err == nil {
		t.Fatal("Expected error for nil vote function")
	}

	if !strings.Contains(err.Error(), "vote function cannot be nil") {
		t.Errorf("Expected 'vote function cannot be nil' error, got: %v", err)
	}
}

func TestConsensus_VoteFunctionError(t *testing.T) {
	agents := []calque.Handler{
		mockAgent("response1"),
		mockAgent("response2"),
	}

	// Vote function that always returns an error
	errorVote := func(responses []string) (string, error) {
		return "", fmt.Errorf("voting error")
	}

	consensus := SimpleConsensus(agents, errorVote, 1)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := consensus.ServeFlow(req, res)
	if err == nil {
		t.Fatal("Expected error from vote function")
	}

	if !strings.Contains(err.Error(), "voting failed") {
		t.Errorf("Expected 'voting failed' error, got: %v", err)
	}
}

func TestConsensus_EmptyResponses(t *testing.T) {
	agents := []calque.Handler{
		mockAgent(""),    // Empty response
		mockAgent("   "), // Whitespace only
		mockAgent("valid"),
	}

	consensus := SimpleConsensus(agents, firstVote, 1)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := consensus.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should only get the valid response (empty ones filtered out)
	result := output.String()
	if result != "valid" {
		t.Errorf("Expected 'valid', got: %s", result)
	}
}

func TestConsensus_AllEmptyResponses(t *testing.T) {
	agents := []calque.Handler{
		mockAgent(""),    // Empty response
		mockAgent("   "), // Whitespace only
	}

	consensus := SimpleConsensus(agents, firstVote, 1)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := consensus.ServeFlow(req, res)
	if err == nil {
		t.Fatal("Expected error for all empty responses")
	}

	if !strings.Contains(err.Error(), "insufficient responses: got 0, need 1") {
		t.Errorf("Expected insufficient responses error, got: %v", err)
	}
}
