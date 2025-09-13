// Package main demonstrates comprehensive gRPC integration with go-calque.
// This example shows:
// - Distributed AI, memory, and tool services via gRPC
// - Service registry and connection management
// - Protobuf serialization and streaming
// - Type-safe service calls and error handling
// - Health checks and service orchestration
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/memory"
	grpcmw "github.com/calque-ai/go-calque/pkg/middleware/remote/grpc"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
	calquepb "github.com/calque-ai/go-calque/proto"
)

func main() {
	// Use channels for coordination
	servicesReady := make(chan struct{})

	// Start distributed services
	go startDistributedServices(servicesReady)

	// Wait for services to be ready
	<-servicesReady

	// Demonstrate distributed architecture
	demonstrateDistributedAI()
	demonstrateDistributedMemory()
	demonstrateDistributedTools()
	demonstrateFullDistributedFlow()
}

func startDistributedServices(ready chan struct{}) {
	// Channel to track service readiness
	servicesStarted := make(chan struct{}, 3)

	// Start AI service
	go startAIService(":8080", servicesStarted)

	// Start Memory service
	go startMemoryService(":8081", servicesStarted)

	// Start Tools service
	go startToolsService(":8082", servicesStarted)

	// Wait for all services to start
	for i := 0; i < 3; i++ {
		<-servicesStarted
	}

	// Signal that services are ready
	close(ready)
}

func startAIService(addr string, started chan struct{}) {
	server := grpcmw.NewServer(addr)

	// Create a simple AI flow
	flow := calque.NewFlow().
		UseFunc(func(req *calque.Request, res *calque.Response) error {
			var input string
			if err := calque.Read(req, &input); err != nil {
				return err
			}

			// Simple AI processing
			response := fmt.Sprintf("AI Response: %s", input)
			return calque.Write(res, response)
		})

	server.RegisterFlow("ai-flow", flow)

	// Register AI service
	_ = grpcmw.NewFlowService(server)
	calquepb.RegisterAIServiceServer(server.GetServer(), &AIServiceImpl{server: server})

	log.Printf("Starting AI service on %s", addr)

	// Signal that service is ready to start
	started <- struct{}{}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start AI service: %v", err)
	}
}

func startMemoryService(addr string, started chan struct{}) {
	server := grpcmw.NewServer(addr)

	// Create memory flow
	flow := calque.NewFlow().
		UseFunc(func(req *calque.Request, res *calque.Response) error {
			var input string
			if err := calque.Read(req, &input); err != nil {
				return err
			}

			// Simple memory processing
			response := fmt.Sprintf("Memory stored: %s", input)
			return calque.Write(res, response)
		})

	server.RegisterFlow("memory-flow", flow)

	// Register memory service
	_ = grpcmw.NewFlowService(server)
	calquepb.RegisterMemoryServiceServer(server.GetServer(), &MemoryServiceImpl{server: server})

	log.Printf("Starting Memory service on %s", addr)

	// Signal that service is ready to start
	started <- struct{}{}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start Memory service: %v", err)
	}
}

func startToolsService(addr string, started chan struct{}) {
	server := grpcmw.NewServer(addr)

	// Create tools flow
	flow := calque.NewFlow().
		UseFunc(func(req *calque.Request, res *calque.Response) error {
			var input string
			if err := calque.Read(req, &input); err != nil {
				return err
			}

			// Simple tool processing
			response := fmt.Sprintf("Tool executed: %s", input)
			return calque.Write(res, response)
		})

	server.RegisterFlow("tools-flow", flow)

	// Register tools service
	_ = grpcmw.NewFlowService(server)
	calquepb.RegisterToolsServiceServer(server.GetServer(), &ToolsServiceImpl{server: server})

	log.Printf("Starting Tools service on %s", addr)

	// Signal that service is ready to start
	started <- struct{}{}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start Tools service: %v", err)
	}
}

func demonstrateDistributedAI() {
	fmt.Println("\n=== Distributed AI Service ===")

	// Create gRPC AI client
	aiClient, err := ai.NewGRPCClient("localhost:8080")
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// Create flow with distributed AI
	flow := calque.NewFlow().
		Use(ai.Agent(aiClient))

	// Execute flow
	ctx := context.Background()
	var result string
	err = flow.Run(ctx, "Hello from distributed AI!", &result)
	if err != nil {
		aiClient.Close()
		log.Fatalf("Flow execution failed: %v", err)
	}

	fmt.Printf("AI Result: %s\n", result)

	// Cleanup
	aiClient.Close()
}

func demonstrateDistributedMemory() {
	fmt.Println("\n=== Distributed Memory Service ===")

	// Create gRPC memory store
	memoryStore, err := memory.NewGRPCStore("localhost:8081")
	if err != nil {
		log.Fatalf("Failed to create memory store: %v", err)
	}

	// Test basic memory operations directly
	fmt.Println("Testing memory operations...")

	// Test Set operation
	err = memoryStore.Set("test-key", []byte("test-value"))
	if err != nil {
		log.Printf("Set operation failed: %v", err)
	} else {
		fmt.Println("✓ Set operation successful")
	}

	// Test Get operation
	value, err := memoryStore.Get("test-key")
	if err != nil {
		log.Printf("Get operation failed: %v", err)
	} else {
		fmt.Printf("✓ Get operation successful: %s\n", string(value))
	}

	// Test Exists operation
	exists := memoryStore.Exists("test-key")
	fmt.Printf("✓ Exists operation successful: %t\n", exists)

	// Test List operation
	keys := memoryStore.List()
	fmt.Printf("✓ List operation successful: %v\n", keys)

	// Test Delete operation
	err = memoryStore.Delete("test-key")
	if err != nil {
		log.Printf("Delete operation failed: %v", err)
	} else {
		fmt.Println("✓ Delete operation successful")
	}

	// Cleanup
	memoryStore.Close()
}

func demonstrateDistributedTools() {
	fmt.Println("\n=== Distributed Tools Service ===")

	// Create gRPC tool executor
	toolExecutor, err := tools.NewGRPCExecutor("localhost:8082")
	if err != nil {
		log.Fatalf("Failed to create tool executor: %v", err)
	}

	// Create flow with distributed tools
	flow := calque.NewFlow().
		Use(toolExecutor.Execute())

	// Execute flow
	ctx := context.Background()
	var result string
	err = flow.Run(ctx, `{"tool_calls": [{"name": "search", "arguments": "{\"query\": \"golang\"}"}]}`, &result)
	if err != nil {
		toolExecutor.Close()
		log.Fatalf("Flow execution failed: %v", err)
	}

	fmt.Printf("Tools Result: %s\n", result)

	// Cleanup
	toolExecutor.Close()
}

func demonstrateFullDistributedFlow() {
	fmt.Println("\n=== Full Distributed Flow ===")

	// Create distributed clients
	aiClient, err := ai.NewGRPCClient("localhost:8080")
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	memoryStore, err := memory.NewGRPCStore("localhost:8081")
	if err != nil {
		aiClient.Close()
		log.Fatalf("Failed to create memory store: %v", err)
	}

	toolExecutor, err := tools.NewGRPCExecutor("localhost:8082")
	if err != nil {
		aiClient.Close()
		memoryStore.Close()
		log.Fatalf("Failed to create tool executor: %v", err)
	}

	// Create comprehensive distributed flow
	flow := calque.NewFlow().
		Use(memory.NewContextWithStore(memoryStore).Input("session1", 4000)). // Distributed memory
		Use(ai.Agent(aiClient)).                                              // Distributed AI
		Use(toolExecutor.Execute())                                           // Distributed tools

	// Execute comprehensive flow
	ctx := context.Background()
	var result string
	err = flow.Run(ctx, "Process this with distributed services", &result)
	if err != nil {
		aiClient.Close()
		memoryStore.Close()
		toolExecutor.Close()
		log.Fatalf("Flow execution failed: %v", err)
	}

	fmt.Printf("Distributed Flow Result: %s\n", result)

	// Cleanup
	aiClient.Close()
	memoryStore.Close()
	toolExecutor.Close()
}

// Service implementations (simplified for demonstration)
type AIServiceImpl struct {
	calquepb.UnimplementedAIServiceServer
	server *grpcmw.Server
}

// StreamChat implements the AI service streaming chat method
func (s *AIServiceImpl) StreamChat(req *calquepb.AIRequest, stream calquepb.AIService_StreamChatServer) error {
	// Check if this looks like a tool call request (contains the distributed services text)
	if strings.Contains(req.Prompt, "Process this with distributed services") {
		// Return a proper tool call response for the full distributed flow
		// The tool executor expects OpenAI format with tool_calls field
		toolCallResponse := `{"tool_calls": [{"type": "function", "function": {"name": "search", "arguments": "{\"query\": \"distributed services\"}"}}]}`
		resp := &calquepb.AIResponse{
			Response: toolCallResponse,
		}
		return stream.Send(resp)
	}

	// For other requests, send response in chunks to simulate streaming
	chunks := []string{"AI ", "Response: ", req.Prompt}

	// Use channel for timing control instead of time.Sleep
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for i, chunk := range chunks {
		resp := &calquepb.AIResponse{
			Response: chunk,
		}
		if err := stream.Send(resp); err != nil {
			return helpers.WrapGRPCError(err, "failed to send AI response")
		}

		// Wait for ticker on all chunks except the last one
		if i < len(chunks)-1 {
			<-ticker.C
		}
	}

	return nil
}

type MemoryServiceImpl struct {
	calquepb.UnimplementedMemoryServiceServer
	server *grpcmw.Server
}

// ProcessMemory implements the memory service method
func (s *MemoryServiceImpl) ProcessMemory(_ context.Context, req *calquepb.MemoryRequest) (*calquepb.MemoryResponse, error) {
	switch req.Operation {
	case "get":
		// Return proper JSON structure that the memory middleware expects
		// The memory middleware expects a JSON object with max_tokens and content fields
		// The content field should be base64-encoded bytes
		contextData := `{"max_tokens":4000,"content":"UHJldmlvdXMgY29udGV4dCBjb250ZW50Cg=="}`
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   contextData,
		}, nil
	case "set":
		// Simulate setting a value - just acknowledge success
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "stored",
		}, nil
	case "delete":
		// Simulate deleting a value
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "deleted",
		}, nil
	case "list":
		// Simulate listing keys
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "session1",
		}, nil
	case "exists":
		// Simulate checking if key exists
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "true",
		}, nil
	default:
		return &calquepb.MemoryResponse{
			Success:      false,
			ErrorMessage: "unknown operation",
		}, nil
	}
}

type ToolsServiceImpl struct {
	calquepb.UnimplementedToolsServiceServer
	server *grpcmw.Server
}

// ExecuteTool implements the tools service method
func (s *ToolsServiceImpl) ExecuteTool(_ context.Context, req *calquepb.ToolRequest) (*calquepb.ToolResponse, error) {
	// Simulate tool execution with proper JSON format
	result := map[string]interface{}{
		"tool":      req.Name,
		"result":    "executed successfully",
		"arguments": req.Arguments,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &calquepb.ToolResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to marshal result: %v", err),
		}, nil
	}

	return &calquepb.ToolResponse{
		Success: true,
		Result:  string(resultJSON),
	}, nil
}
