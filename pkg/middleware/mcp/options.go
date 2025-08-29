package mcp

import (
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Option configures MCP client behavior
type Option func(*Client)

// WithCapabilities filters which MCP capabilities the client will use.
//
// Input: capability names ("tools", "resources", "prompts")
// Output: Option function
// Behavior: Restricts client to specified capabilities only
//
// Allows limiting client functionality to specific MCP capabilities.
// Useful for security or performance when only certain features are needed.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"}, 
//		mcp.WithCapabilities("tools", "resources"))
func WithCapabilities(caps ...string) Option {
	return func(c *Client) {
		c.capabilities = caps
	}
}

// WithTimeout sets the request timeout for MCP operations.
//
// Input: time.Duration for timeout
// Output: Option function  
// Behavior: Sets timeout for all MCP requests
//
// Configures how long to wait for MCP server responses before timing out.
// Applies to tool calls, resource fetches, and prompt operations.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"}, 
//		mcp.WithTimeout(45*time.Second))
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithImplementation sets client identification for MCP handshake.
//
// Input: client name and version strings
// Output: Option function
// Behavior: Identifies client to MCP server during connection
//
// MCP servers can use this information for logging, compatibility checks,
// or feature detection. Similar to HTTP User-Agent headers.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"}, 
//		mcp.WithImplementation("my-app", "v1.2.0"))
func WithImplementation(name, version string) Option {
	return func(c *Client) {
		c.implementation = &mcp.Implementation{
			Name:    name,
			Version: version,
		}
	}
}

// WithOnError configures error handling for MCP operations.
//
// Input: error callback function
// Output: Option function
// Behavior: Enables resilient error handling instead of failing flow
//
// When provided, MCP errors are passed to the callback and the flow continues.
// Without this option, MCP errors will bubble up and stop the entire flow.
// Follows the same pattern as cache middleware error handling.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"}, 
//		mcp.WithOnError(func(err error) { 
//			log.Printf("MCP error: %v", err) 
//		}))
func WithOnError(callback func(error)) Option {
	return func(c *Client) {
		c.onError = callback
	}
}

// WithCompletion enables auto-completion support for MCP operations.
//
// Input: boolean flag to enable/disable completion
// Output: Option function
// Behavior: Enables completion capability for prompt/resource arguments
//
// When enabled, the client can provide auto-completion suggestions for
// prompt arguments and resource URIs. Helps users discover valid parameter
// values and reduces input errors in interactive environments.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"}, 
//		mcp.WithCompletion(true))
func WithCompletion(enabled bool) Option {
	return func(c *Client) {
		c.completionEnabled = enabled
	}
}