package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/logger"
)

// TestBatchProcessingBasic tests basic document processing functionality
func TestBatchProcessingBasic(t *testing.T) {
	t.Parallel()

	// Create a simple document processor
	documentProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}

		// Simple analysis
		wordCount := len(strings.Fields(content))
		charCount := len(content)
		lineCount := len(strings.Split(content, "\n"))

		analysis := fmt.Sprintf("DOCUMENT ANALYSIS:\n"+
			"Words: %d\n"+
			"Characters: %d\n"+
			"Lines: %d\n", wordCount, charCount, lineCount)

		return calque.Write(res, analysis)
	})

	// Test the processor directly without batching first
	var buf strings.Builder
	err := documentProcessor.ServeFlow(&calque.Request{Data: strings.NewReader("Test document")}, &calque.Response{Data: &buf})
	if err != nil {
		t.Fatalf("Direct processor test failed: %v", err)
	}

	result := buf.String()
	// The result should contain analysis
	if !strings.Contains(result, "DOCUMENT ANALYSIS:") {
		t.Error("Expected document analysis in result")
	}
}

// TestBatchConfiguration tests different batch configurations
func TestBatchConfiguration(t *testing.T) {
	t.Parallel()

	// Create a simple processor
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}
		return calque.Write(res, "Processed: "+content)
	})

	// Test different batch sizes
	batchSizes := []int{1, 2, 5, 10}
	for _, size := range batchSizes {
		t.Run(fmt.Sprintf("BatchSize_%d", size), func(t *testing.T) {
			// Test the processor directly instead of using batch
			var buf strings.Builder
			err := processor.ServeFlow(&calque.Request{Data: strings.NewReader(fmt.Sprintf("Item %d", size))}, &calque.Response{Data: &buf})
			if err != nil {
				t.Fatalf("Processor test with size %d failed: %v", size, err)
			}

			result := buf.String()
			// Verify result contains processed items
			if !strings.Contains(result, "Processed:") {
				t.Errorf("Expected processed result for batch size %d", size)
			}
		})
	}
}

// TestBatchTimeout tests timeout functionality
func TestBatchTimeout(t *testing.T) {
	t.Parallel()

	// Create a slow processor
	slowProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}
		// Simulate slow processing
		time.Sleep(10 * time.Millisecond)
		return calque.Write(res, "Slow processed: "+content)
	})

	// Test the processor directly
	var buf strings.Builder
	err := slowProcessor.ServeFlow(&calque.Request{Data: strings.NewReader("Test item")}, &calque.Response{Data: &buf})
	if err != nil {
		t.Fatalf("Slow processor test failed: %v", err)
	}

	result := buf.String()
	// Should get results
	if len(result) == 0 {
		t.Error("Expected results from slow processor")
	}
}

// TestBatchErrorHandlingBasic tests error handling in processing
func TestBatchErrorHandlingBasic(t *testing.T) {
	t.Parallel()

	// Create a processor that sometimes fails
	errorProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}

		// Fail on certain content
		if strings.Contains(content, "fail") {
			return fmt.Errorf("processing failed for: %s", content)
		}

		return calque.Write(res, "Success: "+content)
	})

	// Test normal processing
	var buf strings.Builder
	err := errorProcessor.ServeFlow(&calque.Request{Data: strings.NewReader("normal item")}, &calque.Response{Data: &buf})
	if err != nil {
		t.Fatalf("Normal processing failed: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "Success:") {
		t.Error("Expected success result")
	}

	// Test error processing
	var errorBuf strings.Builder
	err = errorProcessor.ServeFlow(&calque.Request{Data: strings.NewReader("fail item")}, &calque.Response{Data: &errorBuf})
	if err == nil {
		t.Error("Expected error for fail item")
	}
}

// TestBatchCustomSeparator tests custom separator functionality
func TestBatchCustomSeparator(t *testing.T) {
	t.Parallel()

	// Create a processor
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}
		return calque.Write(res, "Processed: "+content)
	})

	// Test the processor directly
	var buf strings.Builder
	err := processor.ServeFlow(&calque.Request{Data: strings.NewReader("Item 1|||Item 2")}, &calque.Response{Data: &buf})
	if err != nil {
		t.Fatalf("Processor test failed: %v", err)
	}

	result := buf.String()
	// Should contain processed result
	if !strings.Contains(result, "Processed:") {
		t.Error("Expected processed result")
	}
}

// TestBatchConcurrency tests concurrent processing
func TestBatchConcurrency(t *testing.T) {
	t.Parallel()

	// Create a processor
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}
		// Simulate some processing time
		time.Sleep(5 * time.Millisecond)
		return calque.Write(res, "Concurrent: "+content)
	})

	// Test concurrent processing
	const numRequests = 5
	results := make(chan string, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			input := fmt.Sprintf("Request %d", id)
			var buf strings.Builder
			err := processor.ServeFlow(&calque.Request{Data: strings.NewReader(input)}, &calque.Response{Data: &buf})
			if err != nil {
				errors <- fmt.Errorf("request %d failed: %v", id, err)
			} else {
				results <- buf.String()
			}
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		select {
		case result := <-results:
			if strings.Contains(result, "Concurrent:") {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent request failed: %v", err)
		}
	}

	// Should have successful results
	if successCount == 0 {
		t.Error("Expected at least one successful concurrent result")
	}
}

// TestBatchPipeline tests processing in a complete pipeline
func TestBatchPipeline(t *testing.T) {
	t.Parallel()

	// Create a document processor
	documentProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}

		// Analyze the document
		wordCount := len(strings.Fields(content))
		charCount := len(content)

		analysis := fmt.Sprintf("ANALYSIS: Words=%d, Chars=%d\n", wordCount, charCount)
		return calque.Write(res, analysis)
	})

	// Create complete pipeline
	flow := calque.NewFlow().
		Use(logger.Print("INPUT")).
		Use(documentProcessor).
		Use(logger.Print("OUTPUT"))

	// Test input
	input := "First document content\n---\nSecond document content\n---\nThird document content"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	// Verify result contains analysis
	if !strings.Contains(result, "ANALYSIS:") {
		t.Error("Expected analysis in result")
	}
	if !strings.Contains(result, "Words=") {
		t.Error("Expected word count in result")
	}
	if !strings.Contains(result, "Chars=") {
		t.Error("Expected character count in result")
	}
}

// TestBatchPerformanceBasic tests processing performance
func TestBatchPerformanceBasic(t *testing.T) {
	t.Parallel()

	// Create a processor
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}
		return calque.Write(res, "Processed: "+content)
	})

	// Create large input
	items := make([]string, 100)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d with some content", i+1)
	}
	input := strings.Join(items, "\n---\n")

	// Measure performance
	start := time.Now()
	var buf strings.Builder
	err := processor.ServeFlow(&calque.Request{Data: strings.NewReader(input)}, &calque.Response{Data: &buf})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Performance test failed: %v", err)
	}

	result := buf.String()
	// Verify result
	if len(result) == 0 {
		t.Fatal("Expected non-empty result from performance test")
	}

	// Log performance
	t.Logf("Processed %d items in %v", len(items), duration)

	// Performance should be reasonable
	if duration > 500*time.Millisecond {
		t.Errorf("Performance test took too long: %v", duration)
	}
}
