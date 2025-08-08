package ai

import (
	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/invopop/jsonschema"
)

// Client interface - interface for AI providers
type Client interface {
	// Single method handles all cases through options
	Chat(r *calque.Request, w *calque.Response, opts *AgentOptions) error
}

// Config holds LLM model configuration parameters
type Config struct {
	// Model parameters
	Temperature      *float32 `json:"temperature,omitempty"`       // 0.0 - 2.0, controls randomness
	TopP             *float32 `json:"top_p,omitempty"`             // 0.0 - 1.0, nucleus sampling
	MaxTokens        *int     `json:"max_tokens,omitempty"`        // Maximum tokens to generate
	Stop             []string `json:"stop,omitempty"`              // Stop sequences
	PresencePenalty  *float32 `json:"presence_penalty,omitempty"`  // -2.0 - 2.0, penalize new tokens
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"` // -2.0 - 2.0, penalize frequent tokens

	// Response format
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // JSON schema for structured output
	Streaming      *bool           `json:"streaming,omitempty"`       // Enable/disable streaming
}

// ResponseFormat defines structured output requirements
type ResponseFormat struct {
	Type   string             `json:"type"`             // "json_object" or "json_schema"
	Schema *jsonschema.Schema `json:"schema,omitempty"` // JSON schema for validation
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Temperature: Float32Ptr(0.7),
		Streaming:   BoolPtr(true),
	}
}

// Helper functions for pointer creation
func Float32Ptr(f float32) *float32 { return &f }
func IntPtr(i int) *int             { return &i }
func BoolPtr(b bool) *bool          { return &b }
