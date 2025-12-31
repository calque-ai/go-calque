package ctrl

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestRateLimit(t *testing.T) {
	tests := []struct {
		name          string
		rate          int
		per           time.Duration
		requests      int
		expectMinTime time.Duration
		expectMaxTime time.Duration
	}{
		{
			name:          "single request passes immediately",
			rate:          5,
			per:           time.Second,
			requests:      1,
			expectMinTime: 0,
			expectMaxTime: 50 * time.Millisecond,
		},
		{
			name:          "multiple requests within rate limit",
			rate:          10,
			per:           time.Second,
			requests:      3,
			expectMinTime: 0,
			expectMaxTime: 100 * time.Millisecond,
		},
		{
			name:          "requests exceed initial tokens",
			rate:          2,
			per:           time.Second,
			requests:      5,
			expectMinTime: 400 * time.Millisecond,
			expectMaxTime: 2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RateLimit(tt.rate, tt.per)

			start := time.Now()
			var wg sync.WaitGroup

			for i := 0; i < tt.requests; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()

					var buf bytes.Buffer
					input := "request-" + string(rune('0'+index))
					reader := strings.NewReader(input)

					req := calque.NewRequest(context.Background(), reader)
					res := calque.NewResponse(&buf)
					err := handler.ServeFlow(req, res)
					if err != nil {
						t.Errorf("Request %d failed: %v", index, err)
						return
					}

					if got := buf.String(); got != input {
						t.Errorf("Request %d: got %q, want %q", index, got, input)
					}
				}(i)
			}

			wg.Wait()
			elapsed := time.Since(start)

			if elapsed < tt.expectMinTime {
				t.Errorf("Requests completed too quickly: %v < %v", elapsed, tt.expectMinTime)
			}

			if elapsed > tt.expectMaxTime {
				t.Errorf("Requests took too long: %v > %v", elapsed, tt.expectMaxTime)
			}
		})
	}
}

func TestRateLimitContextCancellation(t *testing.T) {
	handler := RateLimit(1, time.Second)

	var buf bytes.Buffer
	reader := strings.NewReader("first-request")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("First request failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	buf.Reset()
	reader = strings.NewReader("second-request")

	req = calque.NewRequest(ctx, reader)
	res = calque.NewResponse(&buf)
	err = handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "rate limit wait failed") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected rate limit or context error, got: %v", err)
	}
}

func TestRateLimiterWait(t *testing.T) {
	limiter := &rateLimiter{
		tokens:     0,
		maxTokens:  2,
		refillRate: 100 * time.Millisecond,
		lastRefill: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := limiter.Wait(ctx)
	if err == nil {
		t.Error("Expected context timeout, got nil")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected deadline exceeded, got: %v", err)
	}
}

func TestRateLimiterRefill(t *testing.T) {
	now := time.Now()
	limiter := &rateLimiter{
		tokens:     0,
		maxTokens:  5,
		refillRate: 100 * time.Millisecond,
		lastRefill: now.Add(-250 * time.Millisecond),
	}

	limiter.refill()

	expectedTokens := 2
	if limiter.tokens != expectedTokens {
		t.Errorf("Expected %d tokens, got %d", expectedTokens, limiter.tokens)
	}

	if limiter.lastRefill.Before(now) {
		t.Error("lastRefill should be updated to recent time")
	}
}

func TestRateLimiterRefillCapping(t *testing.T) {
	limiter := &rateLimiter{
		tokens:     0,
		maxTokens:  3,
		refillRate: 10 * time.Millisecond,
		lastRefill: time.Now().Add(-1 * time.Second),
	}

	limiter.refill()

	if limiter.tokens != limiter.maxTokens {
		t.Errorf("Tokens should be capped at maxTokens %d, got %d", limiter.maxTokens, limiter.tokens)
	}
}

func TestRateLimiterNoRefillNeeded(t *testing.T) {
	now := time.Now()
	limiter := &rateLimiter{
		tokens:     5,
		maxTokens:  10,
		refillRate: 100 * time.Millisecond,
		lastRefill: now.Add(-50 * time.Millisecond),
	}

	originalTokens := limiter.tokens
	limiter.refill()

	if limiter.tokens != originalTokens {
		t.Errorf("Tokens should not change, expected %d, got %d", originalTokens, limiter.tokens)
	}
}

func TestRateLimitConcurrentAccess(t *testing.T) {
	handler := RateLimit(10, time.Second)

	numGoroutines := 20
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := range numGoroutines {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			var buf bytes.Buffer
			input := "concurrent-" + string(rune('0'+(index%10)))
			reader := strings.NewReader(input)

			req := calque.NewRequest(ctx, reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()

				if got := buf.String(); got != input {
					t.Errorf("Request %d: got %q, want %q", index, got, input)
				}
			}
		}(i)
	}

	wg.Wait()

	if successCount == 0 {
		t.Error("Expected at least some requests to succeed")
	}

	if successCount > numGoroutines {
		t.Errorf("Success count %d cannot exceed total requests %d", successCount, numGoroutines)
	}
}

func TestRateLimitWithZeroRate(t *testing.T) {
	handler := RateLimit(0, time.Second)

	var buf bytes.Buffer
	reader := strings.NewReader("zero-rate-test")

	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected error with zero rate, got nil")
		return
	}

	expectedMsg := "invalid rate limit: rate must be greater than 0, got 0"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got: %v", expectedMsg, err)
	}
}

func TestRateLimitBurstCapacity(t *testing.T) {
	handler := RateLimit(5, time.Second)

	start := time.Now()
	var wg sync.WaitGroup

	for i := range 5 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			var buf bytes.Buffer
			input := "burst-" + string(rune('0'+index))
			reader := strings.NewReader(input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			if err != nil {
				t.Errorf("Burst request %d failed: %v", index, err)
				return
			}

			if got := buf.String(); got != input {
				t.Errorf("Burst request %d: got %q, want %q", index, got, input)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("Burst requests should complete quickly, took %v", elapsed)
	}
}

func TestRateLimitTokenConsumption(t *testing.T) {
	limiter := &rateLimiter{
		tokens:     3,
		maxTokens:  5,
		refillRate: 100 * time.Millisecond,
		lastRefill: time.Now(),
	}

	ctx := context.Background()

	err := limiter.Wait(ctx)
	if err != nil {
		t.Errorf("First wait failed: %v", err)
	}

	if limiter.tokens != 2 {
		t.Errorf("Expected 2 tokens after first wait, got %d", limiter.tokens)
	}

	err = limiter.Wait(ctx)
	if err != nil {
		t.Errorf("Second wait failed: %v", err)
	}

	if limiter.tokens != 1 {
		t.Errorf("Expected 1 token after second wait, got %d", limiter.tokens)
	}
}

func TestRateLimitRefillTiming(t *testing.T) {
	handler := RateLimit(2, 200*time.Millisecond)

	start := time.Now()

	var buf bytes.Buffer
	reader := strings.NewReader("first")
	req := calque.NewRequest(context.Background(), reader)
	res := calque.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("First request failed: %v", err)
	}

	buf.Reset()
	reader = strings.NewReader("second")
	req = calque.NewRequest(context.Background(), reader)
	res = calque.NewResponse(&buf)
	err = handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Second request failed: %v", err)
	}

	time.Sleep(120 * time.Millisecond)

	buf.Reset()
	reader = strings.NewReader("third")
	req = calque.NewRequest(context.Background(), reader)
	res = calque.NewResponse(&buf)
	err = handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Third request failed: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Errorf("Requests with refill should complete in reasonable time, took %v", elapsed)
	}
}
