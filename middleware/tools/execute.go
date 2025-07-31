package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
)

// ToolCall represents a parsed tool call from LLM output
type ToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
	ID        string `json:"id,omitempty"`
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolCall ToolCall `json:"tool_call"`
	Result   []byte   `json:"result"`
	Error    string   `json:"error,omitempty"`
}

// Execute parses LLM output for tool calls and executes them using tools from Registry.
//
// Input: string containing LLM output (buffered - needs full response to parse)
// Output: either original LLM output (if no tools) or formatted tool results
// Behavior: BUFFERED - must read entire LLM response to parse tool calls
//
// The middleware looks for tool calls in various formats:
// 1. JSON format: {"tool_calls": [{"name": "tool_name", "arguments": "args"}]}
// 2. Simple format: TOOL:tool_name:arguments
// 3. XML-like format: <tool name="tool_name">arguments</tool>
//
// If no tool calls are found, the original LLM output is passed through unchanged.
// If tool calls are found, they are executed and results are formatted for output.
//
// Example:
//
//	execute := tools.Execute()
//	pipe.Use(tools.Registry(calc, search)).
//	     Use(llm.Chat(provider)).
//	     Use(execute) // Executes any tool calls from LLM
func Execute() core.Handler {
	return ExecuteWithOptions(ExecuteConfig{
		PassThroughOnError:    false,
		MaxConcurrentTools:    0, // No limit
		IncludeOriginalOutput: false,
		EnableStreaming:       true,  // Use streaming by default
		StreamingBufferSize:   200,   // Default buffer size
	})
}

// ExecuteWithConfig allows configuring the Execute middleware behavior
type ExecuteConfig struct {
	// PassThroughOnError - if true, returns original LLM output when tool execution fails
	PassThroughOnError bool
	// MaxConcurrentTools - maximum number of tools to execute concurrently (0 = no limit)
	MaxConcurrentTools int
	// IncludeOriginalOutput - if true, includes original LLM output in results
	IncludeOriginalOutput bool
	// EnableStreaming - if true, uses streaming mode (buffer first ~200 bytes, then stream if no tools)
	EnableStreaming bool
	// StreamingBufferSize - size of initial buffer for streaming mode (defaults to 200)
	StreamingBufferSize int
}

// ExecuteWithOptions creates an Execute middleware with custom configuration
func ExecuteWithOptions(config ExecuteConfig) core.Handler {
	// Choose between streaming and buffering implementation
	if config.EnableStreaming {
		bufferSize := config.StreamingBufferSize
		if bufferSize <= 0 {
			bufferSize = 200 // Default buffer size
		}
		return ExecuteWithStreaming(config, bufferSize)
	}
	
	// Use traditional buffering approach
	return executeWithBuffering(config)
}

// executeWithBuffering implements the traditional full-buffering approach
func executeWithBuffering(config ExecuteConfig) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Get tools from context first
		tools := GetTools(ctx)
		if len(tools) == 0 {
			// No tools available, pass through with streaming
			_, err := io.Copy(w, r)
			return err
		}

		var llmOutput string
		if err := core.Read(r, &llmOutput); err != nil {
			return err
		}

		toolCalls := parseToolCalls(llmOutput)
		if len(toolCalls) == 0 {
			return core.Write(w, llmOutput)
		}

		// Execute tools with configuration
		results := executeToolCallsWithConfig(ctx, tools, toolCalls, config)

		// Check if we should pass through on error
		if config.PassThroughOnError {
			hasErrors := false
			for _, result := range results {
				if result.Error != "" {
					hasErrors = true
					break
				}
			}
			if hasErrors {
				return core.Write(w, llmOutput)
			}
		}

		// Format results
		var output string
		if config.IncludeOriginalOutput {
			output = formatToolResultsWithOriginal(results, llmOutput)
		} else {
			output = formatToolResults(results, llmOutput)
		}

		return core.Write(w, output)
	})
}

// parseToolCalls extracts tool calls from LLM output using multiple parsing strategies
func parseToolCalls(output string) []ToolCall {
	var toolCalls []ToolCall

	// Strategy 1: JSON format
	if calls := parseJSONToolCalls(output); len(calls) > 0 {
		toolCalls = append(toolCalls, calls...)
	}

	// Strategy 2: Simple TOOL:name:args format
	if calls := parseSimpleToolCalls(output); len(calls) > 0 {
		toolCalls = append(toolCalls, calls...)
	}

	// Strategy 3: XML-like format
	if calls := parseXMLToolCalls(output); len(calls) > 0 {
		toolCalls = append(toolCalls, calls...)
	}

	return toolCalls
}

// parseJSONToolCalls parses JSON format tool calls
func parseJSONToolCalls(output string) []ToolCall {
	var result struct {
		ToolCalls []ToolCall `json:"tool_calls"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil
	}

	return result.ToolCalls
}

// parseSimpleToolCalls parses TOOL:name:args format
func parseSimpleToolCalls(output string) []ToolCall {
	// Pattern: TOOL:tool_name:arguments
	re := regexp.MustCompile(`TOOL:([^:]+):(.*)`)
	matches := re.FindAllStringSubmatch(output, -1)

	var toolCalls []ToolCall
	for i, match := range matches {
		if len(match) >= 3 {
			toolCalls = append(toolCalls, ToolCall{
				Name:      strings.TrimSpace(match[1]),
				Arguments: strings.TrimSpace(match[2]),
				ID:        fmt.Sprintf("call_%d", i),
			})
		}
	}

	return toolCalls
}

// parseXMLToolCalls parses XML-like format tool calls
func parseXMLToolCalls(output string) []ToolCall {
	// Pattern: <tool name="tool_name">arguments</tool>
	re := regexp.MustCompile(`<tool\s+name="([^"]+)"[^>]*>(.*?)</tool>`)
	matches := re.FindAllStringSubmatch(output, -1)

	var toolCalls []ToolCall
	for i, match := range matches {
		if len(match) >= 3 {
			toolCalls = append(toolCalls, ToolCall{
				Name:      strings.TrimSpace(match[1]),
				Arguments: strings.TrimSpace(match[2]),
				ID:        fmt.Sprintf("xml_call_%d", i),
			})
		}
	}

	return toolCalls
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

	// Execute the tool
	var result bytes.Buffer
	args := strings.NewReader(toolCall.Arguments)

	if err := tool.ServeFlow(ctx, args, &result); err != nil {
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

// executeToolCallsWithConfig executes multiple tool calls with configuration
func executeToolCallsWithConfig(ctx context.Context, tools []Tool, toolCalls []ToolCall, config ExecuteConfig) []ToolResult {
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

// formatToolResults formats tool execution results for output
func formatToolResults(results []ToolResult, originalOutput string) string {
	if len(results) == 0 {
		return originalOutput
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
func formatToolResultsWithOriginal(results []ToolResult, originalOutput string) string {
	var output strings.Builder

	output.WriteString("Original LLM Output:\n")
	output.WriteString(originalOutput)
	output.WriteString("\n\n")

	output.WriteString(formatToolResults(results, originalOutput))

	return output.String()
}

// hasToolCalls detects if the initial chunk contains tool call patterns
func hasToolCalls(data []byte) bool {
	content := string(data)
	
	// High confidence patterns - JSON format
	if strings.Contains(content, `{"tool_calls":`) {
		return true
	}
	
	// High confidence patterns - XML format  
	if strings.Contains(content, `<tool name="`) {
		return true
	}
	
	// Medium confidence patterns - Simple format
	// Look for TOOL: followed by word characters and a colon to avoid false positives
	if regexp.MustCompile(`TOOL:\w+:\w`).MatchString(content) {
		return true
	}
	
	return false
}

// executeBuffered handles full buffering when tools are detected
func executeBuffered(ctx context.Context, initialChunk []byte, r io.Reader, w io.Writer, tools []Tool, config ExecuteConfig) error {
	// Read remaining data and combine with initial chunk
	remainingData, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	
	// Combine initial chunk with remaining data
	fullOutput := string(initialChunk) + string(remainingData)
	
	// Parse and execute tools using existing logic
	toolCalls := parseToolCalls(fullOutput)
	if len(toolCalls) == 0 {
		// No tool calls found after all, just write the full output
		return core.Write(w, fullOutput)
	}
	
	// Execute tool calls with configuration
	results := executeToolCallsWithConfig(ctx, tools, toolCalls, config)
	
	// Check if we should pass through on error
	if config.PassThroughOnError {
		hasErrors := false
		for _, result := range results {
			if result.Error != "" {
				hasErrors = true
				break
			}
		}
		if hasErrors {
			return core.Write(w, fullOutput)
		}
	}
	
	// Format results based on configuration
	var output string
	if config.IncludeOriginalOutput {
		output = formatToolResultsWithOriginal(results, fullOutput)
	} else {
		output = formatToolResults(results, fullOutput)
	}
	
	return core.Write(w, output)
}

// streamWithInitial streams the initial chunk and remaining data
func streamWithInitial(initialChunk []byte, r io.Reader, w io.Writer) error {
	// Write the initial chunk first
	if len(initialChunk) > 0 {
		if _, err := w.Write(initialChunk); err != nil {
			return err
		}
	}
	
	// Stream the rest
	_, err := io.Copy(w, r)
	return err
}

// ExecuteWithStreaming creates a streaming-first Execute middleware
// 
// This implementation buffers only the first ~200 bytes to detect tool calls.
// If no tools are detected, it immediately switches to streaming mode for
// optimal user experience. If tools are detected, it switches to buffered
// mode to properly parse and execute tools.
//
// Benefits:
// - No tool call "leakage" to user
// - Fast streaming for non-tool responses  
// - Minimal initial delay (~200 bytes vs full response)
// - Reliable tool detection using proven patterns
func ExecuteWithStreaming(config ExecuteConfig, bufferSize int) core.Handler {
	if bufferSize <= 0 {
		bufferSize = 200 // Default buffer size
	}
	
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		tools := GetTools(ctx)
		if len(tools) == 0 {
			// No tools available, pure streaming passthrough
			_, err := io.Copy(w, r)
			return err
		}
		
		// Buffer initial chunk to detect tool calls
		initialBuffer := make([]byte, bufferSize)
		n, err := r.Read(initialBuffer)
		if err != nil && err != io.EOF {
			return err
		}
		
		initialChunk := initialBuffer[:n]
		
		// Check for tool patterns in initial chunk
		if hasToolCalls(initialChunk) {
			// Tool detected - switch to full buffering mode
			return executeBuffered(ctx, initialChunk, r, w, tools, config)
		}
		
		// No tools detected - stream the rest
		return streamWithInitial(initialChunk, r, w)
	})
}
