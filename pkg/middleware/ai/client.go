package ai

import (
	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/invopop/jsonschema"
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

// Float32Ptr creates a pointer to a float32 value.
//
// Input: float32 value
// Output: *float32 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Temperature = ai.Float32Ptr(0.9)
func Float32Ptr(f float32) *float32 { return &f }

// IntPtr creates a pointer to an int value.
//
// Input: int value
// Output: *int pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.MaxTokens = ai.IntPtr(1500)
func IntPtr(i int) *int { return &i }

// Int32Ptr creates a pointer to an int32 value.
//
// Input: int32 value
// Output: *int32 pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Seed = ai.Int32Ptr(1500)
func Int32Ptr(i int32) *int32 { return &i }

// BoolPtr creates a pointer to a bool value.
//
// Input: bool value
// Output: *bool pointer
// Behavior: Helper for optional config fields
//
// Example:
//
//	config.Streaming = ai.BoolPtr(false)
func BoolPtr(b bool) *bool { return &b }
