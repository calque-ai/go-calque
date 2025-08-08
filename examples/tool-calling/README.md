# Tool Calling Examples

This directory demonstrates how to create AI agents that can execute functions and tools through Calque-Pipe.

## What You'll Learn

### Simple Agent with Basic Tools (`runSimpleAgent`)

- **Core Concepts**: AI agents that can call external functions
- **Features Covered**:
  - Creating tools with `tools.Simple()`
  - Function definition with JSON argument parsing
  - Multi-tool agent setup with `ai.WithTools()`
  - Error handling in tool functions

### Configured Agent with Advanced Tools (`runConfiguredAgent`)

- **Advanced Tool Management**: Custom configuration and concurrent execution
- **Features Covered**:
  - Tool configuration with `tools.Config`
  - Concurrent tool execution settings (`MaxConcurrentTools`)
  - Error passthrough behavior (`PassThroughOnError`)
  - Original output inclusion (`IncludeOriginalOutput`)

## Key Tool Calling Concepts

- **Function Registration**: Register Go functions as AI-callable tools
- **JSON Interface**: Tools receive JSON arguments and return string responses
- **Tool Discovery**: AI agents automatically discover and describe available tools
- **Concurrent Execution**: Multiple tools can be executed simultaneously for performance
- **Error Handling**: Configurable behavior when tools fail or return errors
- **Provider Compatibility**: Works with different AI providers (Ollama, Gemini, etc.)

## Tool Creation Patterns

### Simple Tools

```go
calculator := tools.Simple("calculator", "Performs basic math", func(jsonArgs string) string {
    // Parse JSON arguments
    var args struct {
        Input string `json:"input"`
    }
    json.Unmarshal([]byte(jsonArgs), &args)

    // Process and return result
    return processCalculation(args.Input)
})
```

### Tool Configuration

```go
config := tools.Config{
    PassThroughOnError:    true,  // Continue on tool errors
    MaxConcurrentTools:    2,     // Parallel execution limit
    IncludeOriginalOutput: true,  // Include AI reasoning
}
```

## Running the Examples

```bash
go run main.go
```

### Prerequisites

#### For Simple Agent (Example 1)

- Get API key from [Google AI Studio](https://aistudio.google.com/app/apikey)
- Create `.env` file with: `GOOGLE_API_KEY=your_api_key`
- Ensure internet connectivity for Gemini API

#### For Configured Agent (Example 2)

- Install [Ollama](https://ollama.ai)
- Pull the model: `ollama pull llama3.2:1b`
- Ensure Ollama is running: `ollama serve`

## Example Output

The examples demonstrate AI tool integration:

- **Simple Agent**: AI performs calculations and time lookups using provided tools
- **Configured Agent**: AI analyzes text and performs math with custom configuration

Both examples show:

- AI understanding of available tools and their purposes
- Automatic tool selection based on user requests
- Function execution with proper argument parsing
- Result integration into conversational responses
- Error handling and graceful failure modes

## Tool Implementation Best Practices

1. **Clear Descriptions**: Provide detailed tool descriptions for AI understanding
2. **JSON Structure**: Use consistent JSON argument patterns for reliability
3. **Error Handling**: Return meaningful error messages as strings
4. **Performance**: Consider concurrent execution limits for resource management
5. **Validation**: Validate tool inputs before processing
6. **Documentation**: Include usage examples in tool descriptions

## Advanced Configuration Options

| Option                  | Description                            | Default |
| ----------------------- | -------------------------------------- | ------- |
| `PassThroughOnError`    | Continue pipeline on tool failures     | `false` |
| `MaxConcurrentTools`    | Maximum parallel tool executions       | `1`     |
| `IncludeOriginalOutput` | Include AI reasoning with tool results | `false` |

## Next Steps

After mastering tool calling, explore:

- **JSON Schema Examples**: Structured tool parameters and validation
- **Memory Management**: Stateful tools with persistent data
- **Advanced Prompting**: Tool-aware prompt engineering
