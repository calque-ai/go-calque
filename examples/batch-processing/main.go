package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/logger"
)

func main() {
	fmt.Println("Batch Processing Examples")
	fmt.Println("=========================")

	fmt.Println("\n1. Document Processing with Batching:")
	documentProcessingExample()

	fmt.Println("\n2. API Batching Simulation:")
	apiBatchingExample()

	fmt.Println("\n3. Performance Comparison:")
	performanceComparisonExample()

	fmt.Println("\n4. Error Handling in Batches:")
	errorHandlingExample()

	fmt.Println("\n5. Different Batch Configurations:")
	batchConfigurationExample()
}

// documentProcessingExample demonstrates batch processing of multiple text files
func documentProcessingExample() {
	// Create a document processor that analyzes text content
	documentProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var content string
		if err := calque.Read(req, &content); err != nil {
			return err
		}

		// Analyze the document
		wordCount := len(strings.Fields(content))
		charCount := len(content)
		lineCount := len(strings.Split(content, "\n"))

		// Find the most common words (simple analysis)
		words := strings.Fields(strings.ToLower(content))
		wordFreq := make(map[string]int)
		for _, word := range words {
			if len(word) > 3 { // Skip short words
				wordFreq[word]++
			}
		}

		// Find top 3 most common words
		var topWords []string
		maxFreq := 0
		for word, freq := range wordFreq {
			if freq > maxFreq {
				maxFreq = freq
				topWords = []string{word}
			} else if freq == maxFreq {
				topWords = append(topWords, word)
			}
		}

		analysis := fmt.Sprintf("DOCUMENT ANALYSIS:\n"+
			"Words: %d\n"+
			"Characters: %d\n"+
			"Lines: %d\n"+
			"Most common words: %v\n"+
			"---BATCH_SEPARATOR---\n", wordCount, charCount, lineCount, topWords)

		return calque.Write(res, analysis)
	})

	// Create batch processor - process 3 documents at a time, wait max 2 seconds
	batchProcessor := ctrl.Batch(documentProcessor, 3, 2*time.Second)

	// Load documents from the data directory
	documents, err := loadDocuments("data/documents")
	if err != nil {
		fmt.Printf("   Error loading documents: %v\n", err)
		return
	}

	fmt.Printf("   Processing %d documents in batches of 3...\n", len(documents))

	// Process documents through the batch pipeline
	flow := calque.NewFlow().
		Use(logger.Print("INPUT")).
		Use(batchProcessor).
		Use(logger.Print("OUTPUT"))

	var result string
	err = flow.Run(context.Background(), strings.Join(documents, "\n---BATCH_SEPARATOR---\n"), &result)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Batch processing completed successfully!\n")
}

// apiBatchingExample simulates batching API requests to external services
func apiBatchingExample() {
	// Simulate an API client that processes requests
	apiClient := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var requests string
		if err := calque.Read(req, &requests); err != nil {
			return err
		}

		// Simulate API processing time
		time.Sleep(100 * time.Millisecond)

		// Process each request in the batch
		requestList := strings.Split(requests, "\n---BATCH_SEPARATOR---\n")
		var responses []string

		for i, request := range requestList {
			if request == "" {
				continue
			}
			// Simulate API response
			response := fmt.Sprintf("API Response %d: Processed '%s' at %s",
				i+1,
				strings.TrimSpace(request),
				time.Now().Format("15:04:05.000"))
			responses = append(responses, response)
		}

		result := strings.Join(responses, "\n---BATCH_SEPARATOR---\n")
		return calque.Write(res, result)
	})

	// Create batch processor for API requests - batch 5 requests, wait max 1 second
	batchAPI := ctrl.Batch(apiClient, 5, 1*time.Second)

	// Simulate multiple API requests
	apiRequests := []string{
		"Get user profile for user123",
		"Update user settings for user456",
		"Fetch product catalog",
		"Process payment for order789",
		"Send notification to user101",
		"Validate email address",
		"Generate report for Q1",
		"Backup database",
	}

	fmt.Printf("   Sending %d API requests in batches of 5...\n", len(apiRequests))

	flow := calque.NewFlow().
		Use(logger.Print("API REQUESTS")).
		Use(batchAPI).
		Use(logger.Print("API RESPONSES"))

	var result string
	err := flow.Run(context.Background(), strings.Join(apiRequests, "\n---BATCH_SEPARATOR---\n"), &result)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   API batching completed successfully!\n")
}

// performanceComparisonExample compares individual vs batch processing
func performanceComparisonExample() {
	// Simple processor that simulates work
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		// Simulate processing time
		time.Sleep(50 * time.Millisecond)

		result := fmt.Sprintf("Processed: %s", strings.TrimSpace(input))
		return calque.Write(res, result)
	})

	// Create batch version
	batchProcessor := ctrl.Batch(processor, 4, 500*time.Millisecond)

	// Test data
	testItems := []string{"Item 1", "Item 2", "Item 3", "Item 4", "Item 5", "Item 6", "Item 7", "Item 8"}

	fmt.Printf("   Comparing individual vs batch processing for %d items...\n", len(testItems))

	// Individual processing (simulated)
	start := time.Now()
	for _, item := range testItems {
		flow := calque.NewFlow().Use(processor)
		var result string
		flow.Run(context.Background(), item, &result)
	}
	individualTime := time.Since(start)

	// Batch processing
	start = time.Now()
	flow := calque.NewFlow().Use(batchProcessor)
	var batchResult string
	flow.Run(context.Background(), strings.Join(testItems, "\n---BATCH_SEPARATOR---\n"), &batchResult)
	batchTime := time.Since(start)

	fmt.Printf("   Individual processing time: %v\n", individualTime)
	fmt.Printf("   Batch processing time: %v\n", batchTime)
	fmt.Printf("   Performance improvement: %.1fx faster\n", float64(individualTime)/float64(batchTime))
}

// errorHandlingExample demonstrates how errors are handled in batch processing
func errorHandlingExample() {
	// Processor that sometimes fails
	unreliableProcessor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		// Simulate occasional failures
		if strings.Contains(input, "fail") {
			return fmt.Errorf("processing failed for: %s", input)
		}

		result := fmt.Sprintf("Successfully processed: %s", strings.TrimSpace(input))
		return calque.Write(res, result)
	})

	// Create batch processor
	batchProcessor := ctrl.Batch(unreliableProcessor, 3, 1*time.Second)

	// Test data with some items that will fail
	testItems := []string{
		"Normal item 1",
		"This will fail",
		"Normal item 2",
		"Another fail item",
		"Normal item 3",
	}

	fmt.Printf("   Testing error handling with %d items (some will fail)...\n", len(testItems))

	flow := calque.NewFlow().
		Use(logger.Print("INPUT")).
		Use(batchProcessor).
		Use(logger.Print("OUTPUT"))

	var result string
	err := flow.Run(context.Background(), strings.Join(testItems, "\n---BATCH_SEPARATOR---\n"), &result)
	if err != nil {
		fmt.Printf("   Batch processing failed: %v\n", err)
	} else {
		fmt.Printf("   Batch processing completed (some items may have failed)\n")
	}
}

// batchConfigurationExample shows different batch configurations
func batchConfigurationExample() {
	processor := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}

		// Simulate processing
		time.Sleep(20 * time.Millisecond)

		result := fmt.Sprintf("Processed: %s", strings.TrimSpace(input))
		return calque.Write(res, result)
	})

	testItems := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}

	configurations := []struct {
		name    string
		maxSize int
		maxWait time.Duration
		desc    string
	}{
		{"Small batches", 2, 100 * time.Millisecond, "Process 2 items or wait 100ms"},
		{"Medium batches", 5, 200 * time.Millisecond, "Process 5 items or wait 200ms"},
		{"Large batches", 10, 500 * time.Millisecond, "Process 10 items or wait 500ms"},
		{"Time-based", 100, 50 * time.Millisecond, "Wait 50ms regardless of size"},
	}

	for _, config := range configurations {
		fmt.Printf("   Testing %s (%s)...\n", config.name, config.desc)

		batchProcessor := ctrl.Batch(processor, config.maxSize, config.maxWait)

		start := time.Now()
		flow := calque.NewFlow().Use(batchProcessor)
		var result string
		err := flow.Run(context.Background(), strings.Join(testItems, "\n---BATCH_SEPARATOR---\n"), &result)
		processingTime := time.Since(start)

		if err != nil {
			fmt.Printf("     Error: %v\n", err)
		} else {
			fmt.Printf("     Completed in %v\n", processingTime)
		}
	}
}

// loadDocuments reads all text files from a directory
func loadDocuments(dirPath string) ([]string, error) {
	var documents []string

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".txt") {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %v", filePath, err)
		}

		documents = append(documents, string(content))
	}

	return documents, nil
}
