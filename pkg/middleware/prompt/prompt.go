package prompt

import (
	"bytes"
	"fmt"
	"maps"
	"text/template"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Template creates a middleware that applies a Go template to the input
//
// The template receives the input as `.Input` and any additional data as template variables.
// This is useful for formatting prompts, adding context, or structuring LLM inputs.
//
// Example:
//
//	prompt := prompt.Template("You are a helpful assistant. Question: {{.Input}}")
//	promptWithData := prompt.Template("Role: {{.Role}}\nQuestion: {{.Input}}", map[string]any{
//	  "Role": "coding expert",
//	})
func Template(templateStr string, data ...map[string]any) calque.Handler {
	// Parse template once at creation time
	tmpl, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		// Return a handler that always returns the parsing error
		return calque.HandlerFunc(func(_ *calque.Request, _ *calque.Response) error {
			return fmt.Errorf("template parse error: %w", err)
		})
	}

	return FromTemplate(tmpl, data...)
}

// System creates a flexible system message handler
//
// Adds a system message prefix without assuming specific formatting.
// Can be combined with other prompt functions for custom formats.
//
// Example:
//
//	system := prompt.System("You are a helpful coding assistant.")
//	// Input: "How do I sort an array?"
//	// Output: "You are a helpful coding assistant.\n\nHow do I sort an array?"
func System(systemMessage string) calque.Handler {
	return Template("{{.System}}\n\n{{.Input}}", map[string]any{
		"System": systemMessage,
	})
}

// SystemUser creates a system/user formatted prompt
//
// Uses the traditional "System:" and "User:" prefixes for clear role separation.
//
// Example:
//
//	systemUser := prompt.SystemUser("You are a helpful coding assistant.")
//	// Input: "How do I sort an array?"
//	// Output: "System: You are a helpful coding assistant.\n\nUser: How do I sort an array?"
func SystemUser(systemMessage string) calque.Handler {
	return Template("System: {{.System}}\n\nUser: {{.Input}}", map[string]any{
		"System": systemMessage,
	})
}

// Chat creates a chat-style message with role formatting
//
// Supports both simple role prefixes and conversation starters.
//
// Example:
//
//	assistant := prompt.Chat("assistant", "I'm here to help!")
//	// Input: "Hello"
//	// Output: "assistant: I'm here to help!\nuser: Hello"
//
//	user := prompt.Chat("user")
//	// Input: "Hello"
//	// Output: "user: Hello"
func Chat(role string, initialMessage ...string) calque.Handler {
	if len(initialMessage) == 0 {
		// Just format as role message
		return Template("{{.Role}}: {{.Input}}", map[string]any{
			"Role": role,
		})
	}

	return Template("{{.Role}}: {{.InitialMessage}}\nuser: {{.Input}}", map[string]any{
		"Role":           role,
		"InitialMessage": initialMessage[0],
	})
}

// Instruct creates instruction-style prompts
//
// Common pattern for instruction-following models with clear sections.
//
// Example:
//
//	instruct := prompt.Instruct("Translate to French")
//	// Input: "Hello world"
//	// Output: "### Instruction: Translate to French\n### Input: Hello world\n### Response:"
func Instruct(instruction string) calque.Handler {
	return Template("### Instruction: {{.Instruction}}\n### Input: {{.Input}}\n### Response:", map[string]any{
		"Instruction": instruction,
	})
}

// FromTemplate creates a middleware from a pre-parsed template
//
// This is the most flexible prompting function, working with any *template.Template.
// Useful for file-based templates, embedded templates, or complex template structures.
// The template receives the input as {{.Input}} and any additional data as template variables.
//
// Example:
//
//	// From file
//	tmpl := template.Must(template.ParseFiles("system_prompt.tmpl"))
//	prompt.FromTemplate(tmpl, data)
//
//	// From embedded FS
//	//go:embed templates/*.tmpl
//	var templates embed.FS
//	tmpl := template.Must(template.ParseFS(templates, "templates/system.tmpl"))
//	prompt.FromTemplate(tmpl, data)
//
//	// Complex templates with inheritance
//	tmpl := template.Must(template.New("main").Funcs(funcMap).ParseGlob("*.tmpl"))
//	prompt.FromTemplate(tmpl.Lookup("system"), data)
func FromTemplate(tmpl *template.Template, data ...map[string]any) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Read input
		var inputBytes []byte
		err := calque.Read(req, &inputBytes)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		// Prepare template data
		templateData := map[string]any{
			"Input": string(inputBytes),
		}

		// Merge additional data if provided
		if len(data) > 0 {
			maps.Copy(templateData, data[0])
		}

		// Execute template
		var output bytes.Buffer
		if err := tmpl.Execute(&output, templateData); err != nil {
			return fmt.Errorf("template execution error: %w", err)
		}

		// Write result
		_, err = res.Data.Write(output.Bytes())
		return err
	})
}
