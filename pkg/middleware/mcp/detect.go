package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/convert"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// DetectOptions configures detection behavior for MCP capabilities
type DetectOptions struct {
	// PromptTemplate is the LLM prompt template used for selection
	// If not provided, uses the default template for the capability type
	PromptTemplate string
}

// DetectOption configures detection handlers
type DetectOption interface {
	Apply(*DetectOptions)
}

type promptTemplateOption struct{ template string }

func (o promptTemplateOption) Apply(opts *DetectOptions) { opts.PromptTemplate = o.template }

// WithPromptTemplate sets a custom prompt template for detection.
//
// Input: template string with {{.Input}} and capability-specific placeholders
// Output: DetectOption for configuration
// Behavior: Overrides the default detection prompt template
//
// Allows customizing how the LLM analyzes user input to select capabilities.
// Template must include {{.Input}} for user input and capability-specific
// placeholders like {{.Tools}}, {{.Resources}}, or {{.Prompts}}.
//
// Example:
//
//	template := `Custom tool selection prompt...
//	User request: {{.Input}}
//	Available tools: {{range .Tools}}...{{end}}`
//	detector := mcp.DetectTool(llmClient, mcp.WithPromptTemplate(template))
func WithPromptTemplate(template string) DetectOption {
	return promptTemplateOption{template: template}
}

// detectConfig holds configuration for capability detection
type detectConfig[T any] struct {
	// capabilityName is the human-readable name of the capability (e.g., "tool", "resource", "prompt")
	capabilityName string

	// getItems retrieves the list of available items from the request context
	// Returns the items in their native type (e.g., []*Tool, []*mcp.Resource, []*mcp.Prompt)
	getItems func(context.Context) any

	// checkEmpty checks if the items list is nil or empty
	// Receives items from getItems and returns true if there are no items available
	checkEmpty func(any) bool

	// promptTemplate is the LLM prompt template used for selection
	// Should instruct the LLM to analyze user input and select the appropriate item
	promptTemplate string

	// getTemplateData creates template data for the prompt
	// Receives user input string and items, returns map for template rendering
	getTemplateData func(string, any) map[string]any

	// validator validates and normalizes the LLM's selection
	// Receives the selected name from LLM and available items
	// Returns the validated/normalized name, or empty string if invalid
	validator func(string, any) string

	// getSelectedName extracts the selected item name from the LLM response
	// Receives pointer to the typed response struct and returns the selection field value
	getSelectedName func(*T) string

	// contextKey is the key used to store the selection in the request context
	// Should be a unique empty struct type for each capability
	contextKey any
}

// DetectTool creates a handler that uses LLM-based intent detection to select the appropriate MCP tool.
// This is the MCP equivalent of tools.Detect(), but uses semantic analysis instead of parsing tool calls.
//
// Input: Natural language user request
// Output: Same user input with optional tool selection stored in context
// Behavior: BUFFERED - reads full input for analysis, then passes through with optional tool selection
//
// The handler analyzes the user's natural language input using an LLM to determine which
// MCP tool would be most appropriate to handle the request. If a suitable tool is found,
// it's stored in the context for use by downstream handlers like Execute(). If no tool
// is appropriate, the input passes through unchanged.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	llmClient, _ := openai.New("gpt-4o-mini")
//	detector := mcp.DetectTool(client, llmClient)
//	flow.Use(mcp.ToolRegistry(client)).
//	     Use(detector).
//	     Use(mcp.ExecuteTool())  // Only executes if tool was selected
//
//	// Input: "search for golang tutorials" → selects "search" tool
//	// Input: "connect to localhost:8080" → selects "connect" tool
//	// Input: "hello world" → no tool selected, passes through
func DetectTool(llmClient ai.Client, opts ...DetectOption) calque.Handler {
	options := &DetectOptions{}
	for _, opt := range opts {
		opt.Apply(options)
	}

	template := toolSelectionPromptTemplate
	if options.PromptTemplate != "" {
		template = options.PromptTemplate
	}

	return detectCapability(llmClient, detectConfig[ToolSelectionResponse]{
		capabilityName: "tool",
		getItems: func(ctx context.Context) any {
			return GetTools(ctx)
		},
		checkEmpty: func(items any) bool {
			tools, ok := items.([]*Tool)
			return !ok || len(tools) == 0
		},
		promptTemplate: template,
		getTemplateData: func(userInput string, items any) map[string]any {
			tools := items.([]*Tool)
			return getToolSelectionTemplateData(userInput, tools)
		},
		validator: func(selected string, items any) string {
			tools := items.([]*Tool)
			return validateToolSelection(selected, tools)
		},
		getSelectedName: func(resp *ToolSelectionResponse) string {
			return resp.SelectedTool
		},
		contextKey: selectedToolContextKey{},
	})
}

// DetectResource creates a handler that uses LLM-based intent detection to select the appropriate MCP resource.
//
// Input: Natural language user request
// Output: Same user input with optional resource selection stored in context
// Behavior: BUFFERED - reads full input for analysis, then passes through with optional resource selection
//
// The handler analyzes the user's natural language input using an LLM to determine which
// MCP resource would be most appropriate to handle the request. If a suitable resource is found,
// it's stored in the context for use by downstream handlers like ExecuteResource().
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	llmClient, _ := openai.New("gpt-4o-mini")
//	flow.Use(mcp.ResourceRegistry(client)).
//	     Use(mcp.DetectResource(llmClient)).
//	     Use(mcp.ExecuteResource())
func DetectResource(llmClient ai.Client, opts ...DetectOption) calque.Handler {
	options := &DetectOptions{}
	for _, opt := range opts {
		opt.Apply(options)
	}

	template := resourceSelectionPromptTemplate
	if options.PromptTemplate != "" {
		template = options.PromptTemplate
	}

	return detectCapability(llmClient, detectConfig[ResourceSelectionResponse]{
		capabilityName: "resource",
		getItems: func(ctx context.Context) any {
			return GetResources(ctx)
		},
		checkEmpty: func(items any) bool {
			resources, ok := items.([]*mcpsdk.Resource)
			return !ok || len(resources) == 0
		},
		promptTemplate: template,
		getTemplateData: func(userInput string, items any) map[string]any {
			resources := items.([]*mcpsdk.Resource)
			return getResourceSelectionTemplateData(userInput, resources)
		},
		validator: func(selected string, items any) string {
			resources := items.([]*mcpsdk.Resource)
			return validateResourceSelection(selected, resources)
		},
		getSelectedName: func(resp *ResourceSelectionResponse) string {
			return resp.SelectedResource
		},
		contextKey: selectedResourceContextKey{},
	})
}

// DetectPrompt creates a handler that uses LLM-based intent detection to select the appropriate MCP prompt.
//
// Input: Natural language user request
// Output: Same user input with optional prompt selection stored in context
// Behavior: BUFFERED - reads full input for analysis, then passes through with optional prompt selection
//
// The handler analyzes the user's natural language input using an LLM to determine which
// MCP prompt would be most appropriate to handle the request. If a suitable prompt is found,
// it's stored in the context for use by downstream handlers like ExecutePrompt().
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	llmClient, _ := openai.New("gpt-4o-mini")
//	flow.Use(mcp.PromptRegistry(client)).
//	     Use(mcp.DetectPrompt(llmClient)).
//	     Use(mcp.ExecutePrompt())
func DetectPrompt(llmClient ai.Client, opts ...DetectOption) calque.Handler {
	options := &DetectOptions{}
	for _, opt := range opts {
		opt.Apply(options)
	}

	template := promptSelectionPromptTemplate
	if options.PromptTemplate != "" {
		template = options.PromptTemplate
	}

	return detectCapability(llmClient, detectConfig[PromptSelectionResponse]{
		capabilityName: "prompt",
		getItems: func(ctx context.Context) any {
			return GetPrompts(ctx)
		},
		checkEmpty: func(items any) bool {
			prompts, ok := items.([]*mcpsdk.Prompt)
			return !ok || len(prompts) == 0
		},
		promptTemplate: template,
		getTemplateData: func(userInput string, items any) map[string]any {
			prompts := items.([]*mcpsdk.Prompt)
			return getPromptSelectionTemplateData(userInput, prompts)
		},
		validator: func(selected string, items any) string {
			prompts := items.([]*mcpsdk.Prompt)
			return validatePromptSelection(selected, prompts)
		},
		getSelectedName: func(resp *PromptSelectionResponse) string {
			return resp.SelectedPrompt
		},
		contextKey: selectedPromptContextKey{},
	})
}

// detectCapability is the internal helper function that implements generic detection logic
func detectCapability[T any](llmClient ai.Client, cfg detectConfig[T]) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		// Read user input for analysis
		var userInput string
		if err := calque.Read(req, &userInput); err != nil {
			return fmt.Errorf("failed to read user input for %s detection: %w", cfg.capabilityName, err)
		}

		// Get available items from context
		items := cfg.getItems(req.Context)
		if cfg.checkEmpty(items) {
			// No items available - just pass through
			return calque.Write(res, userInput)
		}

		if strings.TrimSpace(userInput) == "" {
			// Empty input - just pass through
			return calque.Write(res, userInput)
		}

		// Use prompt template pipeline for selection
		templateData := cfg.getTemplateData(userInput, items)

		pipe := calque.NewFlow()
		pipe.Use(prompt.Template(cfg.promptTemplate, templateData)).
			Use(ai.Agent(llmClient, ai.WithSchemaFor[T]()))

		var selectionResponse T
		err := pipe.Run(req.Context, userInput, convert.FromJSON(&selectionResponse))
		if err != nil {
			// Pipeline error - just pass through without selection
			return calque.Write(res, userInput)
		}

		// Extract selected name from response
		selectedName := cfg.getSelectedName(&selectionResponse)

		// Validate the selection
		validatedName := cfg.validator(selectedName, items)

		if validatedName != "" {
			// Store selection in context for Execute handler
			contextWithSelection := context.WithValue(req.Context, cfg.contextKey, validatedName)
			req.Context = contextWithSelection

			// Pass through the original user input
			return calque.Write(res, userInput)
		}

		// No selection or validation failed - pass through original input
		return calque.Write(res, userInput)
	})
}
