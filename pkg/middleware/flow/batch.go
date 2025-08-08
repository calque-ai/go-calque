package flow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/calque-ai/calque-pipe/pkg/core"
)

type requestBatcher struct {
	handler  core.Handler
	maxSize  int
	maxWait  time.Duration
	requests chan *batchRequest
}

type batchRequest struct {
	input    []byte
	response chan batchResponse
	ctx      context.Context
}

type batchResponse struct {
	data []byte
	err  error
}

// Batch accumulates multiple requests and processes them together
//
// Input: any data type (buffered - accumulates inputs)
// Output: combined results from batch processing
// Behavior: BUFFERED - accumulates until size/time threshold met
//
// Collects inputs until either maxSize items accumulated or maxWait time
// elapsed, then processes the batch through the handler. Results are
// distributed back to waiting requests in order.
//
// Example:
//
//	batch := flow.Batch(handler, 10, 5*time.Second) // 10 items or 5s
func Batch(handler core.Handler, maxSize int, maxWait time.Duration) core.Handler {
	batcher := &requestBatcher{
		handler:  handler,
		maxSize:  maxSize,
		maxWait:  maxWait,
		requests: make(chan *batchRequest, maxSize*2),
	}

	go batcher.processBatches()

	return core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}

		batchReq := &batchRequest{
			input:    input,
			response: make(chan batchResponse, 1),
			ctx:      req.Context,
		}

		select {
		case batcher.requests <- batchReq:
			select {
			case resp := <-batchReq.response:
				if resp.err != nil {
					return resp.err
				}
				_, err := res.Data.Write(resp.data)
				return err
			case <-req.Context.Done():
				return req.Context.Err()
			}
		case <-req.Context.Done():
			return req.Context.Err()
		}
	})
}

// processBatches runs in background to collect and process batches
func (rb *requestBatcher) processBatches() {
	var batch []*batchRequest
	timer := time.NewTimer(rb.maxWait)
	timer.Stop() // Don't start timer until we have requests

	for {
		select {
		case req := <-rb.requests:
			if req == nil {
				return // Channel closed, shutdown
			}

			batch = append(batch, req)

			// Start timer on first request in batch
			if len(batch) == 1 {
				timer.Reset(rb.maxWait)
			}

			// Process batch if it's full
			if len(batch) >= rb.maxSize {
				timer.Stop()
				rb.processBatch(batch)
				batch = nil
			}

		case <-timer.C:
			// Process batch if timer expired and we have requests
			if len(batch) > 0 {
				rb.processBatch(batch)
				batch = nil
			}
		}
	}
}

// processBatch handles a single batch of requests
func (rb *requestBatcher) processBatch(batch []*batchRequest) {
	if len(batch) == 0 {
		return
	}

	// Combine all inputs with separators
	var combinedInput bytes.Buffer
	for i, req := range batch {
		if i > 0 {
			combinedInput.WriteString("\n---BATCH_SEPARATOR---\n")
		}
		combinedInput.Write(req.input)
	}

	// Process the combined batch
	var output bytes.Buffer
	ctx := batch[0].ctx // Use context from first request
	req := core.NewRequest(ctx, &combinedInput)
	res := core.NewResponse(&output)
	err := rb.handler.ServeFlow(req, res)

	if err != nil {
		// If batch processing fails, send error to all requests
		for _, req := range batch {
			select {
			case req.response <- batchResponse{nil, err}:
			case <-req.ctx.Done():
			}
		}
		return
	}

	// Split the output back to individual responses
	responses := bytes.Split(output.Bytes(), []byte("\n---BATCH_SEPARATOR---\n"))

	// Ensure we have the right number of responses
	if len(responses) != len(batch) {
		// If we can't split properly, return the full response to the first request
		// and errors to the rest
		for i, req := range batch {
			var resp batchResponse
			if i == 0 {
				resp = batchResponse{output.Bytes(), nil}
			} else {
				resp = batchResponse{nil, fmt.Errorf("batch response splitting failed")}
			}

			select {
			case req.response <- resp:
			case <-req.ctx.Done():
			}
		}
		return
	}

	// Send individual responses back
	for i, req := range batch {
		select {
		case req.response <- batchResponse{responses[i], nil}:
		case <-req.ctx.Done():
		}
	}
}
