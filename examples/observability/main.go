// Package main demonstrates the observability middleware components.
//
// This example shows how to use metrics, tracing, and health checks
// in a go-calque flow.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/observability"
)

func main() {
	ctx := context.Background()

	fmt.Println("---- Metrics Example ----")
	runMetricsExample(ctx)

	fmt.Println("\n----Tracing Example ----")
	runTracingExample(ctx)

	fmt.Println("\n----Health Check Example ----")
	runHealthCheckExample(ctx)

	fmt.Println("\n----Combined Observability Example ----")
	runCombinedExample(ctx)
}

// runMetricsExample demonstrates metrics collection
func runMetricsExample(ctx context.Context) {
	// Create a Prometheus metrics provider
	promProvider := observability.NewPrometheusProvider()

	// Define service labels
	labels := map[string]string{
		"service": "ai-assistant",
		"version": "v1.0.0",
	}

	// Create a handler that processes text
	textProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		// Simulate processing
		time.Sleep(10 * time.Millisecond)
		return calque.Write(res, strings.ToUpper(input))
	})

	// Wrap with metrics
	metricsHandler := observability.MetricsHandler(promProvider, labels, textProcessor)

	// Create and execute flow
	flow := calque.NewFlow().Use(metricsHandler)

	var result string
	err := flow.Run(ctx, "hello world", &result)
	if err != nil {
		log.Fatalf("Flow failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
	fmt.Println("Metrics are now available at /metrics endpoint")

	// In a real application, you would expose the metrics handler:
	// http.Handle("/metrics", promProvider.Handler())
	_ = promProvider.Handler() // Suppress unused warning
}

// runTracingExample demonstrates distributed tracing
func runTracingExample(ctx context.Context) {
	// For this example, use in-memory tracer (in production, use OTLP)
	tracerProvider := observability.NewInMemoryTracerProvider()

	// Create handlers for different stages
	inputValidation := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		if len(input) == 0 {
			return calque.NewErr(req.Context, "input cannot be empty")
		}
		return calque.Write(res, input)
	})

	processing := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		// Simulate AI processing
		time.Sleep(20 * time.Millisecond)
		return calque.Write(res, fmt.Sprintf("Processed: %s", strings.ToUpper(input)))
	})

	outputValidation := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		// Pass through
		return calque.Write(res, input)
	})

	// Create flow with tracing at each stage
	flow := calque.NewFlow().
		Use(observability.TracingHandler(tracerProvider, "input-validation", inputValidation)).
		Use(observability.TracingHandler(tracerProvider, "ai-processing", processing)).
		Use(observability.TracingHandler(tracerProvider, "output-validation", outputValidation))

	var result string
	err := flow.Run(ctx, "test input", &result)
	if err != nil {
		log.Fatalf("Flow failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)

	// Print recorded spans
	spans := tracerProvider.GetSpans()
	fmt.Printf("Recorded %d spans:\n", len(spans))
	for _, span := range spans {
		duration := span.EndTime.Sub(span.StartTime)
		fmt.Printf("  - %s (duration: %v, status: %v)\n", span.Name, duration, span.Status)
	}
}

// runHealthCheckExample demonstrates health checking
func runHealthCheckExample(ctx context.Context) {
	// Create health checks
	checks := []observability.HealthChecker{
		// Simulated database check
		&observability.FuncHealthCheck{
			CheckName: "database",
			CheckFunc: func(_ context.Context) error {
				// Simulate DB ping
				time.Sleep(5 * time.Millisecond)
				return nil
			},
			CheckTimeout: 2 * time.Second,
		},
		// Simulated cache check
		&observability.FuncHealthCheck{
			CheckName: "cache",
			CheckFunc: func(_ context.Context) error {
				// Simulate cache ping
				time.Sleep(2 * time.Millisecond)
				return nil
			},
			CheckTimeout: 1 * time.Second,
		},
	}

	// Create health check handler
	healthHandler := observability.HealthCheck(checks)

	// Execute health check
	flow := calque.NewFlow().Use(healthHandler)
	var result string
	err := flow.Run(ctx, "", &result)
	if err != nil {
		log.Fatalf("Health check failed: %v", err)
	}

	fmt.Printf("Health Report:\n%s\n", result)
}

// runCombinedExample shows all observability features together
func runCombinedExample(ctx context.Context) {
	// Create providers
	metricsProvider := observability.NewInMemoryMetricsProvider()
	tracerProvider := observability.NewInMemoryTracerProvider()

	labels := map[string]string{"service": "combined-example"}

	// Create the main business logic handler
	businessLogic := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		// Simulate work
		time.Sleep(15 * time.Millisecond)

		output := fmt.Sprintf("Processed at %s: %s",
			time.Now().Format(time.RFC3339),
			strings.ToUpper(input))

		return calque.Write(res, output)
	})

	// Wrap with both metrics and tracing
	tracedHandler := observability.TracingHandler(tracerProvider, "business-logic", businessLogic)
	observedHandler := observability.MetricsHandler(metricsProvider, labels, tracedHandler)

	// Create flow
	flow := calque.NewFlow().Use(observedHandler)

	// Execute multiple times
	for i := 0; i < 3; i++ {
		var result string
		err := flow.Run(ctx, fmt.Sprintf("request %d", i+1), &result)
		if err != nil {
			log.Fatalf("Request %d failed: %v", i+1, err)
		}
		fmt.Printf("Request %d result: %s\n", i+1, result)
	}

	// Print metrics summary
	fmt.Printf("\nMetrics Summary:\n")
	fmt.Printf("  Requests: %d\n", metricsProvider.GetCounter("calque_flow_requests_total", labels))

	// Print trace summary
	spans := tracerProvider.GetSpans()
	fmt.Printf("  Spans recorded: %d\n", len(spans))

	// Create health check registry
	registry := observability.NewHealthCheckRegistry()
	registry.Register(&observability.FuncHealthCheck{
		CheckName: "metrics-provider",
		CheckFunc: func(_ context.Context) error { return nil },
	})
	registry.Register(&observability.FuncHealthCheck{
		CheckName: "tracer-provider",
		CheckFunc: func(_ context.Context) error { return nil },
	})

	report := registry.RunAll(ctx)
	fmt.Printf("  Health Status: %s\n", report.Status)
	fmt.Printf("  Uptime: %v\n", report.Uptime.Round(time.Second))
}

func init() {
	// Check if we should start an HTTP server for metrics
	if os.Getenv("SERVE_METRICS") == "true" {
		promProvider := observability.NewPrometheusProvider()
		http.Handle("/metrics", promProvider.Handler())
		go func() {
			log.Println("Metrics server starting on :9090")
			if err := http.ListenAndServe(":9090", nil); err != nil {
				log.Printf("Metrics server error: %v", err)
			}
		}()
	}
}
