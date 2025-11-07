// Package main provides an MCP server for calque examples
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

type CalculateParams struct {
	A int `json:"a" jsonschema:"first number"`
	B int `json:"b" jsonschema:"second number"`
}

func multiplyTool(_ context.Context, _ *mcp.CallToolRequest, args CalculateParams) (*mcp.CallToolResult, any, error) {
	result := args.A * args.B
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("%d", result)},
		},
	}, nil, nil
}

type SearchParams struct {
	Query string `json:"query" jsonschema:"search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results"`
}

func searchTool(_ context.Context, _ *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	results := []string{
		"Go programming tutorial - Learn Go basics",
		"Advanced Go patterns and best practices",
		"Go concurrency with goroutines and channels",
		"Building web APIs in Go",
		"Go testing frameworks and strategies",
	}

	if args.Limit > 0 && args.Limit < len(results) {
		results = results[:args.Limit]
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for '%s':\n", args.Query))
	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output.String()},
		},
	}, nil, nil
}

type FileParams struct {
	Path string `json:"path" jsonschema:"file path to read"`
}

func readFileTool(_ context.Context, _ *mcp.CallToolRequest, args FileParams) (*mcp.CallToolResult, any, error) {
	content, err := os.ReadFile(args.Path)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error reading file: %v", err)},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(content)},
		},
	}, nil, nil
}

type ProgressParams struct {
	Steps int `json:"steps" jsonschema:"number of steps to simulate"`
}

func progressTool(_ context.Context, _ *mcp.CallToolRequest, args ProgressParams) (*mcp.CallToolResult, any, error) {
	progressToken := fmt.Sprintf("progress-%d", time.Now().Unix())

	// Simulate work with progress updates
	for i := 0; i < args.Steps; i++ {
		// Note: In a real implementation, you'd send progress notifications here
		// For this example, we'll just simulate the work
		time.Sleep(100 * time.Millisecond)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Completed %d steps", args.Steps)},
		},
		Meta: map[string]any{
			"progressToken": progressToken,
		},
	}, nil, nil
}

type ErrorParams struct {
	ErrorType string `json:"error_type" jsonschema:"type of error to simulate (validation, auth, network, internal)"`
	Message   string `json:"message" jsonschema:"custom error message (optional)"`
}

func errorTool(_ context.Context, _ *mcp.CallToolRequest, args ErrorParams) (*mcp.CallToolResult, any, error) {
	var errorMessage string

	switch args.ErrorType {
	case "validation":
		if args.Message != "" {
			errorMessage = fmt.Sprintf("Validation Error: %s", args.Message)
		} else {
			errorMessage = "Validation Error: Invalid input parameters provided"
		}
	case "auth":
		if args.Message != "" {
			errorMessage = fmt.Sprintf("Authentication Error: %s", args.Message)
		} else {
			errorMessage = "Authentication Error: Invalid API key or insufficient permissions"
		}
	case "network":
		if args.Message != "" {
			errorMessage = fmt.Sprintf("Network Error: %s", args.Message)
		} else {
			errorMessage = "Network Error: Unable to connect to external service"
		}
	case "internal":
		if args.Message != "" {
			errorMessage = fmt.Sprintf("Internal Server Error: %s", args.Message)
		} else {
			errorMessage = "Internal Server Error: An unexpected error occurred"
		}
	default:
		if args.Message != "" {
			errorMessage = fmt.Sprintf("Error: %s", args.Message)
		} else {
			errorMessage = "Error: Unknown error type specified. Valid types: validation, auth, network, internal"
		}
	}

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: errorMessage},
		},
	}, nil, nil
}

func docResourceHandler(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	switch req.Params.URI {
	case "file:///api-docs":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: "# API Documentation\n\n## Authentication\nUse API key in Authorization header.\n\n## Endpoints\n- GET /users - List users\n- POST /users - Create user",
				},
			},
		}, nil
	case "file:///config-schema":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: `{"type": "object", "properties": {"database": {"type": "object", "properties": {"host": {"type": "string"}, "port": {"type": "number"}}}}}`,
				},
			},
		}, nil
	default:
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
}

func configResourceHandler(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	path := strings.TrimPrefix(req.Params.URI, "file:///configs/")

	switch path {
	case "database.json":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: `{"host": "localhost", "port": 5432, "database": "app_db"}`,
				},
			},
		}, nil
	case "cache.json":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: `{"redis_url": "redis://localhost:6379", "ttl": 3600}`,
				},
			},
		}, nil
	default:
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
}

func codeReviewPromptHandler(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	language := "unknown"
	style := "standard"

	if req.Params.Arguments != nil {
		if lang, ok := req.Params.Arguments["language"]; ok {
			language = lang
		}
		if s, ok := req.Params.Arguments["style"]; ok {
			style = s
		}
	}

	promptText := fmt.Sprintf("Please review this %s code using %s review criteria. Focus on:", language, style)
	if style == "security" {
		promptText += "\n- Security vulnerabilities\n- Input validation\n- Authentication/authorization"
	} else {
		promptText += "\n- Code quality\n- Best practices\n- Performance"
	}

	return &mcp.GetPromptResult{
		Description: "Code review prompt template",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: promptText},
			},
		},
	}, nil
}

func NewServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "calque-example-server",
		Version: "1.0.0",
	}, nil)

	// Add tools
	mcp.AddTool(server, &mcp.Tool{
		Name: "greet",
		Description: `Send a personalized greeting to a person.

WHEN TO USE: When the user asks to greet someone, say hello, or send regards to a specific person.

HOW TO USE:
- Provide the person's name in the 'name' parameter
- The name should be a proper name (e.g., "Alice", "Bob", "Dr. Smith")

WHAT YOU GET:
- A friendly greeting message addressed to the person

EXAMPLES:
- "Say hello to Alice" → greet(name="Alice")
- "Greet my friend Bob" → greet(name="Bob")

IMPORTANT: Always use the exact name provided by the user.`,
	}, greetTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "multiply",
		Description: `Multiply two integer numbers and return the result.

WHEN TO USE: When the user asks to multiply numbers, calculate products, or perform multiplication operations.

HOW TO USE:
- Provide two integers in parameters 'a' and 'b'
- Both numbers must be integers (whole numbers)
- Numbers can be positive or negative

WHAT YOU GET:
- The mathematical product of the two numbers as an integer

EXAMPLES:
- "What is 6 times 7?" → multiply(a=6, b=7) → "42"
- "Multiply 12 and 5" → multiply(a=12, b=5) → "60"
- "Calculate 3 × 4" → multiply(a=3, b=4) → "12"

ERRORS:
- Non-integer values will cause a type error
- Very large numbers may overflow

IMPORTANT: Only use this for multiplication. For other arithmetic operations, inform the user this tool only handles multiplication.`,
	}, multiplyTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "search",
		Description: `Search for information and return relevant results.

WHEN TO USE: When the user requests to search, find, or look up information on a topic.

HOW TO USE:
- Provide a clear search query in the 'query' parameter describing what to find
- Optionally set 'limit' to control the maximum number of results (default: all results)
- Use specific keywords for better results

WHAT YOU GET:
- A numbered list of search results relevant to the query
- Each result includes a title and brief description
- Results are ordered by relevance

EXAMPLES:
- "Search for golang tutorials" → search(query="golang tutorials")
- "Find information about Go testing" → search(query="Go testing", limit=3)

NEXT:
- Present all search results to the user
- If the user wants more specific results, refine the query

IMPORTANT: The search query should be concise and focused on the main topic.`,
	}, searchTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "read_file",
		Description: `Read and return the contents of a file from the filesystem.

WHEN TO USE: When the user asks to read, view, show, or display file contents.

HOW TO USE:
- Provide the full file path in the 'path' parameter
- Path must be absolute (e.g., "/etc/hosts", "/home/user/config.json")
- Ensure you have read permissions for the requested file

WHAT YOU GET:
- The complete text contents of the file
- Binary files may return garbled text

EXAMPLES:
- "Read the file /etc/hosts" → read_file(path="/etc/hosts")
- "Show me the contents of config.json" → read_file(path="/full/path/to/config.json")

ERRORS:
- File not found: The path doesn't exist or is incorrect
- Permission denied: Insufficient permissions to read the file
- Directory provided: Can only read files, not directories

SECURITY:
- Never read files containing sensitive information like passwords or API keys
- Be cautious with system files that could contain sensitive data

IMPORTANT: Always use the exact file path provided by the user. If path is relative, inform the user you need an absolute path.`,
	}, readFileTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "progress_demo",
		Description: `Demonstrate long-running operations with progress tracking.

WHEN TO USE: When you want to show how progress updates work for multi-step operations.

HOW TO USE:
- Provide the number of steps to simulate in the 'steps' parameter
- Each step will report progress updates
- Useful for testing progress callback handling

WHAT YOU GET:
- A completion message after all steps finish
- Progress notifications during execution (if supported)

EXAMPLES:
- "Show me a progress demo with 5 steps" → progress_demo(steps=5)

IMPORTANT: This is a demonstration tool. Use it only when explicitly asked or when testing progress features.`,
	}, progressTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "error_simulator",
		Description: `Simulate different types of errors for testing error handling.

WHEN TO USE: When testing error handling, debugging, or demonstrating error scenarios.

HOW TO USE:
- Specify 'error_type': "validation", "auth", "network", or "internal"
- Optionally provide a custom 'message' for the error
- Each type simulates a realistic error scenario

ERROR TYPES:
- validation: Invalid input or malformed data errors
- auth: Authentication or authorization failures
- network: Connection or timeout errors
- internal: Unexpected server-side errors

WHAT YOU GET:
- An error response with the specified error type and message
- Helps test error handling in client code

EXAMPLES:
- "Simulate an auth error" → error_simulator(error_type="auth")
- "Test validation error handling" → error_simulator(error_type="validation", message="Email format invalid")

IMPORTANT: This tool ALWAYS returns an error. Only use when explicitly testing error scenarios.`,
	}, errorTool)

	// Add static resources
	server.AddResource(&mcp.Resource{
		URI:         "file:///api-docs",
		Name:        "API Documentation",
		Description: "REST API documentation",
	}, docResourceHandler)

	server.AddResource(&mcp.Resource{
		URI:         "file:///config-schema",
		Name:        "Configuration Schema",
		Description: "JSON schema for configuration files",
	}, docResourceHandler)

	// Add resource template
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "file:///configs/{name}",
		Name:        "Configuration Files",
		Description: "Access to various config files",
	}, configResourceHandler)

	// Add prompt
	server.AddPrompt(&mcp.Prompt{
		Name:        "code_review",
		Description: "Generate code review prompts",
	}, codeReviewPromptHandler)

	return server
}

func main() {
	server := NewServer()
	transport := &mcp.StdioTransport{}
	if err := server.Run(context.Background(), transport); err != nil {
		panic(err)
	}
}
