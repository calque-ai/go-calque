package ai

import (
	"github.com/invopop/jsonschema"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// ToolResultFormatterFunc is a function that creates a handler for formatting tool execution results.
// It receives the AI client (can be nil if not needed), the original user input,
// and returns a handler that processes tool results to produce the final output.
//
// The returned handler receives tool execution results as input and should write
// the formatted final response to the output. This is only used when tools are executed.
type ToolResultFormatterFunc func(client Client, originalInput []byte) calque.Handler

// AgentOptions holds all configuration for an AI agent request.
//
// Configures tools, response schemas, multimodal data, and tool execution behavior.
// Used to customize agent behavior for specific use cases.
//
// Example:
//
//	opts := &ai.AgentOptions{
//		Tools: []tools.Tool{searchTool, calcTool},
//		Schema: jsonSchema,
//		MultimodalData: &multimodalInput,
//	}
type AgentOptions struct {
	Schema              *ResponseFormat
	Tools               []tools.Tool
	ToolsConfig         *tools.Config
	MultimodalData      *MultimodalInput
	ToolResultFormatter ToolResultFormatterFunc
	ToolFormatterClient Client
	UsageHandler        func(*UsageMetadata)
}

// AgentOption interface for functional options pattern.
//
// Enables flexible configuration of agent options using the functional
// options pattern. Allows composing multiple configuration options.
//
// Example:
//
//	agent := ai.Agent(client, ai.WithTools(tools...), ai.WithSchema(schema))
type AgentOption interface {
	Apply(*AgentOptions)
}

// Option implementations
type toolsOption struct{ tools []tools.Tool }

func (o toolsOption) Apply(opts *AgentOptions) { opts.Tools = o.tools }

type schemaOption struct{ schema *ResponseFormat }

func (o schemaOption) Apply(opts *AgentOptions) { opts.Schema = o.schema }

type toolsConfigOption struct{ config *tools.Config }

func (o toolsConfigOption) Apply(opts *AgentOptions) { opts.ToolsConfig = o.config }

type multimodalDataOption struct{ data *MultimodalInput }

func (o multimodalDataOption) Apply(opts *AgentOptions) { opts.MultimodalData = o.data }

type toolResultFormatterOption struct {
	formatter ToolResultFormatterFunc
	client    Client
}

func (o toolResultFormatterOption) Apply(opts *AgentOptions) {
	opts.ToolResultFormatter = o.formatter
	opts.ToolFormatterClient = o.client
}

// WithTools adds tools to the agent.
//
// Input: variadic tools.Tool arguments
// Output: AgentOption for configuration
// Behavior: Enables tool calling in agent conversations
//
// Allows the agent to call functions and execute tools during conversations.
// Tools are executed automatically when the agent requests them.
//
// Example:
//
//	agent := ai.Agent(client, ai.WithTools(searchTool, calcTool))
func WithTools(tools ...tools.Tool) AgentOption {
	return toolsOption{tools: tools}
}

// WithSchema adds a response schema to the agent.
// Accepts either a *ResponseFormat or any struct/pointer for automatic schema generation.
//
// Examples:
//
//	ai.WithSchema(&UserProfile{})           // Automatic schema from struct
//	ai.WithSchema(existingResponseFormat)   // Direct ResponseFormat
func WithSchema(schemaSource any) AgentOption {
	var resultSchema *ResponseFormat

	switch v := schemaSource.(type) {
	case *ResponseFormat:
		// Direct use (backwards compatible)
		resultSchema = v
	case ResponseFormat:
		// Value passed, convert to pointer
		resultSchema = &v
	default:
		// Generate schema from struct/pointer
		reflector := jsonschema.Reflector{}
		schema := reflector.Reflect(v)
		resultSchema = &ResponseFormat{
			Type:   "json_schema",
			Schema: schema,
		}
	}

	return schemaOption{schema: resultSchema}
}

// WithSchemaFor is a generic version of WithSchema for compile-time type safety.
// Use this for better performance when the type is known at compile time.
//
// Example: ai.WithSchemaFor[UserProfile]()
func WithSchemaFor[T any]() AgentOption {
	var zero T
	reflector := jsonschema.Reflector{}
	schema := reflector.Reflect(zero)
	return schemaOption{schema: &ResponseFormat{
		Type:   "json_schema",
		Schema: schema,
	}}
}

// WithToolsConfig configures tool behavior.
//
// Input: tools.Config with execution settings
// Output: AgentOption for configuration
// Behavior: Controls tool execution concurrency and error handling
//
// Configures how tools are executed including concurrency limits,
// error handling, and output formatting.
//
// Example:
//
//	config := tools.Config{MaxConcurrentTools: 3}
//	agent := ai.Agent(client, ai.WithToolsConfig(config))
func WithToolsConfig(config tools.Config) AgentOption {
	return toolsConfigOption{config: &config}
}

// WithMultimodalData provides multimodal content to the agent.
//
// Input: *MultimodalInput containing text, images, audio, and/or video
// Output: AgentOption for configuration
// Behavior: Enables multimodal AI interactions with streaming content
//
// Provides the original MultimodalInput structure with io.Reader fields
// to the AI client, enabling streaming processing of binary data.
// Must be used in conjunction with convert.ToJSON() input for metadata.
//
// Example:
//
//	input := ai.Multimodal(
//		ai.Text("What's in this image?"),
//		ai.Image(imageReader, "image/jpeg"),
//	)
//	agent := ai.Agent(client, ai.WithMultimodalData(&input))
//	err := flow.Run(ctx, convert.ToJSON(input), &result)
func WithMultimodalData(data *MultimodalInput) AgentOption {
	return multimodalDataOption{data: data}
}

// WithToolResultFormatter provides a custom formatter for tool execution results.
//
// Input: formatter function and optional AI client for formatting
// Output: AgentOption for configuration
// Behavior: Customizes how tool results are formatted into the final response
//
// The formatter function receives the original user input and tool execution results,
// allowing you to customize the final output. You can optionally provide a different
// AI client (e.g., a smaller/faster model) for result synthesis, or omit it entirely
// for manual formatting.
//
// If no custom formatter is provided, the agent uses a default formatter that makes
// an LLM call to synthesize a natural language answer from the tool results.
//
// Examples:
//
//	// Custom formatting with LLM synthesis using main client
//	ai.WithToolResultFormatter(func(client ai.Client, originalInput []byte) calque.Handler {
//		return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
//			var toolResults []byte
//			calque.Read(r, &toolResults)
//			// Use client to synthesize, then add custom formatting
//			return customFormat(client, originalInput, toolResults, w)
//		})
//	})
//
//	// Custom formatting with different LLM client
//	ai.WithToolResultFormatter(myFormatter, smallerClient)
//
//	// Manual formatting without LLM
//	ai.WithToolResultFormatter(func(client ai.Client, originalInput []byte) calque.Handler {
//		return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
//			var toolResults []byte
//			calque.Read(r, &toolResults)
//			// Format manually, ignore client parameter
//			formatted := fmt.Sprintf("Results:\n%s", toolResults)
//			return calque.Write(w, formatted)
//		})
//	})
func WithToolResultFormatter(formatter ToolResultFormatterFunc, client ...Client) AgentOption {
	opt := toolResultFormatterOption{formatter: formatter}
	if len(client) > 0 {
		opt.client = client[0]
	}
	return opt
}

type usageHandlerOption struct{ handler func(*UsageMetadata) }

func (o usageHandlerOption) Apply(opts *AgentOptions) {
	opts.UsageHandler = o.handler
}

// WithUsageHandler sets a callback for token usage tracking.
//
// Input: handler function called after each AI request
// Output: AgentOption for configuration
// Behavior: Invokes handler with usage metadata after each AI API call
//
// The handler receives token count information from the AI provider.
// In tool-calling scenarios, the handler is called multiple times:
// once for the initial request and once for the synthesis request.
//
// Users are responsible for any required synchronization if tracking
// cumulative usage across concurrent requests.
//
// Example:
//
//	// Simple logging
//	agent := ai.Agent(client,
//		ai.WithUsageHandler(func(usage *ai.UsageMetadata) {
//			log.Printf("Used %d tokens", usage.TotalTokens)
//		}),
//	)
//
//	// Track cumulative usage
//	var totalTokens int
//	var mu sync.Mutex
//	agent := ai.Agent(client,
//		ai.WithUsageHandler(func(usage *ai.UsageMetadata) {
//			mu.Lock()
//			defer mu.Unlock()
//			totalTokens += usage.TotalTokens
//		}),
//	)
func WithUsageHandler(handler func(*UsageMetadata)) AgentOption {
	return usageHandlerOption{handler: handler}
}
