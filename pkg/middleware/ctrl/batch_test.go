package ctrl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestBatch(t *testing.T) {
	echoHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		return calque.Write(res, "processed:"+input)
	})

	uppercaseHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		return calque.Write(res, strings.ToUpper(input))
	})

	errorHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		return errors.New("handler error")
	})

	tests := []struct {
		name     string
		handler  calque.Handler
		maxSize  int
		maxWait  time.Duration
		inputs   []string
		expected []string
		wantErr  bool
	}{
		{
			name:     "single request below batch size",
			handler:  echoHandler,
			maxSize:  3,
			maxWait:  100 * time.Millisecond,
			inputs:   []string{"hello"},
			expected: []string{"processed:hello"},
			wantErr:  false,
		},
		{
			name:     "batch size reached",
			handler:  uppercaseHandler,
			maxSize:  2,
			maxWait:  1 * time.Second,
			inputs:   []string{"hello", "world"},
			expected: []string{"HELLO", "WORLD"},
			wantErr:  false,
		},
		{
			name:     "timeout triggers batch processing",
			handler:  uppercaseHandler,
			maxSize:  5,
			maxWait:  50 * time.Millisecond,
			inputs:   []string{"timeout", "test"},
			expected: []string{"TIMEOUT", "TEST"},
			wantErr:  false,
		},
		{
			name:     "empty input",
			handler:  echoHandler,
			maxSize:  2,
			maxWait:  100 * time.Millisecond,
			inputs:   []string{""},
			expected: []string{"processed:"},
			wantErr:  false,
		},
		{
			name:     "handler error affects all batch requests",
			handler:  errorHandler,
			maxSize:  2,
			maxWait:  100 * time.Millisecond,
			inputs:   []string{"fail1", "fail2"},
			expected: []string{"", ""},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchHandler := Batch(tt.handler, tt.maxSize, tt.maxWait)

			var wg sync.WaitGroup
			results := make([]string, len(tt.inputs))
			errors := make([]error, len(tt.inputs))

			for i, input := range tt.inputs {
				wg.Add(1)
				go func(index int, inp string) {
					defer wg.Done()

					var buf bytes.Buffer
					reader := strings.NewReader(inp)

					req := calque.NewRequest(context.Background(), reader)
					res := calque.NewResponse(&buf)
					err := batchHandler.ServeFlow(req, res)
					results[index] = buf.String()
					errors[index] = err
				}(i, input)
			}

			wg.Wait()

			for i, result := range results {
				if tt.wantErr {
					if errors[i] == nil {
						t.Errorf("Expected error for input %d, got nil", i)
					}
				} else {
					if errors[i] != nil {
						t.Errorf("Unexpected error for input %d: %v", i, errors[i])
					}

					if len(tt.expected) > i && result != tt.expected[i] {
						t.Errorf("Input %d: got %q, want %q", i, result, tt.expected[i])
					}
				}
			}
		})
	}
}

func TestBatchProcessingOrder(t *testing.T) {
	orderHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		parts := strings.Split(input, "\n---BATCH_SEPARATOR---\n")

		var processedParts []string
		for _, part := range parts {
			processedParts = append(processedParts, "order:"+part)
		}

		result := strings.Join(processedParts, "\n---BATCH_SEPARATOR---\n")
		return calque.Write(res, result)
	})

	batchHandler := Batch(orderHandler, 3, 100*time.Millisecond)

	inputs := []string{"first", "second", "third"}
	expected := []string{"order:first", "order:second", "order:third"}

	var wg sync.WaitGroup
	results := make([]string, len(inputs))

	for i, input := range inputs {
		wg.Add(1)
		go func(index int, inp string) {
			defer wg.Done()

			var buf bytes.Buffer
			reader := strings.NewReader(inp)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := batchHandler.ServeFlow(req, res)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			results[index] = buf.String()
		}(i, input)
	}

	wg.Wait()

	for i, result := range results {
		if result != expected[i] {
			t.Errorf("Input %d: got %q, want %q", i, result, expected[i])
		}
	}
}

func TestBatchContextCancellation(t *testing.T) {
	slowHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		select {
		case <-req.Context.Done():
			return req.Context.Err()
		case <-time.After(200 * time.Millisecond):
			var input string
			if err := calque.Read(req, &input); err != nil {
				return err
			}
			return calque.Write(res, "slow:"+input)
		}
	})

	batchHandler := Batch(slowHandler, 2, 1*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	reader := strings.NewReader("cancel-test")

	req := calque.NewRequest(ctx, reader)
	res := calque.NewResponse(&buf)
	err := batchHandler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestBatchResponseSplittingFailure(t *testing.T) {
	malformedHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		calque.Read(req, &input)
		t.Logf("Handler received input: %q", input)
		return calque.Write(res, "response-without-separators")
	})

	batchHandler := Batch(malformedHandler, 2, 100*time.Millisecond)

	inputs := []string{"input1", "input2"}
	var wg sync.WaitGroup
	results := make([]string, len(inputs))
	errors := make([]error, len(inputs))

	for i, input := range inputs {
		wg.Add(1)
		go func(index int, inp string) {
			defer wg.Done()

			var buf bytes.Buffer
			reader := strings.NewReader(inp)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := batchHandler.ServeFlow(req, res)
			results[index] = buf.String()
			errors[index] = err
			t.Logf("Request %d: result=%q, error=%v", index, results[index], errors[index])
		}(i, input)
	}

	wg.Wait()

	// Check what actually happened
	t.Logf("Final results: %v", results)
	t.Logf("Final errors: %v", errors)

	// When splitting fails, one request gets the full response and others get errors
	// Due to concurrent processing, we can't predict which request will be first
	successCount := 0
	errorCount := 0

	for i, err := range errors {
		if err == nil {
			successCount++
			if results[i] != "response-without-separators" {
				t.Errorf("Successful request got %q, want %q", results[i], "response-without-separators")
			}
		} else {
			errorCount++
			if !strings.Contains(err.Error(), "batch response splitting failed") {
				t.Errorf("Failed request error = %v, want splitting failure error", err)
			}
			if results[i] != "" {
				t.Errorf("Failed request should have empty result, got %q", results[i])
			}
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful request, got %d", successCount)
	}
	if errorCount != 1 {
		t.Errorf("Expected exactly 1 failed request, got %d", errorCount)
	}
}

func TestBatchEmptyBatch(t *testing.T) {
	handler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		return calque.Write(res, "should-not-be-called")
	})

	batcher := &requestBatcher{
		handler: handler,
		maxSize: 5,
		maxWait: 100 * time.Millisecond,
	}

	batcher.processBatch([]*batchRequest{})
}

func TestBatchConcurrentRequests(t *testing.T) {
	concurrentHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		t.Logf("Handler processing batch input: %q", input)

		// Split the batched input and process each part
		parts := strings.Split(input, "\n---BATCH_SEPARATOR---\n")
		var processedParts []string
		for _, part := range parts {
			processedParts = append(processedParts, "concurrent:"+part)
		}
		result := strings.Join(processedParts, "\n---BATCH_SEPARATOR---\n")

		t.Logf("Handler returning: %q", result)
		return calque.Write(res, result)
	})

	batchHandler := Batch(concurrentHandler, 5, 200*time.Millisecond)

	numRequests := 10
	var wg sync.WaitGroup
	results := make([]string, numRequests)
	errors := make([]error, numRequests)

	for i := range numRequests {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			var buf bytes.Buffer
			input := fmt.Sprintf("request-%d", index)
			reader := strings.NewReader(input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := batchHandler.ServeFlow(req, res)
			results[index] = buf.String()
			errors[index] = err
		}(i)
	}

	wg.Wait()

	successCount := 0
	for i, err := range errors {
		if err != nil {
			t.Errorf("Request %d failed with error: %v", i, err)
		} else {
			successCount++
			if !strings.HasPrefix(results[i], "concurrent:") {
				t.Errorf("Request %d got unexpected result: %q", i, results[i])
			}
		}
	}

	if successCount != numRequests {
		t.Errorf("Expected %d successful requests, got %d", numRequests, successCount)
	}
}

func TestBatchTimerBehavior(t *testing.T) {
	callCount := 0
	timerHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		callCount++
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		return calque.Write(res, fmt.Sprintf("call-%d:%s", callCount, input))
	})

	batchHandler := Batch(timerHandler, 10, 100*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	var buf bytes.Buffer
	reader := strings.NewReader("timer-test")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := batchHandler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 handler call, got %d", callCount)
	}

	expected := "call-1:timer-test"
	if got := buf.String(); got != expected {
		t.Errorf("Got %q, want %q", got, expected)
	}
}
