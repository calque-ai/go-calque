package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/core"
)

// LLMProvider interface for agent functions (matches the LLM middleware interface)
type LLMProvider interface {
	Chat(ctx context.Context, input io.Reader, output io.Writer) error
}

// AgentConfig configures the behavior of tool-enabled agents
type AgentConfig struct {
	// MaxIterations - maximum number of tool-calling iterations (default: 5)
	MaxIterations int
	// FormatStyle - how to format tool information for the LLM (default: FormatStyleDetailed)
	FormatStyle FormatStyle
	// ExecuteConfig - configuration for tool execution
	ExecuteConfig ExecuteConfig
	// Timeout - maximum time for the entire agent execution (default: no timeout)
	Timeout time.Duration
}

// DefaultAgentConfig returns a sensible default configuration
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		MaxIterations: 5,
		FormatStyle:   FormatStyleDetailed,
		ExecuteConfig: ExecuteConfig{
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
//	calc := tools.Quick("calculator", func(expr string) string { return evaluate(expr) })
//	search := tools.Quick("search", func(query string) string { return search(query) })
//
//	agent := tools.Agent(geminiProvider, calc, search)
//
//	var result string
//	err := pipe.Use(agent).Run(ctx, "What's 25 * 4 and search for Go tutorials", &result)
func Agent(provider LLMProvider, tools ...Tool) core.Handler {
	return AgentWithConfig(provider, DefaultAgentConfig(), tools...)
}

// AgentWithConfig creates a tool-calling agent with custom configuration
func AgentWithConfig(provider LLMProvider, config AgentConfig, tools ...Tool) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Apply timeout if configured
		if config.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, config.Timeout)
			defer cancel()
		}

		// Read initial input
		var input string
		if err := core.Read(r, &input); err != nil {
			return err
		}

		// Store tools in context
		ctx = context.WithValue(ctx, toolsContextKey{}, tools)

		// Create the tool loop
		return executeToolLoop(ctx, provider, input, config, w)
	})
}

// executeToolLoop handles the iterative tool-calling process
func executeToolLoop(ctx context.Context, provider LLMProvider, initialInput string, config AgentConfig, w io.Writer) error {
	currentInput := initialInput

	for iteration := 0; iteration < config.MaxIterations; iteration++ {
		// Format input with tool information
		formattedInput, err := formatInputWithTools(ctx, currentInput, config.FormatStyle)
		if err != nil {
			return fmt.Errorf("failed to format input: %w", err)
		}

		// Send to LLM
		llmResponse, err := callLLM(ctx, provider, formattedInput)
		if err != nil {
			return fmt.Errorf("LLM call failed: %w", err)
		}

		// Check if LLM wants to use tools
		toolCalls := parseToolCalls(llmResponse)
		if len(toolCalls) == 0 {
			// No tool calls - we're done
			return core.Write(w, llmResponse)
		}

		// Execute tool calls
		toolResults, err := executeToolCallsForAgent(ctx, toolCalls, config.ExecuteConfig)
		if err != nil {
			if config.ExecuteConfig.PassThroughOnError {
				return core.Write(w, llmResponse)
			}
			return fmt.Errorf("tool execution failed: %w", err)
		}

		// Prepare next iteration input with tool results
		currentInput = formatToolResultsForNextIteration(toolResults, llmResponse)
	}

	// Max iterations reached
	return fmt.Errorf("agent reached maximum iterations (%d) without completing", config.MaxIterations)
}

// ToolLoop creates a middleware that handles tool calling in a loop until completion.
// Unlike Agent, this can be composed with other middleware in a pipeline.
//
// Example:
//
//	pipeline := core.New().
//	    Use(tools.Registry(calc, search)).
//	    Use(tools.Format(tools.FormatStyleDetailed)).
//	    Use(tools.ToolLoop(provider, 3)) // Max 3 iterations
func ToolLoop(provider LLMProvider, maxIterations int) core.Handler {
	config := DefaultAgentConfig()
	config.MaxIterations = maxIterations
	config.FormatStyle = FormatStyleSimple // Don't double-format if Format middleware already used

	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		var input string
		if err := core.Read(r, &input); err != nil {
			return err
		}

		return executeToolLoop(ctx, provider, input, config, w)
	})
}

// SingleToolExecution creates a middleware that executes tools once without looping.
// Useful when you want manual control over the tool-calling process.
//
// Example:
//
//	pipeline := core.New().
//	    Use(tools.Registry(calc, search)).
//	    Use(tools.Format(tools.FormatStyleJSON)).
//	    Use(llm.Chat(provider)).
//	    Use(tools.SingleToolExecution()) // Execute tools once, don't loop
func SingleToolExecution() core.Handler {
	return ExecuteWithOptions(ExecuteConfig{
		PassThroughOnError:    false,
		MaxConcurrentTools:    0,
		IncludeOriginalOutput: false,
	})
}

// formatInputWithTools formats the input with tool information based on style
func formatInputWithTools(ctx context.Context, input string, style FormatStyle) (string, error) {
	tools := GetTools(ctx)
	if tools == nil || len(tools) == 0 {
		return input, nil
	}

	config := FormatConfig{
		Style:           style,
		IncludeExamples: style == FormatStyleDetailed,
		Prefix:          "\n\nAvailable tools:\n",
		Suffix:          "\n\nPlease use the tools if needed to help with this request.\n",
	}

	toolInfo := formatTools(tools, config)
	return input + config.Prefix + toolInfo + config.Suffix, nil
}

// callLLM makes a call to the LLM provider
func callLLM(ctx context.Context, provider LLMProvider, input string) (string, error) {
	var response bytes.Buffer
	err := provider.Chat(ctx, strings.NewReader(input), &response)
	if err != nil {
		return "", err
	}
	return response.String(), nil
}

// executeToolCallsForAgent executes tool calls specifically for agent use
func executeToolCallsForAgent(ctx context.Context, toolCalls []ToolCall, config ExecuteConfig) ([]ToolResult, error) {
	tools := GetTools(ctx)
	if tools == nil {
		return nil, fmt.Errorf("no tools available in context")
	}

	results := make([]ToolResult, len(toolCalls))
	for i, toolCall := range toolCalls {
		result := executeToolCall(ctx, tools, toolCall)
		results[i] = result

		// If we encounter an error and PassThroughOnError is false, fail immediately
		if result.Error != "" && !config.PassThroughOnError {
			return nil, fmt.Errorf("tool execution failed: %s", result.Error)
		}
	}

	return results, nil
}

// formatToolResultsForNextIteration formats tool results for the next LLM iteration
func formatToolResultsForNextIteration(results []ToolResult, originalResponse string) string {
	var formatted strings.Builder

	formatted.WriteString("Previous response: ")
	formatted.WriteString(originalResponse)
	formatted.WriteString("\n\nTool execution results:\n")

	for i, result := range results {
		formatted.WriteString(fmt.Sprintf("\nTool %d (%s):\n", i+1, result.ToolCall.Name))
		if result.Error != "" {
			formatted.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		} else {
			formatted.WriteString(fmt.Sprintf("Result: %s\n", result.Result))
		}
	}

	formatted.WriteString("\nPlease continue your response based on these tool results.")

	return formatted.String()
}

// QuickAgent creates an agent with minimal configuration for rapid prototyping
func QuickAgent(provider LLMProvider, tools ...Tool) core.Handler {
	config := AgentConfig{
		MaxIterations: 3,                 // Fewer iterations for quick testing
		FormatStyle:   FormatStyleSimple, // Simpler format
		ExecuteConfig: ExecuteConfig{
			PassThroughOnError: true, // More forgiving
		},
	}
	return AgentWithConfig(provider, config, tools...)
}

// RobustAgent creates an agent with robust error handling and detailed formatting
func RobustAgent(provider LLMProvider, tools ...Tool) core.Handler {
	config := AgentConfig{
		MaxIterations: 10,                  // More iterations allowed
		FormatStyle:   FormatStyleDetailed, // Detailed tool descriptions
		ExecuteConfig: ExecuteConfig{
			PassThroughOnError:    false, // Strict error handling
			MaxConcurrentTools:    3,     // Limit concurrent execution
			IncludeOriginalOutput: true,  // Include context
		},
		Timeout: 2 * time.Minute, // Safety timeout
	}
	return AgentWithConfig(provider, config, tools...)
}
