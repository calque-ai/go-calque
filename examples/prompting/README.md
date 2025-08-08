# Prompting Examples

This directory demonstrates different approaches to prompt engineering and construction with Calque-Pipe AI agents.

## What You'll Learn

### Basic Template Usage (`basicTemplateExample`)

- **Core Concepts**: Simple Go template-based prompt construction
- **Features Covered**:
  - Using `prompt.Template()` with basic string interpolation
  - Accessing input data via `{{.Input}}` template variable
  - Simple prompt formatting and structure
  - Direct template-to-AI pipeline flow

### Template with Additional Data (`templateWithDataExample`)

- **Advanced Templating**: Enriching prompts with custom variables
- **Features Covered**:
  - Passing additional data with `map[string]any` parameters
  - Using multiple template variables (e.g., `{{.Role}}`, `{{.Language}}`)
  - Complex prompt construction with contextual information
  - Dynamic prompt personalization based on data

### System Prompt Convenience (`systemPromptExample`)

- **System Messages**: Structured system-level prompt formatting
- **Features Covered**:
  - Using `prompt.System()` for consistent system message formatting
  - Establishing AI behavior and personality upfront
  - Clean separation of system instructions from user input
  - Provider-agnostic system message handling

### Chat Prompt Formatting (`chatPromptExample`)

- **Conversational Structure**: Role-based message formatting
- **Features Covered**:
  - Using `prompt.Chat()` with role specifications
  - Multi-turn conversation preparation
  - Role-based message structuring (assistant, user, system)
  - Chat-style interaction patterns

## Key Prompting Concepts

- **Template System**: Go's `text/template` package powers flexible prompt construction
- **Variable Interpolation**: Access input data and custom variables within templates
- **Prompt Layering**: Combine multiple prompt middleware for complex structures
- **Role-based Messaging**: Structure conversations with proper role attribution
- **Context Injection**: Enrich prompts with additional data and parameters
- **Composability**: Mix and match prompt types within the same pipeline

## Template Variables

### Built-in Variables
- `{{.Input}}` - The pipeline input data passed to `pipe.Run()`

### Custom Variables
- Pass `map[string]any` as second parameter to `prompt.Template()`
- Access via `{{.YourKey}}` in template strings
- Support for nested data structures and complex types

## Running the Examples

```bash
go run main.go
```

### Prerequisites

- Install [Ollama](https://ollama.ai)
- Pull the model: `ollama pull llama3.2:1b`  
- Ensure Ollama is running: `ollama serve`

## Example Output

The examples demonstrate progressive prompt engineering complexity:

- **Basic Template**: Simple input interpolation with system context
- **Template with Data**: Multi-variable prompts with role and specialization
- **System Prompt**: Structured system message with behavior guidelines  
- **Chat Prompt**: Role-based conversational formatting

Each example shows:
- Input logging and template variable setup
- Prompt construction with different approaches
- Template rendering and variable substitution
- AI agent execution with constructed prompts
- Response processing and display

## Prompt Engineering Best Practices

1. **Clear Instructions**: Use specific, actionable language in prompts
2. **Context Setting**: Establish AI role and behavior expectations early
3. **Variable Separation**: Keep dynamic data in template variables, not hardcoded
4. **Consistent Formatting**: Use structured approaches (System, Chat) for reliability
5. **Input Validation**: Ensure template variables are properly populated
6. **Composability**: Layer multiple prompt types for complex interactions

## Next Steps

After mastering prompt construction, explore:
- **JSON Schema Examples**: Structured prompts with output validation
- **Memory Management**: Stateful conversations with prompt context
- **Tool Calling**: Prompts that trigger AI function execution
- **Advanced AI Clients**: Provider-specific prompt optimization
- **Multi-stage Pipelines**: Chaining prompts for complex workflows