# HTTP API Example

This directory demonstrates how to transform Calque-Pipe processing pipelines into HTTP API endpoints.

## What You'll Learn

### Pipeline-to-API Pattern (`main`)

- **Core Concepts**: Converting pipelines into web services
- **Features Covered**:
  - Pipeline reuse across HTTP requests
  - JSON request/response handling
  - HTTP handler integration with `pipeline.Run()`
  - Context propagation from HTTP requests

## Key HTTP Integration Concepts

- **Pipeline Reuse**: Create pipelines once, use across multiple requests
- **Context Integration**: HTTP request context flows through the pipeline
- **Stream Processing**: Pipeline processes `r.Body` directly as input
- **JSON Interface**: Standard REST API request/response patterns
- **Error Mapping**: Pipeline errors become HTTP error responses

## Running the Example

```bash
go run main.go
```

### Test the API

```bash
curl -X POST http://localhost:8080/agent \
  -H 'Content-Type: application/json' \
  -d '{"message":"hello world","user_id":"123"}'
```

## Example Output

The server demonstrates pipeline-to-HTTP transformation:

- **Input**: JSON message via HTTP POST
- **Processing**: Message transformation (uppercase + prefix)
- **Output**: JSON response with processed result and timestamp

Example response:

```json
{
  "result": "Processed: HELLO WORLD",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

## Next Steps

After understanding HTTP API integration, explore:

- **AI Client Examples**: Expose AI agents as web services
- **Tool Calling**: HTTP APIs that trigger AI function execution
- **JSON Schema**: Structured API validation with schema middleware
- **Memory Management**: Stateful HTTP APIs with persistent context
