// Package observability provides middleware for health checks, metrics, and distributed tracing.
// Health checks verify that your application's dependencies (databases, APIs, caches)
// are working correctly.
package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// startTime records when the application started (for uptime calculation)
var startTime = time.Now()

// HealthCheckConfig configures the health check middleware behavior.
type HealthCheckConfig struct {
	// Timeout is the default timeout for individual health checks.
	// If a check doesn't define its own timeout, this is used.
	// Default: 5 seconds
	Timeout time.Duration

	// FailureThreshold is the number of consecutive failures before marking unhealthy.
	// This prevents flapping between healthy/unhealthy states.
	// Default: 3 consecutive failures
	FailureThreshold int

	// CacheDuration caches health check results to reduce load.
	// Useful for expensive checks or high-traffic health endpoints.
	// Default: 10 seconds
	CacheDuration time.Duration
}

// DefaultHealthCheckConfig returns the default health check configuration
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
		CacheDuration:    10 * time.Second,
	}
}

// HealthCheckOption configures the health check middleware
type HealthCheckOption func(*HealthCheckConfig)

// WithHealthCheckTimeout sets the default timeout for health checks
func WithHealthCheckTimeout(timeout time.Duration) HealthCheckOption {
	return func(cfg *HealthCheckConfig) {
		cfg.Timeout = timeout
	}
}

// WithFailureThreshold sets the failure threshold
func WithFailureThreshold(threshold int) HealthCheckOption {
	return func(cfg *HealthCheckConfig) {
		cfg.FailureThreshold = threshold
	}
}

// WithCacheDuration sets how long to cache health check results
func WithCacheDuration(duration time.Duration) HealthCheckOption {
	return func(cfg *HealthCheckConfig) {
		cfg.CacheDuration = duration
	}
}

// HealthCheck creates a middleware that runs health checks and returns a JSON report.
//
// All health checks run concurrently for fast response times. If any check fails,
// the overall status is marked as "unhealthy".
//
// Parameters:
//   - checks: List of health checkers to run (TCP, HTTP, custom functions)
//   - opts: Optional configuration (timeout, failure threshold, cache duration)
//
// Example:
//
//	checks := []HealthChecker{
//	    &TCPHealthCheck{CheckName: "postgres", Addr: "db:5432"},
//	    &HTTPHealthCheck{CheckName: "api", URL: "http://api/health"},
//	}
//	handler := HealthCheck(checks)
//	flow := calque.NewFlow().Use(handler)
//
// Output Format:
//
//	{
//	  "status": "healthy",         // or "unhealthy" if any check fails
//	  "checks": {
//	    "postgres": {
//	      "name": "postgres",
//	      "status": "ok",          // or "error"
//	      "latency": 5000000       // nanoseconds
//	    },
//	    "redis": { ... }
//	  },
//	  "uptime": 3600000000000,     // nanoseconds since start
//	  "timestamp": "2024-01-15T10:30:00Z"
//	}
func HealthCheck(checks []HealthChecker, opts ...HealthCheckOption) calque.Handler {
	cfg := DefaultHealthCheckConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context
		report := runHealthChecks(ctx, checks, cfg)

		// Serialize to JSON
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return calque.WrapErr(ctx, err, "failed to marshal health report")
		}

		return calque.Write(res, string(data))
	})
}

// runHealthChecks runs all health checks concurrently and returns a report
func runHealthChecks(ctx context.Context, checks []HealthChecker, cfg HealthCheckConfig) HealthReport {
	report := HealthReport{
		Status:    HealthStatusHealthy,
		Checks:    make(map[string]HealthCheckResult),
		Uptime:    time.Since(startTime),
		Timestamp: time.Now(),
	}

	if len(checks) == 0 {
		return report
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(chan HealthCheckResult, len(checks))

	for _, check := range checks {
		wg.Add(1)
		go func(c HealthChecker) {
			defer wg.Done()

			timeout := c.Timeout()
			if timeout == 0 {
				timeout = cfg.Timeout
			}

			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			start := time.Now()
			err := c.Check(checkCtx)
			latency := time.Since(start)

			result := HealthCheckResult{
				Name:    c.Name(),
				Status:  "ok",
				Latency: latency,
			}

			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
			}

			results <- result
		}(check)
	}

	// Close results channel when all checks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	hasErrors := false
	for result := range results {
		mu.Lock()
		report.Checks[result.Name] = result
		if result.Status != "ok" {
			hasErrors = true
		}
		mu.Unlock()
	}

	// If any check fails, the overall status is marked as unhealthy
	if hasErrors {
		report.Status = HealthStatusUnhealthy
	}

	return report
}

// TCPHealthCheck checks if a TCP endpoint is reachable.
//
// This is useful for checking databases, caches, or any service that listens on TCP.
// It simply tries to open a TCP connection - if it succeeds, the check passes.
type TCPHealthCheck struct {
	CheckName    string // Name shown in health report (e.g., "postgres")
	Addr         string // TCP address to check (e.g., "localhost:5432")
	CheckTimeout time.Duration
}

// Name returns the name of this health check
func (c *TCPHealthCheck) Name() string {
	return c.CheckName
}

// Check performs the TCP health check
func (c *TCPHealthCheck) Check(ctx context.Context) error {
	timeout := c.Timeout()
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return calque.WrapErr(ctx, err, "tcp connection failed")
	}
	defer func() { _ = conn.Close() }()
	return nil
}

// Timeout returns the timeout for this health check
func (c *TCPHealthCheck) Timeout() time.Duration {
	return c.CheckTimeout
}

// HTTPHealthCheck checks if an HTTP endpoint returns a successful response.
//
// This is useful for checking APIs, web services, or any HTTP endpoint.
// By default, it expects a 200 OK response, but you can customize this.
type HTTPHealthCheck struct {
	CheckName          string // Name shown in health report
	URL                string // URL to check
	Method             string // defaults to GET
	ExpectedStatusCode int    // defaults to 200
	Headers            map[string]string
	CheckTimeout       time.Duration
	Client             *http.Client
}

// Name returns the name of this health check
func (c *HTTPHealthCheck) Name() string {
	return c.CheckName
}

// Check performs the HTTP health check
func (c *HTTPHealthCheck) Check(ctx context.Context) error {
	method := c.Method
	if method == "" {
		method = http.MethodGet
	}

	expectedStatus := c.ExpectedStatusCode
	if expectedStatus == 0 {
		expectedStatus = http.StatusOK
	}

	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: c.Timeout()}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.URL, nil)
	if err != nil {
		return calque.WrapErr(ctx, err, "failed to create request")
	}

	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return calque.WrapErr(ctx, err, "http request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != expectedStatus {
		return calque.NewErr(ctx, fmt.Sprintf("unexpected status code: %d (expected %d)", resp.StatusCode, expectedStatus))
	}

	return nil
}

// Timeout returns the timeout for this health check
func (c *HTTPHealthCheck) Timeout() time.Duration {
	if c.CheckTimeout == 0 {
		return 5 * time.Second
	}
	return c.CheckTimeout
}

// FuncHealthCheck wraps a custom function as a health check.
//
// Use this when the built-in TCP/HTTP checks aren't enough. You can implement
// any custom logic: query a database, check disk space, verify API keys, etc.
type FuncHealthCheck struct {
	CheckName    string                          // Name shown in health report
	CheckFunc    func(ctx context.Context) error // Function that returns nil if healthy
	CheckTimeout time.Duration
}

// Name returns the name of this health check
func (c *FuncHealthCheck) Name() string {
	return c.CheckName
}

// Check performs the health check
func (c *FuncHealthCheck) Check(ctx context.Context) error {
	return c.CheckFunc(ctx)
}

// Timeout returns the timeout for this health check
func (c *FuncHealthCheck) Timeout() time.Duration {
	return c.CheckTimeout
}

// HealthCheckRegistry manages a dynamic collection of health checks.
//
// Use the registry when you need to add/remove health checks at runtime,
// or when checks are registered by different parts of your application.
type HealthCheckRegistry struct {
	mu     sync.RWMutex
	checks map[string]HealthChecker
	config HealthCheckConfig
}

// NewHealthCheckRegistry creates a new health check registry
// Instead of using the default configuration, you can pass in options to configure the registry.
//
// Example:
//
//	registry := NewHealthCheckRegistry(WithHealthCheckTimeout(10 * time.Second))
//	registry.Register(&TCPHealthCheck{CheckName: "postgres", Addr: "db:5432"})
//	registry.Register(&HTTPHealthCheck{CheckName: "api", URL: "http://api/health"})
//	handler := registry.Handler()
//	flow := calque.NewFlow().Use(handler)
//
// The handler will run all registered health checks and return a report.
func NewHealthCheckRegistry(opts ...HealthCheckOption) *HealthCheckRegistry {
	cfg := DefaultHealthCheckConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return &HealthCheckRegistry{
		checks: make(map[string]HealthChecker),
		config: cfg,
	}
}

// Register adds a health check to the registry
func (r *HealthCheckRegistry) Register(check HealthChecker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[check.Name()] = check
}

// Unregister removes a health check from the registry
func (r *HealthCheckRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.checks, name)
}

// RunAll runs all registered health checks and returns a report
func (r *HealthCheckRegistry) RunAll(ctx context.Context) HealthReport {
	r.mu.RLock()
	checks := make([]HealthChecker, 0, len(r.checks))
	for _, check := range r.checks {
		checks = append(checks, check)
	}
	r.mu.RUnlock()

	return runHealthChecks(ctx, checks, r.config)
}

// Handler returns a calque.Handler that runs all health checks
func (r *HealthCheckRegistry) Handler() calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		report := r.RunAll(req.Context)

		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return calque.WrapErr(req.Context, err, "failed to marshal health report")
		}

		return calque.Write(res, string(data))
	})
}
