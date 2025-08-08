package tools

import (
	"bytes"
	"context"
	"io"

	"github.com/calque-ai/calque-pipe/pkg/calque"
)

// Detect creates a conditional handler that detects tool calls and routes accordingly.
//
// Input: any data (streaming with 200-byte buffer for tool detection)
// Output: depends on which handler is executed (ifHandler or elseHandler)
// Behavior: STREAMING - buffers first ~200 bytes to detect tools, then routes appropriately
//
// The middleware detects tool calls in JSON format:
// {"tool_calls": [{"name": "tool_name", "arguments": "args"}]}
//
// If tool calls are detected, the full input is buffered and passed to ifHandler.
// If no tool calls are detected, the input is streamed through to elseHandler.
//
// Example:
//
//	detector := tools.Detect(
//	    tools.Execute(),        // Handle tool calls
//	    flow.PassThrough(),     // Pass through if no tools
//	)
//	pipe.Use(tools.Registry(calc, search)).
//	     Use(llm.Chat(provider)).
//	     Use(detector)
func Detect(ifHandler, elseHandler calque.Handler) calque.Handler {
	return DetectWithBufferSize(ifHandler, elseHandler, 200)
}

// DetectWithBufferSize creates a tool detection handler with custom buffer size
func DetectWithBufferSize(ifHandler, elseHandler calque.Handler, bufferSize int) calque.Handler {
	if bufferSize <= 0 {
		bufferSize = 200 // Default buffer size
	}

	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Buffer initial chunk for detection
		initialBuffer := make([]byte, bufferSize)
		n, err := r.Data.Read(initialBuffer)
		if err != nil && err != io.EOF {
			return err
		}

		initialChunk := initialBuffer[:n]

		if !hasToolCalls(initialChunk) {
			// No tools detected - stream to elseHandler
			return streamToHandler(elseHandler, r.Context, initialChunk, r.Data, w.Data)
		}

		// Tools detected - buffer full input and pass to ifHandler
		return bufferToHandler(ifHandler, r.Context, initialChunk, r.Data, w.Data)
	})
}

// streamToHandler streams the initial chunk and remaining data to the given handler
func streamToHandler(handler calque.Handler, ctx context.Context, initialChunk []byte, r io.Reader, w io.Writer) error {
	// Create a reader that provides the initial chunk followed by remaining data
	combinedReader := io.MultiReader(bytes.NewReader(initialChunk), r)
	req := calque.NewRequest(ctx, combinedReader)
	res := calque.NewResponse(w)
	return handler.ServeFlow(req, res)
}

// bufferToHandler buffers all input and passes it to the given handler
func bufferToHandler(handler calque.Handler, ctx context.Context, initialChunk []byte, r io.Reader, w io.Writer) error {
	// Read remaining data and combine with initial chunk
	remainingData, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Combine initial chunk with remaining data
	var fullInput bytes.Buffer
	fullInput.Write(initialChunk)
	fullInput.Write(remainingData)

	// Pass combined input to handler
	req := calque.NewRequest(ctx, &fullInput)
	res := calque.NewResponse(w)
	return handler.ServeFlow(req, res)
}
