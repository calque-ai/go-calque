package ctrl

import (
	"bytes"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/pkg/calque"
)

// Chain creates a sequential middleware chain that executes handlers one after another
// with proper context propagation, similar to HTTP middleware chains.
//
// Unlike the default parallel execution in calque.Flow, Chain ensures:
// 1. Handlers execute sequentially (Handler1 completes before Handler2 starts)
// 2. Context values are properly passed from one handler to the next
// 3. Data is buffered between handlers (no streaming between chain handlers)
//
// This is useful when you need HTTP middleware-like behavior where context
// modifications (like authentication, tool registration, etc.) need to be
// available to subsequent handlers.
//
// Example:
//
//	// These handlers will execute sequentially with context propagation
//	flow.Use(ctrl.Chain(
//	    tools.Registry(calc, search),  // Adds tools to context
//	    addToolInformation(),          // Uses tools from context
//	    llm.Chat(provider),           // LLM processing
//	    tools.Detect(...),            // Uses tools from context
//	))
//
// Note: The chain itself runs as a single handler in the parallel flow,
// so you can still have parallel handlers outside the chain.
// Data is buffered between handlers in the chain, but each individual handler
// can still stream internally.
func Chain(handlers ...calque.Handler) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		if len(handlers) == 0 {
			// Empty chain - just pass through
			_, err := io.Copy(res.Data, req.Data)
			return err
		}

		// Read all input data for buffered sequential processing
		inputData, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}

		// Execute handlers sequentially with context propagation
		currentData := inputData
		currentCtx := req.Context

		for i, handler := range handlers {
			if i == len(handlers)-1 {
				// Last handler - write directly to final output (allows streaming)
				finalReq := &calque.Request{
					Context: currentCtx,
					Data:    io.NopCloser(strings.NewReader(string(currentData))),
				}
				return handler.ServeFlow(finalReq, res)
			}

			// Intermediate handler - buffer output
			var buf bytes.Buffer
			tempReq := &calque.Request{
				Context: currentCtx,
				Data:    io.NopCloser(strings.NewReader(string(currentData))),
			}
			tempRes := &calque.Response{Data: &buf}

			if err := handler.ServeFlow(tempReq, tempRes); err != nil {
				return err
			}

			// Update data and context for next handler
			currentData = buf.Bytes()
			currentCtx = tempReq.Context // Context may have been modified by handler
		}

		return nil
	})
}
