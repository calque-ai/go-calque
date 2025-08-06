package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/tools"
	"github.com/ollama/ollama/api"
)

// OllamaProvider implements the LLMProvider interface for Ollama
type OllamaProvider struct {
	client        *api.Client
	model         string
	defaultConfig *Config
}

// NewOllamaProvider creates a new Ollama provider with configuration
// If host is empty, it uses ClientFromEnvironment() which defaults to localhost:11434
// If config is nil, uses DefaultConfig()
func NewOllamaProvider(host, model string, config *Config) (*OllamaProvider, error) {
	if model == "" {
		model = "llama3.2" // Default model
	}

	if config == nil {
		config = DefaultConfig()
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
		client:        client,
		model:         model,
		defaultConfig: config,
	}, nil
}

// Chat implements the LLMProvider interface with streaming support
func (o *OllamaProvider) Chat(r *core.Request, w *core.Response) error {
	return o.ChatWithTools(r, w)
}

// ChatWithTools implements native Ollama function calling
func (o *OllamaProvider) ChatWithTools(r *core.Request, w *core.Response, toolList ...tools.Tool) error {
	return o.ChatWithSchema(r, w, nil, toolList...)
}

func (o *OllamaProvider) ChatWithSchema(r *core.Request, w *core.Response, schema *ResponseFormat, toolList ...tools.Tool) error {
	// Read input
	inputBytes, err := io.ReadAll(r.Data)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Use provider's default config
	finalConfig := o.defaultConfig
	
	// Override response format if provided
	if schema != nil {
		finalConfig = o.mergeConfigs(&Config{ResponseFormat: schema}, o.defaultConfig)
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
	}

	// Apply configuration
	if finalConfig.Temperature != nil {
		req.Options = make(map[string]interface{})
		req.Options["temperature"] = *finalConfig.Temperature
	}
	if finalConfig.TopP != nil {
		if req.Options == nil {
			req.Options = make(map[string]interface{})
		}
		req.Options["top_p"] = *finalConfig.TopP
	}
	if finalConfig.MaxTokens != nil {
		if req.Options == nil {
			req.Options = make(map[string]interface{})
		}
		req.Options["num_predict"] = *finalConfig.MaxTokens
	}
	if finalConfig.Stop != nil && len(finalConfig.Stop) > 0 {
		if req.Options == nil {
			req.Options = make(map[string]interface{})
		}
		req.Options["stop"] = finalConfig.Stop
	}

	// Add tools to request if provided
	if len(toolList) > 0 {
		req.Tools = o.convertToOllamaTools(toolList)
	}

	// Collect response to check for tool calls
	var fullResponse strings.Builder
	var toolCalls []api.ToolCall

	responseFunc := func(resp api.ChatResponse) error {
		// Collect tool calls
		if len(resp.Message.ToolCalls) > 0 {
			toolCalls = append(toolCalls, resp.Message.ToolCalls...)
		}

		// For tool responses, don't stream text - wait for full response
		if len(toolList) > 0 {
			fullResponse.WriteString(resp.Message.Content)
		} else {
			// Stream text content as it arrives for non-tool responses
			if resp.Message.Content != "" {
				if _, writeErr := w.Data.Write([]byte(resp.Message.Content)); writeErr != nil {
					return writeErr
				}
			}
		}
		return nil
	}

	// Send chat request
	err = o.client.Chat(r.Context, req, responseFunc)
	if err != nil {
		return fmt.Errorf("failed to chat with ollama: %w", err)
	}

	// If we have tool calls, format them as OpenAI JSON for the agent
	if len(toolCalls) > 0 {
		// fmt.Printf("Found %d tool calls from Ollama\n", len(toolCalls))
		return o.writeOllamaToolCalls(toolCalls, w)
	}

	// If we have tools but no tool calls detected, check if response contains tool syntax
	if len(toolList) > 0 && fullResponse.Len() > 0 {
		responseText := fullResponse.String()
		// fmt.Printf("No structured tool calls found, checking response text: %s\n", responseText)

		// Try to parse the text response for tool information
		if strings.Contains(responseText, `"name":`) && strings.Contains(responseText, `"parameters":`) {
			// fmt.Printf("Found tool-like text, attempting to convert to OpenAI format\n")
			return o.convertTextToToolCalls(responseText, w)
		}
	}

	return nil
}

// convertToOllamaTools converts our tool interface to Ollama's tool format
func (o *OllamaProvider) convertToOllamaTools(toolList []tools.Tool) []api.Tool {
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
func (o *OllamaProvider) writeOllamaToolCalls(toolCalls []api.ToolCall, w *core.Response) error {
	// Convert to OpenAI format
	var openAIToolCalls []map[string]interface{}

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

		toolCall := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":      call.Function.Name,
				"arguments": argsJSON,
			},
		}
		openAIToolCalls = append(openAIToolCalls, toolCall)
	}

	// Create OpenAI format JSON
	result := map[string]interface{}{
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
func (o *OllamaProvider) convertTextToToolCalls(responseText string, w *core.Response) error {
	// This is a fallback for when Ollama returns tool calls as text instead of structured data
	// For now, just write the text response - this needs more sophisticated parsing
	_, err := w.Data.Write([]byte(responseText))
	return err
}

// Name returns the provider name
func (o *OllamaProvider) Name() string {
	return "ollama"
}

// SupportedFeatures returns the features supported by Ollama
func (o *OllamaProvider) SupportedFeatures() ProviderFeatures {
	return ProviderFeatures{
		Streaming:        true,
		FunctionCalling:  true, // Ollama supports function calling
		StructuredOutput: false, // Ollama doesn't support structured JSON output natively
		Vision:           true,  // Some Ollama models support vision
		SystemPrompts:    true,
	}
}

// mergeConfigs merges multiple configs with priority order
func (o *OllamaProvider) mergeConfigs(configs ...*Config) *Config {
	result := &Config{}
	
	for _, config := range configs {
		if config == nil {
			continue
		}
		
		if config.Temperature != nil {
			result.Temperature = config.Temperature
		}
		if config.TopP != nil {
			result.TopP = config.TopP
		}
		if config.MaxTokens != nil {
			result.MaxTokens = config.MaxTokens
		}
		if config.Stop != nil {
			result.Stop = config.Stop
		}
		if config.PresencePenalty != nil {
			result.PresencePenalty = config.PresencePenalty
		}
		if config.FrequencyPenalty != nil {
			result.FrequencyPenalty = config.FrequencyPenalty
		}
		if config.ResponseFormat != nil {
			result.ResponseFormat = config.ResponseFormat
		}
		if config.Streaming != nil {
			result.Streaming = config.Streaming
		}
	}
	
	return result
}
