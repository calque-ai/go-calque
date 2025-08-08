package ai

import (
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ctrl"
	"github.com/calque-ai/calque-pipe/pkg/middleware/tools"
)

// Agent creates an AI agent handler with optional configuration
func Agent(client Client, opts ...AgentOption) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Build options
		agentOpts := &AgentOptions{}
		for _, opt := range opts {
			opt.Apply(agentOpts)
		}

		// Determine behavior based on options
		if len(agentOpts.Tools) > 0 {
			// Tool-calling agent behavior (full agent loop)
			return runToolCallingAgent(client, agentOpts, r, w)
		} else {
			// Simple chat behavior
			return client.Chat(r, w, agentOpts)
		}
	})
}

// runToolCallingAgent implements the full agent loop with tools
func runToolCallingAgent(client Client, agentOpts *AgentOptions, r *calque.Request, w *calque.Response) error {
	// Use default tools config if none provided
	if agentOpts.ToolsConfig == nil {
		defaultConfig := tools.Config{
			PassThroughOnError:    false,
			MaxConcurrentTools:    0, // No limit
			IncludeOriginalOutput: false,
		}
		agentOpts.ToolsConfig = &defaultConfig
	}

	input, err := io.ReadAll(r.Data)
	if err != nil {
		return err
	}

	flow := calque.Flow()

	// Chain: Registry → AddToolInfo → LLM → Detect → [Execute + Synthesize] OR PassThrough
	flow.Use(ctrl.Chain(
		tools.Registry(agentOpts.Tools...),   // Register tools in context
		addToolInformation(),                 // Add tool schema using tools from context
		clientChatHandler(client, agentOpts), // Direct LLM call
		tools.Detect(
			// If tools detected → Execute tools, then synthesize final answer
			ctrl.Chain(
				tools.ExecuteWithOptions(*agentOpts.ToolsConfig), // Execute tools
				synthesizeFinalAnswer(client, input),             // Second LLM call with original input + results
			),
			// No tools detected → just pass through the LLM response
			ctrl.PassThrough(),
		),
	))

	// Execute the flow
	var output []byte
	if err := flow.Run(r.Context, input, &output); err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Write final result
	_, err = w.Data.Write(output)
	return err
}

// clientChatHandler creates a handler that calls client.Chat directly
func clientChatHandler(client Client, agentOpts *AgentOptions) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		return client.Chat(r, w, agentOpts)
	})
}

// addToolInformation adds tool schema to the input (replaces formatInputWithTools)
func addToolInformation() calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Read input
		input, err := io.ReadAll(r.Data)
		if err != nil {
			return err
		}

		// Get tools from context
		toolList := tools.GetTools(r.Context)
		if len(toolList) == 0 {
			// No tools - pass through unchanged
			_, err := w.Data.Write(input)
			return err
		}

		// Add tool schema using OpenAI format
		toolSchema := tools.FormatToolsAsOpenAI(toolList)
		result := make([]byte, len(input)+len(toolSchema))
		copy(result, input)
		copy(result[len(input):], []byte(toolSchema))

		_, err = w.Data.Write(result)
		return err
	})
}

// synthesizeFinalAnswer creates a handler that makes a second LLM call to synthesize a final answer
// from the original question and tool execution results
func synthesizeFinalAnswer(client Client, originalInput []byte) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		toolResults, err := io.ReadAll(r.Data)
		if err != nil {
			return err
		}

		// Create synthesis prompt combining original question with tool results
		synthesisPrompt := fmt.Sprintf(`Original question: %s

Tool execution results:
%s

Please provide a complete answer to the original question using the tool results above. Be concise and direct.`,
			string(originalInput), string(toolResults))

		// Make LLM call without tools for synthesis
		req := calque.NewRequest(r.Context, strings.NewReader(synthesisPrompt))
		return client.Chat(req, w, &AgentOptions{})
	})
}
