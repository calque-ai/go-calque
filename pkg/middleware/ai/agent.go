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

	toolList := tools.GetTools(r.Context)
	if len(toolList) == 0 {
		toolList = agentOpts.Tools
	}

	final, err := runAgentLoop(client, agentOpts, r, string(input), toolList, maxIterations)
	if err != nil {
		return calque.WrapErr(r.Context, err, "agent failed")
	}

	req := calque.NewRequest(r.Context, calque.NewReader(final))
	return formatter(formatterClient, input).ServeFlow(req, w)
}

// runAgentLoop drives the LLM<->tool round trips and returns the final
// assistant response bytes once the model stops requesting tools (or the
// iteration cap forces a final answer).
func runAgentLoop(client Client, agentOpts *AgentOptions, r *calque.Request, input string, toolList []tools.Tool, maxIterations int) ([]byte, error) {
	history := []Message{{Role: RoleUser, Content: input}}

	for range maxIterations {
		response, err := callChat(client, r, &AgentOptions{
			History:      history,
			Tools:        toolList,
			ToolsConfig:  agentOpts.ToolsConfig,
			UsageHandler: agentOpts.UsageHandler,
		})
		if err != nil {
			return nil, err
		}

		if !tools.HasToolCalls(response) {
			return response, nil
		}

		calls := tools.ParseToolCalls(response)
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

	// MaxIterations reached with the model still mid tool-chain - force a
	// final answer instead of erroring, with no further tools available.
	return callChat(client, r, &AgentOptions{
		History:      history,
		Schema:       agentOpts.Schema,
		UsageHandler: agentOpts.UsageHandler,
	})
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
