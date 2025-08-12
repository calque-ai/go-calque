// Package main demonstrates a streaming chat API using the SSE converter
// and contextual memory middleware.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/pkg/calque"
	"github.com/calque-ai/calque-pipe/pkg/convert"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ai"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ai/ollama"
	"github.com/calque-ai/calque-pipe/pkg/middleware/ctrl"
	"github.com/calque-ai/calque-pipe/pkg/middleware/logger"
	"github.com/calque-ai/calque-pipe/pkg/middleware/memory"
	"github.com/calque-ai/calque-pipe/pkg/middleware/prompt"
)

//go:embed index.html
var indexHTML []byte

// ChatRequest represents the incoming chat request
type ChatRequest struct {
	Message string `json:"message"`
	UserID  string `json:"user_id"`
}

func main() {
	fmt.Println("Calque-Pipe Streaming Chat")
	fmt.Println("Using SSE converter and memory middleware")

	// Initialize AI client (expensive resource created once)
	client, err := ollama.New("llama3.2:1b")
	if err != nil {
		log.Fatal("Failed to create Ollama provider:", err)
	}

	// Create conversation memory
	conversationMemory := memory.NewConversation()

	// Set up routes with initialized resources
	http.HandleFunc("POST /chat", handleStreamingChat(client, conversationMemory))
	http.HandleFunc("GET /", serveHTML)

	// Start server
	fmt.Println("\nServer starting on port 8080...")
	fmt.Println("Open http://localhost:8080 for a web chat interface")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// handleStreamingChat creates an SSE handler for streaming chat responses
func handleStreamingChat(client *ollama.Client, conversationMemory *memory.ConversationMemory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var chatReq ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if chatReq.UserID == "" || chatReq.Message == "" {
			http.Error(w, "user_id and message are required", http.StatusBadRequest)
			return
		}

		// Create SSE converter with chunk by word and custom event fields
		sseConverter := convert.ToSSE(w).WithChunkMode(convert.SSEChunkByWord).
			WithEventFields(map[string]any{
				"user_id":   chatReq.UserID,
				"timestamp": time.Now(),
				"session":   "chat_session",
			})

		// Create agents with fallback
		primaryAgent := ai.Agent(client)
		fallbackAgent := ai.Agent(ai.NewMockClient("Hi there! I'm a mock backup assistant ready to help."))

		// Build pipeline with direct userID access (no context needed)
		pipeline := calque.Flow().
			// 1. Rate limiting
			Use(ctrl.RateLimit(10, time.Second)).
			// 2. Request logging
			Use(logger.Head("CHAT_REQUEST", 100)).
			// 3. Memory input - retrieves memory using userID as a key
			Use(conversationMemory.Input(chatReq.UserID)).
			// 4. Chat prompt template
			Use(prompt.Template("You are a helpful but zany AI assistant. Continue the conversation naturally.\n\n{{.Input}}\n\nagent:")).
			// 5. Agent with fallback
			Use(ctrl.Fallback(primaryAgent, fallbackAgent)).
			// 6. Memory output - stores response with userID
			Use(conversationMemory.Output(chatReq.UserID)).
			// 7. Response logging
			Use(logger.Head("CHAT_RESPONSE", 100))

		// Run pipeline
		err := pipeline.Run(r.Context(), strings.NewReader(chatReq.Message), sseConverter)
		if err != nil {
			log.Printf("Pipeline error: %v", err)
			sseConverter.WriteError(err) // SSE converter handles error formatting
		}
	}
}

// serveHTML serves the embedded HTML chat interface
func serveHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}
