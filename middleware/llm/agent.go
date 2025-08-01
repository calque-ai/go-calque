package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

// agentToolsKey is used to store tools in context for agent functions
type agentToolsKey struct{}

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
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Apply timeout if configured
		if config.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, config.Timeout)
			defer cancel()
		}

		// Build the iteration handler using framework components
		iterationHandler := buildIterationHandler(provider, config, toolList...)

		// Execute the iteration handler
		return iterationHandler.ServeFlow(ctx, r, w)
	})
}

// buildIterationHandler creates the iterative tool-calling handler using the framework
func buildIterationHandler(provider LLMProvider, config AgentConfig, toolList ...tools.Tool) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Read initial input
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		currentInput := input
		fmt.Printf("Run Iteration handler \n")
		for iteration := 0; iteration < config.MaxIterations; iteration++ {
			fmt.Printf("Begin Iteration %d\n", iteration)
			// Build the single iteration pipeline with logging
			pipe := core.New()
			pipe.Use(flow.Logger(fmt.Sprintf("Iteration-%d", iteration), 200))
			pipe.Use(tools.Registry(toolList...)) // Register tools for this iteration
			pipe.Use(flow.Logger("After-Registry", 200))
			pipe.Use(addToolInformation()) // Add tool schema to input
			pipe.Use(flow.Logger("After-AddToolInfo", 200))
			pipe.Use(Chat(provider))
			pipe.Use(flow.Logger("After-Chat", 200))
			pipe.Use(tools.Detect(
				tools.ExecuteWithOptions(config.ExecuteConfig), // Execute tools if found
				terminateIteration(),                           // No tools - terminate
			))
			pipe.Use(flow.Logger("After-Detect", 200))

			// Execute single iteration
			var output []byte
			if err := pipe.Run(ctx, currentInput, &output); err != nil {
				return fmt.Errorf("iteration %d failed: %w", iteration, err)
			}

			// Debug logging
			outputStr := string(output)
			fmt.Printf("Iteration %d output length: %d\n", iteration, len(output))
			fmt.Printf("Iteration %d output preview: %.100s...\n", iteration, outputStr)
			fmt.Printf("Contains tool results: %v\n", containsToolResults(outputStr))

			// Check if we should continue iterating
			if !containsToolResults(outputStr) {
				// No tools executed - write final output and stop
				fmt.Printf("No tools detected, terminating at iteration %d\n", iteration)
				_, err := w.Write(output)
				return err
			}

			// Tools executed - continue with results as next input
			fmt.Printf("Tools detected, continuing to iteration %d\n", iteration+1)
			currentInput = output
		}

		// Max iterations reached - return error
		return fmt.Errorf("agent reached maximum iterations (%d) without completing", config.MaxIterations)
	})
}

// addToolInformation adds tool schema to the input (replaces formatInputWithTools)
func addToolInformation() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Read input
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		// Get tools from context
		toolList := tools.GetTools(ctx)
		if len(toolList) == 0 {
			// No tools - pass through unchanged
			_, err := w.Write(input)
			return err
		}

		// Add tool schema using OpenAI format
		toolSchema := tools.FormatToolsAsOpenAI(toolList)
		result := make([]byte, len(input)+len(toolSchema))
		copy(result, input)
		copy(result[len(input):], []byte(toolSchema))

		_, err = w.Write(result)
		return err
	})
}

// terminateIteration creates a handler that indicates iteration should stop
func terminateIteration() core.Handler {
	return flow.PassThrough() // Just pass through - no special termination needed
}

// func executeTool(ctx context.Context, provider LLMProvider, initialInput []byte, config AgentConfig, w io.Writer) error {

// 	pipe := core.New()
// 	pipe.Use(tools.Registry(calc, search))

// }

// executeToolLoop handles the iterative tool-calling process using the new tools.Detect pattern
func executeToolLoop(ctx context.Context, provider LLMProvider, initialInput []byte, config AgentConfig, w io.Writer) error {
	currentInput := initialInput

	for iteration := 0; iteration < config.MaxIterations; iteration++ {
		// Format input with tool information
		formattedInput, err := formatInputWithTools(ctx, currentInput)
		if err != nil {
			return fmt.Errorf("failed to format input: %w", err)
		}

		// Create pipe to connect LLM output to tool detection
		llmReader, llmWriter := io.Pipe()
		var llmErr error

		// Start LLM call in goroutine to stream output
		go func() {
			defer llmWriter.Close()
			llmErr = callLLM(ctx, provider, formattedInput, llmWriter)
		}()

		// Create the detection handler using the new pattern
		var toolOutput bytes.Buffer
		var finalOutput io.Writer = w

		// For intermediate iterations, capture output to continue iteration
		if iteration < config.MaxIterations-1 {
			finalOutput = &toolOutput
		}

		// Create detection handler: if tools detected -> Execute, else -> PassThrough to output
		detectHandler := tools.Detect(
			tools.ExecuteWithOptions(config.ExecuteConfig), // Handle tool calls
			flow.PassThrough(), // No tools - pass through
		)

		// Execute detection and routing
		if err := detectHandler.ServeFlow(ctx, llmReader, finalOutput); err != nil {
			return fmt.Errorf("tool detection/execution failed: %w", err)
		}

		// Check for LLM errors
		if llmErr != nil {
			return fmt.Errorf("LLM call failed: %w", llmErr)
		}

		// If this was the final iteration or we wrote directly to output, we're done
		if finalOutput == w {
			return nil
		}

		// Check if we need to continue - simple heuristic based on output content
		outputStr := toolOutput.String()
		if !containsToolResults(outputStr) {
			// No tools were executed - write final output and finish
			_, err := w.Write(toolOutput.Bytes())
			return err
		}

		// Tools were executed - continue with tool results as next input
		currentInput = []byte(outputStr)
	}

	// Max iterations reached
	return fmt.Errorf("agent reached maximum iterations (%d) without completing", config.MaxIterations)
}

// getAgentTools retrieves tools from the agent context
func getAgentTools(ctx context.Context) []tools.Tool {
	if toolList, ok := ctx.Value(agentToolsKey{}).([]tools.Tool); ok {
		return toolList
	}
	return nil
}

// formatInputWithTools formats the input with tool information using OpenAI standard
func formatInputWithTools(ctx context.Context, input []byte) ([]byte, error) {
	toolList := getAgentTools(ctx)
	if len(toolList) == 0 {
		return input, nil
	}

	// Use OpenAI function calling format (no extra instructional text needed)
	toolSchema := tools.FormatToolsAsOpenAI(toolList)
	result := make([]byte, len(input)+len(toolSchema))
	copy(result, input)
	copy(result[len(input):], []byte(toolSchema))
	return result, nil
}

// callLLM makes a call to the LLM provider and streams response to writer
func callLLM(ctx context.Context, provider LLMProvider, input []byte, w io.Writer) error {
	return provider.Chat(ctx, bytes.NewReader(input), w)
}

// containsToolResults checks if the output contains tool execution results
// This is a simple heuristic to determine if tools were executed
func containsToolResults(output string) bool {
	return strings.Contains(output, "Tool execution results:") ||
		strings.Contains(output, "Tool 1:") ||
		strings.Contains(output, "Result:")
}
