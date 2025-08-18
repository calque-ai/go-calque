package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
)

func TestBatchProcessing(t *testing.T) {
	// Test processor that counts words
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		wordCount := len(strings.Fields(input))
		result := fmt.Sprintf("Words: %d", wordCount)
		return calque.Write(res, result)
	})

	// Create batch processor
	batchProcessor := ctrl.Batch(processor, 3, 100*time.Millisecond)

	// Test data
	testItems := []string{
		"hello world",
		"this is a test",
		"batch processing example",
		"fourth item",
		"fifth item",
	}

	// Process through batch
	flow := calque.NewFlow().Use(batchProcessor)
	var result string
	err := flow.Run(context.Background(), strings.Join(testItems, "\n---BATCH_SEPARATOR---\n"), &result)
	if err != nil {
		t.Fatalf("Batch processing failed: %v", err)
	}

	// Verify result contains expected output
	if !strings.Contains(result, "Words:") {
		t.Errorf("Expected result to contain word count, got: %s", result)
	}
}

func TestBatchErrorHandling(t *testing.T) {
	// Processor that fails on certain inputs
	unreliableProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		if strings.Contains(input, "fail") {
			return fmt.Errorf("processing failed for: %s", input)
		}

		result := fmt.Sprintf("Success: %s", strings.TrimSpace(input))
		return calque.Write(res, result)
	})

	// Create batch processor
	batchProcessor := ctrl.Batch(unreliableProcessor, 2, 50*time.Millisecond)

	// Test data with some items that will fail
	testItems := []string{
		"normal item",
		"this will fail",
		"another normal item",
	}

	// Process through batch
	flow := calque.NewFlow().Use(batchProcessor)
	var result string
	err := flow.Run(context.Background(), strings.Join(testItems, "\n---BATCH_SEPARATOR---\n"), &result)

	// Should fail due to the "fail" item
	if err == nil {
		t.Error("Expected batch processing to fail, but it succeeded")
	}
}

func TestBatchPerformance(t *testing.T) {
	// Simple processor with simulated work
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		// Simulate processing time
		time.Sleep(10 * time.Millisecond)

		result := fmt.Sprintf("Processed: %s", strings.TrimSpace(input))
		return calque.Write(res, result)
	})

	// Create batch processor
	batchProcessor := ctrl.Batch(processor, 5, 100*time.Millisecond)

	// Test data
	testItems := []string{"A", "B", "C", "D", "E", "F", "G", "H"}

	// Measure batch processing time
	start := time.Now()
	flow := calque.NewFlow().Use(batchProcessor)
	var result string
	err := flow.Run(context.Background(), strings.Join(testItems, "\n---BATCH_SEPARATOR---\n"), &result)
	batchTime := time.Since(start)

	if err != nil {
		t.Fatalf("Batch processing failed: %v", err)
	}

	// Verify reasonable performance (should be faster than individual processing)
	expectedMaxTime := 200 * time.Millisecond // 2 batches * 100ms max wait
	if batchTime > expectedMaxTime {
		t.Errorf("Batch processing took too long: %v (expected < %v)", batchTime, expectedMaxTime)
	}

	// Verify result contains expected output
	// The batch processor processes the entire batch as one item
	expectedItems := 1
	processedCount := strings.Count(result, "Processed:")
	if processedCount != expectedItems {
		t.Errorf("Expected %d processed items, got %d. Result: %s", expectedItems, processedCount, result)
	}

	// Verify all input items are present in the result
	for _, item := range testItems {
		if !strings.Contains(result, item) {
			t.Errorf("Expected result to contain item '%s', but it doesn't", item)
		}
	}
}

func TestLoadDocuments(t *testing.T) {
	// Test loading documents from the data directory
	documents, err := loadDocuments("data/documents")
	if err != nil {
		t.Fatalf("Failed to load documents: %v", err)
	}

	// Should have 3 documents
	if len(documents) != 3 {
		t.Errorf("Expected 3 documents, got %d", len(documents))
	}

	// Each document should have content
	for i, doc := range documents {
		if len(strings.TrimSpace(doc)) == 0 {
			t.Errorf("Document %d is empty", i)
		}
	}
}
