package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/tools"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"google.golang.org/genai"
)

// GeminiProvider implements the LLMProvider interface for Google Gemini
type GeminiProvider struct {
	client       *genai.Client
	model        string
	defaultConfig *Config
}

// NewGeminiProvider creates a new Gemini provider with configuration
// If apiKey is empty, it will try to read from GOOGLE_API_KEY environment variable
// If config is nil, uses DefaultConfig()
func NewGeminiProvider(apiKey, model string, config *Config) (*GeminiProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable not set")
	}
	if model == "" {
		model = "gemini-1.5-flash" // Default to free tier model
	}

	if config == nil {
		config = DefaultConfig()
	}

	// Configure the GenAI client
	clientConfig := &genai.ClientConfig{
		APIKey: apiKey,
	}

	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &GeminiProvider{
		client:        client,
		model:         model,
		defaultConfig: config,
	}, nil
}

// Chat implements the LLMProvider interface with streaming support
func (g *GeminiProvider) Chat(r *core.Request, w *core.Response) error {
	return g.ChatWithTools(r, w)
}

func (g *GeminiProvider) ChatWithTools(r *core.Request, w *core.Response, tools ...tools.Tool) error {
	return g.ChatWithSchema(r, w, nil, tools...)
}

func (g *GeminiProvider) ChatWithSchema(r *core.Request, w *core.Response, schema *ResponseFormat, tools ...tools.Tool) error {
	// Read input
	inputBytes, err := io.ReadAll(r.Data)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Use provider's default config
	finalConfig := g.defaultConfig
	
	// Override response format if provided
	if schema != nil {
		finalConfig = g.mergeConfigs(&Config{ResponseFormat: schema}, g.defaultConfig)
	}

	// Create chat configuration
	genaiConfig := &genai.GenerateContentConfig{}
	
	// Apply configuration
	if finalConfig.Temperature != nil {
		genaiConfig.Temperature = genai.Ptr(*finalConfig.Temperature)
	}
	if finalConfig.MaxTokens != nil {
		genaiConfig.MaxOutputTokens = int32(*finalConfig.MaxTokens)
	}
	if finalConfig.TopP != nil {
		genaiConfig.TopP = genai.Ptr(*finalConfig.TopP)
	}
	if finalConfig.Stop != nil && len(finalConfig.Stop) > 0 {
		genaiConfig.StopSequences = finalConfig.Stop
	}

	// Apply structured output format
	if finalConfig.ResponseFormat != nil {
		switch finalConfig.ResponseFormat.Type {
		case "json_object":
			// Request JSON format without schema
			genaiConfig.ResponseMIMEType = "application/json"
		case "json_schema":
			// Request JSON format with schema (Gemini 1.5 Pro supports this)
			genaiConfig.ResponseMIMEType = "application/json"
			if finalConfig.ResponseFormat.Schema != nil {
				// Convert JSON schema to Gemini schema format
				genaiConfig.ResponseSchema = convertJSONSchemaToGemini(finalConfig.ResponseFormat.Schema)
			}
		}
	}

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

// Convert your OpenAI JSON schema tools to Gemini format
func convertToolsToGeminiFunctions(tools []tools.Tool) []*genai.FunctionDeclaration {
	var functions []*genai.FunctionDeclaration

	for _, tool := range tools {
		functions = append(functions, &genai.FunctionDeclaration{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  convertSchemaToGeminiSchema(tool.ParametersSchema()),
		})
	}

	return functions
}

func convertSchemaToGeminiSchema(schema *jsonschema.Schema) *genai.Schema {
	return &genai.Schema{
		Type:       genai.Type(schema.Type),
		Properties: convertProperties(schema.Properties),
		Required:   schema.Required,
	}
}

func convertProperties(properties *orderedmap.OrderedMap[string, *jsonschema.Schema]) map[string]*genai.Schema {
	if properties == nil {
		return nil
	}
	
	result := make(map[string]*genai.Schema)
	for pair := properties.Oldest(); pair != nil; pair = pair.Next() {
		result[pair.Key] = convertSchemaToGeminiSchema(pair.Value)
	}
	
	return result
}

// writeFunctionCalls formats Gemini function calls as OpenAI JSON format for the agent
func (g *GeminiProvider) writeFunctionCalls(functionCalls []*genai.FunctionCall, w *core.Response) error {
	// Convert to OpenAI format
	var toolCalls []map[string]interface{}
	
	for _, call := range functionCalls {
		// Convert Gemini args to JSON string
		var argsJSON string
		if call.Args != nil && call.Args["input"] != nil {
			argsJSON = fmt.Sprintf(`{"input": "%v"}`, call.Args["input"])
		} else {
			argsJSON = `{"input": ""}`
		}
		
		// OpenAI format with type and function fields
		toolCall := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":      call.Name,
				"arguments": argsJSON,
			},
		}
		toolCalls = append(toolCalls, toolCall)
	}
	
	// Use json.Marshal for proper JSON formatting
	result := map[string]interface{}{
		"tool_calls": toolCalls,
	}
	
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}
	
	_, err = w.Data.Write(jsonBytes)
	return err
}

// Name returns the provider name
func (g *GeminiProvider) Name() string {
	return "gemini"
}

// SupportedFeatures returns the features supported by Gemini
func (g *GeminiProvider) SupportedFeatures() ProviderFeatures {
	return ProviderFeatures{
		Streaming:        true,
		FunctionCalling:  true,
		StructuredOutput: true, // Gemini supports JSON mode
		Vision:           true,
		SystemPrompts:    true,
	}
}

// mergeConfigs merges multiple configs with priority order
func (g *GeminiProvider) mergeConfigs(configs ...*Config) *Config {
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

// convertJSONSchemaToGemini converts a JSON schema to Gemini's schema format
func convertJSONSchemaToGemini(schema *jsonschema.Schema) *genai.Schema {
	if schema == nil {
		return nil
	}
	
	return &genai.Schema{
		Type:       genai.Type(schema.Type),
		Properties: convertProperties(schema.Properties),
		Required:   schema.Required,
	}
}
