package calque

import (
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
