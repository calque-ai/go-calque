package ctrl

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

type rateLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
}

// RateLimit creates a rate limiting middleware using token bucket algorithm
//
// Input: any data type (streaming - processes immediately if tokens available)
// Output: same as input (pass-through when allowed)
// Behavior: STREAMING - blocks until tokens available, then streams through
//
// Uses token bucket algorithm for smooth rate limiting. Tokens are replenished
// at the specified rate. Each request consumes one token. If no tokens available,
// the request blocks until a token becomes available.
//
// Example:
//
//	rateLimit := ctrl.RateLimit(10, time.Second) // 10 requests/second
//	pipe.Use(rateLimit)
func RateLimit(rate int, per time.Duration) calque.Handler {
	// <= 0 requests per n makes no sense.
	if rate <= 0 {
		return calque.HandlerFunc(func(r *calque.Request, _ *calque.Response) error {
			return calque.NewErr(r.Context, fmt.Sprintf("invalid rate limit: rate must be greater than 0, got %d", rate))
		})
	}

	limiter := &rateLimiter{
		tokens:     rate,
		maxTokens:  rate,
		refillRate: time.Duration(per.Nanoseconds() / int64(rate)),
		lastRefill: time.Now(),
	}

	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		if err := limiter.Wait(r.Context); err != nil {
			return calque.WrapErr(r.Context, err, "rate limit wait failed")
		}

		_, err := io.Copy(w.Data, r.Data)
		return err
	})
}

// Wait blocks until a token is available or context is cancelled
func (rl *rateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()

		if rl.tokens > 0 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}

		// Calculate how long to wait for the next token
		nextRefill := rl.lastRefill.Add(rl.refillRate)
		wait := time.Until(nextRefill)
		rl.mu.Unlock()

		if wait <= 0 {
			wait = time.Nanosecond // Minimal wait if we just missed it
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			// Try again after waiting
		}
	}
}

// refill adds tokens based on elapsed time (must be called with mutex held)
func (rl *rateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	tokensToAdd := int(elapsed / rl.refillRate)
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		// Increment lastRefill by exact intervals to avoid drift and maintain precision
		rl.lastRefill = rl.lastRefill.Add(time.Duration(tokensToAdd) * rl.refillRate)
	}
}
