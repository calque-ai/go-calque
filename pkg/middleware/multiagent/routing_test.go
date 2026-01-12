package multiagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
)

// createMockHandler creates a simple test handler
func createMockHandler(name, response string) calque.Handler {
	return calque.HandlerFunc(func(_ *calque.Request, res *calque.Response) error {
		_, err := res.Data.Write([]byte(name + ": " + response))
		return err
	})
}

func TestRoute(t *testing.T) {
	baseHandler := createMockHandler("base", "response")
	routedHandler := Route(baseHandler, "test-handler", "Test handler description", "test,mock")

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := routedHandler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Route handler failed: %v", err)
	}

	expected := "base: response"
	if output.String() != expected {
		t.Errorf("Expected %q, got %q", expected, output.String())
	}

	// Verify it's a routeHandler
	if rh, ok := routedHandler.(*routeHandler); ok {
		if rh.name != "test-handler" {
			t.Errorf("Expected name 'test-handler', got %q", rh.name)
		}
		if rh.description != "Test handler description" {
			t.Errorf("Expected description 'Test handler description', got %q", rh.description)
		}
		if len(rh.keywords) != 2 || rh.keywords[0] != "test" || rh.keywords[1] != "mock" {
			t.Errorf("Expected keywords [test, mock], got %v", rh.keywords)
		}
	} else {
		t.Error("Route should return a *routeHandler")
	}
}

func TestRouter(t *testing.T) {
	mathHandler := Route(createMockHandler("math", "42"), "math", "Mathematical calculations", "calculate,solve")
	codeHandler := Route(createMockHandler("code", "func() {}"), "code", "Programming tasks", "program,debug")

	// Mock client that returns valid JSON schema response for "math"
	mockClient := ai.NewMockClient(`{"route": "math", "confidence": 0.95, "reasoning": "Mathematical calculation request"}`)

	router := Router(mockClient, mathHandler, codeHandler)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("What is 2+2?"))
	res := calque.NewResponse(&output)

	err := router.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Router failed: %v", err)
	}

	expected := "math: 42"
	if output.String() != expected {
		t.Errorf("Expected %q, got %q", expected, output.String())
	}
}

func TestRouterWithNonRoutedHandlers(t *testing.T) {
	// Mix routed and non-routed handlers
	routedHandler := Route(createMockHandler("routed", "routed response"), "routed", "Routed handler", "routed")
	nonRoutedHandler := createMockHandler("plain", "plain response")

	// Mock client that chooses "handler_1" (the non-routed handler)
	mockClient := ai.NewMockClient(`{"route": "handler_1", "confidence": 0.8}`)

	router := Router(mockClient, routedHandler, nonRoutedHandler)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := router.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Router failed: %v", err)
	}

	expected := "plain: plain response"
	if output.String() != expected {
		t.Errorf("Expected %q, got %q", expected, output.String())
	}
}

func TestRouterFallbackOnSelectorError(t *testing.T) {
	mathHandler := Route(createMockHandler("math", "42"), "math", "Math", "calculate")
	codeHandler := Route(createMockHandler("code", "func() {}"), "code", "Code", "program")

	// Mock client that always fails
	mockClient := ai.NewMockClientWithError("selector failed")

	router := Router(mockClient, mathHandler, codeHandler)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
	res := calque.NewResponse(&output)

	err := router.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Router failed: %v", err)
	}

	// Should fallback to first handler (math)
	expected := "math: 42"
	if output.String() != expected {
		t.Errorf("Expected fallback to %q, got %q", expected, output.String())
	}
}

func TestLoadBalancer(t *testing.T) {
	t.Run("RoundRobin", func(t *testing.T) {
		handler1 := createMockHandler("handler1", "response1")
		handler2 := createMockHandler("handler2", "response2")
		handler3 := createMockHandler("handler3", "response3")

		lb := LoadBalancer(handler1, handler2, handler3)

		// Test round-robin distribution
		expected := []string{"handler1: response1", "handler2: response2", "handler3: response3", "handler1: response1"}

		for i, expectedResp := range expected {
			var output bytes.Buffer
			req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
			res := calque.NewResponse(&output)

			err := lb.ServeFlow(req, res)
			if err != nil {
				t.Errorf("LoadBalancer failed on iteration %d: %v", i, err)
				continue
			}

			if output.String() != expectedResp {
				t.Errorf("Iteration %d: expected %q, got %q", i, expectedResp, output.String())
			}
		}
	})

	t.Run("Concurrent", func(t *testing.T) {
		var counts [3]atomic.Int64

		handlers := make([]calque.Handler, 3)
		for i := range handlers {
			idx := i
			handlers[i] = calque.HandlerFunc(func(_ *calque.Request, res *calque.Response) error {
				counts[idx].Add(1)
				_, err := fmt.Fprintf(res.Data, "handler%d", idx)
				return err
			})
		}

		lb := LoadBalancer(handlers...)

		const numGoroutines = 100
		const requestsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for range numGoroutines {
			go func() {
				defer wg.Done()
				for range requestsPerGoroutine {
					var output bytes.Buffer
					req := calque.NewRequest(context.Background(), strings.NewReader("test"))
					res := calque.NewResponse(&output)

					if err := lb.ServeFlow(req, res); err != nil {
						t.Errorf("LoadBalancer failed: %v", err)
						return
					}
				}
			}()
		}

		wg.Wait()

		// Verify total requests
		totalRequests := int64(numGoroutines * requestsPerGoroutine)
		var totalCounted int64
		for i := range counts {
			totalCounted += counts[i].Load()
		}

		if totalCounted != totalRequests {
			t.Errorf("Expected %d total requests, got %d", totalRequests, totalCounted)
		}
	})
}

func TestEmptyHandlers(t *testing.T) {
	// Test Router with no handlers
	mockClient := ai.NewMockClient("math")
	router := Router(mockClient)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("test"))
	res := calque.NewResponse(&output)

	err := router.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error for Router with no handlers")
	}

	// Test LoadBalancer with no handlers
	lb := LoadBalancer()
	err = lb.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error for LoadBalancer with no handlers")
	}
}

func TestFindHandlerByID(t *testing.T) {
	routes := []*routeHandler{
		{name: "math", handler: createMockHandler("math", "math response")},
		{name: "code", handler: createMockHandler("code", "code response")},
		{name: "search", handler: createMockHandler("search", "search response")},
	}

	tests := []struct {
		routeID  string
		expected string
	}{
		{"math", "math"},
		{"code", "code"},
		{"search", "search"},
		{"unknown", ""}, // no match
	}

	for _, test := range tests {
		handler := findHandlerByID(test.routeID, routes)
		if test.expected == "" {
			if handler != nil {
				t.Errorf("Expected nil for route ID %q, got handler", test.routeID)
			}
		} else {
			if handler == nil {
				t.Errorf("Expected handler for route ID %q, got nil", test.routeID)
				continue
			}

			var output bytes.Buffer
			req := calque.NewRequest(context.Background(), strings.NewReader("test"))
			res := calque.NewResponse(&output)
			handler.ServeFlow(req, res)

			expectedOutput := test.expected + ": " + test.expected + " response"
			if output.String() != expectedOutput {
				t.Errorf("Route ID %q: expected %q, got %q", test.routeID, expectedOutput, output.String())
			}
		}
	}
}

func TestRouterWithAIClient(t *testing.T) {
	// Test with actual AI mock client
	mathClient := ai.NewMockClient("math")
	codeClient := ai.NewMockClient("code")
	// Mock client that returns valid JSON schema response
	selectorClient := ai.NewMockClient(`{"route": "math", "confidence": 0.9}`)

	mathHandler := Route(ai.Agent(mathClient), "math", "Mathematical calculations", "calculate,solve")
	codeHandler := Route(ai.Agent(codeClient), "code", "Programming tasks", "program,debug")

	router := Router(selectorClient, mathHandler, codeHandler)

	var output bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("Calculate 2+2"))
	res := calque.NewResponse(&output)

	err := router.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Router with AI clients failed: %v", err)
	}

	// The output should contain the math client's response
	if !strings.Contains(output.String(), "math") {
		t.Errorf("Expected output to contain 'math', got %q", output.String())
	}
}

func TestRouterSchemaValidation(t *testing.T) {
	mathHandler := Route(createMockHandler("math", "42"), "math", "Mathematical calculations", "calculate,solve")
	codeHandler := Route(createMockHandler("code", "func() {}"), "code", "Programming tasks", "program,debug")

	tests := []struct {
		name             string
		selectorResponse string
		expectedHandler  string
		shouldFallback   bool
	}{
		{
			name:             "Valid JSON with correct route",
			selectorResponse: `{"route": "math", "confidence": 0.95}`,
			expectedHandler:  "math",
			shouldFallback:   false,
		},
		{
			name:             "Valid JSON with invalid route",
			selectorResponse: `{"route": "invalid", "confidence": 0.8}`,
			expectedHandler:  "math", // fallback to first
			shouldFallback:   true,
		},
		{
			name:             "Invalid JSON",
			selectorResponse: `invalid json`,
			expectedHandler:  "math", // fallback to first
			shouldFallback:   true,
		},
		{
			name:             "Missing route field",
			selectorResponse: `{"confidence": 0.9}`,
			expectedHandler:  "math", // fallback to first
			shouldFallback:   true,
		},
		{
			name:             "Empty route field",
			selectorResponse: `{"route": "", "confidence": 0.9}`,
			expectedHandler:  "math", // fallback to first
			shouldFallback:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := ai.NewMockClient(test.selectorResponse)

			router := Router(mockClient, mathHandler, codeHandler)

			var output bytes.Buffer
			req := calque.NewRequest(context.Background(), strings.NewReader("test input"))
			res := calque.NewResponse(&output)

			err := router.ServeFlow(req, res)
			if err != nil {
				t.Fatalf("Router failed: %v", err)
			}

			expectedOutput := test.expectedHandler + ": " +
				map[string]string{"math": "42", "code": "func() {}"}[test.expectedHandler]
			if output.String() != expectedOutput {
				t.Errorf("Expected %q, got %q", expectedOutput, output.String())
			}
		})
	}
}

func TestCallSelectorWithSchema(t *testing.T) {
	// Test the schema input generation
	routerInput := RouterInput{
		Request: "test request",
		Routes: []RouteOption{
			{ID: "math", Name: "math", Description: "Mathematical calculations"},
			{ID: "code", Name: "code", Description: "Programming tasks"},
		},
	}

	// Mock selector that returns valid response
	selector := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Verify the input contains schema and route options
		var input map[string]any
		if err := json.NewDecoder(req.Data).Decode(&input); err != nil {
			return err
		}

		// Check that schema is present
		if _, exists := input["$schema"]; !exists {
			return fmt.Errorf("schema not found in input")
		}

		// Check that router input is present
		routerInputRaw, exists := input["routerinput"]
		if !exists {
			return fmt.Errorf("routerinput not found in input")
		}

		// Verify the router input structure
		routerInputMap, ok := routerInputRaw.(map[string]any)
		if !ok {
			return fmt.Errorf("routerinput is not a map")
		}

		if routerInputMap["request"] != "test request" {
			return fmt.Errorf("request field mismatch")
		}

		response := `{"route": "math", "confidence": 0.95, "reasoning": "test"}`
		_, err := res.Data.Write([]byte(response))
		return err
	})

	selection, err := callSelectorWithSchema(context.Background(), selector, routerInput)
	if err != nil {
		t.Fatalf("callSelectorWithSchema failed: %v", err)
	}

	if selection.Route != "math" {
		t.Errorf("Expected route 'math', got %q", selection.Route)
	}
	if selection.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", selection.Confidence)
	}
	if selection.Reasoning != "test" {
		t.Errorf("Expected reasoning 'test', got %q", selection.Reasoning)
	}
}
