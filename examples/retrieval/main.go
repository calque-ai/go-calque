// Package main demonstrates retrieval package usage including vector search and RAG pipelines.
// It shows basic search with JSON results, context strategies, and AI-augmented generation.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/calque-ai/go-calque/examples/retrieval/mock"
	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/ollama"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/logger"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
	"github.com/calque-ai/go-calque/pkg/middleware/retrieval"
)

func main() {
	// Create mock vector store and load knowledge base
	store := mock.New()
	loadKnowledgeBase(store)

	runBasicSearchExample(store)

	runContextStrategyExample(store)

	// Initialize AI client (using Ollama as a free, local option)
	client, err := ollama.New("llama3.2:1b")
	if err != nil {
		log.Printf("Warning: Could not connect to Ollama: %v", err)
		log.Println("Skipping RAG example. Please ensure Ollama is running.")
		return
	}

	runRAGExample(store, client)
}

// loadKnowledgeBase populates the store with sample knowledge documents
func loadKnowledgeBase(store retrieval.VectorStore) {
	docs := []retrieval.Document{
		{
			ID:      "calque-intro",
			Content: "Calque is a Go framework for building AI-powered data processing pipelines. It provides middleware patterns for composing flows that can process, transform, and analyze data through AI models.",
			Metadata: map[string]any{
				"category": "overview",
				"topic":    "calque",
			},
			Created: time.Now(),
		},
		{
			ID:      "calque-flows",
			Content: "Flows in Calque are the primary abstraction for data processing. You create a flow using calque.NewFlow() and chain middleware handlers using the Use() method. Flows can be composed into larger pipelines.",
			Metadata: map[string]any{
				"category": "concepts",
				"topic":    "flows",
			},
			Created: time.Now(),
		},
		{
			ID:      "calque-middleware",
			Content: "Middleware in Calque processes data as it flows through the pipeline. Types include text transformers, AI agents, logging, branching, and retrieval. Middleware can be streaming or buffered.",
			Metadata: map[string]any{
				"category": "concepts",
				"topic":    "middleware",
			},
			Created: time.Now(),
		},
		{
			ID:      "retrieval-search",
			Content: "The retrieval package provides VectorSearch middleware for semantic search. It supports multiple strategies: StrategyRelevant for score-based ranking, StrategyRecent for timestamp ordering, and StrategyDiverse for MMR-based diversity selection.",
			Metadata: map[string]any{
				"category": "retrieval",
				"topic":    "vector-search",
			},
			Created: time.Now(),
		},
		{
			ID:      "retrieval-documents",
			Content: "DocumentLoader middleware loads documents from files and URLs. It supports glob patterns for file paths and can fetch from HTTP/HTTPS endpoints. Documents include content, metadata, and timestamps.",
			Metadata: map[string]any{
				"category": "retrieval",
				"topic":    "document-loader",
			},
			Created: time.Now(),
		},
	}

	err := store.Store(context.Background(), docs)
	if err != nil {
		log.Fatalf("Failed to load knowledge base: %v", err)
	}
}

// runBasicSearchExample demonstrates basic vector search returning SearchResult JSON
func runBasicSearchExample(store retrieval.VectorStore) {
	fmt.Println("\n=== Example 1: Basic Vector Search ===")

	opts := &retrieval.SearchOptions{
		Threshold: 0.2,
		Limit:     3,
	}

	flow := calque.NewFlow().
		Use(logger.Print("QUERY")).
		Use(retrieval.VectorSearch(store, opts)).
		Use(logger.Print("RESULTS"))

	query := "How do I build data processing pipelines?"
	fmt.Printf("Query: %q\n\n", query)

	var result string
	err := flow.Run(context.Background(), query, &result)
	if err != nil {
		log.Printf("Search error: %v", err)
		return
	}

	// Pretty print the JSON result
	var searchResult retrieval.SearchResult
	if err := json.Unmarshal([]byte(result), &searchResult); err == nil {
		fmt.Printf("Found %d documents:\n", searchResult.Total)
		for i, doc := range searchResult.Documents {
			fmt.Printf("  %d. [Score: %.2f] %s\n", i+1, doc.Score, truncate(doc.Content, 60))
		}
	}
}

// runContextStrategyExample demonstrates search with diversity strategy
func runContextStrategyExample(store retrieval.VectorStore) {
	fmt.Println("\n=== Example 2: Search with Diverse Strategy ===")

	strategy := retrieval.StrategyDiverse
	opts := &retrieval.SearchOptions{
		Threshold: 0.2,
		Limit:     3,
		Strategy:  &strategy,
		MaxTokens: 1000,
	}

	flow := calque.NewFlow().
		Use(logger.Print("QUERY")).
		Use(retrieval.VectorSearch(store, opts)).
		Use(logger.Print("DIVERSE_CONTEXT"))

	query := "How do I use Calque for building flows with retrieval?"
	fmt.Printf("Query: %q\n\n", query)

	var result string
	err := flow.Run(context.Background(), query, &result)
	if err != nil {
		log.Printf("Retrieval error: %v", err)
		return
	}

	fmt.Printf("Retrieved Context (diverse):\n%s\n", result)
}

// runRAGExample demonstrates a complete RAG pipeline with AI generation
func runRAGExample(store retrieval.VectorStore, client ai.Client) {
	fmt.Println("\n=== Example 3: RAG Pipeline ===")

	// Configure retrieval with relevance strategy
	strategy := retrieval.StrategyRelevant
	searchOpts := &retrieval.SearchOptions{
		Threshold: 0.2,
		Limit:     3,
		Strategy:  &strategy,
		MaxTokens: 800,
		Separator: "\n\n",
	}

	// Build RAG pipeline
	flow := calque.NewFlow().
		Use(logger.Print("USER_QUERY")).
		Use(retrieval.VectorSearch(store, searchOpts)).
		Use(prompt.Template(`Based on the following context, answer the user's question.

Context:
{{.Input}}

Question: How do I create a data processing flow with retrieval in Calque?

Answer:`)).
		Use(logger.Print("RAG_PROMPT")).
		Use(ctrl.Timeout(ai.Agent(client), 30*time.Second)).
		Use(logger.Head("AI_RESPONSE", 500))

	query := "How do I create a data processing flow with retrieval in Calque?"
	fmt.Printf("Query: %q\n\n", query)

	var result string
	err := flow.Run(context.Background(), query, &result)
	if err != nil {
		log.Printf("RAG pipeline error: %v", err)
		return
	}

	fmt.Printf("\nFinal Answer:\n%s\n", result)
}

// truncate shortens a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
