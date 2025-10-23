package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/calque-ai/go-calque/pkg/calque"
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

// RawToolOutput represents the raw JSON output structure when RawOutput is enabled
type RawToolOutput struct {
	OriginalOutput string       `json:"original_output,omitempty"`
	Results        []ToolResult `json:"results"`
}

// Config allows configuring the Execute middleware behavior
type Config struct {
	// MaxConcurrentTools - maximum number of tools to execute concurrently (0 = no limit)
	MaxConcurrentTools int
	// IncludeOriginalOutput - if true, includes original LLM output in results
	IncludeOriginalOutput bool
	// RawOutput - if true, returns JSON-marshaled results instead of formatted text
	RawOutput bool
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
//	flow.Use(tools.Registry(calc, search)).
//	     Use(llm.Chat(provider)).
//	     Use(detector)
func Execute() calque.Handler {
	return ExecuteWithOptions(Config{
		MaxConcurrentTools:    0, // No limit
		IncludeOriginalOutput: false,
	})
}

// ExecuteWithOptions creates an Execute middleware with custom configuration
// This assumes tool calls are present in the input and will error if none are found
func ExecuteWithOptions(config Config) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		tools := GetTools(r.Context)
		if len(tools) == 0 {
			return fmt.Errorf("no tools available in context")
		}

		// Read all input - we assume tools are present so no streaming needed
		var inputBytes []byte
		err := calque.Read(r, &inputBytes)

		if err != nil {
			return err
		}

		// Parse and execute tools directly from input bytes
		return executeFromBytes(r.Context, inputBytes, w.Data, tools, config)
	})
}

// executeFromBytes executes tools directly from input bytes
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

	// Handle errors always fail on tool execution errors
	if hasErrors {
		return fmt.Errorf("tool execution failed: %s", firstError)
	}

	// Format results based on configuration
	var output []byte

	// Handle raw JSON output
	if config.RawOutput {
		var marshalErr error
		output, marshalErr = formatRawOutput(results, inputBytes, config.IncludeOriginalOutput)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal tool results: %w", marshalErr)
		}
		_, writeErr := w.Write(output)
		return writeErr
	}

	// Handle formatted text output
	var formatted string
	if config.IncludeOriginalOutput {
		formatted = formatToolResultsWithOriginal(results, inputBytes)
	} else {
		formatted = formatToolResults(results, inputBytes)
	}
	output = []byte(formatted)

	_, writeErr := w.Write(output)
	return writeErr
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
	if len(toolCalls) == 1 { // Single tool call execute directly
		return []ToolResult{executeToolCall(ctx, tools, toolCalls[0])}
	}

	results := make([]ToolResult, len(toolCalls))

	// Determine worker count
	workers := len(toolCalls) // unlimited max concurrency
	if config.MaxConcurrentTools > 0 && config.MaxConcurrentTools < workers {
		workers = config.MaxConcurrentTools
	}

	jobs := make(chan int, len(toolCalls))
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = executeToolCall(ctx, tools, toolCalls[i])
			}
		}()
	}

	// Send jobs
	for i := range toolCalls {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	return results
}

// executeToolCall executes a single tool call
func executeToolCall(ctx context.Context, tools []Tool, toolCall ToolCall) ToolResult {
	// If the tool call already has an error (e.g., from parsing), return it immediately
	if toolCall.Error != "" {
		return ToolResult{
			ToolCall: toolCall,
			Error:    toolCall.Error,
		}
	}

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
	req := calque.NewRequest(ctx, args)
	res := calque.NewResponse(&result)

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

// formatRawOutput returns JSON-marshaled tool results
func formatRawOutput(results []ToolResult, inputBytes []byte, includeOriginal bool) ([]byte, error) {
	if includeOriginal {
		rawOutput := RawToolOutput{
			OriginalOutput: string(inputBytes),
			Results:        results,
		}
		return json.Marshal(rawOutput)
	}
	return json.Marshal(results)
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
