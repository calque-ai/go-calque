package ai

import (
	"github.com/calque-ai/calque-pipe/pkg/middleware/tools"
	"github.com/invopop/jsonschema"
)

// AgentOptions holds all configuration for an AI agent request
type AgentOptions struct {
	Schema      *ResponseFormat
	Tools       []tools.Tool
	ToolsConfig *tools.Config
}

// AgentOption interface for functional options pattern
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

// WithTools adds tools to the agent
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

// WithToolsConfig configures tool behavior
func WithToolsConfig(config tools.Config) AgentOption {
	return toolsConfigOption{config: &config}
}
