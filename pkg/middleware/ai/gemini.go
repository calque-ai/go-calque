package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/calque-ai/calque-pipe/pkg/core"
	"github.com/calque-ai/calque-pipe/pkg/middleware/tools"
	"google.golang.org/genai"
)

// GeminiClient implements the Client interface for Google Gemini
type GeminiClient struct {
	client *genai.Client
	model  string
	config *GeminiConfig
}

// GeminiConfig holds Gemini-specific configuration
type GeminiConfig struct {
	// Required. API key for Google AI/Vertex AI authentication
	APIKey string

	// Optional. Controls randomness in token selection (0.0-2.0)
	// Lower values = more deterministic, higher values = more creative
	Temperature *float32

	// Optional. Nucleus sampling parameter (0.0-1.0)
	// Tokens are selected until their probabilities sum to this value
	TopP *float32

	// Optional. Top-k sampling - select from k highest probability tokens
	// Lower values = less random, higher values = more random
	TopK *float32

	// Optional. Maximum number of tokens in the response
	MaxTokens *int

	// Optional. Strings that stop text generation when encountered
	Stop []string

	// Optional. System instructions to steer model behavior
	// Example: "Answer as concisely as possible" or "Don't use technical terms"
	SystemInstruction string

	// Optional. Penalize tokens that already appear in generated text (-2.0 to 2.0)
	// Positive values increase content diversity
	PresencePenalty *float32

	// Optional. Penalize frequently repeated tokens (-2.0 to 2.0)
	// Positive values reduce repetition
	FrequencyPenalty *float32

	// Optional. Fixed seed for reproducible responses
	Seed *int32

	// Optional. Number of response variations to generate
	CandidateCount *int32

	// Optional. Response format configuration (JSON schema, etc.)
	ResponseFormat *ResponseFormat

	// Optional. Safety settings to block unsafe content
	SafetySettings []*genai.SafetySetting
}

// GeminiOption interface for functional options pattern
type GeminiOption interface {
	Apply(*GeminiConfig)
}

// configOption implements GeminiOption
type geminiConfigOption struct{ config *GeminiConfig }

func (o geminiConfigOption) Apply(opts *GeminiConfig) { *opts = *o.config }

// WithGeminiConfig sets custom Gemini configuration
func WithGeminiConfig(config *GeminiConfig) GeminiOption {
	return geminiConfigOption{config: config}
}

// DefaultGeminiConfig returns sensible defaults for Gemini
func DefaultGeminiConfig() *GeminiConfig {
	return &GeminiConfig{
		APIKey:      os.Getenv("GOOGLE_API_KEY"),
		Temperature: Float32Ptr(0.7),
	}
}

// NewGemini creates a new Gemini client with optional configuration
func NewGemini(model string, opts ...GeminiOption) (*GeminiClient, error) {
	if model == "" {
		return nil, fmt.Errorf("model name is required")
	}

	// Build config from options
	config := DefaultGeminiConfig()
	for _, opt := range opts {
		opt.Apply(config)
	}

	// Validate API key
	if config.APIKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable not set or provided in config")
	}

	// Configure the GenAI client
	clientConfig := &genai.ClientConfig{
		APIKey: config.APIKey,
	}

	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &GeminiClient{
		client: client,
		model:  model,
		config: config,
	}, nil
}

// Chat implements the Client interface with streaming support
func (g *GeminiClient) Chat(r *core.Request, w *core.Response, opts *AgentOptions) error {
	// Extract options
	var tools []tools.Tool
	var schema *ResponseFormat

	if opts != nil {
		tools = opts.Tools
		schema = opts.Schema
	}
	// Read input
	inputBytes, err := io.ReadAll(r.Data)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Create chat configuration
	genaiConfig := g.buildGenerateConfig(schema)

	// Convert your tools to Gemini format
	if len(tools) > 0 {
		geminiFunctions := convertToolsToGeminiFunctions(tools)
		genaiConfig.Tools = []*genai.Tool{{FunctionDeclarations: geminiFunctions}}
	}

	// Create a new chat
	chat, err := g.client.Chats.Create(r.Context, g.model, genaiConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to create chat: %w", err)
	}

	// Create message part
	part := genai.Part{Text: string(inputBytes)}

	// Send message with streaming
	var functionCalls []*genai.FunctionCall
	for result, err := range chat.SendMessageStream(r.Context, part) {
		if err != nil {
			return fmt.Errorf("failed to get response: %w", err)
		}

		// Check if this chunk contains function calls
		for _, candidate := range result.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					functionCalls = append(functionCalls, part.FunctionCall)
				}
			}
		}

		// Stream text parts as they arrive
		if text := result.Text(); text != "" {
			if _, writeErr := w.Data.Write([]byte(text)); writeErr != nil {
				return writeErr
			}
		}
	}

	// If we have function calls, format them as tool calls for the agent
	if len(functionCalls) > 0 {
		return g.writeFunctionCalls(functionCalls, w)
	}

	return nil

}

// writeFunctionCalls formats Gemini function calls as OpenAI JSON format for the agent
func (g *GeminiClient) writeFunctionCalls(functionCalls []*genai.FunctionCall, w *core.Response) error {
	// Convert to OpenAI format
	var toolCalls []map[string]any

	for _, call := range functionCalls {
		// Convert Gemini args to JSON string
		var argsJSON string
		if call.Args != nil && call.Args["input"] != nil {
			argsJSON = fmt.Sprintf(`{"input": "%v"}`, call.Args["input"])
		} else {
			argsJSON = `{"input": ""}`
		}

		// OpenAI format with type and function fields
		toolCall := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":      call.Name,
				"arguments": argsJSON,
			},
		}
		toolCalls = append(toolCalls, toolCall)
	}

	// Use json.Marshal for proper JSON formatting
	result := map[string]any{
		"tool_calls": toolCalls,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = w.Data.Write(jsonBytes)
	return err
}

// buildGenerateConfig creates a Gemini GenerateContentConfig from provider config and optional schema override
func (g *GeminiClient) buildGenerateConfig(schemaOverride *ResponseFormat) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}

	// Apply client configuration
	if g.config.Temperature != nil {
		config.Temperature = genai.Ptr(*g.config.Temperature)
	}
	if g.config.TopP != nil {
		config.TopP = genai.Ptr(*g.config.TopP)
	}
	if g.config.TopK != nil {
		config.TopK = genai.Ptr(*g.config.TopK)
	}
	if g.config.MaxTokens != nil {
		config.MaxOutputTokens = int32(*g.config.MaxTokens)
	}
	if len(g.config.Stop) > 0 {
		config.StopSequences = g.config.Stop
	}
	if g.config.SystemInstruction != "" {
		systemContent := genai.Text(g.config.SystemInstruction)
		if len(systemContent) > 0 {
			config.SystemInstruction = systemContent[0]
		}
	}
	if g.config.PresencePenalty != nil {
		config.PresencePenalty = genai.Ptr(*g.config.PresencePenalty)
	}
	if g.config.FrequencyPenalty != nil {
		config.FrequencyPenalty = genai.Ptr(*g.config.FrequencyPenalty)
	}
	if g.config.Seed != nil {
		config.Seed = genai.Ptr(*g.config.Seed)
	}
	if g.config.CandidateCount != nil {
		config.CandidateCount = *g.config.CandidateCount
	}
	if len(g.config.SafetySettings) > 0 {
		config.SafetySettings = g.config.SafetySettings
	}

	// Apply response format - request override takes priority
	var responseFormat *ResponseFormat
	if schemaOverride != nil {
		responseFormat = schemaOverride
	} else {
		responseFormat = g.config.ResponseFormat
	}

	if responseFormat != nil {
		switch responseFormat.Type {
		case "json_object":
			config.ResponseMIMEType = "application/json"
		case "json_schema":
			config.ResponseMIMEType = "application/json"
			if responseFormat.Schema != nil {
				config.ResponseJsonSchema = responseFormat.Schema
			}
		}
	}

	return config
}

// Convert your OpenAI JSON schema tools to Gemini format
func convertToolsToGeminiFunctions(tools []tools.Tool) []*genai.FunctionDeclaration {
	var functions []*genai.FunctionDeclaration

	for _, tool := range tools {
		functions = append(functions, &genai.FunctionDeclaration{
			Name:                 tool.Name(),
			Description:          tool.Description(),
			ParametersJsonSchema: tool.ParametersSchema(), // Use raw JSON schema like response format
		})
	}

	return functions
}

// Note: Schema conversion functions removed since we now use raw JSON schemas
// directly with ResponseJsonSchema and ParametersJsonSchema fields
