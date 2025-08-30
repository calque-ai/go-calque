package mcp

import (
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewStdio creates an MCP client using stdio transport.
//
// Input: command string, arguments slice, optional configuration
// Output: *Client, error
// Behavior: Creates client that communicates via stdin/stdout with subprocess
//
// Launches a subprocess with the specified command and arguments, then
// communicates with it using the MCP protocol over stdin/stdout.
// Most common transport for MCP servers implemented as scripts.
//
// Example:
//
//	client, err := mcp.NewStdio("python", []string{"mcp_server.py"},
//		mcp.WithTimeout(30*time.Second))
//	if err != nil { return err }
//	flow.Use(client.Tool("search", map[string]any{"query": "golang"}))
func NewStdio(command string, args []string, opts ...Option) (*Client, error) {
	mcpClient := mcp.NewClient(defaultImplementation(), nil)

	client := newClient(mcpClient, opts...)

	// Create CommandTransport following MCP SDK pattern
	cmd := exec.Command(command, args...)
	client.transport = &mcp.CommandTransport{
		Command: cmd,
	}

	return client, nil
}

// NewSSE creates an MCP client using Server-Sent Events transport.
//
// Input: SSE endpoint URL, optional configuration
// Output: *Client, error
// Behavior: Creates client that communicates via HTTP SSE
//
// Connects to an MCP server that exposes itself over HTTP using
// Server-Sent Events for server-to-client communication and
// HTTP POST for client-to-server communication.
//
// Example:
//
//	client, err := mcp.NewSSE("http://localhost:3000/mcp",
//		mcp.WithCapabilities("tools"))
//	if err != nil { return err }
//	flow.Use(client.Resource("file:///data/config.json"))
func NewSSE(url string, opts ...Option) (*Client, error) {
	mcpClient := mcp.NewClient(defaultImplementation(), nil)

	client := newClient(mcpClient, opts...)

	// Create SSEClientTransport following MCP SDK pattern
	client.transport = &mcp.SSEClientTransport{
		Endpoint: url,
	}

	return client, nil
}
