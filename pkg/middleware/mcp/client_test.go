package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Test server implementation for unit tests
type testServer struct {
	server  *mcp.Server
	session *mcp.ServerSession
}

// Test tool: simple greeting
type GreetParams struct {
	Name string `json:"name" jsonschema:"the name of the person to greet"`
}

func greetTool(ctx context.Context, req *mcp.CallToolRequest, args GreetParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hello, " + args.Name + "!"},
		},
	}, nil, nil
}

// Test resource handler - handles both static and template-resolved resources
func testResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	switch req.Params.URI {
	case "file:///test/doc.md":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: "# Test Documentation\nThis is a test document for API usage.",
				},
			},
		}, nil
	case "file:///configs/app.json":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: `{"app_name": "test-app", "version": "1.0.0"}`,
				},
			},
		}, nil
	default:
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
}

// Test prompt handler  
func testPromptHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	switch req.Params.Name {
	case "code_review":
		language := "unknown"
		if req.Params.Arguments != nil {
			if lang, ok := req.Params.Arguments["language"]; ok {
				language = lang
			}
		}
		
		return &mcp.GetPromptResult{
			Description: "Code review prompt template",
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("Please review this %s code for best practices and potential issues.", language),
					},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("prompt not found: %s", req.Params.Name)
	}
}

// setupTestServer creates a test server with in-memory transport
func setupTestServer(t *testing.T) (*Client, func()) {
	ctx := context.Background()
	
	// Create in-memory transports
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	
	// Create and configure server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	
	// Add test tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet", 
		Description: "Greet a person by name",
	}, greetTool)
	
	// Add test resource
	server.AddResource(&mcp.Resource{
		URI:         "file:///test/doc.md",
		Name:        "Test Documentation", 
		Description: "Test API documentation",
	}, testResourceHandler)
	
	// Add test resource template  
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "file:///configs/{name}",
		Name:        "Configuration Files",
		Description: "Dynamic access to config files",
	}, testResourceHandler)
	
	// Add test prompt
	server.AddPrompt(&mcp.Prompt{
		Name:        "code_review",
		Description: "Code review prompt template",
	}, testPromptHandler)
	
	// Start server session
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	
	// Create client with in-memory transport
	mcpClient := mcp.NewClient(defaultImplementation(), nil)
	client := &Client{
		client:         mcpClient,
		transport:      clientTransport,
		timeout:        30 * time.Second,
		capabilities:   []string{"tools", "resources", "prompts"},
		implementation: defaultImplementation(),
	}
	
	cleanup := func() {
		if client.session != nil {
			client.session.Close()
		}
		serverSession.Close()
	}
	
	return client, cleanup
}

func TestNewStdio(t *testing.T) {
	client, err := NewStdio("echo", []string{"hello"})
	if err != nil {
		t.Fatalf("NewStdio failed: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	if client.timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", client.timeout)
	}

	if len(client.capabilities) != 3 {
		t.Errorf("Expected 3 default capabilities, got %d", len(client.capabilities))
	}
}

func TestNewStdioWithOptions(t *testing.T) {
	errorCallback := func(error) {}

	client, err := NewStdio("python", []string{"server.py"},
		WithCapabilities("tools"),
		WithTimeout(45*time.Second),
		WithImplementation("test-app", "v1.0.0"),
		WithOnError(errorCallback))

	if err != nil {
		t.Fatalf("NewStdio with options failed: %v", err)
	}

	if len(client.capabilities) != 1 || client.capabilities[0] != "tools" {
		t.Errorf("Expected capabilities [tools], got %v", client.capabilities)
	}

	if client.timeout != 45*time.Second {
		t.Errorf("Expected timeout 45s, got %v", client.timeout)
	}

	if client.implementation.Name != "test-app" {
		t.Errorf("Expected implementation name 'test-app', got %s", client.implementation.Name)
	}

	if client.implementation.Version != "v1.0.0" {
		t.Errorf("Expected implementation version 'v1.0.0', got %s", client.implementation.Version)
	}

	if client.onError == nil {
		t.Error("Expected onError callback to be set")
	}
}

func TestNewSSE(t *testing.T) {
	client, err := NewSSE("http://localhost:3000/mcp")
	if err != nil {
		t.Fatalf("NewSSE failed: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	sseTransport, ok := client.transport.(*mcp.SSEClientTransport)
	if !ok {
		t.Fatal("Expected SSEClientTransport")
	}

	if sseTransport.Endpoint != "http://localhost:3000/mcp" {
		t.Errorf("Expected endpoint 'http://localhost:3000/mcp', got %s", sseTransport.Endpoint)
	}
}

func TestToolHandler(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test tool handler with real MCP server
	handler := client.Tool("greet")
	
	// Input: tool arguments as JSON
	toolArgs := map[string]any{"name": "Alice"}
	argsJSON, _ := json.Marshal(toolArgs)
	
	req := calque.NewRequest(context.Background(), strings.NewReader(string(argsJSON)))
	var output strings.Builder
	res := calque.NewResponse(&output)
	
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Tool handler failed: %v", err)
	}
	
	result := output.String()
	if !strings.Contains(result, "Hello, Alice!") {
		t.Errorf("Expected greeting result, got: %s", result)
	}
}

func TestResourceHandler(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test resource handler with real MCP server
	handler := client.Resource("file:///test/doc.md")

	req := calque.NewRequest(context.Background(), strings.NewReader("How do I use this API?"))
	var output strings.Builder
	res := calque.NewResponse(&output)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Resource handler failed: %v", err)
	}
	
	result := output.String()
	if !strings.Contains(result, "=== User Query ===") {
		t.Errorf("Expected user query section, got: %s", result)
	}
	if !strings.Contains(result, "=== Resource 1 ===") {
		t.Errorf("Expected resource section, got: %s", result) 
	}
	if !strings.Contains(result, "Test Documentation") {
		t.Errorf("Expected resource content, got: %s", result)
	}
}

func TestPromptHandler(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test prompt handler with real MCP server
	handler := client.Prompt("code_review")

	// Test input: prompt arguments as JSON
	promptArgs := map[string]string{
		"language": "go",
		"style":    "detailed",
	}
	argsJSON, _ := json.Marshal(promptArgs)

	req := calque.NewRequest(context.Background(), strings.NewReader(string(argsJSON)))
	var output strings.Builder
	res := calque.NewResponse(&output)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Prompt handler failed: %v", err)
	}
	
	result := output.String()
	if !strings.Contains(result, "user: Please review this go code") {
		t.Errorf("Expected prompt template result, got: %s", result)
	}
}

func TestToolIntegration(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test tool handler with real MCP server
	handler := client.Tool("greet")
	
	// Input: tool arguments as JSON
	toolArgs := map[string]any{"name": "World"}
	argsJSON, _ := json.Marshal(toolArgs)
	
	req := calque.NewRequest(context.Background(), strings.NewReader(string(argsJSON)))
	var output strings.Builder
	res := calque.NewResponse(&output)
	
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Tool handler failed: %v", err)
	}
	
	result := output.String()
	if !strings.Contains(result, "Hello, World!") {
		t.Errorf("Expected greeting result, got: %s", result)
	}
}

func TestResourceTemplateHandler(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test resource template handler with real MCP server
	handler := client.ResourceTemplate("file:///configs/{name}")

	// Input: template variables as JSON
	templateVars := map[string]string{"name": "app.json"}
	varsJSON, _ := json.Marshal(templateVars)

	req := calque.NewRequest(context.Background(), strings.NewReader(string(varsJSON)))
	var output strings.Builder
	res := calque.NewResponse(&output)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Resource template handler failed: %v", err)
	}
	
	result := output.String()
	if !strings.Contains(result, "=== User Query ===") {
		t.Errorf("Expected user query section, got: %s", result)
	}
	if !strings.Contains(result, "=== Resource 1 ===") {
		t.Errorf("Expected resolved resource section, got: %s", result) 
	}
	if !strings.Contains(result, "test-app") {
		t.Errorf("Expected resolved resource content, got: %s", result)
	}
}

func TestCapabilityValidation(t *testing.T) {
	// Test client with limited capabilities
	client, cleanup := setupTestServer(t)
	defer cleanup()
	
	// Override capabilities to test validation
	client.capabilities = []string{"tools"} // Only tools, no resources
	
	// Force reconnection to trigger validation
	if client.session != nil {
		client.session.Close()
		client.session = nil
	}
	
	// This should work - tools are supported
	toolHandler := client.Tool("greet")
	toolArgs := map[string]any{"name": "Test"}
	argsJSON, _ := json.Marshal(toolArgs)
	
	req := calque.NewRequest(context.Background(), strings.NewReader(string(argsJSON)))
	var output strings.Builder
	res := calque.NewResponse(&output)
	
	err := toolHandler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Tool should work with tools capability: %v", err)
	}
}

func TestErrorHandling(t *testing.T) {
	// Test fail-fast behavior (no onError)
	client := &Client{}
	err := client.handleError(fmt.Errorf("test error"))
	if err == nil {
		t.Error("Expected error to bubble up without onError callback")
	}

	// Test resilient behavior (with onError)
	var capturedError error
	client.onError = func(err error) {
		capturedError = err
	}

	testErr := fmt.Errorf("test error")
	err = client.handleError(testErr)
	if err != nil {
		t.Errorf("Expected error to be handled, got: %v", err)
	}

	if capturedError == nil {
		t.Error("Expected error to be captured by callback")
	}

	if capturedError.Error() != "test error" {
		t.Errorf("Expected 'test error', got: %v", capturedError)
	}
}
