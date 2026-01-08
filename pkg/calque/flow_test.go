package calque

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test helper handlers
func TestNew(t *testing.T) {
	flow := NewFlow()
	if flow == nil {
		t.Fatal("Flow() returned nil")
	}
	if len(flow.handlers) != 0 {
		t.Errorf("Flow() handlers length = %d, want 0", len(flow.handlers))
	}
}

func TestNewFlow_Configuration(t *testing.T) {
	tests := []struct {
		name         string
		config       *FlowConfig
		expectSemNil bool
		expectSemCap int
		description  string
	}{
		{
			name:         "default_no_config",
			config:       nil,
			expectSemNil: true,
			description:  "Default should use unlimited concurrency (sem = nil)",
		},
		{
			name:         "unlimited_explicit",
			config:       &FlowConfig{MaxConcurrent: ConcurrencyUnlimited},
			expectSemNil: true,
			description:  "ConcurrencyUnlimited should set sem = nil",
		},
		{
			name:         "auto_with_default_multiplier",
			config:       &FlowConfig{MaxConcurrent: ConcurrencyAuto},
			expectSemNil: false,
			expectSemCap: runtime.GOMAXPROCS(0) * DefaultCPUMultiplier,
			description:  "ConcurrencyAuto should calculate semaphore size from CPU cores",
		},
		{
			name:         "auto_with_custom_multiplier",
			config:       &FlowConfig{MaxConcurrent: ConcurrencyAuto, CPUMultiplier: 100},
			expectSemNil: false,
			expectSemCap: runtime.GOMAXPROCS(0) * 100,
			description:  "ConcurrencyAuto should use custom multiplier",
		},
		{
			name:         "auto_with_zero_multiplier",
			config:       &FlowConfig{MaxConcurrent: ConcurrencyAuto, CPUMultiplier: 0},
			expectSemNil: false,
			expectSemCap: runtime.GOMAXPROCS(0) * DefaultCPUMultiplier,
			description:  "ConcurrencyAuto with zero multiplier should use default",
		},
		{
			name:         "auto_with_negative_multiplier",
			config:       &FlowConfig{MaxConcurrent: ConcurrencyAuto, CPUMultiplier: -5},
			expectSemNil: false,
			expectSemCap: runtime.GOMAXPROCS(0) * DefaultCPUMultiplier,
			description:  "ConcurrencyAuto with negative multiplier should use default",
		},
		{
			name:         "fixed_positive",
			config:       &FlowConfig{MaxConcurrent: 50},
			expectSemNil: false,
			expectSemCap: 50,
			description:  "Fixed positive value should create semaphore with that capacity",
		},
		{
			name:         "fixed_small",
			config:       &FlowConfig{MaxConcurrent: 1},
			expectSemNil: false,
			expectSemCap: 1,
			description:  "Fixed value of 1 should work",
		},
		{
			name:         "negative_value_treated_as_unlimited",
			config:       &FlowConfig{MaxConcurrent: -5},
			expectSemNil: true,
			description:  "Negative values (other than ConcurrencyAuto) should be unlimited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var flow *Flow
			if tt.config == nil {
				flow = NewFlow()
			} else {
				flow = NewFlow(*tt.config)
			}

			if flow == nil {
				t.Fatal("NewFlow returned nil")
			}

			// Test semaphore presence
			if tt.expectSemNil {
				if flow.sem != nil {
					t.Errorf("%s: expected sem = nil but got non-nil semaphore", tt.description)
				}
			} else {
				if flow.sem == nil {
					t.Errorf("%s: expected non-nil semaphore but got nil", tt.description)
				} else if cap(flow.sem) != tt.expectSemCap {
					t.Errorf("%s: expected semaphore capacity %d but got %d",
						tt.description, tt.expectSemCap, cap(flow.sem))
				}
			}

			// Test that flow is functional
			if len(flow.handlers) != 0 {
				t.Errorf("Flow should start with 0 handlers, got %d", len(flow.handlers))
			}
		})
	}
}

func TestFlow_Use(t *testing.T) {
	flow := NewFlow()

	handler1 := HandlerFunc(func(_ *Request, _ *Response) error {
		return nil
	})

	handler2 := HandlerFunc(func(_ *Request, _ *Response) error {
		return nil
	})

	// Test chaining
	result := flow.Use(handler1).Use(handler2)

	// Should return the same flow instance for chaining
	if result != flow {
		t.Error("Use() should return the same flow instance for chaining")
	}

	// Should have added handlers
	if len(flow.handlers) != 2 {
		t.Errorf("Use() handlers length = %d, want 2", len(flow.handlers))
	}
}

func TestFlow_UseFunc(t *testing.T) {
	flow := NewFlow()

	handlerFunc := func(_ *Request, _ *Response) error {
		return nil
	}

	result := flow.UseFunc(handlerFunc)

	// Should return the same flow instance for chaining
	if result != flow {
		t.Error("UseFunc() should return the same flow instance for chaining")
	}

	// Should have added the handler
	if len(flow.handlers) != 1 {
		t.Errorf("UseFunc() handlers length = %d, want 1", len(flow.handlers))
	}
}

func TestFlow_Run_NoHandlers(t *testing.T) {
	flow := NewFlow()

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "string input",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "byte slice input",
			input:    []byte("byte data"),
			expected: "byte data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output string
			err := flow.Run(context.Background(), tt.input, &output)

			if err != nil {
				t.Errorf("Run() error = %v, want nil", err)
			}

			if output != tt.expected {
				t.Errorf("Run() output = %q, want %q", output, tt.expected)
			}
		})
	}
}

func TestFlow_Run_SingleHandler(t *testing.T) {
	tests := []struct {
		name     string
		handler  Handler
		input    string
		expected string
		wantErr  bool
	}{
		{
			name: "echo handler",
			handler: HandlerFunc(func(req *Request, res *Response) error {
				_, err := io.Copy(res.Data, req.Data)
				return err
			}),
			input:    "echo test",
			expected: "echo test",
			wantErr:  false,
		},
		{
			name: "transform handler",
			handler: HandlerFunc(func(req *Request, res *Response) error {
				data, err := io.ReadAll(req.Data)
				if err != nil {
					return err
				}
				_, err = res.Data.Write([]byte(strings.ToUpper(string(data))))
				return err
			}),
			input:    "transform me",
			expected: "TRANSFORM ME",
			wantErr:  false,
		},
		{
			name: "error handler",
			handler: HandlerFunc(func(_ *Request, _ *Response) error {
				return errors.New("handler error")
			}),
			input:   "error test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewFlow().Use(tt.handler)

			var output string
			err := flow.Run(context.Background(), tt.input, &output)

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && output != tt.expected {
				t.Errorf("Run() output = %q, want %q", output, tt.expected)
			}
		})
	}
}

func TestFlow_Run_MultipleHandlers(t *testing.T) {
	// Create a pipeline: input -> uppercase -> add prefix -> add suffix
	uppercaseHandler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte(strings.ToUpper(string(data))))
		return err
	})

	prefixHandler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("PREFIX:" + string(data)))
		return err
	})

	suffixHandler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte(string(data) + ":SUFFIX"))
		return err
	})

	flow := NewFlow().
		Use(uppercaseHandler).
		Use(prefixHandler).
		Use(suffixHandler)

	var output string
	err := flow.Run(context.Background(), "hello world", &output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	expected := "PREFIX:HELLO WORLD:SUFFIX"
	if output != expected {
		t.Errorf("Run() output = %q, want %q", output, expected)
	}
}

func TestFlow_Run_HandlerError(t *testing.T) {
	// Create handlers where the second one fails
	handler1 := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("processed:" + string(data)))
		return err
	})

	errorHandler := HandlerFunc(func(_ *Request, _ *Response) error {
		return errors.New("processing failed")
	})

	flow := NewFlow().
		Use(handler1).
		Use(errorHandler)

	var output string
	err := flow.Run(context.Background(), "test input", &output)

	if err == nil {
		t.Error("Run() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "processing failed") {
		t.Errorf("Run() error = %v, want error containing 'processing failed'", err)
	}
}

func TestFlow_Run_ConcurrentHandlerError(t *testing.T) {
	// Test that when one handler fails, the flow returns an error
	// Note: Due to concurrent execution, any handler might fail first
	var executionCount int64

	handler1 := HandlerFunc(func(_ *Request, _ *Response) error {
		atomic.AddInt64(&executionCount, 1)
		time.Sleep(10 * time.Millisecond) // Small delay
		return errors.New("handler1 failed")
	})

	handler2 := HandlerFunc(func(_ *Request, _ *Response) error {
		atomic.AddInt64(&executionCount, 1)
		time.Sleep(5 * time.Millisecond) // Shorter delay, likely to complete first
		return errors.New("handler2 failed")
	})

	flow := NewFlow().Use(handler1).Use(handler2)

	var output string
	err := flow.Run(context.Background(), "test input", &output)

	if err == nil {
		t.Error("Run() error = nil, want error from one of the handlers")
	}

	// Should get an error from one of the handlers
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "failed") {
		t.Errorf("Run() error = %v, want error containing 'failed'", err)
	}

	// Both handlers should have started (due to concurrent execution)
	if atomic.LoadInt64(&executionCount) == 0 {
		t.Error("Expected at least one handler to execute")
	}
}

func TestFlow_Run_ContextCancellation(t *testing.T) {
	// Create a handler that checks for context cancellation
	blockingHandler := HandlerFunc(func(req *Request, res *Response) error {
		select {
		case <-req.Context.Done():
			return req.Context.Err()
		case <-time.After(100 * time.Millisecond):
			// Should not reach here if context is cancelled quickly
			_, err := res.Data.Write([]byte("completed"))
			return err
		}
	})

	flow := NewFlow().Use(blockingHandler)

	// Create a context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	var output string
	err := flow.Run(ctx, "test", &output)

	if err == nil {
		t.Error("Run() error = nil, want context cancellation error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Run() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestFlow_Run_ConcurrentExecution(t *testing.T) {
	// Test that handlers can process data concurrently/in streaming fashion
	var startTimes [3]time.Time
	var completeTimes [3]time.Time

	// Create handlers that record timing
	handler1 := HandlerFunc(func(req *Request, res *Response) error {
		startTimes[0] = time.Now()
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)
		_, err = res.Data.Write([]byte("h1:" + string(data)))
		completeTimes[0] = time.Now()
		return err
	})

	handler2 := HandlerFunc(func(req *Request, res *Response) error {
		startTimes[1] = time.Now()
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
		_, err = res.Data.Write([]byte("h2:" + string(data)))
		completeTimes[1] = time.Now()
		return err
	})

	handler3 := HandlerFunc(func(req *Request, res *Response) error {
		startTimes[2] = time.Now()
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
		_, err = res.Data.Write([]byte("h3:" + string(data)))
		completeTimes[2] = time.Now()
		return err
	})

	flow := NewFlow().
		Use(handler1).
		Use(handler2).
		Use(handler3)

	start := time.Now()
	var output string
	err := flow.Run(context.Background(), "concurrent", &output)
	totalTime := time.Since(start)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	expected := "h3:h2:h1:concurrent"
	if output != expected {
		t.Errorf("Run() output = %q, want %q", output, expected)
	}

	// Verify concurrent execution - total time should be less than sum of handler times
	// Since handlers run concurrently, total time should be closer to the longest handler time
	// rather than the sum of all handler times
	if totalTime > 200*time.Millisecond {
		t.Errorf("Run() took %v, expected concurrent execution to be faster", totalTime)
	}
}

func TestFlow_Run_StreamingBehavior(t *testing.T) {
	// Test that data can flow between handlers in a streaming fashion
	var processOrder []string
	var mu sync.Mutex

	addToOrder := func(name string) {
		mu.Lock()
		processOrder = append(processOrder, name)
		mu.Unlock()
	}

	handler1 := HandlerFunc(func(req *Request, res *Response) error {
		addToOrder("h1-start")
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("h1:" + string(data)))
		addToOrder("h1-end")
		return err
	})

	handler2 := HandlerFunc(func(req *Request, res *Response) error {
		addToOrder("h2-start")
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write([]byte("h2:" + string(data)))
		addToOrder("h2-end")
		return err
	})

	flow := NewFlow().Use(handler1).Use(handler2)

	var output string
	err := flow.Run(context.Background(), "stream", &output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Handlers should be able to start concurrently
	mu.Lock()
	order := make([]string, len(processOrder))
	copy(order, processOrder)
	mu.Unlock()

	if len(order) != 4 {
		t.Errorf("Expected 4 process events, got %d: %v", len(order), order)
	}
}

func TestFlow_Run_EmptyInput(t *testing.T) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			_, err = res.Data.Write([]byte("empty-processed"))
		} else {
			_, err = res.Data.Write(data)
		}
		return err
	})

	flow := NewFlow().Use(handler)

	var output string
	err := flow.Run(context.Background(), "", &output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if output != "empty-processed" {
		t.Errorf("Run() output = %q, want %q", output, "empty-processed")
	}
}

func TestFlow_Run_LargeData(t *testing.T) {
	// Test with large data to ensure streaming works properly
	largeInput := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 10000) // ~260KB

	handler := HandlerFunc(func(req *Request, res *Response) error {
		// Process data in chunks to simulate streaming
		buffer := make([]byte, 1024)
		for {
			n, err := req.Data.Read(buffer)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			_, err = res.Data.Write(buffer[:n])
			if err != nil {
				return err
			}
		}
		return nil
	})

	flow := NewFlow().Use(handler)

	var output string
	err := flow.Run(context.Background(), largeInput, &output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if output != largeInput {
		t.Errorf("Run() failed to process large data correctly, lengths: got %d, want %d",
			len(output), len(largeInput))
	}
}

func TestFlow_Run_MultipleGoroutines(t *testing.T) {
	// Test that flow can be used concurrently from multiple goroutines
	handler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond) // Simulate processing
		_, err = res.Data.Write([]byte("processed:" + string(data)))
		return err
	})

	flow := NewFlow().Use(handler)

	var wg sync.WaitGroup
	var successCount int64
	numGoroutines := 10

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			var output string
			input := fmt.Sprintf("input-%d", id)
			err := flow.Run(context.Background(), input, &output)

			if err == nil && strings.Contains(output, input) {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount != int64(numGoroutines) {
		t.Errorf("Expected %d successful runs, got %d", numGoroutines, successCount)
	}
}

// Edge case tests
func TestFlow_Run_PipeClosureHandling(t *testing.T) {
	// Test that pipe closure is handled gracefully
	closingHandler := HandlerFunc(func(req *Request, res *Response) error {
		// Read some data
		buffer := make([]byte, 10)
		n, err := req.Data.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}

		// Close writer immediately (simulating early closure)
		if closer, ok := res.Data.(io.Closer); ok {
			closer.Close()
		}

		// Try to write after closing (should handle gracefully)
		_, writeErr := res.Data.Write(buffer[:n])
		return writeErr
	})

	flow := NewFlow().Use(closingHandler)

	var output string
	err := flow.Run(context.Background(), "test", &output)

	// Should handle pipe closure gracefully
	if err == nil {
		t.Error("Expected error due to pipe closure, got nil")
	}
}

func TestFlow_Run_InputOutputTypes(t *testing.T) {
	// Test various input/output type combinations
	echoHandler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	flow := NewFlow().Use(echoHandler)

	tests := []struct {
		name   string
		input  any
		output any
		verify func(t *testing.T, input, output any)
	}{
		{
			name:   "string to string",
			input:  "hello",
			output: new(string),
			verify: func(t *testing.T, input, output any) {
				if *(output.(*string)) != "hello" {
					t.Errorf("Expected %q, got %q", input, *(output.(*string)))
				}
			},
		},
		{
			name:   "bytes to bytes",
			input:  []byte("byte test"),
			output: new([]byte),
			verify: func(t *testing.T, input, output any) {
				expected := string(input.([]byte))
				actual := string(*(output.(*[]byte)))
				if actual != expected {
					t.Errorf("Expected %q, got %q", expected, actual)
				}
			},
		},
		{
			name:   "string to bytes",
			input:  "string to bytes",
			output: new([]byte),
			verify: func(t *testing.T, input, output any) {
				expected := input.(string)
				actual := string(*(output.(*[]byte)))
				if actual != expected {
					t.Errorf("Expected %q, got %q", expected, actual)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := flow.Run(context.Background(), tt.input, tt.output)
			if err != nil {
				t.Errorf("Run() error = %v, want nil", err)
				return
			}
			tt.verify(t, tt.input, tt.output)
		})
	}
}

func TestFlow_Run_PartialWrite(t *testing.T) {
	// Test handler that does partial writes
	partialHandler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}

		// Write data in small chunks to test streaming
		input := string(data)
		for i := 0; i < len(input); i += 2 {
			end := i + 2
			if end > len(input) {
				end = len(input)
			}
			_, err := res.Data.Write([]byte(input[i:end]))
			if err != nil {
				return err
			}
			// Small delay to simulate streaming
			time.Sleep(1 * time.Millisecond)
		}
		return nil
	})

	flow := NewFlow().Use(partialHandler)

	var output string
	err := flow.Run(context.Background(), "streaming test data", &output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if output != "streaming test data" {
		t.Errorf("Run() output = %q, want %q", output, "streaming test data")
	}
}

func TestFlow_Run_BinaryData(t *testing.T) {
	// Test handling of binary data
	binaryHandler := HandlerFunc(func(req *Request, res *Response) error {
		// Copy binary data exactly
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	flow := NewFlow().Use(binaryHandler)

	// Create test binary data
	binaryInput := make([]byte, 256)
	for i := 0; i < 256; i++ {
		binaryInput[i] = byte(i)
	}

	var output []byte
	err := flow.Run(context.Background(), binaryInput, &output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if !bytes.Equal(output, binaryInput) {
		t.Error("Binary data not preserved through flow")
	}
}

func TestFlow_Run_InvalidOutput(t *testing.T) {
	// Test that flow handles invalid output parameters gracefully
	flow := NewFlow()

	// Test with non-pointer output (should fail)
	var invalidOutput string // not a pointer
	err := flow.Run(context.Background(), "test", invalidOutput)
	if err == nil {
		t.Error("Expected error with non-pointer output, got nil")
	}
}

func TestFlow_Run_ResourceCleanup(t *testing.T) {
	// Test that resources are cleaned up properly
	var resourcesCleaned int64

	resourceHandler := HandlerFunc(func(req *Request, res *Response) error {
		// Simulate resource acquisition
		defer func() {
			atomic.AddInt64(&resourcesCleaned, 1)
		}()

		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}

		_, err = res.Data.Write(data)
		return err
	})

	flow := NewFlow().Use(resourceHandler)

	var output string
	err := flow.Run(context.Background(), "cleanup test", &output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Give some time for cleanup
	time.Sleep(10 * time.Millisecond)

	if atomic.LoadInt64(&resourcesCleaned) != 1 {
		t.Errorf("Expected 1 resource cleanup, got %d", atomic.LoadInt64(&resourcesCleaned))
	}
}

// Benchmark tests for performance
func BenchmarkFlow_Run_SingleHandler(b *testing.B) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	flow := NewFlow().Use(handler)
	input := "benchmark test data"

	for b.Loop() {
		var output string
		flow.Run(context.Background(), input, &output)
	}
}

func BenchmarkFlow_Run_MultipleHandlers(b *testing.B) {
	handler1 := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	handler2 := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	handler3 := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	flow := NewFlow().Use(handler1).Use(handler2).Use(handler3)
	input := "benchmark test data"

	for b.Loop() {
		var output string
		flow.Run(context.Background(), input, &output)
	}
}

func BenchmarkFlow_Run_DataSizes(b *testing.B) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	sizes := []struct {
		name string
		size int
	}{
		{"Small1KB", 1 * 1024},
		{"Medium100KB", 100 * 1024},
		{"Large1MB", 1024 * 1024},
		{"XLarge10MB", 10 * 1024 * 1024},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			data := make([]byte, s.size)
			for i := range data {
				data[i] = byte(i % 256)
			}

			flow := NewFlow().Use(handler)
			b.ResetTimer()
			b.SetBytes(int64(s.size))

			for b.Loop() {
				var output []byte
				err := flow.Run(context.Background(), data, &output)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFlow_Run_HandlerCounts(b *testing.B) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	counts := []int{1, 2, 5, 10, 20}
	input := "benchmark test data for handler scaling"

	for _, count := range counts {
		b.Run(fmt.Sprintf("Handlers%d", count), func(b *testing.B) {
			flow := NewFlow()
			for i := 0; i < count; i++ {
				flow.Use(handler)
			}

			b.ResetTimer()
			b.SetBytes(int64(len(input)))

			for b.Loop() {
				var output string
				err := flow.Run(context.Background(), input, &output)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFlow_Run_ProcessingTypes(b *testing.B) {
	passthroughHandler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	bufferedHandler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		_, err = res.Data.Write(data)
		return err
	})

	transformHandler := HandlerFunc(func(req *Request, res *Response) error {
		data, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		transformed := strings.ToUpper(string(data))
		_, err = res.Data.Write([]byte(transformed))
		return err
	})

	tests := []struct {
		name    string
		handler Handler
	}{
		{"Passthrough", passthroughHandler},
		{"Buffered", bufferedHandler},
		{"Transform", transformHandler},
	}

	input := "benchmark test data for processing types"

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			flow := NewFlow().Use(tt.handler)
			b.ResetTimer()
			b.SetBytes(int64(len(input)))

			for b.Loop() {
				var output string
				err := flow.Run(context.Background(), input, &output)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFlow_Run_ConcurrencyLevels(b *testing.B) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		time.Sleep(1 * time.Millisecond) // Simulate some work
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	configs := []struct {
		name   string
		config FlowConfig
	}{
		{"Unlimited", FlowConfig{MaxConcurrent: ConcurrencyUnlimited}},
		{"Auto", FlowConfig{MaxConcurrent: ConcurrencyAuto, CPUMultiplier: 50}},
		{"Fixed50", FlowConfig{MaxConcurrent: 50}},
		{"Fixed10", FlowConfig{MaxConcurrent: 10}},
	}

	input := "concurrency benchmark data"

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			flow := NewFlow(cfg.config).Use(handler)
			b.ResetTimer()
			b.SetBytes(int64(len(input)))

			for b.Loop() {
				var output string
				err := flow.Run(context.Background(), input, &output)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFlow_runWithStreaming(b *testing.B) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	sizes := []struct {
		name string
		size int
	}{
		{"Small1KB", 1 * 1024},
		{"Medium100KB", 100 * 1024},
		{"Large1MB", 1024 * 1024},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			data := make([]byte, s.size)
			for i := range data {
				data[i] = byte(i % 256)
			}

			flow := NewFlow().Use(handler)
			b.ResetTimer()
			b.SetBytes(int64(s.size))

			for b.Loop() {
				var output bytes.Buffer
				reader := bytes.NewReader(data)
				err := flow.runWithStreaming(context.Background(), reader, &output)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFlow_Run_ZeroCopy(b *testing.B) {
	input := "zero copy benchmark test data"

	b.Run("StringToString_NoHandlers", func(b *testing.B) {
		flow := NewFlow()
		b.SetBytes(int64(len(input)))
		for b.Loop() {
			var output string
			flow.Run(context.Background(), input, &output)
		}
	})

	b.Run("StringToString_WithHandler", func(b *testing.B) {
		handler := HandlerFunc(func(req *Request, res *Response) error {
			_, err := io.Copy(res.Data, req.Data)
			return err
		})
		flow := NewFlow().Use(handler)
		b.SetBytes(int64(len(input)))
		for b.Loop() {
			var output string
			flow.Run(context.Background(), input, &output)
		}
	})
}

func BenchmarkStringConversion(b *testing.B) {
	data := []byte("benchmark test data for string conversion efficiency")

	b.Run("StringsBuilder", func(b *testing.B) {
		for b.Loop() {
			reader := bytes.NewReader(data)
			var builder strings.Builder
			io.Copy(&builder, reader)
			_ = builder.String()
		}
	})

	b.Run("IoReadAll", func(b *testing.B) {
		for b.Loop() {
			reader := bytes.NewReader(data)
			data, _ := io.ReadAll(reader)
			_ = string(data)
		}
	})

	b.Run("BytesBuffer_Copy", func(b *testing.B) {
		for b.Loop() {
			reader := bytes.NewReader(data)
			var buf bytes.Buffer
			io.Copy(&buf, reader)
			_ = buf.String()
		}
	})
}

func BenchmarkStringConversion_LargeData(b *testing.B) {
	data := make([]byte, 100000) // 100KB test data
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.Run("StringsBuilder_100KB", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for b.Loop() {
			reader := bytes.NewReader(data)
			var builder strings.Builder
			io.Copy(&builder, reader)
			_ = builder.String()
		}
	})

	b.Run("IoReadAll_100KB", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for b.Loop() {
			reader := bytes.NewReader(data)
			data, _ := io.ReadAll(reader)
			_ = string(data)
		}
	})

	b.Run("BytesBuffer_Copy_100KB", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for b.Loop() {
			reader := bytes.NewReader(data)
			var buf bytes.Buffer
			io.Copy(&buf, reader)
			_ = buf.String()
		}
	})
}

func BenchmarkIOReaderVsRunWithStreaming(b *testing.B) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	input := "benchmark test data for io.Reader comparison"

	b.Run("Run_IoReader_Output", func(b *testing.B) {
		flow := NewFlow().Use(handler)
		b.SetBytes(int64(len(input)))
		for b.Loop() {
			var output io.Reader
			flow.Run(context.Background(), input, &output)
		}
	})

	b.Run("runWithStreaming_Pure", func(b *testing.B) {
		flow := NewFlow().Use(handler)
		b.SetBytes(int64(len(input)))
		for b.Loop() {
			var output bytes.Buffer
			reader := strings.NewReader(input)
			flow.runWithStreaming(context.Background(), reader, &output)
		}
	})
}

func TestFlow_AutoCreateMetadataBus(t *testing.T) {
	t.Run("auto-creates MetadataBus when not present", func(t *testing.T) {
		var capturedMB *MetadataBus

		handler := HandlerFunc(func(req *Request, res *Response) error {
			capturedMB = GetMetadataBus(req.Context)
			return Write(res, "done")
		})

		flow := NewFlow().Use(handler)

		ctx := context.Background()
		var output string
		err := flow.Run(ctx, "input", &output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedMB == nil {
			t.Error("expected MetadataBus to be auto-created")
		}
	})

	t.Run("uses existing MetadataBus from context", func(t *testing.T) {
		existingMB := NewMetadataBus(50)
		var capturedMB *MetadataBus

		handler := HandlerFunc(func(req *Request, res *Response) error {
			capturedMB = GetMetadataBus(req.Context)
			return Write(res, "done")
		})

		flow := NewFlow().Use(handler)

		ctx := WithMetadataBus(context.Background(), existingMB)
		var output string
		err := flow.Run(ctx, "input", &output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedMB != existingMB {
			t.Error("expected to use existing MetadataBus from context")
		}
	})

	t.Run("custom buffer size in config", func(t *testing.T) {
		var capturedMB *MetadataBus

		handler := HandlerFunc(func(req *Request, res *Response) error {
			capturedMB = GetMetadataBus(req.Context)
			return Write(res, "done")
		})

		flow := NewFlow(FlowConfig{MetadataBusBuffer: 200}).Use(handler)

		ctx := context.Background()
		var output string
		err := flow.Run(ctx, "input", &output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedMB == nil {
			t.Fatal("expected MetadataBus to be auto-created")
		}
		if capturedMB.BufferSize() != 200 {
			t.Errorf("expected buffer size 200, got %d", capturedMB.BufferSize())
		}
	})

	t.Run("handlers can communicate via MetadataBus", func(t *testing.T) {
		var receivedValue any
		var wg sync.WaitGroup
		wg.Add(1)

		handler1 := HandlerFunc(func(req *Request, res *Response) error {
			mb := GetMetadataBus(req.Context)
			mb.Set("shared_key", "shared_value")
			return Write(res, "from handler1")
		})

		handler2 := HandlerFunc(func(req *Request, res *Response) error {
			defer wg.Done()
			mb := GetMetadataBus(req.Context)
			// Small delay to ensure handler1 has time to set the value
			time.Sleep(10 * time.Millisecond)
			receivedValue, _ = mb.Get("shared_key")

			var input string
			Read(req, &input)
			return Write(res, input+" -> handler2")
		})

		flow := NewFlow().Use(handler1).Use(handler2)

		ctx := context.Background()
		var output string
		err := flow.Run(ctx, "input", &output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wg.Wait()
		if receivedValue != "shared_value" {
			t.Errorf("expected 'shared_value', got %v", receivedValue)
		}
	})
}

func BenchmarkByteOutput(b *testing.B) {
	handler := HandlerFunc(func(req *Request, res *Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	sizes := []struct {
		name string
		size int
	}{
		{"Small1KB", 1 * 1024},
		{"Medium100KB", 100 * 1024},
		{"Large1MB", 1024 * 1024},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			data := make([]byte, s.size)
			for i := range data {
				data[i] = byte(i % 256)
			}

			flow := NewFlow().Use(handler)
			b.ResetTimer()
			b.SetBytes(int64(s.size))

			for b.Loop() {
				var output []byte
				err := flow.Run(context.Background(), data, &output)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestFlow_Run_StreamingScenarios tests all 6 streaming scenarios to ensure
// proper streaming behavior and prevent memory leaks
func TestFlow_Run_StreamingScenarios(t *testing.T) {
	t.Run("Scenario1_LongStreamWithOutputConverter_KeepsUp", func(t *testing.T) {
		// Scenario 1: Long-running stream with output converter that keeps up
		// CRITICAL: Output must be consumed in real-time, not buffered

		flow := NewFlow()
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			// Simulate streaming: send data in chunks over time
			for i := 0; i < 10; i++ {
				_, err := res.Data.Write([]byte(fmt.Sprintf("chunk-%d\n", i)))
				if err != nil {
					return err
				}
				time.Sleep(10 * time.Millisecond) // Simulate slow production
			}
			return nil
		}))

		// Create an OutputConverter that tracks when data arrives
		received := make([]string, 0)
		var mu sync.Mutex
		startTime := time.Now()
		chunkTimes := make([]time.Duration, 0)

		converter := &testOutputConverter{
			fromReader: func(r io.Reader) error {
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					mu.Lock()
					received = append(received, scanner.Text())
					chunkTimes = append(chunkTimes, time.Since(startTime))
					mu.Unlock()
				}
				return scanner.Err()
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := flow.Run(ctx, "", converter)
		if err != nil {
			t.Fatalf("Flow.Run failed: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()

		// Verify all chunks received
		if len(received) != 10 {
			t.Errorf("Expected 10 chunks, got %d", len(received))
		}

		// CRITICAL: Verify streaming - first chunk should arrive early (not buffered)
		// If buffered, all chunks would arrive at ~100ms. If streaming, first arrives ~10ms
		if len(chunkTimes) > 0 && chunkTimes[0] > 50*time.Millisecond {
			t.Errorf("First chunk arrived too late (%v), indicates buffering instead of streaming", chunkTimes[0])
		}
	})

	t.Run("Scenario2_LongStreamWithOutputConverter_Slow", func(t *testing.T) {
		// Scenario 2: Long-running stream where output consumer is slower than producer
		// CRITICAL: Should use pipe backpressure, not unbounded buffering

		flow := NewFlow()
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			// Fast producer: send 100 chunks quickly
			for i := 0; i < 100; i++ {
				_, err := res.Data.Write([]byte(fmt.Sprintf("data-%d\n", i)))
				if err != nil {
					return err
				}
			}
			return nil
		}))

		received := make([]string, 0)
		var mu sync.Mutex

		converter := &testOutputConverter{
			fromReader: func(r io.Reader) error {
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					// Slow consumer: process each chunk slowly
					time.Sleep(5 * time.Millisecond)
					mu.Lock()
					received = append(received, scanner.Text())
					mu.Unlock()
				}
				return scanner.Err()
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := flow.Run(ctx, "", converter)
		if err != nil {
			t.Fatalf("Flow.Run failed: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()

		if len(received) != 100 {
			t.Errorf("Expected 100 chunks, got %d", len(received))
		}

		// Test passes if no OOM and all data received
		// Pipe backpressure should limit memory growth
	})

	t.Run("Scenario3_LongStreamWithNilOutput", func(t *testing.T) {
		// Scenario 3: Long-running stream with output not needed
		// CRITICAL: Must discard output without buffering (prevent memory leak)

		flow := NewFlow()
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			// Produce large amounts of data
			for i := 0; i < 1000; i++ {
				_, err := res.Data.Write([]byte(fmt.Sprintf("discarded-data-%d\n", i)))
				if err != nil {
					return err
				}
			}
			return nil
		}))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Run with nil output - should discard without buffering
		err := flow.Run(ctx, "", nil)
		if err != nil {
			t.Fatalf("Flow.Run with nil output failed: %v", err)
		}

		// Test passes if no OOM and completes quickly
		// If buffering occurred, would use ~50KB+ memory
	})

	t.Run("Scenario4_ShortInputWithStringOutput", func(t *testing.T) {
		// Scenario 4: Short input, one-shot processing with *string output
		// CRITICAL: Should buffer (required for *string type)

		flow := NewFlow()
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			data, _ := io.ReadAll(req.Data)
			_, err := res.Data.Write([]byte("processed: " + string(data)))
			return err
		}))

		var output string
		err := flow.Run(context.Background(), "test input", &output)
		if err != nil {
			t.Fatalf("Flow.Run failed: %v", err)
		}

		expected := "processed: test input"
		if output != expected {
			t.Errorf("Expected %q, got %q", expected, output)
		}
	})

	t.Run("Scenario5_ShortInputWithNilOutput", func(t *testing.T) {
		// Scenario 5: Short input with output not needed
		// CRITICAL: Should discard efficiently

		flow := NewFlow()
		handlerCalled := false
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			handlerCalled = true
			_, err := res.Data.Write([]byte("discarded output"))
			return err
		}))

		err := flow.Run(context.Background(), "test input", nil)
		if err != nil {
			t.Fatalf("Flow.Run failed: %v", err)
		}

		if !handlerCalled {
			t.Error("Handler should have been called")
		}
	})

	t.Run("Scenario6_StreamingWithIOWriter", func(t *testing.T) {
		// Test io.Writer output (used by ServeFlow for composability)
		// CRITICAL: Should stream directly without intermediate buffering

		flow := NewFlow()
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			for i := 0; i < 5; i++ {
				_, err := res.Data.Write([]byte(fmt.Sprintf("line-%d\n", i)))
				if err != nil {
					return err
				}
			}
			return nil
		}))

		var buf bytes.Buffer
		err := flow.Run(context.Background(), "", &buf)
		if err != nil {
			t.Fatalf("Flow.Run failed: %v", err)
		}

		expected := "line-0\nline-1\nline-2\nline-3\nline-4\n"
		if buf.String() != expected {
			t.Errorf("Expected %q, got %q", expected, buf.String())
		}
	})

	t.Run("MemoryLeak_Prevention_NilOutput", func(t *testing.T) {
		// Regression test: Ensure nil output doesn't cause memory leak
		// This was the original bug reported

		// Test simply verifies it completes without OOM
		// Buffering 10MB would cause test suite memory pressure
		flow := NewFlow()
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			// Simulate large output that would cause OOM if buffered
			largeData := make([]byte, 1024*1024) // 1MB
			for i := 0; i < 10; i++ {
				_, err := res.Data.Write(largeData)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := flow.Run(ctx, "", nil)
		if err != nil {
			t.Fatalf("Flow.Run failed: %v", err)
		}

		// Test passes if it completes without OOM
		// If buffering occurred, 10MB would be held in memory
	})

	t.Run("RealTime_Streaming_OutputConverter", func(t *testing.T) {
		// Verify OutputConverter receives data in real-time, not after buffering

		flow := NewFlow()
		flow.Use(HandlerFunc(func(req *Request, res *Response) error {
			for i := 0; i < 3; i++ {
				_, err := res.Data.Write([]byte(fmt.Sprintf("msg-%d\n", i)))
				if err != nil {
					return err
				}
				time.Sleep(50 * time.Millisecond)
			}
			return nil
		}))

		receivedAt := make([]time.Time, 0)
		var mu sync.Mutex
		startTime := time.Now()

		converter := &testOutputConverter{
			fromReader: func(r io.Reader) error {
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					mu.Lock()
					receivedAt = append(receivedAt, time.Now())
					mu.Unlock()
				}
				return scanner.Err()
			},
		}

		err := flow.Run(context.Background(), "", converter)
		if err != nil {
			t.Fatalf("Flow.Run failed: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()

		if len(receivedAt) != 3 {
			t.Fatalf("Expected 3 messages, got %d", len(receivedAt))
		}

		// CRITICAL: Messages should arrive incrementally, not all at once
		// First message should arrive around 50ms, not 150ms
		firstDelay := receivedAt[0].Sub(startTime)
		if firstDelay > 100*time.Millisecond {
			t.Errorf("First message delayed %v, indicates buffering instead of streaming", firstDelay)
		}

		// Messages should be spread out (streaming), not bunched (buffered)
		secondDelay := receivedAt[1].Sub(receivedAt[0])
		if secondDelay < 30*time.Millisecond {
			t.Errorf("Messages arrived too close together (%v), indicates buffering", secondDelay)
		}
	})
}

// testOutputConverter is a test helper that implements OutputConverter
type testOutputConverter struct {
	fromReader func(io.Reader) error
}

func (c *testOutputConverter) FromReader(r io.Reader) error {
	if c.fromReader != nil {
		return c.fromReader(r)
	}
	_, err := io.Copy(io.Discard, r)
	return err
}
