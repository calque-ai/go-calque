// Package gemini provides Google Gemini AI model integration for the calque framework.
// It implements the AI client interface to enable chat completions, tool calling,
// and streaming responses using Google's Gemini models including Pro and Flash variants.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"google.golang.org/genai"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

const applicationJSON = "application/json"

// Client implements the Client interface for Google Gemini.
//
// Provides streaming chat completions with tool calling support.
// Supports Gemini Pro, Flash, and other Google AI models.
//
// Example:
//
//	client, _ := gemini.New("gemini-1.5-pro")
//	agent := ai.Agent(client)
type Client struct {
	client *genai.Client
	model  string
	config *Config
}

// Config holds Gemini-specific configuration.
//
// Configures model behavior, safety settings, and response format.
// All fields are optional with sensible defaults.
//
// Example:
//
//	config := &gemini.Config{
//		Temperature: ai.Float32Ptr(0.8),
//		MaxTokens: ai.IntPtr(1000),
//	}
type Config struct {
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
	ResponseFormat *ai.ResponseFormat

	// Optional. Safety settings to block unsafe content
	SafetySettings []*genai.SafetySetting
}

// Option interface for functional options pattern
type Option interface {
	Apply(*Config)
}

// configOption implements Option
type configOption struct{ config *Config }

func (o configOption) Apply(opts *Config) { *opts = *o.config }

// WithConfig sets custom Gemini configuration.
//
// Input: *Config with Gemini settings
// Output: Option for client creation
// Behavior: Overrides default configuration
//
// Example:
//
//	config := &gemini.Config{Temperature: ai.Float32Ptr(0.9)}
//	client, _ := gemini.New("gemini-pro", gemini.WithConfig(config))
func WithConfig(config *Config) Option {
	return configOption{config: config}
}

// DefaultConfig returns sensible defaults for Gemini.
//
// Input: none
// Output: *Config with default settings
// Behavior: Creates config with GOOGLE_API_KEY from env
//
// Sets temperature to 0.7 and API key from environment.
//
// Example:
//
//	config := gemini.DefaultConfig()
//	config.MaxTokens = ai.IntPtr(2000)
func DefaultConfig() *Config {
	return &Config{
		APIKey:      os.Getenv("GOOGLE_API_KEY"),
		Temperature: ai.Float32Ptr(0.7),
	}
}

// New creates a new Gemini client with optional configuration.
//
// Input: model name string, optional config Options
// Output: *Client, error
// Behavior: Initializes authenticated Gemini client
//
// Requires GOOGLE_API_KEY environment variable or config.APIKey.
// Supports all Gemini models: gemini-pro, gemini-1.5-pro, etc.
//
// Example:
//
//	client, err := gemini.New("gemini-1.5-pro")
//	if err != nil { log.Fatal(err) }
func New(model string, opts ...Option) (*Client, error) {
	if model == "" {
		return nil, fmt.Errorf("model name is required")
	}

	// Build config from options
	config := DefaultConfig()
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

	return &Client{
		client: client,
		model:  model,
		config: config,
	}, nil
}

// RequestConfig holds configuration for a Gemini request
type RequestConfig struct {
	GenaiConfig *genai.GenerateContentConfig
	Chat        *genai.Chat
	Parts       []genai.Part
}

// Chat implements the Client interface with streaming support.
//
// Input: user prompt/query via calque.Request
// Output: streamed AI response via calque.Response
// Behavior: STREAMING - outputs tokens as they arrive
//
// Supports tool calling, JSON schema responses, and safety filtering.
// Automatically formats tool calls for agent framework compatibility.
//
// Example:
//
//	err := client.Chat(req, res, &ai.AgentOptions{Tools: tools})
func (g *Client) Chat(r *calque.Request, w *calque.Response, opts *ai.AgentOptions) error {
	// Which input type are we processing?
	input, err := ai.ClassifyInput(r, opts)
	if err != nil {
		return err
	}

	// Build request configuration based on input type
	config, err := g.buildRequestConfig(r.Context, input, ai.GetSchema(opts), ai.GetTools(opts))
	if err != nil {
		return err
	}

	// Execute the request with the configured chat
	return g.executeRequest(config, r, w)
}

// writeFunctionCalls formats Gemini function calls as OpenAI JSON format for the agent
func (g *Client) writeFunctionCalls(functionCalls []*genai.FunctionCall, w *calque.Response) error {
	// Convert to OpenAI format
	toolCalls := make([]map[string]any, 0, len(functionCalls))

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
func (g *Client) buildGenerateConfig(schemaOverride *ai.ResponseFormat) *genai.GenerateContentConfig {
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
	var responseFormat *ai.ResponseFormat
	if schemaOverride != nil {
		responseFormat = schemaOverride
	} else {
		responseFormat = g.config.ResponseFormat
	}

	if responseFormat != nil {
		switch responseFormat.Type {
		case "json_object":
			config.ResponseMIMEType = applicationJSON
		case "json_schema":
			config.ResponseMIMEType = applicationJSON
			if responseFormat.Schema != nil {
				config.ResponseJsonSchema = responseFormat.Schema
			}
		}
	}

	return config
}

// Convert your OpenAI JSON schema tools to Gemini format
func convertToolsToGeminiFunctions(tools []tools.Tool) []*genai.FunctionDeclaration {
	functions := make([]*genai.FunctionDeclaration, 0, len(tools))

	for _, tool := range tools {
		functions = append(functions, &genai.FunctionDeclaration{
			Name:                 tool.Name(),
			Description:          tool.Description(),
			ParametersJsonSchema: tool.ParametersSchema(), // Use raw JSON schema like response format
		})
	}

	return functions
}

// buildRequestConfig creates configuration for the request
func (g *Client) buildRequestConfig(ctx context.Context, input *ai.ClassifiedInput, schema *ai.ResponseFormat, tools []tools.Tool) (*RequestConfig, error) {
	// Build config once
	genaiConfig := g.buildGenerateConfig(schema)

	// Add tools once
	if len(tools) > 0 {
		geminiFunctions := convertToolsToGeminiFunctions(tools)
		genaiConfig.Tools = []*genai.Tool{{FunctionDeclarations: geminiFunctions}}
	}

	// Create chat once
	chat, err := g.client.Chats.Create(ctx, g.model, genaiConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	// Convert to parts once
	parts, err := g.inputToParts(input)
	if err != nil {
		return nil, err
	}

	return &RequestConfig{
		GenaiConfig: genaiConfig,
		Chat:        chat,
		Parts:       parts,
	}, nil
}

// executeRequest executes the configured request
func (g *Client) executeRequest(config *RequestConfig, r *calque.Request, w *calque.Response) error {
	// Send message with streaming
	var functionCalls []*genai.FunctionCall
	for result, err := range config.Chat.SendMessageStream(r.Context, config.Parts...) {
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

// inputToParts converts classified input to genai.Part array
func (g *Client) inputToParts(input *ai.ClassifiedInput) ([]genai.Part, error) {
	switch input.Type {
	case ai.TextInput:
		return []genai.Part{{Text: input.Text}}, nil

	case ai.MultimodalJSONInput, ai.MultimodalStreamingInput:
		return g.multimodalToParts(input.Multimodal)

	default:
		return nil, fmt.Errorf("unsupported input type: %d", input.Type)
	}
}

// multimodalToParts converts multimodal input to genai.Part array
func (g *Client) multimodalToParts(multimodal *ai.MultimodalInput) ([]genai.Part, error) {
	if multimodal == nil {
		return nil, fmt.Errorf("multimodal input cannot be nil")
	}

	var parts []genai.Part

	for _, part := range multimodal.Parts {
		switch part.Type {
		case "text":
			if part.Text != "" {
				parts = append(parts, genai.Part{Text: part.Text})
			}
		case "image", "audio", "video":
			var data []byte
			var err error

			if part.Reader != nil {
				// Read stream data (streaming approach)
				data, err = io.ReadAll(part.Reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read %s data: %w", part.Type, err)
				}
			} else if part.Data != nil {
				// Use embedded data (simple approach)
				data = part.Data
			}

			if data != nil {
				parts = append(parts, genai.Part{
					InlineData: &genai.Blob{
						Data:     data,
						MIMEType: part.MimeType,
					},
				})
			}
		default:
			return nil, fmt.Errorf("unsupported content part type: %s", part.Type)
		}
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("no valid content parts found in multimodal input")
	}

	return parts, nil
}
