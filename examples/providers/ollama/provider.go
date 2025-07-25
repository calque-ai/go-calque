package ollama

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ollama/ollama/api"
)

// OllamaProvider implements the LLMProvider interface for Ollama
type OllamaProvider struct {
	client *api.Client
	model  string
}

// NewOllamaProvider creates a new Ollama provider
// If host is empty, it uses ClientFromEnvironment() which defaults to localhost:11434
func NewOllamaProvider(host, model string) (*OllamaProvider, error) {
	if model == "" {
		model = "llama3.2" // Default model
	}

	var client *api.Client
	var err error

	if host == "" {
		// Use environment-based client (checks OLLAMA_HOST env var)
		client, err = api.ClientFromEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to create client from environment: %w", err)
		}
	} else {
		// Parse the host URL
		u, err := url.Parse(host)
		if err != nil {
			return nil, fmt.Errorf("invalid host URL: %w", err)
		}
		// Create client with custom host
		client = api.NewClient(u, http.DefaultClient)
	}

	return &OllamaProvider{
		client: client,
		model:  model,
	}, nil
}

// Chat implements the LLMProvider interface with streaming support
func (o *OllamaProvider) Chat(ctx context.Context, input io.Reader, output io.Writer) error {
	// Read input
	inputBytes, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Create chat request
	req := &api.ChatRequest{
		Model: o.model,
		Messages: []api.Message{
			{
				Role:    "user",
				Content: string(inputBytes),
			},
		},
		// Streaming is enabled by default when using response function
	}

	// Create response handler that writes to output
	responseFunc := func(resp api.ChatResponse) error {
		// Write each chunk to output as it arrives
		if _, writeErr := output.Write([]byte(resp.Message.Content)); writeErr != nil {
			return writeErr
		}
		return nil
	}

	// Send streaming chat request
	err = o.client.Chat(ctx, req, responseFunc)
	if err != nil {
		return fmt.Errorf("failed to chat with ollama: %w", err)
	}

	return nil
}
