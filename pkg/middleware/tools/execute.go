package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/pkg/core"
)

// ToolCall represents a parsed tool call from LLM output
type ToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
	ID        string `json:"id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// OpenAIToolCall represents a tool call in OpenAI format
type OpenAIToolCall struct {
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolCall ToolCall `json:"tool_call"`
	Result   []byte   `json:"result"`
	Error    string   `json:"error,omitempty"`
}

// Config allows configuring the Execute middleware behavior
type Config struct {
	// PassThroughOnError - if true, returns original LLM output when tool execution fails
	PassThroughOnError bool
	// MaxConcurrentTools - maximum number of tools to execute concurrently (0 = no limit)
	MaxConcurrentTools int
	// IncludeOriginalOutput - if true, includes original LLM output in results
	IncludeOriginalOutput bool
}

// Execute parses LLM output for tool calls and executes them using tools from Registry.
//
// Input: LLM output containing tool calls (assumes tool calls are present)
// Output: formatted tool results
// Behavior: BUFFERED - reads entire input to parse and execute tools
//
// This middleware assumes tool calls are present and will error if none are found.
// Use tools.Detect() to conditionally route inputs with/without tool calls.
//
// Example:
//
//	detector := tools.Detect(tools.Execute(), flow.PassThrough())
//	pipe.Use(tools.Registry(calc, search)).
//	     Use(llm.Chat(provider)).
//	     Use(detector)
func Execute() core.Handler {
	return ExecuteWithOptions(Config{
		PassThroughOnError:    false,
		MaxConcurrentTools:    0, // No limit
		IncludeOriginalOutput: false,
	})
}

// ExecuteWithOptions creates an Execute middleware with custom configuration
// This assumes tool calls are present in the input and will error if none are found
func ExecuteWithOptions(config Config) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		tools := GetTools(r.Context)
		if len(tools) == 0 {
			return fmt.Errorf("no tools available in context")
		}

		// Read all input - we assume tools are present so no streaming needed
		inputBytes, err := io.ReadAll(r.Data)
		if err != nil {
			return err
		}

		// Parse and execute tools directly from input bytes
		return executeFromBytes(r.Context, inputBytes, w.Data, tools, config)
	})
}

// executeFromBytes executes tools directly from input bytes (simplified version for Execute)
func executeFromBytes(ctx context.Context, inputBytes []byte, w io.Writer, tools []Tool, config Config) error {
	// Parse tool calls from input
	toolCalls := parseToolCalls(inputBytes)

	// Error if no tools found since Execute assumes tools are present
	if len(toolCalls) == 0 {
		return fmt.Errorf("no tool calls found in input - use tools.Detect() to handle inputs without tools")
	}

	// Execute tool calls with configuration
	results := executeToolCallsWithConfig(ctx, tools, toolCalls, config)

	// Check for errors in tool execution
	hasErrors := false
	var firstError string
	for _, result := range results {
		if result.Error != "" {
			hasErrors = true
			if firstError == "" {
				firstError = result.Error
			}
		}
	}

	// Handle errors based on configuration
	if hasErrors {
		if config.PassThroughOnError {
			// Pass through original LLM output on error
			_, err := w.Write(inputBytes)
			return err
		} else {
			// Return error when PassThroughOnError is false
			return fmt.Errorf("tool execution failed: %s", firstError)
		}
	}

	// Format results based on configuration
	var output string
	if config.IncludeOriginalOutput {
		output = formatToolResultsWithOriginal(results, inputBytes)
	} else {
		output = formatToolResults(results, inputBytes)
	}

	_, err := w.Write([]byte(output))
	return err
}

// ParseToolCalls extracts tool calls from LLM output using JSON parsing (OpenAI standard)
func parseToolCalls(output []byte) []ToolCall {
	// Only JSON format supported (OpenAI standard)
	return parseJSONToolCalls(output)
}

// parseErrorToolCall creates a standardized parse error ToolCall
func parseErrorToolCall(output []byte, errorMsg string) []ToolCall {
	return []ToolCall{{
		Name:      "_parse_error",
		Arguments: string(output), // Preserve original malformed content
		Error:     errorMsg,
	}}
}

// parseJSONToolCalls parses JSON format tool calls (OpenAI standard)
// This function is only called when hasToolCalls() detected tool patterns,
// so we can safely return parse errors for malformed JSON.
func parseJSONToolCalls(output []byte) []ToolCall {
	var result struct {
		ToolCalls []OpenAIToolCall `json:"tool_calls"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return parseErrorToolCall(output, fmt.Sprintf("Failed to parse tool call JSON: %v", err))
	}

	// parses but toolCalls is empty
	if len(result.ToolCalls) == 0 {
		return parseErrorToolCall(output, "JSON parsed successfully but contains no tool calls")
	}

	// Convert OpenAI format to our internal format and validate structure
	toolCalls := make([]ToolCall, len(result.ToolCalls))
	for i, openaiCall := range result.ToolCalls {
		// Validate that the tool call has the expected OpenAI structure
		if openaiCall.Function.Name == "" {
			return parseErrorToolCall(output, fmt.Sprintf("Tool call %d missing function name - invalid OpenAI format", i))
		}

		toolCalls[i] = ToolCall{
			Name:      openaiCall.Function.Name,
			Arguments: openaiCall.Function.Arguments,
			ID:        fmt.Sprintf("call_%d", i),
		}
	}

	return toolCalls
}

// executeToolCallsWithConfig executes multiple tool calls with configuration
func executeToolCallsWithConfig(ctx context.Context, tools []Tool, toolCalls []ToolCall, config Config) []ToolResult {
	if config.MaxConcurrentTools <= 0 || config.MaxConcurrentTools >= len(toolCalls) {
		// Execute all concurrently or sequentially if no limit
		results := make([]ToolResult, len(toolCalls))
		for i, toolCall := range toolCalls {
			results[i] = executeToolCall(ctx, tools, toolCall)
		}
		return results
	}

	// Execute with concurrency limit
	results := make([]ToolResult, len(toolCalls))
	semaphore := make(chan struct{}, config.MaxConcurrentTools)

	for i, toolCall := range toolCalls {
		semaphore <- struct{}{} // Acquire
		go func(index int, call ToolCall) {
			defer func() { <-semaphore }() // Release
			results[index] = executeToolCall(ctx, tools, call)
		}(i, toolCall)
	}

	// Wait for all to complete
	for i := 0; i < config.MaxConcurrentTools; i++ {
		semaphore <- struct{}{}
	}

	return results
}

// executeToolCall executes a single tool call
func executeToolCall(ctx context.Context, tools []Tool, toolCall ToolCall) ToolResult {
	// Find the tool
	var tool Tool
	for _, t := range tools {
		if t.Name() == toolCall.Name {
			tool = t
			break
		}
	}

	if tool == nil {
		return ToolResult{
			ToolCall: toolCall,
			Error:    fmt.Sprintf("Tool '%s' not found", toolCall.Name),
		}
	}

	// Execute the tool with panic recovery
	var result bytes.Buffer
	args := strings.NewReader(toolCall.Arguments)
	req := core.NewRequest(ctx, args)
	res := core.NewResponse(&result)

	// Execute tool with panic recovery
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("tool panicked: %v", r)
			}
		}()
		err = tool.ServeFlow(req, res)
	}()

	if err != nil {
		return ToolResult{
			ToolCall: toolCall,
			Error:    fmt.Sprintf("Tool execution error: %v", err),
		}
	}

	return ToolResult{
		ToolCall: toolCall,
		Result:   result.Bytes(),
	}
}

// formatToolResults formats tool execution results for output
func formatToolResults(results []ToolResult, originalOutput []byte) string {
	if len(results) == 0 {
		return string(originalOutput)
	}

	var output strings.Builder
	output.WriteString("Tool execution results:\n\n")

	for i, result := range results {
		output.WriteString(fmt.Sprintf("Tool %d: %s\n", i+1, result.ToolCall.Name))
		if result.ToolCall.Arguments != "" {
			output.WriteString(fmt.Sprintf("Arguments: %s\n", result.ToolCall.Arguments))
		}

		if result.Error != "" {
			output.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		} else {
			output.WriteString(fmt.Sprintf("Result: %s\n", string(result.Result)))
		}
		output.WriteString("\n")
	}

	return output.String()
}

// formatToolResultsWithOriginal includes the original LLM output
func formatToolResultsWithOriginal(results []ToolResult, originalOutput []byte) string {
	var output strings.Builder

	output.WriteString("Original LLM Output:\n")
	output.WriteString(string(originalOutput))
	output.WriteString("\n\n")

	output.WriteString(formatToolResults(results, originalOutput))

	return output.String()
}

// hasToolCalls detects if the initial chunk contains tool call patterns
func hasToolCalls(data []byte) bool {
	content := string(data)

	// JSON format (OpenAI standard) - high confidence pattern
	return strings.Contains(content, `{"tool_calls":`)
}
