package flow

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/calque-pipe/pkg/core"
)

func TestFallback(t *testing.T) {
	successHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		var input string
		err := core.Read(req, &input)
		if err != nil {
			return err
		}
		return core.Write(res, "success:"+input)
	})

	primaryFailHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		return errors.New("primary failed")
	})

	fallbackHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		var input string
		err := core.Read(req, &input)
		if err != nil {
			return err
		}
		return core.Write(res, "fallback:"+input)
	})

	alwaysFailHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		return errors.New("always fails")
	})

	tests := []struct {
		name     string
		handlers []core.Handler
		input    string
		expected string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "primary handler succeeds",
			handlers: []core.Handler{successHandler, fallbackHandler},
			input:    "test",
			expected: "success:test",
			wantErr:  false,
		},
		{
			name:     "primary fails, fallback succeeds",
			handlers: []core.Handler{primaryFailHandler, fallbackHandler},
			input:    "test",
			expected: "fallback:test",
			wantErr:  false,
		},
		{
			name:     "single handler success",
			handlers: []core.Handler{successHandler},
			input:    "single",
			expected: "success:single",
			wantErr:  false,
		},
		{
			name:     "single handler failure",
			handlers: []core.Handler{alwaysFailHandler},
			input:    "fail",
			wantErr:  true,
			errMsg:   "all handlers failed",
		},
		{
			name:     "all handlers fail",
			handlers: []core.Handler{alwaysFailHandler, alwaysFailHandler},
			input:    "fail",
			wantErr:  true,
			errMsg:   "all handlers failed",
		},
		{
			name:     "empty input with fallback",
			handlers: []core.Handler{primaryFailHandler, fallbackHandler},
			input:    "",
			expected: "fallback:",
			wantErr:  false,
		},
		{
			name:     "multiple fallbacks - first fallback succeeds",
			handlers: []core.Handler{primaryFailHandler, fallbackHandler, successHandler},
			input:    "multi",
			expected: "fallback:multi",
			wantErr:  false,
		},
		{
			name:     "binary data with fallback",
			handlers: []core.Handler{primaryFailHandler, fallbackHandler},
			input:    "\x00\x01\x02\x03",
			expected: "fallback:\x00\x01\x02\x03",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Fallback(tt.handlers...)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Fallback() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Fallback() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Fallback() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFallbackNoHandlers(t *testing.T) {
	handler := Fallback()

	var buf bytes.Buffer
	reader := strings.NewReader("test")

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Fallback() with no handlers should return error")
	}

	if !strings.Contains(err.Error(), "no handlers provided") {
		t.Errorf("Fallback() error = %v, want error containing 'no handlers provided'", err)
	}
}

func TestCircuitBreakerAllow(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*circuitBreaker)
		expected bool
	}{
		{
			name:     "closed circuit allows requests",
			setup:    func(cb *circuitBreaker) { cb.state = circuitClosed },
			expected: true,
		},
		{
			name: "open circuit blocks requests within timeout",
			setup: func(cb *circuitBreaker) {
				cb.state = circuitOpen
				cb.lastFailure = time.Now()
				cb.timeout = 30 * time.Second
			},
			expected: false,
		},
		{
			name: "open circuit transitions to half-open after timeout",
			setup: func(cb *circuitBreaker) {
				cb.state = circuitOpen
				cb.lastFailure = time.Now().Add(-31 * time.Second)
				cb.timeout = 30 * time.Second
			},
			expected: true,
		},
		{
			name:     "half-open circuit allows requests",
			setup:    func(cb *circuitBreaker) { cb.state = circuitHalfOpen },
			expected: true,
		},
		{
			name:     "invalid state blocks requests",
			setup:    func(cb *circuitBreaker) { cb.state = 999 },
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &circuitBreaker{
				threshold: 5,
				timeout:   30 * time.Second,
			}
			tt.setup(cb)

			got := cb.Allow()
			if got != tt.expected {
				t.Errorf("circuitBreaker.Allow() = %v, want %v", got, tt.expected)
			}

			if tt.name == "open circuit transitions to half-open after timeout" {
				if cb.state != circuitHalfOpen {
					t.Errorf("circuitBreaker state = %v, want %v", cb.state, circuitHalfOpen)
				}
			}
		})
	}
}

func TestCircuitBreakerRecordSuccess(t *testing.T) {
	tests := []struct {
		name          string
		initialState  int
		initialFails  int
		expectedState int
		expectedFails int
	}{
		{
			name:          "success resets closed circuit",
			initialState:  circuitClosed,
			initialFails:  3,
			expectedState: circuitClosed,
			expectedFails: 0,
		},
		{
			name:          "success closes open circuit",
			initialState:  circuitOpen,
			initialFails:  5,
			expectedState: circuitClosed,
			expectedFails: 0,
		},
		{
			name:          "success closes half-open circuit",
			initialState:  circuitHalfOpen,
			initialFails:  2,
			expectedState: circuitClosed,
			expectedFails: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &circuitBreaker{
				state:     tt.initialState,
				failures:  tt.initialFails,
				threshold: 5,
				timeout:   30 * time.Second,
			}

			cb.RecordSuccess()

			if cb.state != tt.expectedState {
				t.Errorf("circuitBreaker.state = %v, want %v", cb.state, tt.expectedState)
			}

			if cb.failures != tt.expectedFails {
				t.Errorf("circuitBreaker.failures = %v, want %v", cb.failures, tt.expectedFails)
			}
		})
	}
}

func TestCircuitBreakerRecordFailure(t *testing.T) {
	tests := []struct {
		name          string
		initialState  int
		initialFails  int
		threshold     int
		expectedState int
		expectedFails int
	}{
		{
			name:          "failure below threshold keeps circuit closed",
			initialState:  circuitClosed,
			initialFails:  2,
			threshold:     5,
			expectedState: circuitClosed,
			expectedFails: 3,
		},
		{
			name:          "failure at threshold opens circuit",
			initialState:  circuitClosed,
			initialFails:  4,
			threshold:     5,
			expectedState: circuitOpen,
			expectedFails: 5,
		},
		{
			name:          "failure above threshold keeps circuit open",
			initialState:  circuitOpen,
			initialFails:  5,
			threshold:     5,
			expectedState: circuitOpen,
			expectedFails: 6,
		},
		{
			name:          "failure in half-open opens circuit",
			initialState:  circuitHalfOpen,
			initialFails:  4,
			threshold:     5,
			expectedState: circuitOpen,
			expectedFails: 5,
		},
		{
			name:          "first failure",
			initialState:  circuitClosed,
			initialFails:  0,
			threshold:     3,
			expectedState: circuitClosed,
			expectedFails: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &circuitBreaker{
				state:     tt.initialState,
				failures:  tt.initialFails,
				threshold: tt.threshold,
				timeout:   30 * time.Second,
			}

			beforeTime := time.Now()
			cb.RecordFailure()
			afterTime := time.Now()

			if cb.state != tt.expectedState {
				t.Errorf("circuitBreaker.state = %v, want %v", cb.state, tt.expectedState)
			}

			if cb.failures != tt.expectedFails {
				t.Errorf("circuitBreaker.failures = %v, want %v", cb.failures, tt.expectedFails)
			}

			if cb.lastFailure.Before(beforeTime) || cb.lastFailure.After(afterTime) {
				t.Errorf("circuitBreaker.lastFailure not set correctly")
			}
		})
	}
}

func TestFallbackWithCircuitBreaker(t *testing.T) {
	callCount := 0
	intermittentHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		callCount++
		if callCount <= 5 {
			return errors.New("intermittent failure")
		}
		var input string
		err := core.Read(req, &input)
		if err != nil {
			return err
		}
		return core.Write(res, "recovered:"+input)
	})

	fallbackHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		var input string
		err := core.Read(req, &input)
		if err != nil {
			return err
		}
		return core.Write(res, "fallback:"+input)
	})

	handler := Fallback(intermittentHandler, fallbackHandler)

	var buf bytes.Buffer

	for i := 1; i <= 7; i++ {
		buf.Reset()
		reader := strings.NewReader("circuit-test")

		req := core.NewRequest(context.Background(), reader)
		res := core.NewResponse(&buf)
		err := handler.ServeFlow(req, res)
		if err != nil {
			t.Errorf("Iteration %d: Fallback() error = %v", i, err)
			continue
		}

		output := buf.String()
		if i <= 5 {
			if output != "fallback:circuit-test" {
				t.Errorf("Iteration %d: expected fallback, got %q", i, output)
			}
		} else {
			if output != "fallback:circuit-test" {
				t.Errorf("Iteration %d: circuit should still block primary, got %q", i, output)
			}
		}
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := &circuitBreaker{
		threshold: 3,
		timeout:   100 * time.Millisecond,
		state:     circuitClosed,
	}

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				allowed := cb.Allow()
				if allowed {
					if j%2 == 0 {
						cb.RecordSuccess()
					} else {
						cb.RecordFailure()
					}
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	cb.mu.RLock()
	state := cb.state
	failures := cb.failures
	cb.mu.RUnlock()

	if state < 0 || state > 2 {
		t.Errorf("Circuit breaker in invalid state: %d", state)
	}

	if failures < 0 {
		t.Errorf("Circuit breaker has negative failures: %d", failures)
	}
}

func TestFallbackContextCancellation(t *testing.T) {
	slowHandler := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		select {
		case <-req.Context.Done():
			return req.Context.Err()
		case <-time.After(200 * time.Millisecond):
			return errors.New("slow handler failed")
		}
	})

	fastFallback := core.HandlerFunc(func(req *core.Request, res *core.Response) error {
		var input string
		err := core.Read(req, &input)
		if err != nil {
			return err
		}
		return core.Write(res, "fast-fallback:"+input)
	})

	handler := Fallback(slowHandler, fastFallback)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	reader := strings.NewReader("context-test")

	req := core.NewRequest(ctx, reader)
	res := core.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Fallback() with context cancellation error = %v", err)
		return
	}

	expected := "fast-fallback:context-test"
	if got := buf.String(); got != expected {
		t.Errorf("Fallback() = %q, want %q", got, expected)
	}
}
