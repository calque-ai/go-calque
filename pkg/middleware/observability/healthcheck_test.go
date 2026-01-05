package observability

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	checks := []HealthChecker{
		&FuncHealthCheck{
			CheckName:    "healthy-check",
			CheckFunc:    func(_ context.Context) error { return nil },
			CheckTimeout: 1 * time.Second,
		},
	}

	handler := HealthCheck(checks)

	req := calque.NewRequest(context.Background(), strings.NewReader(""))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	var report HealthReport
	if err := json.Unmarshal([]byte(buf.String()), &report); err != nil {
		t.Fatalf("Failed to parse health report: %v", err)
	}

	// Check that the health check report is healthy
	if report.Status != HealthStatusHealthy {
		t.Errorf("Expected status healthy, got %s", report.Status)
	}

	if len(report.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(report.Checks))
	}

	if report.Checks["healthy-check"].Status != "ok" {
		t.Errorf("Expected check status 'ok', got '%s'", report.Checks["healthy-check"].Status)
	}
}

func TestHealthCheckWithFailure(t *testing.T) {
	t.Parallel()

	checks := []HealthChecker{
		&FuncHealthCheck{
			CheckName:    "failing-check",
			CheckFunc:    func(_ context.Context) error { return errors.New("service unavailable") },
			CheckTimeout: 1 * time.Second,
		},
	}

	handler := HealthCheck(checks)

	req := calque.NewRequest(context.Background(), strings.NewReader(""))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	var report HealthReport
	if err := json.Unmarshal([]byte(buf.String()), &report); err != nil {
		t.Fatalf("Failed to parse health report: %v", err)
	}

	if report.Status != HealthStatusUnhealthy {
		t.Errorf("Expected status unhealthy, got %s", report.Status)
	}

	if report.Checks["failing-check"].Status != "error" {
		t.Errorf("Expected check status 'error', got '%s'", report.Checks["failing-check"].Status)
	}

	if report.Checks["failing-check"].Error == "" {
		t.Error("Expected error message in check result")
	}
}

func TestHealthCheckMixed(t *testing.T) {
	t.Parallel()

	checks := []HealthChecker{
		&FuncHealthCheck{
			CheckName:    "healthy-check",
			CheckFunc:    func(_ context.Context) error { return nil },
			CheckTimeout: 1 * time.Second,
		},
		&FuncHealthCheck{
			CheckName:    "failing-check",
			CheckFunc:    func(_ context.Context) error { return errors.New("failed") },
			CheckTimeout: 1 * time.Second,
		},
	}

	handler := HealthCheck(checks)

	req := calque.NewRequest(context.Background(), strings.NewReader(""))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	var report HealthReport
	if err := json.Unmarshal([]byte(buf.String()), &report); err != nil {
		t.Fatalf("Failed to parse health report: %v", err)
	}

	// Mixed results should be unhealthy
	if report.Status != HealthStatusUnhealthy {
		t.Errorf("Expected status unhealthy, got %s", report.Status)
	}

	if len(report.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(report.Checks))
	}
}

func TestHTTPHealthCheck(t *testing.T) {
	t.Parallel()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	check := &HTTPHealthCheck{
		CheckName:    "http-check",
		URL:          server.URL,
		CheckTimeout: 5 * time.Second,
	}

	err := check.Check(context.Background())
	if err != nil {
		t.Errorf("HTTP health check failed: %v", err)
	}
}

func TestHTTPHealthCheckFailure(t *testing.T) {
	t.Parallel()

	// Create a test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	check := &HTTPHealthCheck{
		CheckName:    "http-check",
		URL:          server.URL,
		CheckTimeout: 5 * time.Second,
	}

	err := check.Check(context.Background())
	if err == nil {
		t.Error("Expected HTTP health check to fail for 500 response")
	}
}

func TestFuncHealthCheck(t *testing.T) {
	t.Parallel()

	check := &FuncHealthCheck{
		CheckName:    "func-check",
		CheckFunc:    func(_ context.Context) error { return nil },
		CheckTimeout: 1 * time.Second,
	}

	if check.Name() != "func-check" {
		t.Errorf("Expected name 'func-check', got '%s'", check.Name())
	}

	if check.Timeout() != 1*time.Second {
		t.Errorf("Expected timeout 1s, got %v", check.Timeout())
	}

	err := check.Check(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestHealthCheckRegistry(t *testing.T) {
	t.Parallel()

	registry := NewHealthCheckRegistry()

	check1 := &FuncHealthCheck{
		CheckName: "check1",
		CheckFunc: func(_ context.Context) error { return nil },
	}

	check2 := &FuncHealthCheck{
		CheckName: "check2",
		CheckFunc: func(_ context.Context) error { return errors.New("failed") },
	}

	registry.Register(check1)
	registry.Register(check2)

	report := registry.RunAll(context.Background())

	if len(report.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(report.Checks))
	}

	if report.Status != HealthStatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", report.Status)
	}

	// Test unregister
	registry.Unregister("check2")
	report = registry.RunAll(context.Background())

	if len(report.Checks) != 1 {
		t.Errorf("Expected 1 check after unregister, got %d", len(report.Checks))
	}

	if report.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status after unregister, got %s", report.Status)
	}
}

func TestHealthCheckConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultHealthCheckConfig()

	if cfg.Timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", cfg.Timeout)
	}

	if cfg.FailureThreshold != 3 {
		t.Errorf("Expected default failure threshold 3, got %d", cfg.FailureThreshold)
	}

	if cfg.CacheDuration != 10*time.Second {
		t.Errorf("Expected default cache duration 10s, got %v", cfg.CacheDuration)
	}
}
