# JSON Schema Examples

This directory demonstrates how to use JSON Schema with Calque-Pipe AI agents for structured, validated AI interactions.

## What You'll Learn

### Basic WithSchema Usage (`runExample1WithSchema`)

- **Core Concepts**: Structured AI output with automatic validation
- **Features Covered**:
  - Using `ai.WithSchema()` to enforce JSON structure
  - Automatic schema generation from Go structs with `jsonschema` tags
  - Converting AI responses with `convert.FromJSON()`
  - Type-safe AI interactions without manual parsing

### JSON Schema Converters (`runExample2JsonSchemaConverters`)

- **Context-Aware Processing**: Embedding schema information in AI prompts
- **Features Covered**:
  - `convert.ToJSONSchema()` - embeds schema with input data
  - `convert.FromJSONSchema()` - validates output against schema
  - AI receives both data AND schema structure for better understanding
  - Schema-driven prompt engineering

### Multi-Stage Pipeline (`runExample3AdvancedCombined`)

- **Complex Workflows**: Combining approaches for sophisticated data processing
- **Features Covered**:
  - Stage 1: `WithSchema()` + `FromJSON()` for initial data extraction
  - Stage 2: `ToJSONSchema()` + `FromJSONSchema()` for context passing
  - Retry middleware with `flow.Retry()`
  - Generic schema handling with `ai.WithSchemaFor[T]()`
  - Pipeline composition and data flow

## Key JSON Schema Concepts

### Three Integration Approaches

1. **WithSchema() - Simple Structured Output**
   - AI automatically receives schema, returns valid JSON
   - Best for single AI calls requiring structured data
   - Minimal setup, maximum reliability

2. **Schema Converters - Context-Aware Processing**
   - AI sees schema embedded in prompts for better context
   - Input and output schemas guide AI understanding
   - Best when AI needs to understand data relationships

3. **Multi-Stage Pipelines - Complex Workflows**
   - Combine both approaches for sophisticated processing
   - Pass structured data between pipeline stages
   - Build complex AI workflows with validated data flow

### Schema Definition

- Use Go struct tags with `jsonschema` annotations
- Support for validation rules (`required`, `enum`, `minimum`, `maximum`)
- Automatic type inference from Go types
- Rich descriptions for AI context

## Running the Examples

```bash
go run main.go
```

### Prerequisites

- **For Examples 1 & 2**: Install [Ollama](https://ollama.ai), pull model: `ollama pull llama3.2:1b`
- **For Example 3**: Get [Gemini API key](https://aistudio.google.com/app/apikey), set: `export GOOGLE_API_KEY=your_api_key`

## Example Output

The examples demonstrate progressively complex JSON Schema usage:

- **Example 1**: Task analysis with structured output validation
- **Example 2**: Career advice with schema-embedded context
- **Example 3**: Multi-stage profile enhancement with context passing

Each example shows:
- Input validation and preparation
- Schema generation and embedding
- AI processing with structured constraints
- Output validation and parsing
- Error handling and retry logic

## Next Steps

After mastering JSON Schema integration, explore other examples for:
- Tool calling with structured parameters
- Memory management with validated state
- Advanced prompting with schema-driven templates
- Production deployment with validation middleware
