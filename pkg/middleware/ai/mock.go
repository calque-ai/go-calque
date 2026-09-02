package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/invopop/jsonschema"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// JSON Schema type names used when generating mock structured output.
const (
	jsonSchemaTypeString = "string"
	jsonSchemaTypeObject = "object"
)

// MockClient implements the Client interface for testing
type MockClient struct {
	response         string
	responses        []string // Multiple responses for sequential calls
	streamDelay      time.Duration
	shouldError      bool
	errorMessage     string
	simulateTools    bool // Whether to simulate tool calls
	toolCalls        []MockToolCall
	simulateJSONMode bool // Whether to simulate structured JSON output

	// mu guards the fields below, mutated on every Chat() call - the
	// ai.Agent loop and callers like TestMockClientConcurrency can invoke
	// Chat() from multiple goroutines concurrently.
	mu         sync.Mutex
	callCount  int             // Track which response to return
	historyLog [][]Message     // History received on each Chat() call, in order
	optsLog    []*AgentOptions // AgentOptions received on each Chat() call, in order
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
	m.logCall(opts)

	// Check if we should return an error (for testing error handling)
	if m.shouldError {
		return calque.NewErr(req.Context, fmt.Sprintf("mock error: %s", m.errorMessage))
	}

	// Read input
	var inputStr string
	if err := calque.Read(req, &inputStr); err != nil {
		return calque.WrapErr(req.Context, err, "failed to read input")
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

	// If tools are provided and we're configured to simulate tool calls,
	// only on the first call.
	if len(toolList) > 0 && m.simulateTools && len(m.toolCalls) > 0 && m.takeFirstCall() {
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
	toolCalls := make([]map[string]any, len(m.toolCalls))

	for i, call := range m.toolCalls {
		toolCall := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":      call.Name,
				"arguments": call.Arguments,
			},
		}
		toolCalls[i] = toolCall
	}

	// Create OpenAI format JSON
	result := map[string]any{
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
	var mockJSON map[string]any

	// Generate a simple mock JSON response based on the schema type
	switch schema.Type {
	case ResponseFormatJSONObject:
		// Simple JSON object
		mockJSON = map[string]any{
			"message": fmt.Sprintf("Mock JSON response to: %s", input),
			"type":    "mock_response",
			"input":   input,
		}
	case ResponseFormatJSONSchema:
		// Try to generate a response that matches the schema structure
		if schema.Schema != nil {
			mockJSON = m.generateMockFromSchema(schema.Schema, input)
		} else {
			// Fallback to simple JSON
			mockJSON = map[string]any{
				"message": fmt.Sprintf("Mock schema response to: %s", input),
				"schema":  true,
			}
		}
	default:
		// Default JSON response
		mockJSON = map[string]any{
			"response": fmt.Sprintf("Mock response to: %s", input),
		}
	}

	// Marshal and write the JSON response
	jsonBytes, err := json.Marshal(mockJSON)
	if err != nil {
		return calque.WrapErr(context.Background(), err, "failed to marshal mock JSON")
	}

	_, err = res.Data.Write(jsonBytes)
	return err
}

// generateMockFromSchema generates mock data based on JSON schema (simplified)
func (m *MockClient) generateMockFromSchema(schema *jsonschema.Schema, input string) map[string]any {
	result := make(map[string]any)

	// Very basic schema interpretation for testing
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			key := pair.Key
			propSchema := pair.Value

			switch propSchema.Type {
			case jsonSchemaTypeString:
				result[key] = fmt.Sprintf("mock_%s_for_%s", key, input)
			case "integer", "number":
				result[key] = 42
			case "boolean":
				result[key] = true
			case "array":
				result[key] = []any{"mock_item_1", "mock_item_2"}
			case jsonSchemaTypeObject:
				result[key] = map[string]any{"nested": "mock_value"}
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
		m.mu.Lock()
		defer m.mu.Unlock()
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

// logCall records opts (and its History) for the current Chat() call, for
// later inspection via HistoryAt/OptionsAt/CallCount.
func (m *MockClient) logCall(opts *AgentOptions) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.optsLog = append(m.optsLog, opts)
	var history []Message
	if opts != nil {
		history = opts.History
	}
	m.historyLog = append(m.historyLog, history)
}

// takeFirstCall reports whether this is the first Chat() call since
// creation or the last Reset, and atomically marks it as taken.
func (m *MockClient) takeFirstCall() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callCount != 0 {
		return false
	}
	m.callCount++
	return true
}

// Reset resets the call count (useful for testing)
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
	m.historyLog = nil
	m.optsLog = nil
}

// HistoryAt returns the AgentOptions.History the client received on its
// callIndex-th Chat() invocation (0-indexed), or nil if there was no such
// call or no history was set. Use this to assert that a caller (e.g. the
// ai.Agent tool-calling loop) actually threads conversation history through
// to the client on each turn, rather than dropping it.
func (m *MockClient) HistoryAt(callIndex int) []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if callIndex < 0 || callIndex >= len(m.historyLog) {
		return nil
	}
	return m.historyLog[callIndex]
}

// OptionsAt returns the *AgentOptions the client received on its
// callIndex-th Chat() invocation (0-indexed), or nil if there was no such
// call. Use this to assert that fields like Schema or Tools were actually
// passed through on a given turn.
func (m *MockClient) OptionsAt(callIndex int) *AgentOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	if callIndex < 0 || callIndex >= len(m.optsLog) {
		return nil
	}
	return m.optsLog[callIndex]
}

// CallCount returns the number of times Chat() has been invoked.
func (m *MockClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.historyLog)
}

// HistoryContainsToolResult reports whether history has a RoleTool message
// whose Content contains the given substring - useful for asserting that a
// prior turn's tool result was actually fed back to the model.
func HistoryContainsToolResult(history []Message, contains string) bool {
	for _, msg := range history {
		if msg.Role == RoleTool && strings.Contains(msg.Content, contains) {
			return true
		}
	}
	return false
}
