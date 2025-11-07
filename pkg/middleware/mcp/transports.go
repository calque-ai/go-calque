package mcp

import (
	"net"
	"net/http"
	"os/exec"
	"time"

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

	// Set environment variables for stdio transport
	if len(client.env) > 0 {
		// Set only the client-specific environment variables
		cmd.Env = make([]string, 0, len(client.env))
		for key, value := range client.env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}

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
	sseTransport := &mcp.SSEClientTransport{
		Endpoint:   url,
		HTTPClient: createHTTPClientForStreaming(client.timeout, client.env),
	}
	client.transport = sseTransport

	return client, nil
}

// NewStreamableHTTP creates an MCP client using streamable HTTP transport.
//
// Input: HTTP endpoint URL, optional configuration
// Output: *Client, error
// Behavior: Creates client that communicates via streamable HTTP
//
// Connects to an MCP server that exposes itself over HTTP using
// the streamable HTTP transport defined by the 2025-03-26 version
// of the MCP specification. This is the recommended transport for
// HTTP-based MCP servers.
//
// Example:
//
//	client, err := mcp.NewStreamableHTTP("http://localhost:3000/mcp",
//		mcp.WithCapabilities("tools"))
//	if err != nil { return err }
//	flow.Use(client.Tool("search", map[string]any{"query": "golang"}))
func NewStreamableHTTP(url string, opts ...Option) (*Client, error) {
	mcpClient := mcp.NewClient(defaultImplementation(), nil)

	client := newClient(mcpClient, opts...)

	// Create StreamableClientTransport following MCP SDK pattern
	streamableTransport := &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: createHTTPClientForStreaming(client.timeout, client.env),
	}
	client.transport = streamableTransport

	return client, nil
}

// createHTTPClientForStreaming creates an HTTP client optimized for streaming MCP operations
func createHTTPClientForStreaming(timeout time.Duration, env map[string]string) *http.Client {
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // Connection establishment timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 0, // No timeout for streaming
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       300 * time.Second, // 5 minutes
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		DisableKeepAlives:     false,
	}

	// Apply the timeout to the HTTP client
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: baseTransport,
	}

	// Set custom transport with environment variables as headers if needed
	if len(env) > 0 {
		httpClient.Transport = &envHeaderTransport{
			base: baseTransport,
			env:  env,
		}
	}

	return httpClient
}

// envHeaderTransport is an http.RoundTripper that adds environment variables as headers
type envHeaderTransport struct {
	base http.RoundTripper
	env  map[string]string
}

func (t *envHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	reqCopy := req.Clone(req.Context())

	// Add environment variables as headers
	for key, value := range t.env {
		reqCopy.Header.Set(key, value)
	}

	return t.base.RoundTrip(reqCopy)
}
