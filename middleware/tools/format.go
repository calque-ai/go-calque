package tools

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
)

// FormatStyle defines different ways to format tool information for LLMs
type FormatStyle int

const (
	// FormatStyleSimple - Basic name and description format for simple models, quick testing, minimal token usage.
	FormatStyleSimple FormatStyle = iota
	// FormatStyleDetailed - Includes usage examples and argument descriptions best for most production use cases.
	FormatStyleDetailed
	// FormatStyleJSON - JSON schema format for structured LLM understanding
	FormatStyleJSON
	// FormatStyleXML - XML format for XML-aware LLMs
	FormatStyleXML
)

// FormatConfig configures how tools are formatted for LLM consumption
type FormatConfig struct {
	Style           FormatStyle
	IncludeExamples bool
	Prefix          string // Text to add before tool list
	Suffix          string // Text to add after tool list
	Template        string // Custom template (overrides Style if provided)
}

// Format creates a middleware that appends available tool information to the input.
// This helps LLMs understand what tools are available and how to use them.
//
// Input: any data type (buffered - reads input to append tool info)
// Output: original input + formatted tool information
// Behavior: BUFFERED - reads input completely, appends tool info
//
// The middleware retrieves tools from context (set by Registry) and formats them
// according to the specified style. The formatted tool information is appended
// to the original input, creating a prompt that includes tool availability.
//
// Example:
//
//	format := tools.Format(tools.FormatStyleSimple)
//	pipe.Use(tools.Registry(calc, search)).
//	     Use(format). // Appends tool info to input
//	     Use(llm.Chat(provider))
func Format(style FormatStyle) core.Handler {
	config := FormatConfig{
		Style:           style,
		IncludeExamples: style == FormatStyleDetailed,
		Prefix:          "\n\nAvailable tools:\n",
		Suffix:          "\n\nTo use a tool, respond with the appropriate format based on the tool descriptions above.\n",
	}
	return FormatWithConfig(config)
}

// FormatWithConfig creates a Format middleware with custom configuration
// Provides maximum flexibility for customizing how tools are presented to LLMs.
//
// Example:
//
//	config := tools.FormatConfig{
//		Style:           tools.FormatStyleJSON,
//		IncludeExamples: true,
//		Prefix:          "\n\n=== AVAILABLE TOOLS ===\n",
//		Suffix:          "\n=== END TOOLS ===\n\nUse the tools above if needed.\n",
//	}
//	pipeline.Use(tools.FormatWithConfig(config))
func FormatWithConfig(config FormatConfig) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Get tools from context first
		tools := GetTools(ctx)
		if len(tools) == 0 {
			// No tools available, pass through with streaming
			_, err := io.Copy(w, r)
			return err
		}

		// Read original input only if we have tools to format
		var input string
		if err := core.Read(r, &input); err != nil {
			return err
		}

		// Format tools based on configuration
		var toolInfo string
		if config.Template != "" {
			toolInfo = formatWithTemplate(tools, config.Template)
		} else {
			toolInfo = formatTools(tools, config)
		}

		// Combine input with tool information
		output := input + config.Prefix + toolInfo + config.Suffix
		return core.Write(w, output)
	})
}

// formatTools formats tools according to the specified configuration
func formatTools(tools []Tool, config FormatConfig) string {
	switch config.Style {
	case FormatStyleSimple:
		return formatSimple(tools)
	case FormatStyleDetailed:
		return formatDetailed(tools, config.IncludeExamples)
	case FormatStyleJSON:
		return formatJSON(tools)
	case FormatStyleXML:
		return formatXML(tools)
	default:
		return formatSimple(tools)
	}
}

// formatSimple creates a basic tool list with names and descriptions
//
// Output example:
//	1. calculator - Evaluate mathematical expressions
//	2. web_search - Search the web for information
//	3. file_reader - Read file contents
func formatSimple(tools []Tool) string {
	var result strings.Builder

	for i, tool := range tools {
		result.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, tool.Name(), tool.Description()))
	}

	return result.String()
}

// formatDetailed creates a detailed tool list with usage instructions
func formatDetailed(tools []Tool, includeExamples bool) string {
	var result strings.Builder

	for i, tool := range tools {
		result.WriteString(fmt.Sprintf("Tool %d: %s\n", i+1, tool.Name()))
		result.WriteString(fmt.Sprintf("Description: %s\n", tool.Description()))
		result.WriteString("Usage: TOOL:" + tool.Name() + ":<your_arguments>\n")

		if includeExamples {
			result.WriteString(fmt.Sprintf("Example: TOOL:%s:sample input\n", tool.Name()))
		}
		result.WriteString("\n")
	}

	return result.String()
}

// formatJSON creates a JSON representation of available tools
func formatJSON(tools []Tool) string {
	var result strings.Builder

	result.WriteString("Tools available (JSON format):\n")
	result.WriteString("{\n  \"tools\": [\n")

	for i, tool := range tools {
		result.WriteString(fmt.Sprintf("    {\n      \"name\": \"%s\",\n      \"description\": \"%s\"\n    }",
			tool.Name(), strings.ReplaceAll(tool.Description(), "\"", "\\\"")))

		if i < len(tools)-1 {
			result.WriteString(",")
		}
		result.WriteString("\n")
	}

	result.WriteString("  ]\n}\n\n")
	result.WriteString("To use tools, respond with:\n")
	result.WriteString(`{"tool_calls": [{"name": "tool_name", "arguments": "your_arguments"}]}`)
	result.WriteString("\n")

	return result.String()
}

// formatXML creates an XML representation of available tools
func formatXML(tools []Tool) string {
	var result strings.Builder

	result.WriteString("Tools available (XML format):\n")
	result.WriteString("<tools>\n")

	for _, tool := range tools {
		result.WriteString(fmt.Sprintf("  <tool name=\"%s\" description=\"%s\" />\n",
			tool.Name(), strings.ReplaceAll(tool.Description(), "\"", "&quot;")))
	}

	result.WriteString("</tools>\n\n")
	result.WriteString("To use tools, respond with:\n")
	result.WriteString(`<tool name="tool_name">your_arguments</tool>`)
	result.WriteString("\n")

	return result.String()
}

// formatWithTemplate formats tools using a custom template
// Template can use {{.Name}} and {{.Description}} for each tool
func formatWithTemplate(tools []Tool, template string) string {
	var result strings.Builder

	for _, tool := range tools {
		toolText := template
		toolText = strings.ReplaceAll(toolText, "{{.Name}}", tool.Name())
		toolText = strings.ReplaceAll(toolText, "{{.Description}}", tool.Description())
		result.WriteString(toolText)
		result.WriteString("\n")
	}

	return result.String()
}

// AppendToolInfo is a convenience function that appends simple styled tool information to input
// Uses FormatStyleSimple for minimal token usage and quick setup.
//
// Example:
//
//	pipeline := core.New().
//		Use(tools.Registry(calculator, webSearch)).
//		Use(tools.AppendToolInfo()).              // Simple tool list appended
//		Use(llm.Chat(provider))
//
// Output format:
//
//	User input here
//
//	Available tools:
//	1. calculator - Evaluate mathematical expressions
//	2. web_search - Search the web for information
//
//	To use a tool, respond with the appropriate format based on the tool descriptions above.
func AppendToolInfo() core.Handler {
	return Format(FormatStyleSimple)
}

// PrependToolInfo creates a middleware that prepends tool information to the input
// Best for when tools should be "system instructions" rather than appended context.
// Places tool information at the beginning, making it more prominent for the LLM.
//
// Example:
//
//	pipeline := core.New().
//		Use(tools.Registry(calculator, webSearch)).
//		Use(tools.PrependToolInfo(tools.FormatStyleDetailed)). // Tools come first
//		Use(llm.Chat(provider))
//
// Output format:
//
//	Available tools:
//	Tool 1: calculator
//	Description: Evaluate mathematical expressions
//	Usage: TOOL:calculator:<your_arguments>
//	Example: TOOL:calculator:25*4
//
//	User request: What is 25 times 4?
func PrependToolInfo(style FormatStyle) core.Handler {
	config := FormatConfig{
		Style:           style,
		IncludeExamples: style == FormatStyleDetailed,
		Prefix:          "Available tools:\n",
		Suffix:          "\n\nUser request: ",
	}

	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Get tools from context first
		tools := GetTools(ctx)
		if len(tools) == 0 {
			// No tools available, pass through with streaming
			_, err := io.Copy(w, r)
			return err
		}

		// Read original input only if we have tools to format
		var input string
		if err := core.Read(r, &input); err != nil {
			return err
		}

		var toolInfo string
		if config.Template != "" {
			toolInfo = formatWithTemplate(tools, config.Template)
		} else {
			toolInfo = formatTools(tools, config)
		}

		// Prepend instead of append
		output := config.Prefix + toolInfo + config.Suffix + input
		return core.Write(w, output)
	})
}

// InlineToolInfo creates a middleware that formats tools in a compact inline format
// Minimal token usage - shows only tool names, not descriptions. Best for token-limited
// scenarios or when tool names are self-explanatory.
//
// Example:
//
//	pipeline := core.New().
//		Use(tools.Registry(calculator, webSearch, fileReader)).
//		Use(tools.InlineToolInfo()).                          // Compact format
//		Use(llm.Chat(provider))
//
// Output format:
//
//	User input here
//
//	Tools: [calculator, webSearch, fileReader] (use TOOL:name:args format)
func InlineToolInfo() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Get tools from context first
		tools := GetTools(ctx)
		if len(tools) == 0 {
			// No tools available, pass through with streaming
			_, err := io.Copy(w, r)
			return err
		}

		// Read original input only if we have tools to format
		var input string
		if err := core.Read(r, &input); err != nil {
			return err
		}

		// Create compact inline format
		var toolNames []string
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Name())
		}

		toolList := strings.Join(toolNames, ", ")
		output := fmt.Sprintf("%s\n\nTools: [%s] (use TOOL:name:args format)\n", input, toolList)

		return core.Write(w, output)
	})
}
