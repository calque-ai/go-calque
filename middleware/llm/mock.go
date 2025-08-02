package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

// MockProvider implements the LLMProvider interface for testing
type MockProvider struct {
	response      string
	streamDelay   time.Duration
	shouldError   bool
	errorMessage  string
	simulateTools bool // Whether to simulate tool calls
	toolCalls     []MockToolCall
}

// MockToolCall represents a simulated tool call for testing
type MockToolCall struct {
	Name      string
	Arguments string
}

// NewMockProvider creates a new mock provider
func NewMockProvider(response string) *MockProvider {
	return &MockProvider{
		response:    response,
		streamDelay: 50 * time.Millisecond, // Default delay between words
	}
}

// NewMockProviderWithError creates a mock provider that returns an error
func NewMockProviderWithError(errorMessage string) *MockProvider {
	return &MockProvider{
		shouldError:  true,
		errorMessage: errorMessage,
	}
}

// WithStreamDelay sets the delay between streamed words (for testing)
func (m *MockProvider) WithStreamDelay(delay time.Duration) *MockProvider {
	m.streamDelay = delay
	return m
}

// WithToolCalls configures the mock to simulate tool calls
func (m *MockProvider) WithToolCalls(toolCalls ...MockToolCall) *MockProvider {
	m.simulateTools = true
	m.toolCalls = toolCalls
	return m
}

// Chat implements the LLMProvider interface with simulated streaming
func (m *MockProvider) Chat(req *core.Request, res *core.Response) error {
	return m.ChatWithTools(req, res)
}

// ChatWithTools implements tool calling for testing
func (m *MockProvider) ChatWithTools(req *core.Request, res *core.Response, toolList ...tools.Tool) error {
	// Check if we should return an error (for testing error handling)
	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMessage)
	}

	// Read input
	var inputStr string
	if err := core.Read(req, &inputStr); err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	inputStr = strings.TrimSpace(inputStr)

	// If tools are provided and we're configured to simulate tool calls
	if len(toolList) > 0 && m.simulateTools && len(m.toolCalls) > 0 {
		return m.simulateToolCalls(res)
	}

	// Regular text response
	response := m.response
	if response == "" {
		// Default response that echoes the input
		response = fmt.Sprintf("Mock response to: %s", inputStr)
	}

	// Stream the response word by word to simulate real LLM behavior
	return m.streamResponse(response, req, res)
}

// simulateToolCalls generates mock tool calls in OpenAI format
func (m *MockProvider) simulateToolCalls(res *core.Response) error {
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
func (m *MockProvider) streamResponse(response string, req *core.Request, res *core.Response) error {
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
