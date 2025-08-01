package tools

import (
	"github.com/calque-ai/calque-pipe/core"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// Tool defines a function that can be called by the LLM (OpenAI Function Calling standard).
// Tools are handlers that extend the core Handler interface with metadata and schema.
// Tools can be composed, logged, timed, cached like any other handler.
// Tools are streaming-compatible by default.
type Tool interface {
	core.Handler                          // ServeFlow method for execution
	Name() string                         // Function name (e.g., "get_current_weather")
	Description() string                  // What the function does
	ParametersSchema() *jsonschema.Schema // JSON schema for function parameters (OpenAI standard)
}

// toolImpl is the basic implementation of Tool interface
// Wraps any existing core.Handler and adds metadata.
type toolImpl struct {
	name             string
	description      string
	parametersSchema *jsonschema.Schema
	handler          core.Handler
}

func (t *toolImpl) Name() string {
	return t.name
}

func (t *toolImpl) Description() string {
	return t.description
}

func (t *toolImpl) ParametersSchema() *jsonschema.Schema {
	return t.parametersSchema
}

func (t *toolImpl) ServeFlow(r *core.Request, w *core.Response) error {
	return t.handler.ServeFlow(r, w)
}

// New creates a tool with full control over name, description, schema, and handler.
// This is the most flexible constructor for complex tools.
//
// Example:
//
//	schema := &jsonschema.Schema{
//		Type: "object",
//		Properties: map[string]*jsonschema.Schema{
//			"query": {Type: "string", Description: "Search query"},
//		},
//		Required: []string{"query"},
//	}
//	searchTool := tools.New(
//	    "web_search",
//	    "Search the web for current information",
//	    schema,
//	    mySearchHandler,
//	)
func New(name, description string, parametersSchema *jsonschema.Schema, handler core.Handler) Tool {
	return &toolImpl{
		name:             name,
		description:      description,
		parametersSchema: parametersSchema,
		handler:          handler,
	}
}

// simpleTool implements Tool for simple function-based tools
type simpleTool struct {
	name        string
	description string
	fn          func(string) string
}

func (q *simpleTool) Name() string {
	return q.name
}

func (q *simpleTool) Description() string {
	return q.description
}

func (q *simpleTool) ParametersSchema() *jsonschema.Schema {
	// Simple tools use a basic schema with a single "input" string parameter
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("input", &jsonschema.Schema{
		Type:        "string",
		Description: "Input for the " + q.name + " tool",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"input"},
	}
}

func (q *simpleTool) ServeFlow(r *core.Request, w *core.Response) error {
	var input string
	if err := core.Read(r, &input); err != nil {
		return err
	}

	result := q.fn(input)
	return core.Write(w, result)
}

// Simple creates a tool from a string-to-string function.
//
// Example:
//
//	calc := tools.Simple(
//	    "calculator",
//	    "Evaluate mathematical expressions and return numeric results",
//	    func(expr string) string {
//	        result := evaluateExpression(expr)
//	        return fmt.Sprintf("%.2f", result)
//	    },
//	)
func Simple(name, description string, fn func(string) string) Tool {
	return &simpleTool{
		name:        name,
		description: description,
		fn:          fn,
	}
}

// HandlerFunc creates a tool from a handler function with the given metadata.
// This allows creating tools inline without defining separate handlers.
//
// Example:
//
//	tool := tools.HandlerFunc("file_read", "Read file contents",
//	    func(ctx context.Context, r io.Reader, w io.Writer) error {
//	        var filename string
//	        if err := core.Read(r, &filename); err != nil {
//	            return err
//	        }
//	        content, err := os.ReadFile(filename)
//	        if err != nil {
//	            return err
//	        }
//	        return core.Write(w, string(content))
//	    },
//	)
func HandlerFunc(name, description string, fn func(*core.Request, *core.Response) error) Tool {
	// HandlerFunc tools use a basic schema with a single "input" string parameter
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("input", &jsonschema.Schema{
		Type:        "string",
		Description: "Input for the " + name + " tool",
	})

	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"input"},
	}

	return New(name, description, schema, core.HandlerFunc(fn))
}
