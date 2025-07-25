package gemini

import (
	"context"
	"fmt"
	"io"
	"os"

	"google.golang.org/genai"
)

// GeminiProvider implements the LLMProvider interface for Google Gemini
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// NewGeminiProvider creates a new Gemini provider
// If apiKey is empty, it will try to read from GOOGLE_API_KEY environment variable
func NewGeminiProvider(apiKey, model string) (*GeminiProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable not set")
	}
	if model == "" {
		model = "gemini-1.5-flash" // Default to free tier model
	}

	// Configure the GenAI client
	config := &genai.ClientConfig{
		APIKey: apiKey,
	}

	client, err := genai.NewClient(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

// Chat implements the LLMProvider interface with streaming support
func (g *GeminiProvider) Chat(ctx context.Context, input io.Reader, output io.Writer) error {
	// Read input
	inputBytes, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Create chat configuration
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
	}

	// Create a new chat
	chat, err := g.client.Chats.Create(ctx, g.model, config, nil)
	if err != nil {
		return fmt.Errorf("failed to create chat: %w", err)
	}

	// Create message part
	part := genai.Part{Text: string(inputBytes)}

	// Send message with streaming
	for result, err := range chat.SendMessageStream(ctx, part) {
		if err != nil {
			return fmt.Errorf("failed to get response: %w", err)
		}

		// Write each chunk to output as it arrives
		if _, writeErr := output.Write([]byte(result.Text())); writeErr != nil {
			return writeErr
		}
	}

	return nil
}
