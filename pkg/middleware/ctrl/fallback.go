package ctrl

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

const (
	circuitClosed   = 0
	circuitOpen     = 1
	circuitHalfOpen = 2
)

type circuitBreaker struct {
	mu          sync.RWMutex
	failures    int
	threshold   int
	timeout     time.Duration
	lastFailure time.Time
	state       int // 0=closed, 1=open, 2=half-open
}

// Fallback provides graceful degradation when primary handler fails
//
// Input: any data type (buffered - may need to replay for fallback)
// Output: response from primary or fallback handler
// Behavior: BUFFERED - tries primary first, falls back on failure
//
// Attempts primary handler first. On failure, tries fallback handlers
// in sequence until one succeeds. Includes circuit breaker logic to
// skip known-failing handlers temporarily.
//
// Example:
//
//	fallback := ctrl.Fallback(primaryLLM, fallbackLLM, localLLM)
func Fallback(handlers ...calque.Handler) calque.Handler {
	if len(handlers) == 0 {
		return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
			return fmt.Errorf("no handlers provided to fallback")
		})
	}

	breakers := make([]*circuitBreaker, len(handlers))
	for i := range handlers {
		breakers[i] = &circuitBreaker{
			threshold: 5,
			timeout:   30 * time.Second,
		}
	}

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input []byte
		err := calque.Read(req, &input)
		if err != nil {
			return err
		}

		var lastErr error
		for i, handler := range handlers {
			if !breakers[i].Allow() {
				continue // Skip if circuit breaker is open
			}

			var output bytes.Buffer
			handlerReq := calque.NewRequest(req.Context, bytes.NewReader(input))
			handlerRes := calque.NewResponse(&output)
			err := handler.ServeFlow(handlerReq, handlerRes)

			if err == nil {
				breakers[i].RecordSuccess()
				return calque.Write(res, output.Bytes())
			}

			breakers[i].RecordFailure()
			lastErr = err
		}

		return fmt.Errorf("all handlers failed, last error: %w", lastErr)
	})
}

// Allow checks if requests should be allowed through
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		// Check if timeout has passed to transition to half-open
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = circuitHalfOpen
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = circuitClosed
}

// RecordFailure records a failed request
func (cb *circuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = circuitOpen
	}
}
