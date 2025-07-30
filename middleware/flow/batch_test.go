package flow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calque-ai/calque-pipe/core"
)

func TestBatch(t *testing.T) {
	echoHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("processed:" + string(input)))
		return err
	})

	uppercaseHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(strings.ToUpper(string(input))))
		return err
	})

	errorHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		return errors.New("handler error")
	})

	tests := []struct {
		name     string
		handler  core.Handler
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
			batchHandler := Batch[string](tt.handler, tt.maxSize, tt.maxWait)

			var wg sync.WaitGroup
			results := make([]string, len(tt.inputs))
			errors := make([]error, len(tt.inputs))

			for i, input := range tt.inputs {
				wg.Add(1)
				go func(index int, inp string) {
					defer wg.Done()

					var buf bytes.Buffer
					reader := strings.NewReader(inp)

					err := batchHandler.ServeFlow(context.Background(), reader, &buf)
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
	orderHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		inputStr := string(input)
		parts := strings.Split(inputStr, "\n---BATCH_SEPARATOR---\n")

		var processedParts []string
		for _, part := range parts {
			processedParts = append(processedParts, "order:"+part)
		}

		result := strings.Join(processedParts, "\n---BATCH_SEPARATOR---\n")
		_, err = w.Write([]byte(result))
		return err
	})

	batchHandler := Batch[string](orderHandler, 3, 100*time.Millisecond)

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

			err := batchHandler.ServeFlow(context.Background(), reader, &buf)
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
	slowHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
			input, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("slow:" + string(input)))
			return err
		}
	})

	batchHandler := Batch[string](slowHandler, 2, 1*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	reader := strings.NewReader("cancel-test")

	err := batchHandler.ServeFlow(ctx, reader, &buf)
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestBatchResponseSplittingFailure(t *testing.T) {
	malformedHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, _ := io.ReadAll(r)
		t.Logf("Handler received input: %q", string(input))
		_, err := w.Write([]byte("response-without-separators"))
		return err
	})

	batchHandler := Batch[string](malformedHandler, 2, 100*time.Millisecond)

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

			err := batchHandler.ServeFlow(context.Background(), reader, &buf)
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
	handler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		_, err := w.Write([]byte("should-not-be-called"))
		return err
	})

	batcher := &requestBatcher{
		handler: handler,
		maxSize: 5,
		maxWait: 100 * time.Millisecond,
	}

	batcher.processBatch([]*batchRequest{})
}

func TestBatchConcurrentRequests(t *testing.T) {
	concurrentHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		t.Logf("Handler processing batch input: %q", string(input))

		// Split the batched input and process each part
		parts := strings.Split(string(input), "\n---BATCH_SEPARATOR---\n")
		var processedParts []string
		for _, part := range parts {
			processedParts = append(processedParts, "concurrent:"+part)
		}
		result := strings.Join(processedParts, "\n---BATCH_SEPARATOR---\n")

		t.Logf("Handler returning: %q", result)
		_, err = w.Write([]byte(result))
		return err
	})

	batchHandler := Batch[string](concurrentHandler, 5, 200*time.Millisecond)

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

			err := batchHandler.ServeFlow(context.Background(), reader, &buf)
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
	timerHandler := core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		callCount++
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		_, err = w.Write(fmt.Appendf(nil, "call-%d:%s", callCount, string(input)))
		return err
	})

	batchHandler := Batch[string](timerHandler, 10, 100*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	var buf bytes.Buffer
	reader := strings.NewReader("timer-test")

	err := batchHandler.ServeFlow(context.Background(), reader, &buf)
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
