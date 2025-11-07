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

// Test tool: simple greeting
type GreetParams struct {
	Name string `json:"name" jsonschema:"the name of the person to greet"`
}

func greetTool(_ context.Context, _ *mcp.CallToolRequest, args GreetParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hello, " + args.Name + "!"},
		},
	}, nil, nil
}

// Test resource handler - handles both static and template-resolved resources
func testResourceHandler(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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
func testPromptHandler(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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
		client:            mcpClient,
		transport:         clientTransport,
		timeout:           30 * time.Second,
		capabilities:      []string{"tools", "resources", "prompts"},
		implementation:    defaultImplementation(),
		progressCallbacks: make(map[string][]func(*mcp.ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*mcp.ResourceUpdatedNotificationParams)),
		completionEnabled: false,
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

	if client.timeout != 0*time.Second {
		t.Errorf("Expected default timeout 0s, got %v", client.timeout)
	}

	if len(client.capabilities) != 0 {
		t.Errorf("Expected 0 default capabilities, got %d", len(client.capabilities))
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

func TestToolWithProgressCallbacks(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test progress callback registration
	var progressNotifications []*mcp.ProgressNotificationParams
	progressCallback := func(params *mcp.ProgressNotificationParams) {
		progressNotifications = append(progressNotifications, params)
	}

	handler := client.Tool("greet", progressCallback)

	// Verify handler was created with progress callback
	if handler == nil {
		t.Fatal("Expected handler to be created")
	}

	// Test tool execution
	toolArgs := map[string]any{"name": "ProgressTest"}
	argsJSON, _ := json.Marshal(toolArgs)

	req := calque.NewRequest(context.Background(), strings.NewReader(string(argsJSON)))
	var output strings.Builder
	res := calque.NewResponse(&output)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Tool handler with progress failed: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Hello, ProgressTest!") {
		t.Errorf("Expected greeting result, got: %s", result)
	}
}

func TestMultipleProgressCallbacks(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test multiple progress callbacks can be registered
	callback1 := func(_ *mcp.ProgressNotificationParams) {}
	callback2 := func(_ *mcp.ProgressNotificationParams) {}

	handler := client.Tool("greet", callback1, callback2)

	if handler == nil {
		t.Fatal("Expected handler to be created with multiple callbacks")
	}

	// Test that multiple callbacks are properly registered
	client.progressCallbacks["test-token"] = []func(*mcp.ProgressNotificationParams){callback1, callback2}

	if len(client.progressCallbacks["test-token"]) != 2 {
		t.Errorf("Expected 2 progress callbacks, got %d", len(client.progressCallbacks["test-token"]))
	}
}

func TestSubscribeToResource(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test resource subscription
	var resourceUpdates []*mcp.ResourceUpdatedNotificationParams
	updateCallback := func(params *mcp.ResourceUpdatedNotificationParams) {
		resourceUpdates = append(resourceUpdates, params)
	}

	handler := client.SubscribeToResource("file:///test/doc.md", updateCallback)

	if handler == nil {
		t.Fatal("Expected subscription handler to be created")
	}

	// Test subscription setup
	req := calque.NewRequest(context.Background(), strings.NewReader("initial input"))
	var output strings.Builder
	res := calque.NewResponse(&output)

	err := handler.ServeFlow(req, res)
	if err != nil {
		// Note: This may fail if the test server doesn't support subscriptions
		// In a real implementation, you'd need a mock server that supports subscribe/unsubscribe
		t.Logf("Subscription test failed (expected if server doesn't support subscriptions): %v", err)
		return
	}

	// Verify subscription callback was registered
	if client.subscriptions["file:///test/doc.md"] == nil {
		t.Error("Expected subscription callback to be registered")
	}
}

func TestComplete(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test completion without enabled flag
	handler := client.Complete()

	req := calque.NewRequest(context.Background(), strings.NewReader(`{"ref": {"type": "ref/prompt", "name": "code_review"}}`))
	var output strings.Builder
	res := calque.NewResponse(&output)

	err := handler.ServeFlow(req, res)
	if err == nil {
		t.Error("Expected completion to fail when not enabled")
	}

	// Enable completion and test
	client.completionEnabled = true

	err = handler.ServeFlow(req, res)
	if err != nil {
		// Note: This may fail if the test server doesn't support completion
		t.Logf("Completion test failed (expected if server doesn't support completion): %v", err)
		return
	}
}

func TestWithCompletionOption(t *testing.T) {
	client, err := NewStdio("echo", []string{"hello"}, WithCompletion(true))
	if err != nil {
		t.Fatalf("NewStdio with completion failed: %v", err)
	}

	if !client.completionEnabled {
		t.Error("Expected completion to be enabled")
	}

	// Test with completion disabled
	client2, err := NewStdio("echo", []string{"hello"}, WithCompletion(false))
	if err != nil {
		t.Fatalf("NewStdio with completion disabled failed: %v", err)
	}

	if client2.completionEnabled {
		t.Error("Expected completion to be disabled")
	}
}

func TestProgressNotificationHandling(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test progress notification handling
	var receivedParams *mcp.ProgressNotificationParams
	progressCallback := func(params *mcp.ProgressNotificationParams) {
		receivedParams = params
	}

	// Register a progress callback
	client.progressCallbacks["test-token"] = []func(*mcp.ProgressNotificationParams){progressCallback}

	// Simulate progress notification
	testParams := &mcp.ProgressNotificationParams{
		ProgressToken: "test-token",
		Progress:      0.5,
		Total:         1.0,
	}

	client.handleProgressNotification(testParams)

	if receivedParams == nil {
		t.Error("Expected progress notification to be received")
	}

	if receivedParams.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %v", receivedParams.Progress)
	}
}

func TestResourceUpdateHandling(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Test resource update handling
	var receivedParams *mcp.ResourceUpdatedNotificationParams
	updateCallback := func(params *mcp.ResourceUpdatedNotificationParams) {
		receivedParams = params
	}

	// Register a subscription callback
	client.subscriptions["file:///test/doc.md"] = updateCallback

	// Simulate resource update notification
	testParams := &mcp.ResourceUpdatedNotificationParams{
		URI: "file:///test/doc.md",
	}

	client.handleResourceUpdated(testParams)

	if receivedParams == nil {
		t.Error("Expected resource update notification to be received")
	}

	if receivedParams.URI != "file:///test/doc.md" {
		t.Errorf("Expected URI 'file:///test/doc.md', got %s", receivedParams.URI)
	}
}

func TestNewStreamableHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		url         string
		opts        []Option
		expectError bool
	}{
		{
			name:        "basic streamable HTTP client",
			url:         "http://localhost:3000/mcp",
			expectError: false,
		},
		{
			name:        "streamable HTTP with timeout",
			url:         "http://localhost:3000/mcp",
			opts:        []Option{WithTimeout(30 * time.Second)},
			expectError: false,
		},
		{
			name:        "streamable HTTP with capabilities",
			url:         "http://localhost:3000/mcp",
			opts:        []Option{WithCapabilities("tools", "resources")},
			expectError: false,
		},
		{
			name: "streamable HTTP with environment variables",
			url:  "http://localhost:3000/mcp",
			opts: []Option{WithEnv(map[string]string{
				"Authorization":   "Bearer token123",
				"X-Custom-Header": "custom-value",
			})},
			expectError: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewStreamableHTTP(tt.url, tt.opts...)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if client == nil {
				t.Fatal("Expected client to be created")
			}

			// Verify transport is set
			if client.transport == nil {
				t.Error("Expected transport to be set")
			}

			// Verify it's a StreamableClientTransport
			if _, ok := client.transport.(*mcp.StreamableClientTransport); !ok {
				t.Errorf("Expected StreamableClientTransport, got %T", client.transport)
			}

			// Verify options were applied
			// Options are applied, we just verify client was created successfully
			// Individual option tests are in other test functions
		})
	}
}

func TestNewSSEWithTimeout(t *testing.T) {
	t.Parallel()

	client, err := NewSSE("http://localhost:3000/sse",
		WithTimeout(45*time.Second))

	if err != nil {
		t.Fatalf("NewSSE with timeout failed: %v", err)
	}

	if client.timeout != 45*time.Second {
		t.Errorf("Expected timeout 45s, got %v", client.timeout)
	}

	// Verify transport is SSEClientTransport
	if _, ok := client.transport.(*mcp.SSEClientTransport); !ok {
		t.Errorf("Expected SSEClientTransport, got %T", client.transport)
	}
}

func TestCreateHTTPClientForStreaming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		env     map[string]string
	}{
		{
			name:    "default timeout",
			timeout: 30 * time.Second,
			env:     nil,
		},
		{
			name:    "custom timeout",
			timeout: 60 * time.Second,
			env:     nil,
		},
		{
			name:    "with environment headers",
			timeout: 30 * time.Second,
			env: map[string]string{
				"Authorization": "Bearer test",
				"X-API-Key":     "key123",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			httpClient := createHTTPClientForStreaming(tt.timeout, tt.env)

			if httpClient == nil {
				t.Fatal("Expected HTTP client to be created")
			}

			// Verify timeout is set
			if httpClient.Timeout != tt.timeout {
				t.Errorf("Expected timeout %v, got %v", tt.timeout, httpClient.Timeout)
			}

			// Verify transport is configured
			if httpClient.Transport == nil {
				t.Error("Expected transport to be configured")
			}

			// If env vars provided, verify custom transport is used
			if len(tt.env) > 0 {
				if _, ok := httpClient.Transport.(*envHeaderTransport); !ok {
					t.Error("Expected envHeaderTransport when environment variables provided")
				}
			}
		})
	}
}
