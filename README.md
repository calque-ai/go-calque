Essential for LLM workflows:

1. RateLimit[T] - Critical for API rate limiting
   core.RateLimit[string](10, time.Second) // 10 requests/second
2. Cache[T] - Essential for expensive LLM calls
   core.Cache[string](1*time.Hour) // Cache responses for 1 hour
3. Batch[T] - Accumulate multiple requests for batch processing
   core.Batch[string](10, 5\*time.Second) // Batch 10 items or 5 seconds
4. Fallback[T] - Graceful degradation when primary LLM fails
   core.Fallback[string](primaryLLM, fallbackLLM)

LLM Integration Middleware:

1. llm.Chat(provider, model) - Core LLM interaction
   llm.Chat("openai", "gpt-4") // Convert text → LLM response
   llm.Chat("anthropic", "claude-3-5-sonnet")
2. llm.Prompt(template, vars) - Template-based prompting
   llm.Prompt("Summarize: {{.input}}", map[string]any{"input": text})
3. llm.SystemPrompt(prompt) - Inject system context
   llm.SystemPrompt("You are a helpful coding assistant")

Memory Middleware:

4. memory.Conversation(key) - Maintain chat history
   memory.Conversation("user123") // Auto-append to conversation history
5. memory.Context(maxTokens) - Sliding window context
   memory.Context(4000) // Keep last 4000 tokens of context
6. memory.Semantic(embeddings) - Vector-based memory retrieval
   memory.Semantic(embeddingStore) // Retrieve relevant past conversations

Agent Behavior:

7. agent.Tool(name, handler) - Function calling integration
   agent.Tool("search_web", webSearchHandler)
8. agent.Planning(steps) - Multi-step reasoning
9. agent.Reflection() - Self-evaluation of responses
