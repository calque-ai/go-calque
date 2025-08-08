# AI Client Examples

This directory demonstrates how to connect different AI providers to Calque-Pipe processing pipelines.

## What You'll Learn

### Ollama Integration (`ollamaExample`)

- **Local AI Provider**: Running AI models locally with Ollama
- **Features Covered**:
  - Creating Ollama clients with `ai.NewOllama()`
  - Model specification (e.g., `llama3.2:1b`)
  - Input transformation with `str.Transform()`
  - Timeout protection with `flow.Timeout()`
  - Direct agent integration with `ai.Agent(client)`

### Gemini Integration (`geminiExample`)

- **Cloud AI Provider**: Google's Gemini API integration
- **Features Covered**:
  - Custom configuration with `ai.GeminiConfig`
  - Temperature control and model parameters
  - Prompt templating with `prompt.Template()`

## Key AI Integration Concepts

- **Provider Abstraction**: Both local (Ollama) and cloud (Gemini) providers use the same `ai.Client` interface
- **Configuration Flexibility**: Each provider supports custom configuration options for fine-tuning behavior
- **Timeout Management**: AI calls can be wrapped with timeouts to prevent hanging pipelines
- **Prompt Engineering**: Different approaches for preparing input (direct transformation vs. template-based)
- **Error Handling**: Proper connection and execution error management
- **Logging Integration**: Monitor AI interactions with built-in pipeline logging

## Running the Examples

```bash
go run main.go
```

### Prerequisites

#### For Ollama Example

- Install [Ollama](https://ollama.ai)
- Pull the model: `ollama pull llama3.2:1b`
- Ensure Ollama is running: `ollama serve`

#### For Gemini Example

- Get API key from [Google AI Studio](https://aistudio.google.com/app/apikey)
- Create `.env` file with: `GOOGLE_API_KEY=your_api_key`
- Ensure internet connectivity for API calls

## Example Output

The examples demonstrate AI provider integration:

- **Ollama**: Local model execution with input transformation and timeout protection
- **Gemini**: Cloud API calls with custom configuration and prompt templating

Both examples show:

- Input logging and preparation
- Prompt construction (transformation vs. templating)
- AI agent execution with provider-specific configurations
- Response processing and output display
- Error handling for connection and execution issues

## Key Differences Between Providers

| Feature            | Ollama         | Gemini              |
| ------------------ | -------------- | ------------------- |
| **Hosting**        | Local          | Cloud API           |
| **Authentication** | None           | API Key             |
| **Configuration**  | Config options | Config options      |
| **Latency**        | Low (local)    | Variable (network)  |
| **Privacy**        | High (offline) | Depends on provider |
| **Cost**           | Hardware only  | Per-request pricing |

## Next Steps

After understanding AI client integration, explore:

- **JSON Schema Examples**: Structured AI interactions with validation
- **Tool Calling**: AI agents that can execute functions
- **Memory Management**: Stateful conversations and context
- **Streaming**: Real-time AI response processing
- **Advanced Prompting**: Complex template systems and context injection
