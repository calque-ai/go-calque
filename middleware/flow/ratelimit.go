package flow

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/calque-ai/calque-pipe/core"
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
//	rateLimit := flow.RateLimit[string](10, time.Second) // 10 requests/second
//	pipe.Use(rateLimit)
func RateLimit[T any](rate int, per time.Duration) core.Handler {
	// <= 0 requests per n makes no sense.
	if rate <= 0 {
		return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
			return fmt.Errorf("invalid rate limit: rate must be greater than 0, got %d", rate)
		})
	}

	limiter := &rateLimiter{
		tokens:     rate,
		maxTokens:  rate,
		refillRate: time.Duration(per.Nanoseconds() / int64(rate)),
		lastRefill: time.Now(),
	}

	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		if err := limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}

		_, err := io.Copy(w, r)
		return err
	})
}

// Wait blocks until a token is available or context is cancelled
func (rl *rateLimiter) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rl.mu.Lock()
		rl.refill()

		if rl.tokens > 0 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		rl.mu.Unlock()

		// Sleep for a short duration before checking again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(rl.refillRate / 10):
			continue
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
		rl.lastRefill = now
	}
}
