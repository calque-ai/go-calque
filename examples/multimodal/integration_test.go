package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/convert"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
)

// TestMultimodalSimple tests simple multimodal approach with image data
func TestMultimodalSimple(t *testing.T) {
	t.Parallel()

	// Create mock multimodal client that can handle images
	mockClient := ai.NewMockClient("This is a test image showing a beautiful landscape with mountains and trees.")
	agent := ai.Agent(mockClient)

	// Create sample image data (small test image)
	imageData := []byte("fake image data for testing")

	// Create multimodal input with simple approach
	multimodalInput := ai.Multimodal(
		ai.Text("Please analyze this image and describe what you see."),
		ai.ImageData(imageData, "image/jpeg"),
	)

	flow := calque.NewFlow()
	flow.
		Use(inspect.Head("INPUT", 100)).
		Use(agent).
		Use(inspect.Head("RESPONSE", 100))

	// Run the flow
	var result string
	err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		t.Fatalf("Multimodal simple test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "landscape") {
		t.Errorf("Expected response to mention landscape, got: %s", result)
	}
}

// TestMultimodalStreaming tests streaming multimodal approach
func TestMultimodalStreaming(t *testing.T) {
	t.Parallel()

	// Create mock client for streaming
	mockClient := ai.NewMockClient("Streaming analysis: This appears to be a test image with various elements.")
	agent := ai.Agent(mockClient)

	// Create sample image data
	imageData := []byte("streaming test image data")

	// Create multimodal input with streaming approach
	multimodalInput := ai.Multimodal(
		ai.Text("Analyze this image using streaming approach."),
		ai.Image(bytes.NewReader(imageData), "image/png"),
	)

	flow := calque.NewFlow()
	flow.
		Use(inspect.Head("INPUT", 100)).
		Use(agent).
		Use(inspect.Head("RESPONSE", 100))

	// Run the flow
	var result string
	err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		t.Fatalf("Multimodal streaming test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Streaming analysis") {
		t.Errorf("Expected streaming analysis response, got: %s", result)
	}
}

// TestMultimodalTextOnly tests multimodal with text-only input
func TestMultimodalTextOnly(t *testing.T) {
	t.Parallel()

	// Create mock client
	mockClient := ai.NewMockClient("Text analysis: This is a text-only request for testing purposes.")
	agent := ai.Agent(mockClient)

	// Create text-only multimodal input
	multimodalInput := ai.Multimodal(
		ai.Text("This is a text-only request for testing."),
	)

	flow := calque.NewFlow()
	flow.Use(agent)

	// Run the flow
	var result string
	err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		t.Fatalf("Multimodal text-only test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Text analysis") {
		t.Errorf("Expected text analysis response, got: %s", result)
	}
}

// TestMultimodalMultipleImages tests multimodal with multiple images
func TestMultimodalMultipleImages(t *testing.T) {
	t.Parallel()

	// Create mock client
	mockClient := ai.NewMockClient("Multiple images analysis: I can see two test images, both containing sample data.")
	agent := ai.Agent(mockClient)

	// Create sample image data
	image1 := []byte("first test image")
	image2 := []byte("second test image")

	// Create multimodal input with multiple images
	multimodalInput := ai.Multimodal(
		ai.Text("Analyze these two images and compare them."),
		ai.ImageData(image1, "image/jpeg"),
		ai.ImageData(image2, "image/png"),
	)

	flow := calque.NewFlow()
	flow.Use(agent)

	// Run the flow
	var result string
	err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		t.Fatalf("Multimodal multiple images test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Multiple images analysis") {
		t.Errorf("Expected multiple images analysis response, got: %s", result)
	}
}

// TestMultimodalComplexPrompt tests multimodal with complex prompts
func TestMultimodalComplexPrompt(t *testing.T) {
	t.Parallel()

	// Create mock client
	mockClient := ai.NewMockClient("Complex analysis: Based on the image and detailed prompt, I can provide a comprehensive analysis including technical details, artistic elements, and practical applications.")
	agent := ai.Agent(mockClient)

	// Create sample image data
	imageData := []byte("complex test image data")

	// Create complex multimodal input
	complexPrompt := `Please provide a comprehensive analysis of this image including:
1. Technical details and specifications
2. Artistic elements and composition
3. Practical applications and use cases
4. Potential improvements or modifications`

	multimodalInput := ai.Multimodal(
		ai.Text(complexPrompt),
		ai.ImageData(imageData, "image/jpeg"),
	)

	flow := calque.NewFlow()
	flow.Use(agent)

	// Run the flow
	var result string
	err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		t.Fatalf("Multimodal complex prompt test failed: %v", err)
	}

	// Verify the result contains expected elements
	if !strings.Contains(result, "Complex analysis") {
		t.Errorf("Expected complex analysis response, got: %s", result)
	}
	if !strings.Contains(result, "comprehensive") {
		t.Errorf("Expected comprehensive analysis, got: %s", result)
	}
}

// TestMultimodalPipeline tests multimodal in a complex pipeline
func TestMultimodalPipeline(t *testing.T) {
	t.Parallel()

	// Create mock client
	mockClient := ai.NewMockClient("Pipeline processed: This image has been analyzed through multiple processing steps including validation, enhancement, and detailed analysis.")
	agent := ai.Agent(mockClient)

	// Create sample image data
	imageData := []byte("pipeline test image")

	// Create multimodal input
	multimodalInput := ai.Multimodal(
		ai.Text("Process this image through the complete analysis pipeline."),
		ai.ImageData(imageData, "image/jpeg"),
	)

	// Create complex pipeline
	flow := calque.NewFlow()
	flow.
		Use(inspect.Print("START")).
		Use(inspect.Head("INPUT", 200)).
		Use(agent).
		Use(inspect.Head("RESPONSE", 200)).
		Use(inspect.Print("END"))

	// Run the flow
	var result string
	err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		t.Fatalf("Multimodal pipeline test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Pipeline processed") {
		t.Errorf("Expected pipeline processed response, got: %s", result)
	}
}

// TestMultimodalErrorHandling tests error handling in multimodal processing
func TestMultimodalErrorHandling(t *testing.T) {
	t.Parallel()

	// Create mock client that might fail
	mockClient := ai.NewMockClient("Error handling test: Successfully processed despite potential issues.")
	agent := ai.Agent(mockClient)

	// Create multimodal input with potentially problematic data
	multimodalInput := ai.Multimodal(
		ai.Text("Test error handling capabilities."),
		ai.ImageData([]byte(""), "image/jpeg"), // Empty image data
	)

	flow := calque.NewFlow()
	flow.Use(agent)

	// Run the flow - should handle empty data gracefully
	var result string
	err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
	if err != nil {
		t.Fatalf("Multimodal error handling test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Error handling test") {
		t.Errorf("Expected error handling test response, got: %s", result)
	}
}

// TestMultimodalConcurrency tests multimodal processing under concurrent access
func TestMultimodalConcurrency(t *testing.T) {
	t.Parallel()

	// Create mock client
	mockClient := ai.NewMockClient("Concurrent processing: Successfully handled concurrent multimodal requests.")
	agent := ai.Agent(mockClient)

	// Create sample image data
	imageData := []byte("concurrent test image")

	// Create multimodal input
	multimodalInput := ai.Multimodal(
		ai.Text("Process this image concurrently."),
		ai.ImageData(imageData, "image/jpeg"),
	)

	// Create pipeline
	flow := calque.NewFlow()
	flow.Use(agent)

	// Test concurrent processing
	const numRequests = 3
	results := make(chan string, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			var result string
			err := flow.Run(context.Background(), convert.ToJSON(multimodalInput), &result)
			if err != nil {
				errors <- fmt.Errorf("request %d failed: %v", id, err)
			} else {
				results <- result
			}
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		select {
		case result := <-results:
			if strings.Contains(result, "Concurrent processing") {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent request failed: %v", err)
		}
	}

	// Verify we got successful responses
	if successCount == 0 {
		t.Error("Expected at least one successful concurrent response")
	}
}
