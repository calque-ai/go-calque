package ai

import "github.com/calque-ai/calque-pipe/middleware/tools"

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

// WithSchema adds a response schema to the agent
func WithSchema(schema *ResponseFormat) AgentOption {
	return schemaOption{schema: schema}
}

// WithToolsConfig configures tool behavior
func WithToolsConfig(config tools.Config) AgentOption {
	return toolsConfigOption{config: &config}
}
