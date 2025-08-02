package llm

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

// AgentConfig configures the behavior of tool-enabled agents
type AgentConfig struct {
	// MaxIterations - maximum number of tool-calling iterations (default: 5)
	MaxIterations int
	// ExecuteConfig - configuration for tool execution
	ExecuteConfig tools.ExecuteConfig
	// Timeout - maximum time for the entire agent execution (default: no timeout)
	Timeout time.Duration
}

// DefaultAgentConfig returns a sensible default configuration
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		MaxIterations: 5,
		ExecuteConfig: tools.ExecuteConfig{
			PassThroughOnError:    false,
			MaxConcurrentTools:    0, // No limit
			IncludeOriginalOutput: false,
		},
		Timeout: 0, // No timeout
	}
}

// Agent creates a complete tool-calling agent that handles the entire tool loop.
// This is the simplest way to create a fully functional tool-enabled AI agent.
//
// The agent automatically:
// 1. Registers tools in context
// 2. Formats tool information for the LLM
// 3. Sends requests to the LLM
// 4. Executes any tool calls
// 5. Sends tool results back to the LLM
// 6. Repeats until no more tool calls or max iterations reached
//
// Example:
//
//	calc := tools.Simple("calculator", "Evaluate expressions", func(expr string) string { return evaluate(expr) })
//	search := tools.Simple("search", "Search the web", func(query string) string { return search(query) })
//
//	agent := llm.Agent(geminiProvider, calc, search)
//
//	var result string
//	err := pipe.Use(agent).Run(ctx, "What's 25 * 4 and search for Go tutorials", &result)
func Agent(provider LLMProvider, tools ...tools.Tool) core.Handler {
	return AgentWithConfig(provider, DefaultAgentConfig(), tools...)
}

// AgentWithConfig creates a tool-calling agent with custom configuration
func AgentWithConfig(provider LLMProvider, config AgentConfig, toolList ...tools.Tool) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		// Apply timeout if configured
		if config.Timeout > 0 {
			var cancel context.CancelFunc
			r.Context, cancel = context.WithTimeout(r.Context, config.Timeout)
			defer cancel()
		}

		// Build the iteration handler using framework components
		iterationHandler := buildIterationHandler(provider, config, toolList...)

		// Execute the iteration handler
		return iterationHandler.ServeFlow(r, w)
	})
}

// buildIterationHandler creates a single-pass tool-calling handler with synthesis
func buildIterationHandler(provider LLMProvider, config AgentConfig, toolList ...tools.Tool) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		// Read initial input
		input, err := io.ReadAll(r.Data)
		if err != nil {
			return err
		}

		// Build the single-pass pipeline
		pipe := core.New()
		// pipe.Use(flow.Logger("Agent", 200))

		// Chain: Registry → AddToolInfo → Chat → Detect → [Execute + Synthesize] OR PassThrough
		pipe.Use(flow.Chain(
			tools.Registry(toolList...), // Register tools in context
			// flow.Logger("After-Registry", 200),
			addToolInformation(), // Add tool schema using tools from context
			// flow.Logger("After-AddToolInfo", 800),
			ChatWithTools(provider, toolList...), // LLM call with tools
			// flow.Logger("After-Chat", 800),
			tools.Detect(
				// If tools detected -> Execute tools, then synthesize final answer
				flow.Chain(
					tools.ExecuteWithOptions(config.ExecuteConfig), // Execute tools
					// flow.Logger("After-Execute", 800),
					synthesizeFinalAnswer(provider, input), // Second LLM call with original input + results
				),
				// No tools detected -> just pass through the LLM response
				flow.PassThrough(),
			),
		))

		// pipe.Use(flow.Logger("After-Synthesis", 200))

		// Execute the pipeline
		var output []byte
		if err := pipe.Run(r.Context, input, &output); err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		// Write final result
		_, err = w.Data.Write(output)
		return err
	})
}

// addToolInformation adds tool schema to the input (replaces formatInputWithTools)
func addToolInformation() core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
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
func synthesizeFinalAnswer(provider LLMProvider, originalInput []byte) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		// Read tool execution results
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
		req := core.NewRequest(r.Context, strings.NewReader(synthesisPrompt))
		return provider.Chat(req, w)
	})
}
