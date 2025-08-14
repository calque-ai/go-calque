package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
	"github.com/invopop/jsonschema"
)

// MockClient implements the Client interface for testing
type MockClient struct {
	response         string
	responses        []string // Multiple responses for sequential calls
	callCount        int      // Track which response to return
	streamDelay      time.Duration
	shouldError      bool
	errorMessage     string
	simulateTools    bool // Whether to simulate tool calls
	toolCalls        []MockToolCall
	simulateJSONMode bool // Whether to simulate structured JSON output
}

// MockToolCall represents a simulated tool call for testing
type MockToolCall struct {
	Name      string
	Arguments string
}

// NewMockClient creates a new mock client
func NewMockClient(response string) *MockClient {
	return &MockClient{
		response:    response,
		streamDelay: 50 * time.Millisecond, // Default delay between words
	}
}

// NewMockClientWithResponses creates a mock client with multiple responses
func NewMockClientWithResponses(responses []string) *MockClient {
	return &MockClient{
		responses:   responses,
		streamDelay: 50 * time.Millisecond, // Default delay between words
	}
}

// NewMockClientWithError creates a mock client that returns an error
func NewMockClientWithError(errorMessage string) *MockClient {
	return &MockClient{
		shouldError:  true,
		errorMessage: errorMessage,
	}
}

// WithStreamDelay sets the delay between streamed words (for testing)
func (m *MockClient) WithStreamDelay(delay time.Duration) *MockClient {
	m.streamDelay = delay
	return m
}

// WithToolCalls configures the mock to simulate tool calls
func (m *MockClient) WithToolCalls(toolCalls ...MockToolCall) *MockClient {
	m.simulateTools = true
	m.toolCalls = toolCalls
	return m
}

// WithJSONMode configures the mock to simulate structured JSON output
func (m *MockClient) WithJSONMode(enabled bool) *MockClient {
	m.simulateJSONMode = enabled
	return m
}

// Chat implements the Client interface with simulated streaming
func (m *MockClient) Chat(req *calque.Request, res *calque.Response, opts *AgentOptions) error {
	// Extract options
	var toolList []tools.Tool
	var schema *ResponseFormat

	if opts != nil {
		toolList = opts.Tools
		schema = opts.Schema
	}
	// Check if we should return an error (for testing error handling)
	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMessage)
	}

	// Read input
	var inputStr string
	if err := calque.Read(req, &inputStr); err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	inputStr = strings.TrimSpace(inputStr)

	// Check if we have predefined responses first
	if len(m.responses) > 0 {
		response := m.getNextResponse(inputStr)
		// If response contains tool_calls, it means we should return it as-is
		if strings.Contains(response, "tool_calls") {
			_, err := res.Data.Write([]byte(response))
			return err
		}
		// Otherwise stream the response normally
		return m.streamResponse(response, req, res)
	}

	// If tools are provided and we're configured to simulate tool calls
	// Only simulate tool calls on the first call (callCount == 0)
	if len(toolList) > 0 && m.simulateTools && len(m.toolCalls) > 0 && m.callCount == 0 {
		m.callCount++ // Increment for next call
		return m.simulateToolCalls(res)
	}

	// If structured output is requested
	if schema != nil && m.simulateJSONMode {
		return m.simulateStructuredOutput(schema, inputStr, res)
	}

	// Regular text response
	response := m.getNextResponse(inputStr)

	// Stream the response word by word to simulate real LLM behavior
	return m.streamResponse(response, req, res)
}

// simulateToolCalls generates mock tool calls in OpenAI format
func (m *MockClient) simulateToolCalls(res *calque.Response) error {
	// Convert mock tool calls to OpenAI format
	var toolCalls []map[string]interface{}

	for _, call := range m.toolCalls {
		toolCall := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":      call.Name,
				"arguments": call.Arguments,
			},
		}
		toolCalls = append(toolCalls, toolCall)
	}

	// Create OpenAI format JSON
	result := map[string]interface{}{
		"tool_calls": toolCalls,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = res.Data.Write(jsonBytes)
	return err
}

// streamResponse handles streaming text responses
func (m *MockClient) streamResponse(response string, req *calque.Request, res *calque.Response) error {
	words := strings.Fields(response)
	for i, word := range words {
		// Check if context is cancelled
		select {
		case <-req.Context.Done():
			return req.Context.Err()
		default:
		}

		// Add space before word, except first word
		if i > 0 {
			if _, err := res.Data.Write([]byte(" ")); err != nil {
				return err
			}
		}

		// Write the word
		if _, err := res.Data.Write([]byte(word)); err != nil {
			return err
		}

		// Small delay to simulate streaming, skip delay for last word
		if i < len(words)-1 && m.streamDelay > 0 {
			time.Sleep(m.streamDelay)
		}
	}

	return nil
}

// simulateStructuredOutput generates mock structured JSON output
func (m *MockClient) simulateStructuredOutput(schema *ResponseFormat, input string, res *calque.Response) error {
	var mockJSON map[string]interface{}

	// Generate a simple mock JSON response based on the schema type
	switch schema.Type {
	case "json_object":
		// Simple JSON object
		mockJSON = map[string]interface{}{
			"message": fmt.Sprintf("Mock JSON response to: %s", input),
			"type":    "mock_response",
			"input":   input,
		}
	case "json_schema":
		// Try to generate a response that matches the schema structure
		if schema.Schema != nil {
			mockJSON = m.generateMockFromSchema(schema.Schema, input)
		} else {
			// Fallback to simple JSON
			mockJSON = map[string]interface{}{
				"message": fmt.Sprintf("Mock schema response to: %s", input),
				"schema":  true,
			}
		}
	default:
		// Default JSON response
		mockJSON = map[string]interface{}{
			"response": fmt.Sprintf("Mock response to: %s", input),
		}
	}

	// Marshal and write the JSON response
	jsonBytes, err := json.Marshal(mockJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal mock JSON: %w", err)
	}

	_, err = res.Data.Write(jsonBytes)
	return err
}

// generateMockFromSchema generates mock data based on JSON schema (simplified)
func (m *MockClient) generateMockFromSchema(schema *jsonschema.Schema, input string) map[string]interface{} {
	result := make(map[string]interface{})

	// Very basic schema interpretation for testing
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			key := pair.Key
			propSchema := pair.Value

			switch propSchema.Type {
			case "string":
				result[key] = fmt.Sprintf("mock_%s_for_%s", key, input)
			case "integer", "number":
				result[key] = 42
			case "boolean":
				result[key] = true
			case "array":
				result[key] = []interface{}{"mock_item_1", "mock_item_2"}
			case "object":
				result[key] = map[string]interface{}{"nested": "mock_value"}
			default:
				result[key] = fmt.Sprintf("mock_%s", key)
			}
		}
	}

	// If no properties defined, return a simple mock
	if len(result) == 0 {
		result["message"] = fmt.Sprintf("Mock response to: %s", input)
		result["schema_type"] = schema.Type
	}

	return result
}

// getNextResponse returns the next response in sequence or generates a default
func (m *MockClient) getNextResponse(input string) string {
	// If we have multiple responses, use sequential calling
	if len(m.responses) > 0 {
		if m.callCount >= len(m.responses) {
			// Out of responses, return an error message or last response
			return fmt.Sprintf("Mock error: no more responses available (called %d times)", m.callCount)
		}
		response := m.responses[m.callCount]
		m.callCount++
		return response
	}

	// Single response mode
	if m.response != "" {
		return m.response
	}

	// Default response that echoes the input
	return fmt.Sprintf("Mock response to: %s", input)
}

// Reset resets the call count (useful for testing)
func (m *MockClient) Reset() {
	m.callCount = 0
}
