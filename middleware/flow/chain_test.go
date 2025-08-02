package flow

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calque-ai/calque-pipe/core"
)

// Test basic chain functionality
func TestChain_BasicSequentialExecution(t *testing.T) {
	var executionOrder []string
	var orderMutex sync.Mutex

	// Create handlers that record execution order
	handler1 := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		orderMutex.Lock()
		executionOrder = append(executionOrder, "handler1")
		orderMutex.Unlock()
		
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		
		output := string(input) + "-h1"
		_, err = res.Data.Write([]byte(output))
		return err
	})

	handler2 := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		orderMutex.Lock()
		executionOrder = append(executionOrder, "handler2")
		orderMutex.Unlock()
		
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		
		output := string(input) + "-h2"
		_, err = res.Data.Write([]byte(output))
		return err
	})

	// Create chain
	chain := Chain(handler1, handler2)

	// Test execution
	var result string
	pipe := core.New()
	pipe.Use(chain)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err := pipe.Run(ctx, "test", &result)
	if err != nil {
		t.Fatalf("Chain execution failed: %v", err)
	}

	// Verify execution order
	if len(executionOrder) != 2 {
		t.Fatalf("Expected 2 handlers to execute, got %d", len(executionOrder))
	}
	
	if executionOrder[0] != "handler1" || executionOrder[1] != "handler2" {
		t.Errorf("Expected execution order [handler1, handler2], got %v", executionOrder)
	}

	// Verify data flow
	expected := "test-h1-h2"
	if result != expected {
		t.Errorf("Expected result %q, got %q", expected, result)
	}
}

// Test context propagation
func TestChain_ContextPropagation(t *testing.T) {
	type testKey struct{}

	// Handler that adds to context
	setter := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		// Modify the request context
		req.Context = context.WithValue(req.Context, testKey{}, "test-value")
		
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	// Handler that reads from context
	var contextValue string
	getter := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		if val := req.Context.Value(testKey{}); val != nil {
			contextValue = val.(string)
		}
		
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	// Create chain
	chain := Chain(setter, getter)

	// Test execution
	var result string
	pipe := core.New()
	pipe.Use(chain)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err := pipe.Run(ctx, "test", &result)
	if err != nil {
		t.Fatalf("Chain execution failed: %v", err)
	}

	// Verify context was propagated
	if contextValue != "test-value" {
		t.Errorf("Expected context value %q, got %q", "test-value", contextValue)
	}
}

// Test empty chain
func TestChain_EmptyChain(t *testing.T) {
	chain := Chain()

	var result string
	pipe := core.New()
	pipe.Use(chain)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err := pipe.Run(ctx, "test", &result)
	if err != nil {
		t.Fatalf("Empty chain execution failed: %v", err)
	}

	if result != "test" {
		t.Errorf("Expected passthrough result %q, got %q", "test", result)
	}
}

// Test single handler chain
func TestChain_SingleHandler(t *testing.T) {
	handler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}
		
		output := strings.ToUpper(string(input))
		_, err = res.Data.Write([]byte(output))
		return err
	})

	chain := Chain(handler)

	var result string
	pipe := core.New()
	pipe.Use(chain)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err := pipe.Run(ctx, "hello", &result)
	if err != nil {
		t.Fatalf("Single handler chain execution failed: %v", err)
	}

	if result != "HELLO" {
		t.Errorf("Expected result %q, got %q", "HELLO", result)
	}
}

// Test timeout detection (this should help identify deadlocks)
func TestChain_DoesNotDeadlock(t *testing.T) {
	// Create simple pass-through handlers
	handler1 := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})
	
	handler2 := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		_, err := io.Copy(res.Data, req.Data)
		return err
	})

	chain := Chain(handler1, handler2)

	var result string
	pipe := core.New()
	pipe.Use(chain)
	
	// Short timeout to detect deadlocks
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	err := pipe.Run(ctx, "test", &result)
	if err != nil {
		t.Fatalf("Chain execution failed or timed out: %v", err)
	}

	if result != "test" {
		t.Errorf("Expected result %q, got %q", "test", result)
	}
}