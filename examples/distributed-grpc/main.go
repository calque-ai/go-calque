// Package main demonstrates comprehensive gRPC integration with go-calque using the registry pattern.
// This example shows:
// - Distributed services via gRPC using the registry pattern
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
	grpcerrors "github.com/calque-ai/go-calque/pkg/grpc"
	grpcmw "github.com/calque-ai/go-calque/pkg/middleware/remote/grpc"
	calquepb "github.com/calque-ai/go-calque/proto"
)

func main() {
	// Use channels for coordination
	servicesReady := make(chan struct{})

	// Start distributed services
	go startDistributedServices(servicesReady)

	// Wait for services to be ready
	<-servicesReady

	// Demonstrate distributed architecture using registry pattern
	demonstrateRegistryPattern()
	demonstrateTypeSafeCalls()
	demonstrateStreamingServices()
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
			response := fmt.Sprintf("Memory processed: %s", input)
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

func demonstrateRegistryPattern() {
	fmt.Println("\n=== Registry Pattern Demo ===")

	// Create flow using registry pattern
	flow := calque.NewFlow().
		Use(grpcmw.NewRegistryHandler(
			grpcmw.NewService("ai-service", "localhost:8080"),
			grpcmw.NewService("memory-service", "localhost:8081"),
			grpcmw.NewService("tools-service", "localhost:8082"),
		)).
		Use(grpcmw.Call("ai-service")).
		Use(grpcmw.Call("memory-service")).
		Use(grpcmw.Call("tools-service"))

	// Execute flow
	ctx := context.Background()
	var result string
	err := flow.Run(ctx, "Hello from registry pattern!", &result)
	if err != nil {
		log.Fatalf("Registry pattern flow execution failed: %v", err)
	}

	fmt.Printf("Registry Pattern Result: %s\n", result)
}

func demonstrateTypeSafeCalls() {
	fmt.Println("\n=== Type-Safe Calls Demo ===")

	// Create type-safe flow using registry pattern
	flow := calque.NewFlow().
		Use(grpcmw.NewRegistryHandler(
			grpcmw.ServiceWithTypes[*calquepb.AIRequest, *calquepb.AIResponse]("ai-service", "localhost:8080"),
			grpcmw.ServiceWithTypes[*calquepb.MemoryRequest, *calquepb.MemoryResponse]("memory-service", "localhost:8081"),
		)).
		Use(grpcmw.CallWithTypes[*calquepb.AIRequest, *calquepb.AIResponse]("ai-service")).
		Use(grpcmw.CallWithTypes[*calquepb.MemoryRequest, *calquepb.MemoryResponse]("memory-service"))

	// Execute flow
	ctx := context.Background()
	var result string
	err := flow.Run(ctx, "Type-safe call demo", &result)
	if err != nil {
		log.Fatalf("Type-safe calls flow execution failed: %v", err)
	}

	fmt.Printf("Type-Safe Calls Result: %s\n", result)
}

func demonstrateStreamingServices() {
	fmt.Println("\n=== Streaming Services Demo ===")

	// Create streaming flow using registry pattern
	flow := calque.NewFlow().
		Use(grpcmw.NewRegistryHandler(
			grpcmw.StreamingService("ai-service", "localhost:8080"),
			grpcmw.StreamingService("memory-service", "localhost:8081"),
		)).
		Use(grpcmw.Stream("ai-service")).
		Use(grpcmw.Stream("memory-service"))

	// Execute flow
	ctx := context.Background()
	var result string
	err := flow.Run(ctx, "Streaming demo", &result)
	if err != nil {
		log.Fatalf("Streaming services flow execution failed: %v", err)
	}

	fmt.Printf("Streaming Services Result: %s\n", result)
}

func demonstrateFullDistributedFlow() {
	fmt.Println("\n=== Full Distributed Flow with Registry ===")

	// Create comprehensive distributed flow using registry pattern
	flow := calque.NewFlow().
		Use(grpcmw.NewRegistryHandler(
			grpcmw.NewService("ai-service", "localhost:8080").WithTimeout(10*time.Second),
			grpcmw.NewService("memory-service", "localhost:8081").WithTimeout(5*time.Second),
			grpcmw.NewService("tools-service", "localhost:8082").WithTimeout(15*time.Second),
		)).
		Use(grpcmw.Call("ai-service")).
		Use(grpcmw.Call("memory-service")).
		Use(grpcmw.Call("tools-service"))

	// Execute comprehensive flow
	ctx := context.Background()
	var result string
	err := flow.Run(ctx, "Process this with distributed services using registry pattern", &result)
	if err != nil {
		log.Fatalf("Full distributed flow execution failed: %v", err)
	}

	fmt.Printf("Full Distributed Flow Result: %s\n", result)
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
			return grpcerrors.WrapError(err, "failed to send AI response")
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
	// Simple memory processing based on operation
	switch req.Operation {
	case "set":
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "Memory set successfully",
		}, nil
	case "get":
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "Retrieved memory value",
		}, nil
	case "delete":
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "Memory deleted successfully",
		}, nil
	case "list":
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "key1,key2,key3",
		}, nil
	case "exists":
		return &calquepb.MemoryResponse{
			Success: true,
			Value:   "true",
		}, nil
	default:
		return &calquepb.MemoryResponse{
			Success: false,
			Value:   "Unknown operation",
		}, nil
	}
}

type ToolsServiceImpl struct {
	calquepb.UnimplementedToolsServiceServer
	server *grpcmw.Server
}

// ExecuteTool implements the tools service method
func (s *ToolsServiceImpl) ExecuteTool(_ context.Context, req *calquepb.ToolRequest) (*calquepb.ToolResponse, error) {
	// Simple tool execution
	result := map[string]interface{}{
		"tool":      req.Name,
		"arguments": req.Arguments,
		"result":    fmt.Sprintf("Tool %s executed successfully", req.Name),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &calquepb.ToolResponse{
			Success: false,
			Result:  "",
		}, err
	}

	return &calquepb.ToolResponse{
		Success: true,
		Result:  string(resultJSON),
	}, nil
}
