package ctrl

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// DefaultBatchSeparator is the default separator used to combine and split
// batch items. It's designed to be unlikely to appear in normal data content.
const DefaultBatchSeparator = "\n---BATCH_SEPARATOR---\n"

type BatchConfig struct {
	MaxSize   int
	MaxWait   time.Duration
	Separator string
}

type requestBatcher struct {
	handler   calque.Handler
	maxSize   int
	maxWait   time.Duration
	separator string
	requests  chan *batchRequest
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
//	batch := ctrl.Batch(handler, 10, 5*time.Second) // 10 items or 5s
func Batch(handler calque.Handler, maxSize int, maxWait time.Duration) calque.Handler {
	return BatchWithConfig(handler, &BatchConfig{
		MaxSize:   maxSize,
		MaxWait:   maxWait,
		Separator: DefaultBatchSeparator,
	})
}

// BatchWithConfig accumulates multiple requests and processes them together with custom configuration
//
// Input: any data type (buffered - accumulates inputs)
// Output: combined results from batch processing
// Behavior: BUFFERED - accumulates until size/time threshold met
//
// Collects inputs until either config.MaxSize items accumulated or config.MaxWait time
// elapsed, then processes the batch through the handler. Results are distributed back
// to waiting requests in order using the specified separator.
//
// Example:
//
//	config := &ctrl.BatchConfig{
//		MaxSize:   10,
//		MaxWait:   5*time.Second,
//		Separator: " ||| ",
//	}
//	batch := ctrl.BatchWithConfig(handler, config)
func BatchWithConfig(handler calque.Handler, config *BatchConfig) calque.Handler {
	batcher := &requestBatcher{
		handler:   handler,
		maxSize:   config.MaxSize,
		maxWait:   config.MaxWait,
		separator: config.Separator,
		requests:  make(chan *batchRequest, config.MaxSize*2),
	}

	go batcher.processBatches()

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input []byte
		err := calque.Read(req, &input)
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
				// Write the processed batch response
				return calque.Write(res, resp.data)
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
	// Calculate total size to avoid buffer growth
	totalSize := (len(batch) - 1) * len(rb.separator)
	for _, req := range batch {
		totalSize += len(req.input)
	}

	// Pre-allocate buffer with exact capacity
	combinedInput := make([]byte, 0, totalSize)
	for i, req := range batch {
		if i > 0 {
			combinedInput = append(combinedInput, rb.separator...)
		}
		combinedInput = append(combinedInput, req.input...)
	}

	// Process the combined batch
	var output bytes.Buffer
	ctx := batch[0].ctx // Use context from first request
	req := calque.NewRequest(ctx, bytes.NewReader(combinedInput))
	res := calque.NewResponse(&output)
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
	responses := bytes.Split(output.Bytes(), []byte(rb.separator))

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
