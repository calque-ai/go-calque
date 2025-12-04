package ai

import (
	"github.com/invopop/jsonschema"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Client interface for AI providers.
//
// Defines the standard interface that all AI client implementations must satisfy.
// Supports streaming responses, tool calling, and structured output through options.
//
// Example:
//
//	client, _ := ollama.New("llama3.2")
//	err := client.Chat(req, res, &ai.AgentOptions{Tools: tools})
type Client interface {
	// Single method handles all cases through options
	Chat(r *calque.Request, w *calque.Response, opts *AgentOptions) error
}

// ResponseFormat defines structured output requirements.
//
// Configures AI models to return structured JSON responses according to
// specified schemas. Supports both simple JSON objects and JSON Schema validation.
//
// Example:
//
//	format := &ai.ResponseFormat{
//		Type: "json_schema",
//		Schema: userProfileSchema,
//	}
type ResponseFormat struct {
	Type   string             `json:"type"`             // "json_object" or "json_schema"
	Schema *jsonschema.Schema `json:"schema,omitempty"` // JSON schema for validation
}

// UsageMetadata contains token usage information from AI requests.
//
// Tracks the number of tokens consumed by prompts and completions.
// Useful for monitoring costs and optimizing token usage.
//
// Example:
//
//	agent := ai.Agent(client, ai.WithUsageHandler(func(usage *ai.UsageMetadata) {
//		log.Printf("Total tokens: %d", usage.TotalTokens)
//	}))
type UsageMetadata struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
