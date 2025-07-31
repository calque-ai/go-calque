package tools

import (
	"context"
	"io"

	"github.com/calque-ai/calque-pipe/core"
)

// Tool defines a function that can be called by the LLM.
// Tools are handlers that extend the core Handler interface with metadata.
// Tools can be composed, logged, timed, cached like any other handler.
// Tools are streaming-compatible by default.
type Tool interface {
	core.Handler         // ServeFlow method for execution
	Name() string        // LLMs need to know what to call (e.g., "calculator", "web_search")
	Description() string // LLMs need context about what the tool does to use it appropriately
}

// toolImpl is the basic implementation of Tool interface
// Wraps any existing core.Handler and adds metadata.
type toolImpl struct {
	name        string
	description string
	handler     core.Handler
}

func (t *toolImpl) Name() string {
	return t.name
}

func (t *toolImpl) Description() string {
	return t.description
}

func (t *toolImpl) ServeFlow(ctx context.Context, r io.Reader, w io.Writer) error {
	return t.handler.ServeFlow(ctx, r, w)
}

// New creates a tool with full control over name, description, and handler.
// This is the most flexible constructor for complex tools.
//
// Example:
//
//	searchTool := tools.New(
//	    "web_search",
//	    "Search the web for current information",
//	    mySearchHandler,
//	)
func New(name, description string, handler core.Handler) Tool {
	return &toolImpl{
		name:        name,
		description: description,
		handler:     handler,
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

func (q *simpleTool) ServeFlow(ctx context.Context, r io.Reader, w io.Writer) error {
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
func HandlerFunc(name, description string, fn func(context.Context, io.Reader, io.Writer) error) Tool {
	return New(name, description, core.HandlerFunc(fn))
}
