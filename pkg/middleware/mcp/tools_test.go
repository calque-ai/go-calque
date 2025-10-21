package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	funcTools "github.com/calque-ai/go-calque/pkg/middleware/tools"
)

func TestTools(t *testing.T) {
	t.Parallel()

	t.Run("fetches tools from MCP server", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping integration test in short mode")
		}
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"})
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		tools, err := Tools(ctx, client)
		if err != nil {
			t.Fatalf("Tools() failed: %v", err)
		}

		if len(tools) == 0 {
			t.Error("Expected at least one tool from MCP server")
		}

		// Verify tool structure
		for _, tool := range tools {
			if tool.Name() == "" {
				t.Error("Tool should have a name")
			}
			if tool.Description() == "" {
				t.Error("Tool should have a description")
			}
		}
	})

	t.Run("caches tools on subsequent calls", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping integration test in short mode")
		}
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				RegistryTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		ctx := context.Background()

		// First call - cache miss
		tools1, err := Tools(ctx, client)
		if err != nil {
			t.Fatalf("First Tools() call failed: %v", err)
		}

		// Second call - cache hit
		tools2, err := Tools(ctx, client)
		if err != nil {
			t.Fatalf("Second Tools() call failed: %v", err)
		}

		// Should return same number of tools
		if len(tools1) != len(tools2) {
			t.Errorf("Cache hit returned %d tools, cache miss returned %d", len(tools2), len(tools1))
		}

		// Verify tool names match
		for i := range tools1 {
			if tools1[i].Name() != tools2[i].Name() {
				t.Errorf("Tool %d name mismatch: %s != %s", i, tools1[i].Name(), tools2[i].Name())
			}
		}
	})

	t.Run("connection error handling", func(t *testing.T) {
		t.Parallel()

		// Create client with invalid command
		client, err := NewStdio("nonexistent-command", []string{})
		if err == nil {
			t.Skip("Expected client creation to fail, but it succeeded")
		}

		ctx := context.Background()
		_, err = Tools(ctx, client)
		if err == nil {
			t.Error("Expected error when connecting to invalid MCP server")
		}
	})

	t.Run("tools can be called", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping integration test in short mode")
		}
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"})
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		tools, err := Tools(ctx, client)
		if err != nil {
			t.Fatalf("Tools() failed: %v", err)
		}

		// Find the greet tool
		var greetTool funcTools.Tool
		for _, tool := range tools {

			if tool.Name() == "greet" {
				greetTool = tool
				break
			}
		}

		if greetTool == nil {
			t.Skip("Greet tool not found in MCP server")
		}

		// Call the tool (tools.Tool implements calque.Handler)
		input := `{"name": "World"}`
		req := calque.NewRequest(ctx, strings.NewReader(input))
		var output strings.Builder
		res := calque.NewResponse(&output)

		err = greetTool.ServeFlow(req, res)
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		result := output.String()
		if result == "" {
			t.Error("Expected non-empty result from tool execution")
		}

		if !strings.Contains(result, "World") {
			t.Errorf("Expected result to contain 'World', got: %s", result)
		}
	})
}

func TestConvertMCPToTool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	t.Run("converts MCP tool to native tool", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"})
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		if err := client.connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		listResult, err := client.session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to list tools: %v", err)
		}

		if len(listResult.Tools) == 0 {
			t.Skip("No tools available from server")
		}

		// Convert first tool
		mcpTool := listResult.Tools[0]
		nativeTool := convertMCPToTool(client, mcpTool)

		// Verify conversion
		if nativeTool.Name() != mcpTool.Name {
			t.Errorf("Name mismatch: %s != %s", nativeTool.Name(), mcpTool.Name)
		}

		if nativeTool.Description() != mcpTool.Description {
			t.Errorf("Description mismatch: %s != %s", nativeTool.Description(), mcpTool.Description)
		}

		// Verify tool is callable (tools.Tool implements calque.Handler)
		if nativeTool == nil {
			t.Error("Expected non-nil tool")
		}

		// Verify we can call ServeFlow on it
		req := calque.NewRequest(ctx, strings.NewReader(`{}`))
		var output strings.Builder
		res := calque.NewResponse(&output)

		// Just verify it's callable (may error if params are wrong, that's ok)
		_ = nativeTool.ServeFlow(req, res)
	})
}

func TestToolsWithoutCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"})
	if err != nil {
		t.Skipf("Skipping test - MCP server not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Call Tools multiple times without cache
	tools1, err := Tools(ctx, client)
	if err != nil {
		t.Fatalf("First Tools() call failed: %v", err)
	}

	tools2, err := Tools(ctx, client)
	if err != nil {
		t.Fatalf("Second Tools() call failed: %v", err)
	}

	// Should still return consistent results
	if len(tools1) != len(tools2) {
		t.Errorf("Inconsistent results: %d tools vs %d tools", len(tools1), len(tools2))
	}
}
