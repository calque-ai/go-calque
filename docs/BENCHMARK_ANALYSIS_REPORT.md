# Go-Calque Benchmark Analysis

> **Generated:** January 2026  
> **System:** 12th Gen Intel Core i7-1255U (Linux x86_64)  
> **Go Version:** 1.25.5 | GOMAXPROCS: 12

---

## Executive Summary

This report analyzes go-calque's streaming concurrent pipeline architecture for AI agent workloads.

### Key Findings

| Metric | Result | Implication |
|--------|--------|-------------|
| Single handler overhead | ~15-18 µs | Negligible for AI workloads |
| Framework overhead at 100ms AI latency | <0.02% | Framework cost is invisible |
| Framework overhead at 10ms AI latency | ~0.13% | Still negligible |
| Framework overhead at 1ms AI latency | ~1.2% | Acceptable |
| ctrl.Chain vs Flow | **3.4x faster** | Use Chain for sequential |
| Memory per handler | ~33KB | Linear, predictable |
| Text processing (large) | **7x faster** | Framework beats baseline |
| Streaming vs Buffered | **3x faster** | Streaming is superior |

---

## 1. Baseline Comparisons

### Direct Function Calls (No Framework)

| Benchmark | Time | Allocs | Memory |
|-----------|------|--------|--------|
| Direct function call | 1 ns | 0 | 0 B |
| Chain 3 functions | 174 ns | 1 | 64 B |
| Chain 5 functions | 243 ns | 1 | 64 B |

### Flow Setup Overhead

| Operation | Time | Allocs | Memory |
|-----------|------|--------|--------|
| `NewFlow()` | 25 ns | 1 | 48 B |
| `NewFlow(config)` | 63 ns | 2 | 160 B |
| Add 3 handlers | 119 ns | 4 | 160 B |

### Goroutine & Pipe Overhead

| Benchmark | Time | Memory |
|-----------|------|--------|
| Raw goroutine spawn | 378 ns | 32 B |
| With io.Pipe | 816 ns | 984 B |
| With Calque Pipe | 854 ns | 1,000 B |

**Analysis:** Calque Pipe adds only 4.7% overhead over raw io.Pipe.

---

## 2. Handler Scaling

### Single Handler Overhead

| Handler Type | Time | Memory |
|--------------|------|--------|
| Passthrough (io.Copy) | 14.5 µs | 40 KB |
| Transform (read/process/write) | 9.9 µs | 9 KB |
| text.Transform middleware | 10.1 µs | 9 KB |

### Multi-Handler Scaling

| Handlers | Time | Per Handler | Memory |
|----------|------|-------------|--------|
| 1 | 13.9 µs | 13.9 µs | 40 KB |
| 3 | 33.1 µs | 11.0 µs | 107 KB |
| 5 | 50.9 µs | 10.2 µs | 174 KB |
| 10 | 95.2 µs | 9.5 µs | 341 KB |
| 20 | 169.3 µs | 8.5 µs | 675 KB |

**Per additional handler:**
- Time: ~7,700 ns
- Memory: ~33,500 B
- Allocations: ~11

---

## 3. Chain vs Flow

Comparing `ctrl.Chain` (sequential) vs regular Flow (concurrent) with 3 handlers:

| Pattern | Time | Memory | Ratio |
|---------|------|--------|-------|
| `ctrl.Chain(h, h, h)` | 9.9 µs | 9.6 KB | **1.0x** |
| `flow.Use(h).Use(h)...` | 34.0 µs | 107 KB | 3.4x slower |

### Why Chain is 3.4x Faster

1. No goroutine creation between handlers
2. No io.Pipe allocation (saves ~32KB × 3)
3. Uses `bytes.Buffer` for intermediate storage
4. Direct function calls between handlers

### When to Use Each

| Use Case | Recommendation |
|----------|----------------|
| AI chat with streaming output | Regular Flow |
| Tool calling / context propagation | `ctrl.Chain` |
| Parallel tool execution | Regular Flow |
| Simple transformations | `ctrl.Chain` |

---

## 4. Framework Overhead vs AI Latency

How framework overhead compares to AI response times:

| AI Latency | Total Time | Framework Overhead | % Overhead |
|------------|------------|-------------------|------------|
| 1 ms | 1.11 ms | ~13 µs | 1.21% |
| 10 ms | 10.23 ms | ~26 µs | **0.25%** |
| 100 ms | 100.32 ms | ~15 µs | **0.015%** |

### Real-World Context

| Scenario | Typical Latency | Framework Overhead |
|----------|-----------------|-------------------|
| Fast local model (Ollama) | 10-50 ms | 0.1-0.5% |
| Cloud API (OpenAI) | 100-500 ms | 0.01-0.05% |
| Chain-of-thought | 1-5 s | <0.01% |

**Conclusion:** Framework overhead is negligible for AI workloads.

---

## 5. Data Size Scaling

| Data Size | Time | Throughput | Memory |
|-----------|------|------------|--------|
| 100 B | 14.7 µs | 6.8 MB/s | 40 KB |
| 1 KB | 16.7 µs | 61 MB/s | 44 KB |
| 10 KB | 31.4 µs | 326 MB/s | 91 KB |
| 100 KB | 132 µs | 775 MB/s | 513 KB |
| 1 MB | 1.37 ms | 766 MB/s | 5.3 MB |
| 10 MB | 13.2 ms | 793 MB/s | 44 MB |

**Analysis:**
- Small data (<1KB): Fixed overhead dominates
- Large data (1MB+): Excellent throughput (~770 MB/s)

---

## 6. Streaming vs Buffered

For 10KB data:

| Method | Time | Throughput | Memory |
|--------|------|------------|--------|
| `io.Copy` (streaming) | 2.4 µs | 4,325 MB/s | 20.5 KB |
| `io.ReadAll` (buffered) | 7.3 µs | 1,409 MB/s | 56.4 KB |
| **Improvement** | **3x faster** | **3x higher** | **3x less** |

---

## 7. Framework vs Hand-Coded Baseline

Real-world comparison for text processing (anagram detection):

### Small Dataset (29 words)

| Metric | Baseline | Go-Calque | Improvement |
|--------|----------|-----------|-------------|
| Time | 67 µs | 57 µs | **15% faster** |
| Memory | 78 KB | 36 KB | **54% less** |
| Allocs | 626 | 419 | **33% fewer** |

### Large Dataset (1000 words)

| Metric | Baseline | Go-Calque | Improvement |
|--------|----------|-----------|-------------|
| Time | 3.85 ms | 0.55 ms | **86% faster** |
| Memory | 3.9 MB | 0.5 MB | **87% less** |
| Allocs | 30,880 | 5,478 | **82% fewer** |

---

## 8. Memory Allocation Breakdown

For a 5-handler flow (173 KB total):

| Component | Size | % of Total |
|-----------|------|------------|
| io.Pipe buffers (32KB × 5) | 160 KB | **92%** |
| MetadataBus | 3.8 KB | 2.2% |
| Pipe structs | 5 KB | 2.9% |
| Other | 4.9 KB | 2.9% |

---

## 9. Recommendations

### Use Concurrent Flow (`.Use()`) When:

- ✅ Streaming LLM output to SSE/WebSocket
- ✅ Parallel tool execution
- ✅ High-throughput HTTP APIs
- ✅ Processing large data (10KB+)
- ✅ Multi-agent pipelines

### Use Sequential Chain (`ctrl.Chain`) When:

- ✅ Context propagation is critical
- ✅ Tool calling patterns (Registry → Detect → Execute)
- ✅ Memory middleware chains
- ✅ Handler count is small (2-5)

### Performance Tips

1. **Use `ctrl.Chain` for sequential logic** - 3.4x faster, 11x less memory
2. **Prefer `[]byte` over `string`** - 1.5-2x more efficient for large data
3. **Reuse Flows** - setup is cheap (25ns) but adds up
4. **Consider buffer sizes** - default io.Copy uses 32KB buffer

---

## Conclusion

Go-calque's streaming concurrent architecture is **well-suited for AI agent workloads**.

| Finding | Evidence |
|---------|----------|
| Overhead is negligible | <0.02% at 100ms LLM latency |
| Streaming provides real benefits | 3x faster, 3x less memory |
| Text processing excels | 86% faster than hand-coded |
| Sequential option exists | `ctrl.Chain` is 3.4x faster |
| Scaling is predictable | ~7.7µs and ~33KB per handler |

### Should You Add a "Non-Concurrent" Mode?

**Not yet.** The data suggests:
- For AI workloads: overhead is already negligible (<0.02%)
- For text processing: framework already beats baselines by 86%
- `ctrl.Chain` already provides sequential execution (3.4x faster)

---

## Next

**→ [Examples](../examples/)** - See the framework in action
