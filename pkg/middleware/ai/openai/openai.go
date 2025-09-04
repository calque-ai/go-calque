// Package openai provides a Calque middleware client for OpenAI's Chat Completions API.
// It implements streaming and non-streaming chat completions with support for tools,
// multimodal inputs, and structured response formats including JSON schema validation.
//
// The client supports:
//   - Text and multimodal (image) chat completions
//   - Function calling with tool integration
//   - Streaming responses with Server-Sent Events
//   - Structured outputs with JSON schema
//   - Configurable model parameters (temperature, max tokens, etc.)
//
// Example usage:
//
//	client, err := openai.New("gpt-5", &openai.Config{
//		Temperature: openai.Float32(0.7),
//		Stream:      openai.Bool(true),
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Use with Calque flow
//	flow := calque.NewFlow().Use(client)
package openai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/shared"
	"github.com/openai/openai-go/v2/shared/constant"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/config"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// Client implements the Client interface for OpenAI.
//
// Provides streaming chat completions with tool calling and multimodal support.
// Supports GPT-5, GPT-4, GPT-4o, GPT-4o-mini, GPT-3.5, and other OpenAI models with full feature compatibility.
//
// Example:
//
//	client, _ := openai.New("gpt-4o")
//	agent := ai.Agent(client)
type Client struct {
	client *openai.Client
	model  shared.ChatModel
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
//		Temperature: helpers.PtrOf(float32(0.8)),
//		MaxTokens: helpers.PtrOf(1000),
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
	config.Merge(opts, o.config)
}

// WithConfig sets custom OpenAI configuration.
//
// Input: *Config with OpenAI settings
// Output: Option for client creation
// Behavior: Merges with default configuration (only non-zero/nil fields override defaults)
//
// Example:
//
//	config := &openai.Config{Temperature: helpers.PtrOf(float32(0.9))}
//	client, _ := openai.New("gpt-4", openai.WithConfig(config))
func WithConfig(cfg *Config) Option {
	return configOption{config: cfg}
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
//	config.MaxTokens = helpers.PtrOf(2000)
func DefaultConfig() *Config {
	return &Config{
		APIKey:      os.Getenv("OPENAI_API_KEY"),
		Temperature: helpers.PtrOf(float32(0.7)),
		Stream:      helpers.PtrOf(true),
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

	// Create client options
	var clientOptions []option.RequestOption
	clientOptions = append(clientOptions, option.WithAPIKey(config.APIKey))

	if config.BaseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(config.BaseURL))
	}

	openaiClient := openai.NewClient(clientOptions...)

	return &Client{
		client: &openaiClient,
		model:  shared.ChatModel(model),
		config: config,
	}, nil
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

	// Build request parameters
	params, err := c.buildChatParams(input, ai.GetSchema(opts), ai.GetTools(opts))
	if err != nil {
		return err
	}

	// Execute the request
	return c.executeRequest(params, r, w)
}

// buildChatParams creates OpenAI chat completion parameters
func (c *Client) buildChatParams(input *ai.ClassifiedInput, schema *ai.ResponseFormat, toolList []tools.Tool) (openai.ChatCompletionNewParams, error) {
	// Convert input to messages
	messages, err := c.inputToMessages(input)
	if err != nil {
		return openai.ChatCompletionNewParams{}, err
	}

	// Create base parameters
	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: messages,
	}

	// Apply configuration
	c.applyChatConfig(&params, schema)

	// Add tools if provided
	if len(toolList) > 0 {
		tools, err := c.convertToOpenAITools(toolList)
		if err != nil {
			return openai.ChatCompletionNewParams{}, err
		}
		params.Tools = tools
	}

	return params, nil
}

// executeRequest executes the configured request
func (c *Client) executeRequest(params openai.ChatCompletionNewParams, r *calque.Request, w *calque.Response) error {
	// Determine if we should stream
	shouldStream := c.config.Stream == nil || *c.config.Stream

	if shouldStream {
		return c.executeStreamingRequest(params, r, w)
	}

	return c.executeNonStreamingRequest(params, r, w)

}

// executeStreamingRequest executes a streaming request
func (c *Client) executeStreamingRequest(params openai.ChatCompletionNewParams, r *calque.Request, w *calque.Response) (err error) {
	// Create streaming request
	stream := c.client.Chat.Completions.NewStreaming(r.Context, params)
	defer func() {
		if closeErr := stream.Close(); closeErr != nil && err == nil {
			// Only set the error if no other error occurred
			err = fmt.Errorf("failed to close stream: %w", closeErr)
		}
	}()

	// Track multiple tool calls by ID
	toolCalls := make(map[int]*openai.ChatCompletionMessageFunctionToolCall)

	// Process streaming response
	for stream.Next() {
		chunk := stream.Current()

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// Handle content streaming
		if delta.Content != "" {
			if _, writeErr := w.Data.Write([]byte(delta.Content)); writeErr != nil {
				return writeErr
			}
		}

		// Handle tool calls (streaming)
		for _, toolCall := range delta.ToolCalls {
			index := int(toolCall.Index)

			// Initialize tool call if not exists
			if toolCalls[index] == nil {
				toolCalls[index] = &openai.ChatCompletionMessageFunctionToolCall{
					ID: toolCall.ID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "",
						Arguments: "",
					},
				}
			}

			// Accumulate function name and arguments
			if toolCall.Function.Name != "" {
				toolCalls[index].Function.Name = toolCall.Function.Name
			}
			if toolCall.Function.Arguments != "" {
				toolCalls[index].Function.Arguments += toolCall.Function.Arguments
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("failed to receive stream response: %w", err)
	}

	// If we have accumulated tool calls, format them
	if len(toolCalls) > 0 {
		// Convert map to slice, maintaining order by index
		var completedCalls []openai.ChatCompletionMessageFunctionToolCall
		for i := 0; i < len(toolCalls); i++ {
			if call, exists := toolCalls[i]; exists && call.Function.Name != "" {
				completedCalls = append(completedCalls, *call)
			}
		}

		if len(completedCalls) > 0 {
			return c.writeOpenAIToolCalls(completedCalls, w)
		}
	}

	return nil
}

// executeNonStreamingRequest executes a non-streaming request
func (c *Client) executeNonStreamingRequest(params openai.ChatCompletionNewParams, r *calque.Request, w *calque.Response) error {
	// Create request
	response, err := c.client.Chat.Completions.New(r.Context, params)
	if err != nil {
		return fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("no response choices returned")
	}

	// Process all choices (handles N > 1 configurations)
	for i, choice := range response.Choices {
		// Handle tool calls first (they take precedence)
		if len(choice.Message.ToolCalls) > 0 {
			// Convert union tool calls to function tool calls
			var functionToolCalls []openai.ChatCompletionMessageFunctionToolCall
			for _, toolCall := range choice.Message.ToolCalls {
				fnToolCall := toolCall.AsFunction()
				functionToolCalls = append(functionToolCalls, fnToolCall)
			}
			if len(functionToolCalls) > 0 {
				return c.writeOpenAIToolCalls(functionToolCalls, w)
			}
		}

		// Handle content
		if choice.Message.Content != "" {
			// Add separator between multiple choices
			if i > 0 {
				if _, err := w.Data.Write([]byte("\n\n--- Choice " + fmt.Sprintf("%d", i+1) + " ---\n\n")); err != nil {
					return err
				}
			}
			if _, err := w.Data.Write([]byte(choice.Message.Content)); err != nil {
				return err
			}
		}
	}

	return nil
}

// inputToMessages converts classified input to OpenAI message format
func (c *Client) inputToMessages(input *ai.ClassifiedInput) ([]openai.ChatCompletionMessageParamUnion, error) {
	switch input.Type {
	case ai.TextInput:
		return []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(input.Text),
		}, nil

	case ai.MultimodalJSONInput, ai.MultimodalStreamingInput:
		return c.multimodalToMessages(input.Multimodal)

	default:
		return nil, fmt.Errorf("unsupported input type: %d", input.Type)
	}
}

// multimodalToMessages converts multimodal input to OpenAI message format
func (c *Client) multimodalToMessages(multimodal *ai.MultimodalInput) ([]openai.ChatCompletionMessageParamUnion, error) {
	if multimodal == nil {
		return nil, fmt.Errorf("multimodal input cannot be nil")
	}

	messageParts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(multimodal.Parts))

	for _, part := range multimodal.Parts {
		switch part.Type {
		case "text":
			if part.Text != "" {
				messageParts = append(messageParts, openai.ChatCompletionContentPartUnionParam{
					OfText: &openai.ChatCompletionContentPartTextParam{
						Type: constant.Text("").Default(),
						Text: part.Text,
					},
				})
			}
		case "image":
			var dataURL string
			var err error

			if part.Reader != nil {
				// Use streaming base64 encoder to avoid loading entire image into memory
				var buf strings.Builder
				buf.WriteString(fmt.Sprintf("data:%s;base64,", part.MimeType))

				encoder := base64.NewEncoder(base64.StdEncoding, &buf)
				_, err = io.Copy(encoder, part.Reader)
				if err != nil {
					return nil, fmt.Errorf("failed to encode image data: %w", err)
				}

				if closeErr := encoder.Close(); closeErr != nil {
					return nil, fmt.Errorf("failed to finalize image encoding: %w", closeErr)
				}

				dataURL = buf.String()
			} else if part.Data != nil {
				// Use embedded data (simple approach) - still efficient for small images
				base64Data := base64.StdEncoding.EncodeToString(part.Data)
				dataURL = fmt.Sprintf("data:%s;base64,%s", part.MimeType, base64Data)
			}

			if dataURL != "" {
				messageParts = append(messageParts, openai.ChatCompletionContentPartUnionParam{
					OfImageURL: &openai.ChatCompletionContentPartImageParam{
						Type: constant.ImageURL("").Default(),
						ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
							URL: dataURL,
						},
					}})
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

	return []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(messageParts),
	}, nil
}

// applyChatConfig applies client configuration to the chat request
func (c *Client) applyChatConfig(params *openai.ChatCompletionNewParams, schema *ai.ResponseFormat) {
	// Apply client configuration
	if c.config.Temperature != nil {
		params.Temperature = openai.Float(float64(*c.config.Temperature))
	}
	if c.config.TopP != nil {
		params.TopP = openai.Float(float64(*c.config.TopP))
	}
	if c.config.MaxTokens != nil {
		params.MaxCompletionTokens = openai.Int(int64(*c.config.MaxTokens))
	}
	if c.config.N != nil {
		params.N = openai.Int(int64(*c.config.N))
	}
	if len(c.config.Stop) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{OfStringArray: c.config.Stop}
	}
	if c.config.PresencePenalty != nil {
		params.PresencePenalty = openai.Float(float64(*c.config.PresencePenalty))
	}
	if c.config.FrequencyPenalty != nil {
		params.FrequencyPenalty = openai.Float(float64(*c.config.FrequencyPenalty))
	}
	if c.config.User != "" {
		params.User = openai.String(c.config.User)
	}
	if c.config.Seed != nil {
		params.Seed = openai.Int(int64(*c.config.Seed))
	}

	// Apply response format - request override takes priority
	var responseFormat *ai.ResponseFormat
	if schema != nil {
		responseFormat = schema
	} else {
		responseFormat = c.config.ResponseFormat
	}

	if responseFormat != nil {
		c.setResponseFormat(responseFormat, params)
	}
}

// setResponseFormat applies the response format to OpenAI parameters
func (c *Client) setResponseFormat(responseFormat *ai.ResponseFormat, params *openai.ChatCompletionNewParams) {
	switch responseFormat.Type {
	case "json_object":
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{Type: constant.JSONObject("").Default()},
		}

	case "json_schema":
		if responseFormat.Schema != nil {
			c.setJSONSchemaFormat(responseFormat.Schema, params)
		}
	}
}

// setJSONSchemaFormat sets the JSON schema response format
func (c *Client) setJSONSchemaFormat(schema any, params *openai.ChatCompletionNewParams) {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		// Fallback to json_object on error
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{Type: constant.JSONObject("").Default()},
		}
		return
	}

	params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
			Type: constant.JSONSchema("").Default(),
			JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:        "response_schema",
				Schema:      schemaBytes,
				Strict:      openai.Bool(true),
				Description: openai.String("Generated schema for structured response"),
			},
		},
	}
}

// convertToOpenAITools converts our tool interface to OpenAI's tool format
func (c *Client) convertToOpenAITools(toolList []tools.Tool) ([]openai.ChatCompletionToolUnionParam, error) {
	openaiTools := make([]openai.ChatCompletionToolUnionParam, len(toolList))

	for i, tool := range toolList {
		// Convert jsonschema.Schema to map for OpenAI parameters
		var parameters map[string]any
		if schema := tool.ParametersSchema(); schema != nil {
			// Marshal and unmarshal to convert to generic map
			schemaBytes, err := json.Marshal(schema)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool parameters schema: %w", err)
			}
			err = json.Unmarshal(schemaBytes, &parameters)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tool parameters schema: %w", err)
			}

		}

		openaiTools[i] = openai.ChatCompletionFunctionTool(
			openai.FunctionDefinitionParam{
				Name:        tool.Name(),
				Description: openai.String(tool.Description()),
				Parameters:  parameters,
			},
		)
	}

	return openaiTools, nil
}

// writeOpenAIToolCalls formats OpenAI tool calls for the agent framework
func (c *Client) writeOpenAIToolCalls(toolCalls []openai.ChatCompletionMessageFunctionToolCall, w *calque.Response) error {
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
