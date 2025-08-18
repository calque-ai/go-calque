# Batch Processing Examples

This directory demonstrates how to use Go-Calque's batch processing capabilities to efficiently handle multiple items in batches rather than processing them individually.

## What You'll Learn

### 1. Document Processing with Batching (`documentProcessingExample`)

- **Core Concepts**: Process multiple text files in batches for efficiency
- **Features Covered**:
  - Loading multiple documents from a directory
  - Text analysis (word count, character count, line count)
  - Frequency analysis of words
  - Batch processing with size and time limits
  - Logging input and output for monitoring

### 2. API Batching Simulation (`apiBatchingExample`)

- **API Integration**: Simulate batching requests to external services
- **Features Covered**:
  - Simulating API client behavior
  - Processing multiple API requests in batches
  - Response aggregation and timing
  - Real-world API batching patterns

### 3. Performance Comparison (`performanceComparisonExample`)

- **Benchmarking**: Compare individual vs batch processing performance
- **Features Covered**:
  - Side-by-side performance measurement
  - Timing analysis and improvement calculation
  - Demonstrating batch processing benefits
  - Real performance metrics

### 4. Error Handling in Batches (`errorHandlingExample`)

- **Error Management**: Handle failures gracefully in batch operations
- **Features Covered**:
  - Simulating unreliable processors
  - Error propagation in batch contexts
  - Graceful failure handling
  - Partial batch success scenarios

### 5. Different Batch Configurations (`batchConfigurationExample`)

- **Configuration Options**: Explore various batch settings
- **Features Covered**:
  - Small vs medium vs large batch sizes
  - Time-based vs size-based batching
  - Performance impact of different configurations
  - Optimal batch size selection

### 6. Custom Separator Example (`customSeparatorExample`)

- **Custom Separators**: Use application-specific separators for batching
- **Features Covered**:
  - Configuring custom batch separators
  - Processing structured data (CSV-like format)
  - Using `BatchWithConfig` for advanced configuration
  - Separator consistency between input joining and processing

## Key Batch Processing Concepts

### What is Batch Processing?

Batch processing allows you to accumulate multiple requests and process them together, rather than handling each request individually. This provides several benefits:

- **Reduced Overhead**: Fewer function calls and context switches
- **Better Resource Utilization**: More efficient use of CPU and memory
- **Improved Throughput**: Higher processing rates for large datasets
- **Cost Efficiency**: Lower per-item processing costs

### How Go-Calque Batch Processing Works

```go
// Create a batch processor
batchProcessor := ctrl.Batch(handler, maxSize, maxWait)

// maxSize: Maximum number of items to accumulate before processing
// maxWait: Maximum time to wait before processing (even if batch isn't full)
```

The batch processor:

1. **Accumulates** requests until either `maxSize` is reached or `maxWait` time elapses
2. **Processes** the entire batch through your handler
3. **Distributes** results back to waiting requests in order
4. **Handles** errors gracefully across the entire batch

### Batch vs Individual Processing

| Aspect                   | Individual Processing         | Batch Processing               |
| ------------------------ | ----------------------------- | ------------------------------ |
| **Memory Usage**         | Higher (per-request overhead) | Lower (shared overhead)        |
| **Processing Time**      | Slower (sequential)           | Faster (parallel within batch) |
| **Error Handling**       | Isolated failures             | Batch-level failures           |
| **Resource Utilization** | Lower efficiency              | Higher efficiency              |
| **Latency**              | Lower (immediate)             | Higher (waiting for batch)     |

## Running the Examples

```bash
cd examples/batch-processing
go run main.go
```

### Prerequisites

- Go 1.21 or later
- The example uses sample text files in `data/documents/` directory

## Example Output

The examples demonstrate various batch processing scenarios:

### Document Processing

```
Processing 3 documents in batches of 3...
INPUT: [document content]
OUTPUT: DOCUMENT ANALYSIS:
Words: 45
Characters: 234
Lines: 8
Most common words: [pangram, used, testing]
```

### API Batching

```
Sending 8 API requests in batches of 5...
API Response 1: Processed 'Get user profile for user123' at 14:30:45.123
API Response 2: Processed 'Update user settings for user456' at 14:30:45.124
```

### Performance Comparison

```
Comparing individual vs batch processing for 8 items...
Individual processing time: 400ms
Batch processing time: 100ms
Performance improvement: 4.0x faster
```

## Batch Configuration Guidelines

### Choosing Batch Size

| Use Case                 | Recommended Size | Reasoning                                 |
| ------------------------ | ---------------- | ----------------------------------------- |
| **API Calls**            | 5-20             | Balance between efficiency and API limits |
| **File Processing**      | 10-50            | Larger batches for I/O bound operations   |
| **Database Operations**  | 100-1000         | Very large batches for bulk operations    |
| **Real-time Processing** | 1-5              | Smaller batches for low latency           |

### Choosing Wait Time

| Scenario            | Recommended Wait     | Reasoning                         |
| ------------------- | -------------------- | --------------------------------- |
| **High Throughput** | Short (50-200ms)     | Process quickly to maintain flow  |
| **Low Volume**      | Medium (500ms-2s)    | Allow time for batch accumulation |
| **Real-time**       | Very Short (10-50ms) | Minimize latency                  |
| **Batch Jobs**      | Long (5-30s)         | Maximize batch efficiency         |

## Advanced Usage Patterns

### Custom Batch Separators

By default, Go-Calque uses `"\n---BATCH_SEPARATOR---\n"` to separate batched items. You can customize this separator for different data types or to avoid conflicts with your data content.

```go
// Default batch processor (uses DefaultBatchSeparator)
batchProcessor := ctrl.Batch(handler, 10, 1*time.Second)

// Custom separator for different data formats
customBatch := ctrl.BatchWithConfig(handler, &ctrl.BatchConfig{
    MaxSize:   10,
    MaxWait:   1 * time.Second,
    Separator: " ||| ", // Custom separator
})
```

### Error Handling Strategies

```go
// Handle batch-level errors
flow := calque.NewFlow().
    Use(ctrl.Batch(handler, 5, 1*time.Second)).
    Use(ctrl.Fallback(backupHandler, errorHandler))
```

### Performance Monitoring

```go
// Monitor batch processing performance
flow := calque.NewFlow().
    Use(logger.Print("BATCH_START")).
    Use(ctrl.Batch(handler, 10, 500*time.Millisecond)).
    Use(logger.Print("BATCH_END"))
```

## Best Practices

1. **Monitor Performance**: Always measure the impact of batching on your specific use case
2. **Handle Errors Gracefully**: Implement proper error handling for batch failures
3. **Choose Appropriate Sizes**: Balance batch size with latency requirements
4. **Consider Memory Usage**: Large batches may increase memory consumption
5. **Test with Real Data**: Validate batch behavior with your actual data patterns

## Next Steps

After mastering batch processing, explore:

- **Memory Management**: Combine batching with persistent storage
- **Advanced Flow Control**: Use batching with conditional routing
- **Multi-Agent Batching**: Batch requests across multiple AI agents
- **Real-time Streaming**: Combine batching with streaming data sources
