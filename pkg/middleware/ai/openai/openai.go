package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sashabaranov/go-openai"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// Client implements the Client interface for OpenAI.
//
// Provides streaming chat completions with tool calling and multimodal support.
// Supports GPT-5, GPT-4, GPT-4o, GPT-4o-mini, GPT-3.5, and other OpenAI models with full feature compatibility.
//
// Example:
//
//	client, _ := openai.New("gpt-5")
//	agent := ai.Agent(client)
type Client struct {
	client *openai.Client
	model  string
	config *Config
}

// Config holds OpenAI-specific configuration.
//
// Configures model behavior, API settings, and response format.
// All fields are optional with sensible defaults.
//
// Example:
//
//	config := &openai.Config{
//		APIKey: "sk-...",
//		Temperature: ai.Float32Ptr(0.8),
//		MaxTokens: ai.IntPtr(1000),
//	}
type Config struct {
	// Required. API key for OpenAI authentication
	APIKey string

	// Optional. Base URL for OpenAI API (defaults to official OpenAI API)
	BaseURL string

	// Optional. Organization ID for OpenAI API requests
	OrgID string

	// Optional. Controls randomness in token selection (0.0-2.0)
	// Lower values = more deterministic, higher values = more creative
	Temperature *float32

	// Optional. Nucleus sampling parameter (0.0-1.0)
	// Tokens are selected until their probabilities sum to this value
	TopP *float32

	// Optional. Maximum number of tokens in the response
	MaxTokens *int

	// Optional. Number of chat completion choices to generate
	N *int

	// Optional. Strings that stop text generation when encountered
	Stop []string

	// Optional. Penalize tokens that already appear in generated text (-2.0 to 2.0)
	// Positive values increase content diversity
	PresencePenalty *float32

	// Optional. Penalize frequently repeated tokens (-2.0 to 2.0)
	// Positive values reduce repetition
	FrequencyPenalty *float32

	// Optional. User ID for tracking and abuse monitoring
	User string

	// Optional. Fixed seed for reproducible responses (GPT-4 and newer)
	Seed *int

	// Optional. Response format configuration (JSON schema, etc.)
	ResponseFormat *ai.ResponseFormat

	// Optional. Enable/disable streaming of responses (true by default)
	Stream *bool
}

// Option interface for functional options pattern
type Option interface {
	Apply(*Config)
}

// configOption implements Option
type configOption struct {
	config *Config
}

func (o configOption) Apply(opts *Config) {
	*opts = *o.config
}

// WithConfig sets custom OpenAI configuration.
//
// Input: *Config with OpenAI settings
// Output: Option for client creation
// Behavior: Overrides default configuration
//
// Example:
//
//	config := &openai.Config{Temperature: ai.Float32Ptr(0.9)}
//	client, _ := openai.New("gpt-4", openai.WithConfig(config))
func WithConfig(config *Config) Option {
	return configOption{config: config}
}

// DefaultConfig returns sensible defaults for OpenAI.
//
// Input: none
// Output: *Config with default settings
// Behavior: Creates config with OPENAI_API_KEY from env
//
// Sets temperature to 0.7 and API key from environment.
//
// Example:
//
//	config := openai.DefaultConfig()
//	config.MaxTokens = ai.IntPtr(2000)
func DefaultConfig() *Config {
	return &Config{
		APIKey:      os.Getenv("OPENAI_API_KEY"),
		Temperature: ai.Float32Ptr(0.7),
		Stream:      ai.BoolPtr(true),
	}
}

// New creates a new OpenAI client with optional configuration.
//
// Input: model name string, optional config Options
// Output: *Client, error
// Behavior: Initializes authenticated OpenAI client
//
// Requires OPENAI_API_KEY environment variable or config.APIKey.
// Supports all OpenAI models: gpt-4, gpt-3.5-turbo, etc.
//
// Example:
//
//	client, err := openai.New("gpt-4")
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
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set or provided in config")
	}

	// Create OpenAI client configuration
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}
	if config.OrgID != "" {
		clientConfig.OrgID = config.OrgID
	}

	client := openai.NewClientWithConfig(clientConfig)

	return &Client{
		client: client,
		model:  model,
		config: config,
	}, nil
}

// RequestConfig holds configuration for an OpenAI request
type RequestConfig struct {
	ChatRequest openai.ChatCompletionRequest
}

// Chat implements the Client interface with streaming support.
//
// Input: user prompt/query via calque.Request
// Output: streamed AI response via calque.Response
// Behavior: STREAMING - outputs tokens as they arrive
//
// Supports tool calling, JSON schema responses, and multimodal content.
// Automatically formats tool calls for agent framework compatibility.
//
// Example:
//
//	err := client.Chat(req, res, &ai.AgentOptions{Tools: tools})
func (c *Client) Chat(r *calque.Request, w *calque.Response, opts *ai.AgentOptions) error {
	// Which input type are we processing?
	input, err := ai.ClassifyInput(r, opts)
	if err != nil {
		return err
	}

	// Build request configuration based on input type
	config, err := c.buildRequestConfig(input, ai.GetSchema(opts), ai.GetTools(opts))
	if err != nil {
		return err
	}

	// Execute the request with the configured chat
	return c.executeRequest(config, r, w)
}

// buildRequestConfig creates configuration for the request
func (c *Client) buildRequestConfig(input *ai.ClassifiedInput, schema *ai.ResponseFormat, tools []tools.Tool) (*RequestConfig, error) {
	// Create base chat request
	req := openai.ChatCompletionRequest{
		Model: c.model,
	}

	// Convert input to messages
	messages, err := c.inputToMessages(input)
	if err != nil {
		return nil, err
	}
	req.Messages = messages

	// Apply configuration
	c.applyChatConfig(&req, schema)

	// Add tools if provided
	if len(tools) > 0 {
		req.Tools = c.convertToOpenAITools(tools)
		req.ToolChoice = "auto"
	}

	return &RequestConfig{
		ChatRequest: req,
	}, nil
}

// executeRequest executes the configured request
func (c *Client) executeRequest(config *RequestConfig, r *calque.Request, w *calque.Response) error {
	// Determine if we should stream
	shouldStream := c.config.Stream == nil || *c.config.Stream

	if shouldStream {
		return c.executeStreamingRequest(config, r, w)
	} else {
		return c.executeNonStreamingRequest(config, r, w)
	}
}

// executeStreamingRequest executes a streaming request
func (c *Client) executeStreamingRequest(config *RequestConfig, r *calque.Request, w *calque.Response) error {
	// Enable streaming
	config.ChatRequest.Stream = true

	// Create streaming request
	stream, err := c.client.CreateChatCompletionStream(r.Context, config.ChatRequest)
	if err != nil {
		return fmt.Errorf("failed to create chat completion stream: %w", err)
	}
	defer stream.Close()

	var toolCalls []openai.ToolCall
	var functionCallName, functionCallArgs string

	// Process streaming response
	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to receive stream response: %w", err)
		}

		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta

			// Handle content streaming
			if delta.Content != "" {
				if _, writeErr := w.Data.Write([]byte(delta.Content)); writeErr != nil {
					return writeErr
				}
			}

			// Handle tool calls (streaming)
			if len(delta.ToolCalls) > 0 {
				for _, toolCall := range delta.ToolCalls {
					if toolCall.Function.Name != "" {
						functionCallName = toolCall.Function.Name
						functionCallArgs = ""
					}
					if toolCall.Function.Arguments != "" {
						functionCallArgs += toolCall.Function.Arguments
					}
				}
			}
		}
	}

	// If we accumulated a complete tool call, format it
	if functionCallName != "" {
		toolCall := openai.ToolCall{
			Type: "function",
			Function: openai.FunctionCall{
				Name:      functionCallName,
				Arguments: functionCallArgs,
			},
		}
		toolCalls = append(toolCalls, toolCall)
	}

	// Format tool calls if we have any
	if len(toolCalls) > 0 {
		return c.writeOpenAIToolCalls(toolCalls, w)
	}

	return nil
}

// executeNonStreamingRequest executes a non-streaming request
func (c *Client) executeNonStreamingRequest(config *RequestConfig, r *calque.Request, w *calque.Response) error {
	// Disable streaming
	config.ChatRequest.Stream = false

	// Create request
	response, err := c.client.CreateChatCompletion(r.Context, config.ChatRequest)
	if err != nil {
		return fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("no response choices returned")
	}

	choice := response.Choices[0]

	// Handle tool calls
	if len(choice.Message.ToolCalls) > 0 {
		return c.writeOpenAIToolCalls(choice.Message.ToolCalls, w)
	}

	// Handle content
	if choice.Message.Content != "" {
		_, err := w.Data.Write([]byte(choice.Message.Content))
		return err
	}

	return nil
}

// inputToMessages converts classified input to OpenAI message format
func (c *Client) inputToMessages(input *ai.ClassifiedInput) ([]openai.ChatCompletionMessage, error) {
	switch input.Type {
	case ai.TextInput:
		return []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: input.Text,
			},
		}, nil

	case ai.MultimodalJSONInput, ai.MultimodalStreamingInput:
		return c.multimodalToMessages(input.Multimodal)

	default:
		return nil, fmt.Errorf("unsupported input type: %d", input.Type)
	}
}

// multimodalToMessages converts multimodal input to OpenAI message format
func (c *Client) multimodalToMessages(multimodal *ai.MultimodalInput) ([]openai.ChatCompletionMessage, error) {
	if multimodal == nil {
		return nil, fmt.Errorf("multimodal input cannot be nil")
	}

	messageParts := make([]openai.ChatMessagePart, 0, len(multimodal.Parts))

	for _, part := range multimodal.Parts {
		switch part.Type {
		case "text":
			if part.Text != "" {
				messageParts = append(messageParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: part.Text,
				})
			}
		case "image":
			var data []byte
			var err error

			if part.Reader != nil {
				// Read stream data (streaming approach)
				data, err = io.ReadAll(part.Reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read image data: %w", err)
				}
			} else if part.Data != nil {
				// Use embedded data (simple approach)
				data = part.Data
			}

			if data != nil {
				// Convert to base64 data URL
				dataURL := fmt.Sprintf("data:%s;base64,%s", part.MimeType, string(data))
				messageParts = append(messageParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: dataURL,
					},
				})
			}
		case "audio", "video":
			// OpenAI doesn't support audio/video in chat completions yet
			return nil, fmt.Errorf("audio and video content not yet supported by OpenAI Chat Completions API")
		default:
			return nil, fmt.Errorf("unsupported content part type: %s", part.Type)
		}
	}

	if len(messageParts) == 0 {
		return nil, fmt.Errorf("no valid content parts found in multimodal input")
	}

	return []openai.ChatCompletionMessage{
		{
			Role:         openai.ChatMessageRoleUser,
			MultiContent: messageParts,
		},
	}, nil
}

// applyChatConfig applies client configuration to the chat request
func (c *Client) applyChatConfig(req *openai.ChatCompletionRequest, schema *ai.ResponseFormat) {
	// Apply client configuration
	if c.config.Temperature != nil {
		req.Temperature = *c.config.Temperature
	}
	if c.config.TopP != nil {
		req.TopP = *c.config.TopP
	}
	if c.config.MaxTokens != nil {
		req.MaxTokens = *c.config.MaxTokens
	}
	if c.config.N != nil {
		req.N = *c.config.N
	}
	if len(c.config.Stop) > 0 {
		req.Stop = c.config.Stop
	}
	if c.config.PresencePenalty != nil {
		req.PresencePenalty = *c.config.PresencePenalty
	}
	if c.config.FrequencyPenalty != nil {
		req.FrequencyPenalty = *c.config.FrequencyPenalty
	}
	if c.config.User != "" {
		req.User = c.config.User
	}
	if c.config.Seed != nil {
		req.Seed = c.config.Seed
	}

	// Apply response format - request override takes priority
	var responseFormat *ai.ResponseFormat
	if schema != nil {
		responseFormat = schema
	} else {
		responseFormat = c.config.ResponseFormat
	}

	if responseFormat != nil {
		switch responseFormat.Type {
		case "json_object":
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			}
		case "json_schema":
			if responseFormat.Schema != nil {
				// Convert jsonschema.Schema to OpenAI format
				schemaBytes, err := json.Marshal(responseFormat.Schema)
				if err == nil {
					var schemaObj any
					if json.Unmarshal(schemaBytes, &schemaObj) == nil {
						req.ResponseFormat = &openai.ChatCompletionResponseFormat{
							Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
							JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
								Name:   "response_schema",
								Schema: json.RawMessage(schemaBytes),
								Strict: true,
							},
						}
					}
				}
			} else {
				// Fallback to json_object if schema is nil
				req.ResponseFormat = &openai.ChatCompletionResponseFormat{
					Type: openai.ChatCompletionResponseFormatTypeJSONObject,
				}
			}
		}
	}
}

// convertToOpenAITools converts our tool interface to OpenAI's tool format
func (c *Client) convertToOpenAITools(toolList []tools.Tool) []openai.Tool {
	openaiTools := make([]openai.Tool, len(toolList))

	for i, tool := range toolList {
		// Convert jsonschema.Schema to map for OpenAI parameters
		var parameters map[string]any
		if schema := tool.ParametersSchema(); schema != nil {
			// Marshal and unmarshal to convert to generic map
			if schemaBytes, err := json.Marshal(schema); err == nil {
				json.Unmarshal(schemaBytes, &parameters)
			}
		}

		openaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  parameters,
			},
		}
	}

	return openaiTools
}

// writeOpenAIToolCalls formats OpenAI tool calls for the agent framework
func (c *Client) writeOpenAIToolCalls(toolCalls []openai.ToolCall, w *calque.Response) error {
	// Convert to the expected format
	formattedToolCalls := make([]map[string]any, len(toolCalls))

	for i, call := range toolCalls {
		formattedToolCalls[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":      call.Function.Name,
				"arguments": call.Function.Arguments,
			},
		}
	}

	// Create the expected JSON structure
	result := map[string]any{
		"tool_calls": formattedToolCalls,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = w.Data.Write(jsonBytes)
	return err
}
