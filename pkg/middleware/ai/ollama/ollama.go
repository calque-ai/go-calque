package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ai"
	"github.com/calque-ai/calque-pipe/pkg/middleware/tools"
	"github.com/invopop/jsonschema"
	"github.com/ollama/ollama/api"
)

// Client implements the Client interface for Ollama.
//
// Provides streaming chat completions with local model support.
// Connects to Ollama server for running models like Llama, Mistral, etc.
//
// Example:
//
//	client, _ := ollama.New("llama3.2")
//	agent := ai.Agent(client)
type Client struct {
	client *api.Client
	model  string
	config *Config
}

// Config holds Ollama-specific configuration.
//
// Configures connection, model parameters, and response format.
// All fields are optional with sensible defaults.
//
// Example:
//
//	config := &ollama.Config{
//		Host: "http://192.168.1.100:11434",
//		Temperature: ai.Float32Ptr(0.8),
//	}
type Config struct {
	// Optional. Ollama server host (defaults to localhost:11434 or OLLAMA_HOST env)
	Host string

	// Optional. Controls randomness in token selection (0.0-2.0)
	// Lower values = more deterministic, higher values = more creative
	Temperature *float32

	// Optional. Nucleus sampling parameter (0.0-1.0)
	// Tokens are selected until their probabilities sum to this value
	TopP *float32

	// Optional. Maximum number of tokens in the response
	MaxTokens *int

	// Optional. Strings that stop text generation when encountered
	Stop []string

	// Optional. Controls how long the model stays loaded in memory (e.g. "5m", "1h")
	// Use "-1" to keep loaded indefinitely, "0" to unload immediately
	KeepAlive string

	// Optional. Enable/disable streaming of responses (true by default in Ollama)
	Stream *bool

	// Optional. Response format configuration (JSON schema, etc.)
	ResponseFormat *ai.ResponseFormat

	// Optional. Controls whether thinking/reasoning models will think before responding
	Think *bool

	// Optional. Model-specific options (temperature, top_p, etc.)
	// These override the individual fields above if both are set
	Options map[string]any
}

// Option interface for functional options pattern
type Option interface {
	Apply(*Config)
}

// configOption implements Option
type configOption struct{ config *Config }

func (o configOption) Apply(opts *Config) { *opts = *o.config }

// WithConfig sets custom Ollama configuration.
//
// Input: *Config with Ollama settings
// Output: Option for client creation
// Behavior: Overrides default configuration
//
// Example:
//
//	config := &ollama.Config{Host: "http://remote:11434"}
//	client, _ := ollama.New("llama3.2", ollama.WithConfig(config))
func WithConfig(config *Config) Option {
	return configOption{config: config}
}

// DefaultConfig returns sensible defaults for Ollama.
//
// Input: none
// Output: *Config with default settings
// Behavior: Creates config for localhost Ollama server
//
// Sets host to localhost:11434, 5m keep-alive, 0.7 temperature.
//
// Example:
//
//	config := ollama.DefaultConfig()
//	config.MaxTokens = ai.IntPtr(2000)
func DefaultConfig() *Config {
	return &Config{
		Host:        "", // Will use ClientFromEnvironment() default
		Temperature: ai.Float32Ptr(0.7),
		KeepAlive:   "5m",
		Stream:      ai.BoolPtr(true), // Ollama streams by default
	}
}

// New creates a new Ollama client with optional configuration.
//
// Input: model name string, optional config Options
// Output: *Client, error
// Behavior: Initializes HTTP client for Ollama server
//
// Requires Ollama server running with specified model available.
// Use 'ollama list' to see available models.
//
// Example:
//
//	client, err := ollama.New("llama3.2:latest")
//	if err != nil { log.Fatal(err) }
func New(model string, opts ...Option) (*Client, error) {
	if model == "" {
		model = "llama3.2" // Default model
	}

	// Build config from options
	config := DefaultConfig()
	for _, opt := range opts {
		opt.Apply(config)
	}

	// Create Ollama client
	var client *api.Client
	var err error

	if config.Host == "" {
		// Use environment-based client (checks OLLAMA_HOST env var)
		client, err = api.ClientFromEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to create client from environment: %w", err)
		}
	} else {
		// Parse the host URL
		u, err := url.Parse(config.Host)
		if err != nil {
			return nil, fmt.Errorf("invalid host URL: %w", err)
		}
		// Create client with custom host
		client = api.NewClient(u, http.DefaultClient)
	}

	return &Client{
		client: client,
		model:  model,
		config: config,
	}, nil
}

// Chat implements the Client interface.
//
// Input: user prompt/query via calque.Request
// Output: streamed AI response via calque.Response
// Behavior: STREAMING - outputs tokens as they arrive
//
// Supports JSON schema responses and tool calling (model dependent).
// Automatically handles conversation context and system messages.
//
// Example:
//
//	err := client.Chat(req, res, &ai.AgentOptions{Tools: tools})
func (o *Client) Chat(r *calque.Request, w *calque.Response, opts *ai.AgentOptions) error {
	// Extract options
	var toolList []tools.Tool
	var schema *ai.ResponseFormat

	if opts != nil {
		toolList = opts.Tools
		schema = opts.Schema
	}
	// Read input
	inputBytes, err := io.ReadAll(r.Data)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Create chat request with configuration
	req := o.buildChatRequest(string(inputBytes), schema)

	// Add tools to request if provided
	if len(toolList) > 0 {
		req.Tools = o.convertToOllamaTools(toolList)
	}

	// Handle response streaming and processing
	return o.handleChatResponse(r.Context, req, w, toolList, schema)
}

// handleChatResponse manages the streaming response and post-processing
func (o *Client) handleChatResponse(ctx context.Context, req *api.ChatRequest, w *calque.Response, toolList []tools.Tool, schema *ai.ResponseFormat) error {
	var fullResponse strings.Builder
	var toolCalls []api.ToolCall

	// Determine if we need to buffer the response
	shouldBuffer := len(toolList) > 0 || schema != nil

	responseFunc := func(resp api.ChatResponse) error {
		// Collect tool calls
		if len(resp.Message.ToolCalls) > 0 {
			toolCalls = append(toolCalls, resp.Message.ToolCalls...)
		}

		if shouldBuffer {
			// Buffer the response for tools or JSON schema processing
			fullResponse.WriteString(resp.Message.Content)
		} else {
			// Stream directly for plain text responses
			if resp.Message.Content != "" {
				_, err := w.Data.Write([]byte(resp.Message.Content))
				return err
			}
		}
		return nil
	}

	// Send chat request
	err := o.client.Chat(ctx, req, responseFunc)
	if err != nil {
		return fmt.Errorf("failed to chat with ollama: %w", err)
	}

	// Process tool calls if found
	if len(toolCalls) > 0 {
		return o.writeOllamaToolCalls(toolCalls, w)
	}

	// Handle text-based tool calls as fallback
	if len(toolList) > 0 && fullResponse.Len() > 0 {
		responseText := fullResponse.String()
		if strings.Contains(responseText, `"name":`) && strings.Contains(responseText, `"parameters":`) {
			return o.convertTextToToolCalls(responseText, w)
		}
	}

	// Process buffered response for JSON schema
	if schema != nil && fullResponse.Len() > 0 {
		responseText := fullResponse.String()
		responseText = o.cleanFullJSONResponse(responseText)
		_, err := w.Data.Write([]byte(responseText))
		return err
	}

	return nil
}

// convertToOllamaTools converts our tool interface to Ollama's tool format
func (o *Client) convertToOllamaTools(toolList []tools.Tool) []api.Tool {
	var ollamaTools []api.Tool

	for _, tool := range toolList {
		schema := tool.ParametersSchema()

		// Convert schema properties to Ollama format
		properties := make(map[string]struct {
			Type        api.PropertyType `json:"type"`
			Items       any              `json:"items,omitempty"`
			Description string           `json:"description"`
			Enum        []any            `json:"enum,omitempty"`
		})

		if schema.Properties != nil {
			for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
				properties[pair.Key] = struct {
					Type        api.PropertyType `json:"type"`
					Items       any              `json:"items,omitempty"`
					Description string           `json:"description"`
					Enum        []any            `json:"enum,omitempty"`
				}{
					Type:        api.PropertyType{pair.Value.Type},
					Description: pair.Value.Description,
				}
			}
		}

		function := api.ToolFunction{
			Name:        tool.Name(),
			Description: tool.Description(),
		}
		function.Parameters.Type = "object"
		function.Parameters.Properties = properties
		function.Parameters.Required = schema.Required

		ollamaTool := api.Tool{
			Type:     "function",
			Function: function,
		}
		ollamaTools = append(ollamaTools, ollamaTool)
	}

	return ollamaTools
}

// writeOllamaToolCalls converts Ollama tool calls to OpenAI format for the agent
func (o *Client) writeOllamaToolCalls(toolCalls []api.ToolCall, w *calque.Response) error {
	// Convert to OpenAI format
	var openAIToolCalls []map[string]any

	for _, call := range toolCalls {
		// Extract input from tool call arguments
		var argsJSON string
		if call.Function.Arguments != nil {
			if inputValue, ok := call.Function.Arguments["input"]; ok {
				argsJSON = fmt.Sprintf(`{"input": "%v"}`, inputValue)
			} else {
				// Convert all arguments to JSON
				argsBytes, _ := json.Marshal(call.Function.Arguments)
				argsJSON = string(argsBytes)
			}
		} else {
			argsJSON = `{"input": ""}`
		}

		toolCall := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":      call.Function.Name,
				"arguments": argsJSON,
			},
		}
		openAIToolCalls = append(openAIToolCalls, toolCall)
	}

	// Create OpenAI format JSON
	result := map[string]any{
		"tool_calls": openAIToolCalls,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = w.Data.Write(jsonBytes)
	return err
}

// convertTextToToolCalls attempts to parse tool calls from text response
func (o *Client) convertTextToToolCalls(responseText string, w *calque.Response) error {
	// This is a fallback for when Ollama returns tool calls as text instead of structured data
	// For now, just write the text response - this needs more sophisticated parsing
	_, err := w.Data.Write([]byte(responseText))
	return err
}

// buildChatRequest creates an Ollama ChatRequest from provider config and optional schema override
func (o *Client) buildChatRequest(input string, schemaOverride *ai.ResponseFormat) *api.ChatRequest {
	req := &api.ChatRequest{
		Model: o.model,
		Messages: []api.Message{
			{
				Role:    "user",
				Content: input,
			},
		},
		Options: make(map[string]any),
	}

	// Apply client configuration
	if o.config.Temperature != nil {
		req.Options["temperature"] = *o.config.Temperature
	}
	if o.config.TopP != nil {
		req.Options["top_p"] = *o.config.TopP
	}
	if o.config.MaxTokens != nil {
		req.Options["num_predict"] = *o.config.MaxTokens
	}
	if len(o.config.Stop) > 0 {
		req.Options["stop"] = o.config.Stop
	}
	if o.config.KeepAlive != "" {
		req.Options["keep_alive"] = o.config.KeepAlive
	}
	if o.config.Stream != nil {
		req.Stream = o.config.Stream
	}
	if o.config.Think != nil {
		req.Think = o.config.Think
	}

	// Apply custom options (these override individual fields above)
	if len(o.config.Options) > 0 {
		for key, value := range o.config.Options {
			req.Options[key] = value
		}
	}

	// Apply response format - request override takes priority
	var responseFormat *ai.ResponseFormat
	if schemaOverride != nil {
		responseFormat = schemaOverride
	} else {
		responseFormat = o.config.ResponseFormat
	}

	if responseFormat != nil {
		switch responseFormat.Type {
		case "json_object":
			// Ollama supports JSON format via format parameter
			req.Format = json.RawMessage(`"json"`)
		case "json_schema":
			// For JSON schema, pass the actual schema object to Ollama's format field
			if responseFormat.Schema != nil {
				// Convert jsonschema.Schema to the format Ollama expects
				schemaBytes, err := convertJSONSchemaToOllamaFormat(responseFormat.Schema)
				if err == nil {
					req.Format = schemaBytes
				} else {
					req.Format = json.RawMessage(`"json"`)
				}
			} else {
				req.Format = json.RawMessage(`"json"`)
			}
		}
	}

	return req
}

// convertJSONSchemaToOllamaFormat converts a JSON schema to Ollama's format field format
func convertJSONSchemaToOllamaFormat(schema *jsonschema.Schema) (json.RawMessage, error) {
	if schema == nil {
		return json.RawMessage(`"json"`), nil
	}

	// Convert jsonschema.Schema to the format Ollama expects
	// Ollama expects the actual schema object, not a string
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON schema: %w", err)
	}

	return json.RawMessage(schemaBytes), nil
}

// cleanFullJSONResponse removes Ollama JSON formatting artifacts from complete buffered response
func (o *Client) cleanFullJSONResponse(content string) string {
	// Trim whitespace
	content = strings.TrimSpace(content)

	// Remove markdown prefixes
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimPrefix(content, "json")
	content = strings.TrimSpace(content)

	// Find the last closing brace (end of JSON object)
	lastBraceIdx := strings.LastIndex(content, "}")
	if lastBraceIdx != -1 {
		// Check if there's content after the last }
		remainder := content[lastBraceIdx+1:]
		remainder = strings.TrimSpace(remainder)

		// If remainder starts with ``` or contains explanatory text, cut it off
		if strings.HasPrefix(remainder, "```") ||
			strings.Contains(remainder, "Analysis") ||
			len(remainder) > 0 {
			content = content[:lastBraceIdx+1]
		}
	}

	return strings.TrimSpace(content)
}
