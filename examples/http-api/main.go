// Package main demonstrates how to transform a Calque-Pipe pipeline into an HTTP API server.
// This example shows the fundamental pattern for exposing AI agent pipelines as web services.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/calque-ai/calque-pipe/pkg/middleware/logger"
	"github.com/calque-ai/calque-pipe/pkg/middleware/text"
)

// Request represents the incoming API request structure
type Request struct {
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"`
}

// Response represents the API response structure
type Response struct {
	Result    string    `json:"result"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	fmt.Println("Calque-Pipe HTTP API Server Example")
	fmt.Println("This demonstrates transforming pipelines into HTTP endpoints")

	// Create the agent pipeline once and reuse it
	agentPipeline := createAgentPipeline()

	// Set up routes
	http.HandleFunc("POST /agent", handleAgent(agentPipeline))

	// Start the HTTP server
	fmt.Println("\nServer starting on port 8080...")
	fmt.Println("Try: curl -X POST http://localhost:8080/agent -H 'Content-Type: application/json' -d '{\"message\":\"hello world\",\"user_id\":\"123\"}'")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// createAgentPipeline builds the processing pipeline that will handle requests
func createAgentPipeline() *calque.Pipeline {

	// For a pipeline used in an http handler you may want to limit the number of goroutines created
	// since you might have thousands of concurrent requests X each handler in your pipeline.
	config := calque.PipelineConfig{
		MaxConcurrent: calque.ConcurrencyAuto, // Auto uses the default multiplier (50x) with GOMAXPROCS
		// CPUMultiplier: 100,  // Or set your own multiplier
	}

	pipe := calque.Flow(config)
	pipe.
		Use(logger.Head("HTTP_REQUEST", 200)).                                    // Log incoming request
		Use(text.Transform(func(s string) string { return strings.ToUpper(s) })). // Transform message to uppercase
		Use(text.Transform(func(s string) string { return "Processed: " + s })).  // Add prefix
		Use(logger.Head("PROCESSED_MESSAGE", 200))                                // Log processed result

	return pipe
}

// handleAgent creates an HTTP handler that uses the Calque-Pipe pipeline
func handleAgent(pipeline *calque.Pipeline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Process the message through the pipeline
		var processedMessage string
		err := pipeline.Run(r.Context(), r.Body, &processedMessage)
		if err != nil {
			log.Printf("Pipeline error: %v", err)
			http.Error(w, `{"error":"Internal processing error"}`, http.StatusInternalServerError)
			return
		}

		// Create response
		response := Response{
			Result:    processedMessage,
			Timestamp: time.Now(),
		}

		// Send JSON response
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
			http.Error(w, `{"error":"Failed to generate response"}`, http.StatusInternalServerError)
			return
		}
	}
}
