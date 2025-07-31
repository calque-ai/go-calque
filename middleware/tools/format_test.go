package tools

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/calque-pipe/core"
)

func TestFormat(t *testing.T) {
	calc := Simple("calculator", "Calculate something", func(s string) string { return s })
	search := Simple("search", "Search the web for information", func(s string) string { return s })

	tests := []struct {
		name     string
		style    FormatStyle
		tools    []Tool
		input    string
		contains []string
	}{
		{
			name:  "simple format with tools",
			style: FormatStyleSimple,
			tools: []Tool{calc, search},
			input: "Calculate something",
			contains: []string{
				"Calculate something",
				"Available tools:",
				"1. calculator - Calculate something",
				"2. search - Search the web for information",
				"To use a tool, respond with the appropriate format",
			},
		},
		{
			name:  "detailed format with tools",
			style: FormatStyleDetailed,
			tools: []Tool{calc, search},
			input: "Help me",
			contains: []string{
				"Help me",
				"Available tools:",
				"Tool 1: calculator",
				"Description: Calculate something",
				"Usage: TOOL:calculator:<your_arguments>",
				"Example: TOOL:calculator:sample input",
				"Tool 2: search",
				"Description: Search the web for information",
				"Usage: TOOL:search:<your_arguments>",
			},
		},
		{
			name:  "JSON format with tools",
			style: FormatStyleJSON,
			tools: []Tool{calc, search},
			input: "Query",
			contains: []string{
				"Query",
				"Tools available (JSON format):",
				`"tools": [`,
				`"name": "calculator"`,
				`"name": "search"`,
				`"description": "Search the web for information"`,
				`{"tool_calls": [{"name": "tool_name", "arguments": "your_arguments"}]}`,
			},
		},
		{
			name:  "XML format with tools",
			style: FormatStyleXML,
			tools: []Tool{calc, search},
			input: "Request",
			contains: []string{
				"Request",
				"Tools available (XML format):",
				"<tools>",
				`<tool name="calculator"`,
				`<tool name="search"`,
				`description="Search the web for information"`,
				"</tools>",
				`<tool name="tool_name">your_arguments</tool>`,
			},
		},
		{
			name:     "no tools - pass through",
			style:    FormatStyleSimple,
			tools:    []Tool{},
			input:    "No tools available",
			contains: []string{"No tools available"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), toolsContextKey{}, tt.tools)

			format := Format(tt.style)
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			err := format.ServeFlow(ctx, reader, &buf)
			if err != nil {
				t.Errorf("Format() error = %v", err)
				return
			}

			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("Format() output missing expected string %q, got %q", expected, output)
				}
			}
		})
	}
}

func TestFormatWithConfig(t *testing.T) {
	calc := Simple("calculator", "Calculate something", func(s string) string { return s })
	search := Simple("search", "Search the web", func(s string) string { return s })
	tools := []Tool{calc, search}

	tests := []struct {
		name     string
		config   FormatConfig
		input    string
		contains []string
	}{
		{
			name: "custom prefix and suffix",
			config: FormatConfig{
				Style:  FormatStyleSimple,
				Prefix: "\n\n=== TOOLS ===\n",
				Suffix: "\n=== END TOOLS ===\n",
			},
			input: "Test input",
			contains: []string{
				"Test input",
				"=== TOOLS ===",
				"1. calculator",
				"2. search",
				"=== END TOOLS ===",
			},
		},
		{
			name: "custom template",
			config: FormatConfig{
				Template: "Tool: {{.Name}} ({{.Description}})",
			},
			input: "Test",
			contains: []string{
				"Test",
				"Tool: calculator (Calculate something)",
				"Tool: search (Search the web)",
			},
		},
		{
			name: "include examples disabled",
			config: FormatConfig{
				Style:           FormatStyleDetailed,
				IncludeExamples: false,
			},
			input: "Test",
			contains: []string{
				"Tool 1: calculator",
				"Usage: TOOL:calculator:<your_arguments>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), toolsContextKey{}, tools)

			format := FormatWithConfig(tt.config)
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			err := format.ServeFlow(ctx, reader, &buf)
			if err != nil {
				t.Errorf("FormatWithConfig() error = %v", err)
				return
			}

			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("FormatWithConfig() output missing expected string %q, got %q", expected, output)
				}
			}
		})
	}
}

func TestFormatStyles(t *testing.T) {
	calc := Simple("calculator", "Calculate something", func(s string) string { return s })
	search := Simple("web_search", "Search the web", func(s string) string { return s })
	tools := []Tool{calc, search}

	tests := []struct {
		name           string
		style          FormatStyle
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:  "simple style format",
			style: FormatStyleSimple,
			mustContain: []string{
				"1. calculator - Calculate something",
				"2. web_search - Search the web",
			},
			mustNotContain: []string{"Usage:", "Example:", "JSON", "XML"},
		},
		{
			name:  "detailed style format",
			style: FormatStyleDetailed,
			mustContain: []string{
				"Tool 1: calculator",
				"Description: Calculate something",
				"Usage: TOOL:calculator:<your_arguments>",
				"Example: TOOL:calculator:sample input",
			},
			mustNotContain: []string{"JSON", "XML"},
		},
		{
			name:  "JSON style format",
			style: FormatStyleJSON,
			mustContain: []string{
				"Tools available (JSON format)",
				`"tools": [`,
				`"name": "calculator"`,
				`"name": "web_search"`,
				`{"tool_calls":`,
			},
			mustNotContain: []string{"Usage:", "Example:", "XML"},
		},
		{
			name:  "XML style format",
			style: FormatStyleXML,
			mustContain: []string{
				"Tools available (XML format)",
				"<tools>",
				`<tool name="calculator"`,
				`<tool name="web_search"`,
				"</tools>",
				`<tool name="tool_name">`,
			},
			mustNotContain: []string{"Usage:", "Example:", "JSON"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := FormatConfig{Style: tt.style}
			output := formatTools(tools, config)

			for _, expected := range tt.mustContain {
				if !strings.Contains(output, expected) {
					t.Errorf("formatTools() missing expected string %q in output: %q", expected, output)
				}
			}

			for _, notExpected := range tt.mustNotContain {
				if strings.Contains(output, notExpected) {
					t.Errorf("formatTools() contains unexpected string %q in output: %q", notExpected, output)
				}
			}
		})
	}
}

func TestFormatSimple(t *testing.T) {
	tools := []Tool{
		Simple("calc", "Calculate Something", func(s string) string { return s }),
		Simple("search", "Web search tool", func(s string) string { return s }),
	}

	result := formatSimple(tools)

	expected := []string{
		"1. calc - Calculate Something",
		"2. search - Web search tool",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("formatSimple() missing expected string %q", exp)
		}
	}
}

func TestFormatDetailed(t *testing.T) {
	tools := []Tool{
		Simple("calculator", "Math calculations", func(s string) string { return s }),
	}

	// Test with examples
	resultWithExamples := formatDetailed(tools, true)
	expectedWith := []string{
		"Tool 1: calculator",
		"Description: Math calculations",
		"Usage: TOOL:calculator:<your_arguments>",
		"Example: TOOL:calculator:sample input",
	}

	for _, exp := range expectedWith {
		if !strings.Contains(resultWithExamples, exp) {
			t.Errorf("formatDetailed(true) missing expected string %q", exp)
		}
	}

	// Test without examples
	resultWithoutExamples := formatDetailed(tools, false)
	if strings.Contains(resultWithoutExamples, "Example:") {
		t.Error("formatDetailed(false) should not contain examples")
	}
}

func TestFormatJSON(t *testing.T) {
	tools := []Tool{
		Simple("test", "Test tool", func(s string) string { return s }),
		Simple("search", `Description with "quotes"`, func(s string) string { return s }),
	}

	result := formatJSON(tools)

	expected := []string{
		"Tools available (JSON format)",
		`"tools": [`,
		`"name": "test"`,
		`"name": "search"`,
		`"description": "Description with \"quotes\""`, // Quotes should be escaped
		`"tool_calls":`,
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("formatJSON() missing expected string %q", exp)
		}
	}
}

func TestFormatXML(t *testing.T) {
	tools := []Tool{
		Simple("test", "Test Tool", func(s string) string { return s }),
		Simple("search", `Description with "quotes"`, func(s string) string { return s }),
	}

	result := formatXML(tools)

	expected := []string{
		"Tools available (XML format)",
		"<tools>",
		`<tool name="test"`,
		`<tool name="search"`,
		`description="Description with &quot;quotes&quot;"`, // Quotes should be escaped
		"</tools>",
		`<tool name="tool_name">your_arguments</tool>`,
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("formatXML() missing expected string %q", exp)
		}
	}
}

func TestFormatWithTemplate(t *testing.T) {
	tools := []Tool{
		Simple("calc", "Calculator tool", func(s string) string { return s }),
		Simple("search", "Search tool", func(s string) string { return s }),
	}

	template := "Name: {{.Name}}, Desc: {{.Description}}"
	result := formatWithTemplate(tools, template)

	expected := []string{
		"Name: calc, Desc: Calculator tool",
		"Name: search, Desc: Search tool",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("formatWithTemplate() missing expected string %q", exp)
		}
	}
}

func TestAppendToolInfo(t *testing.T) {
	calc := Simple("calculator", "Calculate Something", func(s string) string { return s })
	ctx := context.WithValue(context.Background(), toolsContextKey{}, []Tool{calc})

	handler := AppendToolInfo()
	var buf bytes.Buffer
	reader := strings.NewReader("Test input")

	err := handler.ServeFlow(ctx, reader, &buf)
	if err != nil {
		t.Errorf("AppendToolInfo() error = %v", err)
		return
	}

	output := buf.String()
	expected := []string{
		"Test input",
		"Available tools:",
		"1. calculator",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("AppendToolInfo() missing expected string %q", exp)
		}
	}
}

func TestPrependToolInfo(t *testing.T) {
	calc := Simple("calculator", "Calculate Something", func(s string) string { return s })
	ctx := context.WithValue(context.Background(), toolsContextKey{}, []Tool{calc})

	handler := PrependToolInfo(FormatStyleSimple)
	var buf bytes.Buffer
	reader := strings.NewReader("User request")

	err := handler.ServeFlow(ctx, reader, &buf)
	if err != nil {
		t.Errorf("PrependToolInfo() error = %v", err)
		return
	}

	output := buf.String()

	// Should start with tools, then user request
	if !strings.HasPrefix(output, "Available tools:") {
		t.Error("PrependToolInfo() should start with tool information")
	}

	expected := []string{
		"Available tools:",
		"1. calculator",
		"User request: User request",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("PrependToolInfo() missing expected string %q", exp)
		}
	}
}

func TestInlineToolInfo(t *testing.T) {
	calc := Simple("calculator", "Calculate Something", func(s string) string { return s })
	search := Simple("search", "Search the web", func(s string) string { return s })
	ctx := context.WithValue(context.Background(), toolsContextKey{}, []Tool{calc, search})

	handler := InlineToolInfo()
	var buf bytes.Buffer
	reader := strings.NewReader("Help me")

	err := handler.ServeFlow(ctx, reader, &buf)
	if err != nil {
		t.Errorf("InlineToolInfo() error = %v", err)
		return
	}

	output := buf.String()
	expected := []string{
		"Help me",
		"Tools: [calculator, search]",
		"(use TOOL:name:args format)",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("InlineToolInfo() missing expected string %q", exp)
		}
	}
}

func TestFormatWithNoTools(t *testing.T) {
	// Test all format variations with no tools
	tests := []struct {
		name    string
		handler core.Handler
	}{
		{"Format", Format(FormatStyleSimple)},
		{"AppendToolInfo", AppendToolInfo()},
		{"PrependToolInfo", PrependToolInfo(FormatStyleSimple)},
		{"InlineToolInfo", InlineToolInfo()},
	}

	input := "Test input with no tools"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Empty context (no tools)
			ctx := context.Background()

			var buf bytes.Buffer
			reader := strings.NewReader(input)

			err := tt.handler.ServeFlow(ctx, reader, &buf)
			if err != nil {
				t.Errorf("%s error = %v", tt.name, err)
				return
			}

			output := buf.String()
			// Should pass through unchanged when no tools
			if output != input {
				t.Errorf("%s with no tools = %q, want %q", tt.name, output, input)
			}
		})
	}
}

func TestFormatWithIOError(t *testing.T) {
	format := Format(FormatStyleSimple)
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	err := format.ServeFlow(context.Background(), errorReader, &buf)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Format() with IO error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func TestFormatWithLargeInput(t *testing.T) {
	calc := Simple("calculator", "Calculate Something", func(s string) string { return s })
	ctx := context.WithValue(context.Background(), toolsContextKey{}, []Tool{calc})

	largeInput := strings.Repeat("large input data ", 1000)

	format := Format(FormatStyleSimple)
	var buf bytes.Buffer
	reader := strings.NewReader(largeInput)

	err := format.ServeFlow(ctx, reader, &buf)
	if err != nil {
		t.Errorf("Format() with large input error = %v", err)
		return
	}

	output := buf.String()
	// Should contain both the large input and tool info
	if !strings.Contains(output, largeInput) {
		t.Error("Format() with large input lost original content")
	}

	if !strings.Contains(output, "Available tools:") {
		t.Error("Format() with large input lost tool information")
	}
}
