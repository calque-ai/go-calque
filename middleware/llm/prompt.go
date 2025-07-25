package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"text/template"

	"github.com/calque-ai/calque-pipe/core"
)

// Prompt creates a middleware that applies a Go template to the input
//
// The template receives the input as `.Input` and any additional data as template variables.
// This is useful for formatting prompts, adding context, or structuring LLM inputs.
//
// Example:
//
//	prompt := llm.Prompt("You are a helpful assistant. Question: {{.Input}}")
//	promptWithData := llm.Prompt("Role: {{.Role}}\nQuestion: {{.Input}}", map[string]any{
//	  "Role": "coding expert",
//	})
func Prompt(templateStr string, data ...map[string]any) core.Handler {
	// Parse template once at creation time
	tmpl, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		// Return a handler that always returns the parsing error
		return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
			return fmt.Errorf("template parse error: %w", err)
		})
	}

	return PromptFromTemplate(tmpl, data...)
}

// SystemPrompt creates a middleware that adds a system prompt prefix
//
// This is a convenience function for the common pattern of adding system instructions.
//
// Example:
//
//	system := llm.SystemPrompt("You are a helpful coding assistant.")
//	// Input: "How do I sort an array?"
//	// Output: "System: You are a helpful coding assistant.\n\nUser: How do I sort an array?"
func SystemPrompt(systemMessage string) core.Handler {
	return Prompt("System: {{.System}}\n\nUser: {{.Input}}", map[string]any{
		"System": systemMessage,
	})
}

// ChatPrompt creates a middleware that formats input as a chat conversation
//
// Useful for LLMs that expect chat-style formatting.
//
// Example:
//
//	chat := llm.ChatPrompt("assistant", "I'm a helpful AI assistant.")
//	// Input: "Hello"
//	// Output: "assistant: I'm a helpful AI assistant.\nuser: Hello"
func ChatPrompt(role, initialMessage string) core.Handler {
	if initialMessage == "" {
		// Just format as user message
		return Prompt("{{.Role}}: {{.Input}}", map[string]any{
			"Role": role,
		})
	}

	return Prompt("{{.Role}}: {{.InitialMessage}}\nuser: {{.Input}}", map[string]any{
		"Role":           role,
		"InitialMessage": initialMessage,
	})
}

// PromptFromTemplate creates a middleware that uses a pre-parsed template
//
// This is the most flexible prompting function, working with any *template.Template.
// Useful for file-based templates, embedded templates, or complex template structures.
// The template receives the input as {{.Input}} and any additional data as template variables.
//
// Example:
//
//	// From file
//	tmpl := template.Must(template.ParseFiles("system_prompt.tmpl"))
//	llm.PromptFromTemplate(tmpl, data)
//
//	// From embedded FS
//	//go:embed templates/*.tmpl
//	var templates embed.FS
//	tmpl := template.Must(template.ParseFS(templates, "templates/system.tmpl"))
//	llm.PromptFromTemplate(tmpl, data)
//
//	// Complex templates with inheritance
//	tmpl := template.Must(template.New("main").Funcs(funcMap).ParseGlob("*.tmpl"))
//	llm.PromptFromTemplate(tmpl.Lookup("system"), data)
func PromptFromTemplate(tmpl *template.Template, data ...map[string]any) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Read input
		inputBytes, err := io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		// Prepare template data
		templateData := map[string]any{
			"Input": string(inputBytes),
		}

		// Merge additional data if provided
		if len(data) > 0 {
			for key, value := range data[0] {
				templateData[key] = value
			}
		}

		// Execute template
		var output bytes.Buffer
		if err := tmpl.Execute(&output, templateData); err != nil {
			return fmt.Errorf("template execution error: %w", err)
		}

		// Write result
		_, err = w.Write(output.Bytes())
		return err
	})
}
