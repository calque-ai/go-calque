package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

// TestAgentEndpoint tests the /agent endpoint with various inputs
func TestAgentEndpoint(t *testing.T) {
	// Create the agent flow
	agentFlow := createAgentFlow()

	// Create test server
	handler := handleAgent(agentFlow)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name           string
		request        Request
		expectedStatus int
		shouldContain  []string
	}{
		{
			name: "Customer support request",
			request: Request{
				Message: "I'm having trouble logging into my account. Can you help me reset my password?",
				UserID:  "user_12345",
			},
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"Processed:", "TROUBLE", "ACCOUNT"},
		},
		{
			name: "Product inquiry",
			request: Request{
				Message: "What are the features of your premium subscription plan?",
				UserID:  "prospect_67890",
			},
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"Processed:", "FEATURES", "PREMIUM"},
		},
		{
			name: "Empty message",
			request: Request{
				Message: "",
				UserID:  "test_user",
			},
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"Processed:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create request body
			reqBody, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			// Make HTTP request
			resp, err := http.Post(server.URL+"/agent", "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Parse response
			var response Response
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check response contains expected strings
			for _, expected := range tt.shouldContain {
				if !strings.Contains(response.Result, expected) {
					t.Errorf("Expected response to contain %q, got: %s", expected, response.Result)
				}
			}

			// Check timestamp is recent
			if time.Since(response.Timestamp) > time.Minute {
				t.Errorf("Response timestamp seems too old: %v", response.Timestamp)
			}
		})
	}
}

// TestConcurrentRequests tests multiple concurrent requests to the API
func TestConcurrentRequests(t *testing.T) {
	agentFlow := createAgentFlow()
	handler := handleAgent(agentFlow)
	server := httptest.NewServer(handler)
	defer server.Close()

	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			request := Request{
				Message: fmt.Sprintf("Concurrent request %d", id),
				UserID:  fmt.Sprintf("user_%d", id),
			}

			reqBody, err := json.Marshal(request)
			if err != nil {
				results <- fmt.Errorf("request %d: failed to marshal: %v", id, err)
				return
			}

			resp, err := http.Post(server.URL+"/agent", "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				results <- fmt.Errorf("request %d: failed to make request: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("request %d: expected status 200, got %d", id, resp.StatusCode)
				return
			}

			var response Response
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				results <- fmt.Errorf("request %d: failed to decode response: %v", id, err)
				return
			}

			if !strings.Contains(response.Result, fmt.Sprintf("CONCURRENT REQUEST %d", id)) {
				results <- fmt.Errorf("request %d: unexpected response: %s", id, response.Result)
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}

// TestInvalidJSON tests handling of invalid JSON requests
func TestInvalidJSON(t *testing.T) {
	agentFlow := createAgentFlow()
	handler := handleAgent(agentFlow)
	server := httptest.NewServer(handler)
	defer server.Close()

	invalidPayloads := []string{
		`{"message": "test", "user_id": 123}`, // user_id should be string
		`{"invalid_json":}`,                   // malformed JSON
		``,                                    // empty body
		`{"message": null}`,                   // null message
	}

	for i, payload := range invalidPayloads {
		t.Run(fmt.Sprintf("invalid_payload_%d", i), func(t *testing.T) {

			resp, err := http.Post(server.URL+"/agent", "application/json", strings.NewReader(payload))
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			// Should handle gracefully (either 400 or 200 depending on implementation)
			if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusOK {
				t.Errorf("Expected 400 or 200 status, got %d", resp.StatusCode)
			}
		})
	}
}

// TestDirectFlowExecution tests the underlying flow without HTTP layer
func TestDirectFlowExecution(t *testing.T) {
	// Create a simple flow for direct testing (without HTTP request body handling)
	simpleFlow := calque.NewFlow()
	simpleFlow.
		Use(text.Transform(strings.ToUpper)).
		Use(text.Transform(func(s string) string { return "Processed: " + s }))

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple message",
			input:    "hello world",
			expected: []string{"HELLO WORLD", "Processed:"},
		},
		{
			name:     "Complex message",
			input:    "This is a complex message with many words",
			expected: []string{"COMPLEX MESSAGE", "MANY WORDS", "Processed:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var result string
			err := simpleFlow.Run(context.Background(), tt.input, &result)
			if err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}
