// Package ai provides AI agent middleware for creating intelligent conversational
// agents with optional tool-calling capabilities. It supports both simple chat
// interactions and complex multi-step workflows with external tool integration.
package ai

import (
	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// defaultMaxIterations caps LLM<->tool round trips when AgentOptions.MaxIterations is unset.
const defaultMaxIterations = 5

// Agent creates an AI agent handler with optional configuration.
//
// Input: string prompt/query
// Output: string AI response
// Behavior: BUFFERED - reads entire input for processing
//
// Creates an intelligent agent that can chat or use tools. Without tools,
// provides direct chat completion. With tools, runs a multi-shot loop: the
// LLM may call tools, see their results, and call more tools or answer
// directly, until it stops requesting tools or MaxIterations is reached.
//
// Example:
//
//	// Simple chat agent
//	agent := ai.Agent(client)
//
//	// Agent with tools
//	agent := ai.Agent(client, ai.WithTools(searchTool, calcTool))
//	pipe.Use(agent)
func Agent(client Client, opts ...AgentOption) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Build options
		agentOpts := &AgentOptions{}
		for _, opt := range opts {
			opt.Apply(agentOpts)
		}

		// Determine behavior based on options
		if len(agentOpts.Tools) > 0 {
			// Tool-calling agent behavior
			return runToolCallingAgent(client, agentOpts, r, w)
		}
		// Simple chat behavior
		return client.Chat(r, w, agentOpts)
	})
}

// runToolCallingAgent runs the multi-shot LLM<->tool loop: call the LLM,
// execute any requested tools, feed results back as history, and repeat
// until the LLM stops requesting tools or MaxIterations is reached.
func runToolCallingAgent(client Client, agentOpts *AgentOptions, r *calque.Request, w *calque.Response) error {
	// Use default tools config if none provided
	if agentOpts.ToolsConfig == nil {
		defaultConfig := tools.Config{
			MaxConcurrentTools:    0, // No limit
			IncludeOriginalOutput: false,
		}
		agentOpts.ToolsConfig = &defaultConfig
	}

	// Determine which formatter to use
	formatter := agentOpts.ToolResultFormatter
	if formatter == nil {
		formatter = passThroughToolResultFormatter
	}

	// Determine which client to use for tool formatting
	formatterClient := agentOpts.ToolFormatterClient
	if formatterClient == nil {
		formatterClient = client
	}

	var input []byte
	if err := calque.Read(r, &input); err != nil {
		return err
	}

	maxIterations := agentOpts.MaxIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}

	final, err := runAgentLoop(formatterClient, agentOpts, r, string(input), maxIterations)
	if err != nil {
		return calque.WrapErr(r.Context, err, "agent failed")
	}

	req := calque.NewRequest(r.Context, calque.NewReader(final))
	return formatter(formatterClient, input).ServeFlow(req, w)
}

// runAgentLoop drives the LLM<->tool round trips and returns the final
// assistant response bytes once the model stops requesting tools (or the
// iteration cap forces a final answer). client is the resolved
// formatterClient - the same client used for every turn, including the
// forced final answer, so a caller-configured formatter client is honored
// throughout the loop rather than only for the closing formatter call.
//
// maxIterations is the true cap on total LLM calls: the final iteration
// always omits Tools, so a model that would otherwise keep requesting tools
// is forced to answer directly instead of the loop making one more
// uncounted call beyond the configured budget.
func runAgentLoop(client Client, agentOpts *AgentOptions, r *calque.Request, input string, maxIterations int) ([]byte, error) {
	history := []Message{{Role: RoleUser, Content: input, Multimodal: agentOpts.MultimodalData}}
	toolList := agentOpts.Tools

	for i := range maxIterations {
		lastIteration := i == maxIterations-1

		// Clone agentOpts per turn so every current and future field is
		// carried through by default; only History (and Tools/ToolsConfig/
		// MultimodalData on the final iteration) are overridden explicitly.
		turnOpts := *agentOpts
		turnOpts.History = history
		if lastIteration {
			turnOpts.Tools = nil
			turnOpts.ToolsConfig = nil
			turnOpts.MultimodalData = nil
		}

		response, err := callChat(client, r, &turnOpts)
		if err != nil {
			return nil, err
		}

		if lastIteration || !tools.HasToolCalls(response) {
			return response, nil
		}

		calls := tools.ParseToolCalls(response)
		// Content is intentionally left empty here: the shared tool_calls
		// JSON convention every Client emits (see e.g. openai.go's
		// writeOpenAIToolCalls) has no field for prose accompanying a tool
		// call, so there is currently nothing to preserve. If a provider
		// ever needs to return reasoning text alongside tool_calls, the
		// wire convention itself needs a "content" field before this can
		// round-trip it.
		history = append(history, Message{Role: RoleAssistant, ToolCalls: calls})

		results := tools.ExecuteToolCalls(r.Context, toolList, calls, *agentOpts.ToolsConfig)
		for _, result := range results {
			history = append(history, Message{
				Role:       RoleTool,
				ToolCallID: result.ToolCall.ID,
				Content:    toolResultContent(result),
			})
		}
	}

	// Unreachable: the loop always returns on its last iteration above.
	return nil, calque.NewErr(r.Context, "agent loop exited without a response")
}

// toolResultContent renders a ToolResult as the content of a tool-role
// message, surfacing errors and input-required questions to the model
// instead of aborting the loop.
func toolResultContent(result tools.ToolResult) string {
	switch {
	case result.InputRequired != nil:
		return "Input required: " + result.InputRequired.Question
	case result.Error != "":
		return result.Error
	default:
		return string(result.Result)
	}
}

// callChat runs one LLM call against History and returns the raw response bytes.
// The request body is unused - AgentOptions.History carries the conversation.
func callChat(client Client, r *calque.Request, opts *AgentOptions) ([]byte, error) {
	req := calque.NewRequest(r.Context, calque.NewReader([]byte{}))
	out := calque.NewWriter[[]byte]()
	res := calque.NewResponse(out)
	if err := client.Chat(req, res, opts); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// passThroughToolResultFormatter is the default ToolResultFormatterFunc: the
// final LLM response already reflects the model's own natural continuation
// of the conversation history, so no extra synthesis call is needed.
func passThroughToolResultFormatter(_ Client, _ []byte) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var final []byte
		if err := calque.Read(r, &final); err != nil {
			return err
		}
		return calque.Write(w, final)
	})
}
