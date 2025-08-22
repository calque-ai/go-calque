package memory

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()

	if ctx == nil { //nolint:staticcheck
		t.Error("NewContext() returned nil")
	}

	if ctx.store == nil { //nolint:staticcheck
		t.Error("NewContext() store is nil")
	}
}

func TestNewContextWithStore(t *testing.T) {
	store := NewInMemoryStore()
	ctx := NewContextWithStore(store)

	if ctx == nil { //nolint:staticcheck
		t.Error("NewContextWithStore() returned nil")
	}

	if ctx.store != store { //nolint:staticcheck
		t.Error("NewContextWithStore() did not use provided store")
	}
}

func TestApproximateTokenCount(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int
	}{
		{
			name:     "empty string",
			input:    []byte(""),
			expected: 0,
		},
		{
			name:     "single word",
			input:    []byte("hello"),
			expected: 1, // 1 word * 1.3 ≈ 1
		},
		{
			name:     "multiple words",
			input:    []byte("hello world test"),
			expected: 3, // 3 words * 1.3 ≈ 3
		},
		{
			name:     "text with punctuation",
			input:    []byte("Hello, world! How are you?"),
			expected: 7, // 5 words * 1.3 + punctuation tokens ≈ 7
		},
		{
			name:     "text with numbers",
			input:    []byte("The year 2028 is here"),
			expected: 5, // 5 words * 1.3 ≈ 5
		},
		{
			name:     "text with special characters",
			input:    []byte("email@example.com and http://example.com"),
			expected: 6, // 4 words * 1.3 + special chars ≈ 6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := approximateTokenCount(tt.input)

			// Allow some variance in token estimation
			if got < tt.expected-1 || got > tt.expected+2 {
				t.Errorf("approximateTokenCount() = %d, expected around %d", got, tt.expected)
			}
		})
	}
}

func TestTrimToTokenLimit(t *testing.T) {
	tests := []struct {
		name      string
		content   []byte
		maxTokens int
		expectLen func(int, int) bool // function to validate result length
	}{
		{
			name:      "content within limit",
			content:   []byte("Hello world"),
			maxTokens: 10,
			expectLen: func(got, original int) bool {
				return got == original // Should not be trimmed
			},
		},
		{
			name:      "content exceeds limit",
			content:   []byte("This is a very long sentence that should be trimmed because it exceeds the token limit"),
			maxTokens: 5,
			expectLen: func(got, original int) bool {
				return got < original // Should be trimmed
			},
		},
		{
			name:      "empty content",
			content:   []byte(""),
			maxTokens: 10,
			expectLen: func(got, _ int) bool {
				return got == 0 // Should remain empty
			},
		},
		{
			name:      "sentence boundary trimming",
			content:   []byte("First sentence. Second sentence. Third sentence."),
			maxTokens: 4,
			expectLen: func(got, original int) bool {
				return got < original // Should be trimmed at sentence boundary
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimToTokenLimit(tt.content, tt.maxTokens)

			if !tt.expectLen(len(got), len(tt.content)) {
				t.Errorf("trimToTokenLimit() result length validation failed: got %d bytes, original %d bytes", len(got), len(tt.content))
			}

			// Verify trimmed content has reasonable token count
			if len(got) > 0 {
				tokenCount := approximateTokenCount(got)
				if tokenCount > tt.maxTokens+2 { // Allow some variance
					t.Errorf("trimToTokenLimit() result has %d tokens, want <= %d", tokenCount, tt.maxTokens)
				}
			}
		})
	}
}

func TestContextMemoryGetContext(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ContextMemory)
		key      string
		expected []byte
		wantErr  bool
	}{
		{
			name:     "non-existent context",
			key:      "missing",
			expected: nil,
			wantErr:  false,
		},
		{
			name: "existing context",
			setup: func(cm *ContextMemory) {
				cm.AddToContext("existing", []byte("test content"), 100)
			},
			key:      "existing",
			expected: []byte("test content"),
			wantErr:  false,
		},
		{
			name: "empty context",
			setup: func(cm *ContextMemory) {
				cm.AddToContext("empty", []byte(""), 100)
			},
			key:      "empty",
			expected: []byte(""),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext()

			if tt.setup != nil {
				tt.setup(ctx)
			}

			got, err := ctx.GetContext(tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !bytes.Equal(got, tt.expected) {
				t.Errorf("GetContext() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestContextMemoryGetContextReturnsCopy(t *testing.T) {
	ctx := NewContext()
	original := []byte("original content")

	ctx.AddToContext("test", original, 100)

	retrieved, _ := ctx.GetContext("test")

	// Modify the retrieved data
	if len(retrieved) > 0 {
		retrieved[0] = 'X'
	}

	// Original should be unchanged
	retrievedAgain, _ := ctx.GetContext("test")
	if len(retrievedAgain) > 0 && retrievedAgain[0] == 'X' {
		t.Errorf("GetContext() should return copy, original data was modified")
	}
}

func TestContextMemoryAddToContext(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*ContextMemory)
		key       string
		content   []byte
		maxTokens int
		wantErr   bool
	}{
		{
			name:      "add to new context",
			key:       "new",
			content:   []byte("hello world"),
			maxTokens: 100,
			wantErr:   false,
		},
		{
			name: "add to existing context",
			setup: func(cm *ContextMemory) {
				cm.AddToContext("existing", []byte("previous content"), 100)
			},
			key:       "existing",
			content:   []byte("new content"),
			maxTokens: 100,
			wantErr:   false,
		},
		{
			name:      "add content exceeding limit",
			key:       "limited",
			content:   []byte("This is a very long piece of content that should be trimmed when it exceeds the token limit"),
			maxTokens: 5,
			wantErr:   false,
		},
		{
			name:      "add empty content",
			key:       "empty",
			content:   []byte(""),
			maxTokens: 100,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext()

			if tt.setup != nil {
				tt.setup(ctx)
			}

			err := ctx.AddToContext(tt.key, tt.content, tt.maxTokens)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddToContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify content was added
			if !tt.wantErr {
				retrieved, _ := ctx.GetContext(tt.key)
				if retrieved == nil {
					t.Error("AddToContext() did not store content")
				}

				// Check token count is within limit
				tokenCount := approximateTokenCount(retrieved)
				if tokenCount > tt.maxTokens+2 { // Allow some variance
					t.Errorf("AddToContext() stored content with %d tokens, want <= %d", tokenCount, tt.maxTokens)
				}
			}
		})
	}
}

func TestContextMemoryClear(t *testing.T) {
	ctx := NewContext()

	// Add some content
	ctx.AddToContext("test", []byte("test content"), 100)

	// Verify it exists
	content, _ := ctx.GetContext("test")
	if content == nil {
		t.Error("Expected content to exist before clear")
	}

	// Clear the context
	err := ctx.Clear("test")
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	// Verify it's gone
	content, _ = ctx.GetContext("test")
	if content != nil {
		t.Error("Expected content to be cleared")
	}
}

func TestContextMemoryInfo(t *testing.T) {
	tests := []struct {
		name               string
		setup              func(*ContextMemory)
		key                string
		expectedTokenCount int
		expectedMaxTokens  int
		expectedExists     bool
		wantErr            bool
	}{
		{
			name:               "non-existent context",
			key:                "missing",
			expectedTokenCount: 0,
			expectedMaxTokens:  0,
			expectedExists:     false,
			wantErr:            false,
		},
		{
			name: "existing context",
			setup: func(cm *ContextMemory) {
				cm.AddToContext("existing", []byte("hello world test"), 100)
			},
			key:                "existing",
			expectedTokenCount: 3, // Approximate for "hello world test"
			expectedMaxTokens:  100,
			expectedExists:     true,
			wantErr:            false,
		},
		{
			name: "empty context",
			setup: func(cm *ContextMemory) {
				cm.AddToContext("empty", []byte(""), 50)
			},
			key:                "empty",
			expectedTokenCount: 0,
			expectedMaxTokens:  50,
			expectedExists:     true,
			wantErr:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext()

			if tt.setup != nil {
				tt.setup(ctx)
			}

			tokenCount, maxTokens, exists, err := ctx.Info(tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("Info() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Allow some variance in token count estimation
			if tokenCount < tt.expectedTokenCount-1 || tokenCount > tt.expectedTokenCount+2 {
				t.Errorf("Info() tokenCount = %d, expected around %d", tokenCount, tt.expectedTokenCount)
			}

			if maxTokens != tt.expectedMaxTokens {
				t.Errorf("Info() maxTokens = %d, want %d", maxTokens, tt.expectedMaxTokens)
			}

			if exists != tt.expectedExists {
				t.Errorf("Info() exists = %v, want %v", exists, tt.expectedExists)
			}
		})
	}
}

func TestContextMemoryListKeys(t *testing.T) {
	ctx := NewContext()

	// Initially empty
	keys := ctx.ListKeys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys initially, got %d", len(keys))
	}

	// Add some contexts
	ctx.AddToContext("ctx1", []byte("content1"), 100)
	ctx.AddToContext("ctx2", []byte("content2"), 100)
	ctx.AddToContext("ctx3", []byte("content3"), 100)

	keys = ctx.ListKeys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check all expected keys are present
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}

	expectedKeys := []string{"ctx1", "ctx2", "ctx3"}
	for _, expectedKey := range expectedKeys {
		if !keyMap[expectedKey] {
			t.Errorf("Expected key %q not found in keys list", expectedKey)
		}
	}
}

func TestContextMemoryInput(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*ContextMemory)
		key            string
		maxTokens      int
		input          string
		expectedOutput string
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "first input without context",
			key:            "new",
			maxTokens:      100,
			input:          "Hello world",
			expectedOutput: "Hello world",
			wantErr:        false,
		},
		{
			name: "input with existing context",
			setup: func(cm *ContextMemory) {
				cm.AddToContext("existing", []byte("Previous content\n"), 100)
			},
			key:            "existing",
			maxTokens:      100,
			input:          "New input",
			expectedOutput: "Previous content\n\nNew input",
			wantErr:        false,
		},
		{
			name:      "empty input",
			key:       "empty",
			maxTokens: 100,
			input:     "",
			wantErr:   true,
			errMsg:    "empty input for context",
		},
		{
			name:      "whitespace only input",
			key:       "whitespace",
			maxTokens: 100,
			input:     "   \n\t  ",
			wantErr:   true,
			errMsg:    "empty input for context",
		},
		{
			name:           "input with leading/trailing whitespace",
			key:            "trimmed",
			maxTokens:      100,
			input:          "  Hello world  \n",
			expectedOutput: "Hello world",
			wantErr:        false,
		},
		{
			name:           "input with token limit",
			key:            "limited",
			maxTokens:      5,
			input:          "This is a test input",
			expectedOutput: "This is a test input",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext()

			if tt.setup != nil {
				tt.setup(ctx)
			}

			handler := ctx.Input(tt.key, tt.maxTokens)
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Input() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Input() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}

			if got := buf.String(); got != tt.expectedOutput {
				t.Errorf("Input() output = %q, want %q", got, tt.expectedOutput)
			}

			// Verify input was added to context
			if !tt.wantErr {
				stored, _ := ctx.GetContext(tt.key)
				if stored == nil {
					t.Error("Input() did not store content in context")
				}
			}
		})
	}
}

func TestContextMemoryOutput(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*ContextMemory)
		key            string
		maxTokens      int
		input          string
		expectedOutput string
		wantErr        bool
	}{
		{
			name:           "capture assistant response",
			key:            "response",
			maxTokens:      100,
			input:          "Hello, how can I help?",
			expectedOutput: "Hello, how can I help?",
			wantErr:        false,
		},
		{
			name: "capture response with existing context",
			setup: func(cm *ContextMemory) {
				cm.AddToContext("existing", []byte("Previous context\n"), 100)
			},
			key:            "existing",
			maxTokens:      100,
			input:          "Assistant response",
			expectedOutput: "Assistant response",
			wantErr:        false,
		},
		{
			name:           "empty response",
			key:            "empty",
			maxTokens:      100,
			input:          "",
			expectedOutput: "",
			wantErr:        false,
		},
		{
			name:           "response with token limit",
			key:            "limited",
			maxTokens:      50,
			input:          "This is a response",
			expectedOutput: "This is a response",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext()

			if tt.setup != nil {
				tt.setup(ctx)
			}

			handler := ctx.Output(tt.key, tt.maxTokens)
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Output() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.String(); got != tt.expectedOutput {
				t.Errorf("Output() output = %q, want %q", got, tt.expectedOutput)
			}

			// Verify response was added to context (only if non-empty)
			if !tt.wantErr && tt.input != "" {
				stored, _ := ctx.GetContext(tt.key)
				if stored == nil {
					t.Error("Output() did not store response in context")
				}

				// Check that the response was prefixed with "Assistant: "
				storedStr := string(stored)
				if !strings.Contains(storedStr, "Assistant: "+tt.input) {
					t.Errorf("Output() did not store response with proper prefix, got: %q", storedStr)
				}
			}
		})
	}
}

func TestContextMemoryFullWorkflow(t *testing.T) {
	ctx := NewContext()
	key := "workflow-test"
	maxTokens := 100

	// First user input
	inputHandler := ctx.Input(key, maxTokens)
	var buf1 bytes.Buffer
	reader1 := strings.NewReader("Hello, I need help")

	req1 := calque.NewRequest(context.Background(), reader1)
	res1 := calque.NewResponse(&buf1)
	err := inputHandler.ServeFlow(req1, res1)
	if err != nil {
		t.Errorf("First input error = %v", err)
	}

	expected1 := "Hello, I need help"
	if got := buf1.String(); got != expected1 {
		t.Errorf("First input output = %q, want %q", got, expected1)
	}

	// Assistant response
	outputHandler := ctx.Output(key, maxTokens)
	var buf2 bytes.Buffer
	reader2 := strings.NewReader("Sure, I can help you!")

	req2 := calque.NewRequest(context.Background(), reader2)
	res2 := calque.NewResponse(&buf2)
	err = outputHandler.ServeFlow(req2, res2)
	if err != nil {
		t.Errorf("Output error = %v", err)
	}

	expected2 := "Sure, I can help you!"
	if got := buf2.String(); got != expected2 {
		t.Errorf("Output = %q, want %q", got, expected2)
	}

	// Second user input should include context
	var buf3 bytes.Buffer
	reader3 := strings.NewReader("Thank you!")

	req3 := calque.NewRequest(context.Background(), reader3)
	res3 := calque.NewResponse(&buf3)
	err = inputHandler.ServeFlow(req3, res3)
	if err != nil {
		t.Errorf("Second input error = %v", err)
	}

	// Should include previous context
	output3 := buf3.String()
	if !strings.Contains(output3, "Hello, I need help") {
		t.Errorf("Second input should contain previous context, got: %q", output3)
	}
	if !strings.Contains(output3, "Assistant: Sure, I can help you!") {
		t.Errorf("Second input should contain assistant response, got: %q", output3)
	}
	if !strings.Contains(output3, "Thank you!") {
		t.Errorf("Second input should contain current input, got: %q", output3)
	}

	// Verify context info
	tokenCount, maxTok, exists, err := ctx.Info(key)
	if err != nil {
		t.Errorf("Info error = %v", err)
	}
	if !exists {
		t.Error("Expected context to exist")
	}
	if maxTok != maxTokens {
		t.Errorf("Expected maxTokens %d, got %d", maxTokens, maxTok)
	}
	if tokenCount <= 0 {
		t.Errorf("Expected positive token count, got %d", tokenCount)
	}
}

func TestContextMemoryErrorHandling(t *testing.T) {
	t.Run("input with store get error", func(t *testing.T) {
		ctx := NewContextWithStore(&errorStore{getError: errors.New("get failed")})
		handler := ctx.Input("test", 100)

		var buf bytes.Buffer
		reader := strings.NewReader("Hello")

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)
		err := handler.ServeFlow(req, res)
		if err == nil {
			t.Error("Expected error from store get failure")
		}
		if !strings.Contains(err.Error(), "failed to get context") {
			t.Errorf("Expected get context error, got %v", err)
		}
	})

	t.Run("add to context with store set error", func(t *testing.T) {
		ctx := NewContextWithStore(&errorStore{setError: errors.New("set failed")})

		err := ctx.AddToContext("test", []byte("content"), 100)
		if err == nil {
			t.Error("Expected error from store set failure")
		}
	})

	t.Run("output with store get error", func(t *testing.T) {
		ctx := NewContextWithStore(&errorStore{getError: errors.New("get failed")})
		handler := ctx.Output("test", 100)

		var buf bytes.Buffer
		reader := strings.NewReader("Response")

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)
		err := handler.ServeFlow(req, res)
		if err == nil {
			t.Error("Expected error from store get failure")
		}
		if !strings.Contains(err.Error(), "failed to add response to context") {
			t.Errorf("Expected add response error, got %v", err)
		}
	})

	t.Run("malformed JSON in store", func(t *testing.T) {
		store := NewInMemoryStore()
		store.Set("bad-json", []byte("invalid json"))

		ctx := NewContextWithStore(store)
		_, err := ctx.getContext("bad-json")

		if err == nil {
			t.Error("Expected error from malformed JSON")
		}
		if !strings.Contains(err.Error(), "failed to unmarshal context") {
			t.Errorf("Expected unmarshal error, got %v", err)
		}
	})
}
