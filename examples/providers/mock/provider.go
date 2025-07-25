package mock

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// MockProvider implements the LLMProvider interface for testing
type MockProvider struct {
	response     string
	streamDelay  time.Duration
	shouldError  bool
	errorMessage string
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

// Chat implements the LLMProvider interface with simulated streaming
func (m *MockProvider) Chat(ctx context.Context, input io.Reader, output io.Writer) error {
	// Check if we should return an error (for testing error handling)
	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMessage)
	}

	// Read input
	inputBytes, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Use input in response to make it more realistic
	inputStr := strings.TrimSpace(string(inputBytes))
	response := m.response
	if response == "" {
		// Default response that echoes the input
		response = fmt.Sprintf("Mock response to: %s", inputStr)
	}

	// Stream the response word by word to simulate real LLM behavior
	words := strings.Fields(response)
	for i, word := range words {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Add space before word, except first word
		if i > 0 {
			if _, err := output.Write([]byte(" ")); err != nil {
				return err
			}
		}

		// Write the word
		if _, err := output.Write([]byte(word)); err != nil {
			return err
		}

		// Small delay to simulate streaming, skip delay for last word
		if i < len(words)-1 && m.streamDelay > 0 {
			time.Sleep(m.streamDelay)
		}
	}

	return nil
}
