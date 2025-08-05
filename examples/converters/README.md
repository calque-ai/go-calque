# Converter Examples

This directory demonstrates converter functionality in the Calque-Pipe AI Agent Framework. Converters handle data format transformations at pipeline boundaries - converting input data to streams and output streams back to structured data.

## What You'll Learn

### Basic Converter Pipeline (`runConverterBasics`)

- **Core Concepts**: Input/output converters for structured data
- **Features Covered**:
  - Creating JSON converters with `convert.ToJson()` and `convert.FromJson()`
  - Pipeline data transformation (uppercase conversion)
  - Structured data parsing from strings to structs
  - Error handling for conversion failures
  - Logging conversion steps

### AI-Powered Converter Pipeline (`runAIConverterExample`)

- **AI Integration**: Using converters with AI processing
- **Features Covered**:
  - Converting structs to YAML with `convert.ToYaml()`
  - AI prompt templates with structured data
  - Pipeline processing: struct → YAML → AI → result

## Key Converter Concepts

- **Input Converters**: Transform external data formats (JSON, YAML, XML strings) into pipeline streams
- **Output Converters**: Transform pipeline output streams back into structured data types
- **Format Support**: Built-in support for JSON, YAML, and other structured formats
- **Type Safety**: Converters maintain Go's type safety while enabling format flexibility
- **Pipeline Integration**: Converters work seamlessly with the streaming pipeline architecture

## When to Use Converters

- Working with structured data formats (JSON, YAML, XML)
- Converting between different data representations
- Integrating with APIs that expect specific formats
- Processing structured inputs/outputs with LLMs
- Building data transformation pipelines

## Running the Examples

```bash
go run main.go
```

### Prerequisites for AI Example

- Install [Ollama](https://ollama.ai)
- Pull the model: `ollama pull llama3.2:1b`
- Ensure Ollama is running on `localhost:11434`

## Example Output

The examples demonstrate how structured data gets "calqued" (copied and transformed) through conversion pipelines, showing:

- JSON string parsing into Go structs
- Data transformation during processing
- YAML conversion for AI integration
- Structured data flow through AI analysis
- Error handling and logging at each step

## Next Steps

After understanding the basics of converters, explore other examples in the parent directory for more advanced converter features such as structured converters, and json schema converters.
