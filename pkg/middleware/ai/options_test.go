package ai

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/invopop/jsonschema"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// Test structs for schema generation
type PersonStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type AddressStruct struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	ZipCode string `json:"zip_code"`
}

type ComplexStruct struct {
	ID      string   `json:"id"`
	Tags    []string `json:"tags"`
	Enabled bool     `json:"enabled"`
}

func TestWithSchemaFor(t *testing.T) {
	tests := []struct {
		name         string
		schemaFunc   func() AgentOption
		expectedType string
		checkSchema  func(t *testing.T, schema *ResponseFormat)
	}{
		{
			name: "simple struct",
			schemaFunc: func() AgentOption {
				return WithSchemaFor[PersonStruct]()
			},
			expectedType: "json_schema",
			checkSchema: func(t *testing.T, schema *ResponseFormat) {
				if schema.Schema == nil {
					t.Error("Schema should not be nil")
					return
				}
				// jsonschema.Reflector generates schemas with $ref to $defs
				// Check that the schema has a $ref (standard jsonschema pattern)
				if schema.Schema.Ref == "" {
					t.Error("Schema should have $ref for struct types")
				}
				// Verify definitions were created
				if len(schema.Schema.Definitions) == 0 {
					t.Error("Schema should have definitions")
				}
			},
		},
		{
			name: "address struct",
			schemaFunc: func() AgentOption {
				return WithSchemaFor[AddressStruct]()
			},
			expectedType: "json_schema",
			checkSchema: func(t *testing.T, schema *ResponseFormat) {
				if schema.Schema == nil {
					t.Error("Schema should not be nil")
					return
				}
				// Verify schema was generated (should have $ref and definitions)
				if schema.Schema.Ref == "" {
					t.Error("Schema should have $ref for struct types")
				}
				if len(schema.Schema.Definitions) == 0 {
					t.Error("Schema should have definitions")
				}
			},
		},
		{
			name: "complex struct with array and bool",
			schemaFunc: func() AgentOption {
				return WithSchemaFor[ComplexStruct]()
			},
			expectedType: "json_schema",
			checkSchema: func(t *testing.T, schema *ResponseFormat) {
				if schema.Schema == nil {
					t.Error("Schema should not be nil")
					return
				}
				// Verify schema was generated with $ref and definitions
				if schema.Schema.Ref == "" {
					t.Error("Schema should have $ref for struct types")
				}
				if len(schema.Schema.Definitions) == 0 {
					t.Error("Schema should have definitions")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := tt.schemaFunc()
			opts := &AgentOptions{}
			option.Apply(opts)

			if opts.Schema == nil {
				t.Error("WithSchemaFor() should set Schema")
				return
			}

			if opts.Schema.Type != tt.expectedType {
				t.Errorf("WithSchemaFor() Type = %v, want %v", opts.Schema.Type, tt.expectedType)
			}

			if tt.checkSchema != nil {
				tt.checkSchema(t, opts.Schema)
			}
		})
	}
}

func TestWithSchema(t *testing.T) {
	tests := []struct {
		name         string
		input        any
		expectedType string
		checkResult  func(t *testing.T, opts *AgentOptions)
	}{
		{
			name: "ResponseFormat pointer",
			input: &ResponseFormat{
				Type:   "json_object",
				Schema: nil,
			},
			expectedType: "json_object",
			checkResult: func(t *testing.T, opts *AgentOptions) {
				if opts.Schema.Schema != nil {
					t.Error("Schema.Schema should be nil for json_object")
				}
			},
		},
		{
			name: "ResponseFormat value",
			input: ResponseFormat{
				Type:   "json_schema",
				Schema: &jsonschema.Schema{Type: "object"},
			},
			expectedType: "json_schema",
			checkResult: func(t *testing.T, opts *AgentOptions) {
				if opts.Schema.Schema == nil {
					t.Error("Schema.Schema should not be nil")
				}
			},
		},
		{
			name:         "struct type - generates schema",
			input:        PersonStruct{},
			expectedType: "json_schema",
			checkResult: func(t *testing.T, opts *AgentOptions) {
				if opts.Schema.Schema == nil {
					t.Error("Schema.Schema should be generated from struct")
					return
				}
				// Verify schema was generated with $ref and definitions
				if opts.Schema.Schema.Ref == "" {
					t.Error("Generated schema should have $ref for struct types")
				}
				if len(opts.Schema.Schema.Definitions) == 0 {
					t.Error("Generated schema should have definitions")
				}
			},
		},
		{
			name:         "struct pointer - generates schema",
			input:        &AddressStruct{},
			expectedType: "json_schema",
			checkResult: func(t *testing.T, opts *AgentOptions) {
				if opts.Schema.Schema == nil {
					t.Error("Schema.Schema should be generated from struct pointer")
					return
				}
				// Verify schema was generated with $ref and definitions
				if opts.Schema.Schema.Ref == "" {
					t.Error("Generated schema should have $ref for struct types")
				}
				if len(opts.Schema.Schema.Definitions) == 0 {
					t.Error("Generated schema should have definitions")
				}
			},
		},
		{
			name: "ResponseFormat with explicit schema",
			input: &ResponseFormat{
				Type: "json_schema",
				Schema: &jsonschema.Schema{
					Type: "object",
				},
			},
			expectedType: "json_schema",
			checkResult: func(t *testing.T, opts *AgentOptions) {
				if opts.Schema.Schema == nil {
					t.Error("Schema.Schema should not be nil")
					return
				}
				if opts.Schema.Schema.Type != "object" {
					t.Error("Schema type should be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithSchema(tt.input)
			opts := &AgentOptions{}
			option.Apply(opts)

			if opts.Schema == nil {
				t.Error("WithSchema() should set Schema")
				return
			}

			if opts.Schema.Type != tt.expectedType {
				t.Errorf("WithSchema() Type = %v, want %v", opts.Schema.Type, tt.expectedType)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, opts)
			}
		})
	}
}

func TestWithMultimodalData(t *testing.T) {
	tests := []struct {
		name  string
		input *MultimodalInput
	}{
		{
			name: "single text part",
			input: &MultimodalInput{
				Parts: []ContentPart{
					Text("Hello"),
				},
			},
		},
		{
			name: "text and image",
			input: &MultimodalInput{
				Parts: []ContentPart{
					Text("What's this?"),
					ImageData([]byte("image-data"), "image/jpeg"),
				},
			},
		},
		{
			name: "multiple parts",
			input: &MultimodalInput{
				Parts: []ContentPart{
					Text("Analyze"),
					ImageData([]byte("img"), "image/png"),
					Audio(strings.NewReader("audio"), "audio/wav"),
					Video(strings.NewReader("video"), "video/mp4"),
				},
			},
		},
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "empty parts",
			input: &MultimodalInput{
				Parts: []ContentPart{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithMultimodalData(tt.input)
			opts := &AgentOptions{}
			option.Apply(opts)

			if opts.MultimodalData != tt.input {
				t.Error("WithMultimodalData() should set MultimodalData to exact input")
			}

			// Verify parts are preserved
			if tt.input != nil && opts.MultimodalData != nil {
				if len(opts.MultimodalData.Parts) != len(tt.input.Parts) {
					t.Errorf("Parts length = %v, want %v", len(opts.MultimodalData.Parts), len(tt.input.Parts))
				}
			}
		})
	}
}

func TestWithUsageHandler(t *testing.T) {
	tests := []struct {
		name          string
		handler       func(*UsageMetadata)
		testMetadata  *UsageMetadata
		expectCalled  bool
		validateUsage func(t *testing.T, usage *UsageMetadata)
	}{
		{
			name: "handler receives usage data",
			handler: func(_ *UsageMetadata) {
				// Handler will be tested by validateUsage
			},
			testMetadata: &UsageMetadata{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			expectCalled: true,
			validateUsage: func(t *testing.T, usage *UsageMetadata) {
				if usage.PromptTokens != 100 {
					t.Errorf("PromptTokens = %v, want 100", usage.PromptTokens)
				}
				if usage.CompletionTokens != 50 {
					t.Errorf("CompletionTokens = %v, want 50", usage.CompletionTokens)
				}
				if usage.TotalTokens != 150 {
					t.Errorf("TotalTokens = %v, want 150", usage.TotalTokens)
				}
			},
		},
		{
			name: "handler with zero tokens",
			handler: func(_ *UsageMetadata) {
				// Handler will be tested by validateUsage
			},
			testMetadata: &UsageMetadata{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
			expectCalled: true,
			validateUsage: func(t *testing.T, usage *UsageMetadata) {
				if usage.TotalTokens != 0 {
					t.Errorf("TotalTokens = %v, want 0", usage.TotalTokens)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedUsage *UsageMetadata
			called := false

			// Wrap handler to capture calls
			wrappedHandler := func(usage *UsageMetadata) {
				called = true
				capturedUsage = usage
				if tt.handler != nil {
					tt.handler(usage)
				}
			}

			option := WithUsageHandler(wrappedHandler)
			opts := &AgentOptions{}
			option.Apply(opts)

			if opts.UsageHandler == nil {
				t.Error("WithUsageHandler() should set UsageHandler")
				return
			}

			// Test the handler by calling it
			opts.UsageHandler(tt.testMetadata)

			if called != tt.expectCalled {
				t.Errorf("Handler called = %v, want %v", called, tt.expectCalled)
			}

			if tt.validateUsage != nil && capturedUsage != nil {
				tt.validateUsage(t, capturedUsage)
			}
		})
	}
}

func TestWithUsageHandlerNil(t *testing.T) {
	// Test that nil handler is accepted
	option := WithUsageHandler(nil)
	opts := &AgentOptions{}
	option.Apply(opts)

	if opts.UsageHandler != nil {
		t.Error("WithUsageHandler(nil) should set UsageHandler to nil")
	}
}

func TestWithToolResultFormatter(t *testing.T) {
	tests := []struct {
		name         string
		formatter    ToolResultFormatterFunc
		client       []Client
		expectClient bool
	}{
		{
			name: "formatter without client",
			formatter: func(_ Client, _ []byte) calque.Handler {
				return calque.HandlerFunc(func(_ *calque.Request, w *calque.Response) error {
					return calque.Write(w, []byte("formatted"))
				})
			},
			client:       nil,
			expectClient: false,
		},
		{
			name: "formatter with client",
			formatter: func(_ Client, _ []byte) calque.Handler {
				return calque.HandlerFunc(func(_ *calque.Request, w *calque.Response) error {
					return calque.Write(w, []byte("formatted with client"))
				})
			},
			client:       []Client{NewMockClient("test")},
			expectClient: true,
		},
		{
			name: "formatter that returns nil handler",
			formatter: func(_ Client, _ []byte) calque.Handler {
				// This formatter intentionally returns nil
				return nil
			},
			client:       nil,
			expectClient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var option AgentOption
			if len(tt.client) > 0 {
				option = WithToolResultFormatter(tt.formatter, tt.client[0])
			} else {
				option = WithToolResultFormatter(tt.formatter)
			}

			opts := &AgentOptions{}
			option.Apply(opts)

			if opts.ToolResultFormatter == nil {
				t.Error("WithToolResultFormatter() should set ToolResultFormatter")
				return
			}

			if tt.expectClient && opts.ToolFormatterClient == nil {
				t.Error("WithToolResultFormatter() should set ToolFormatterClient when provided")
			}

			if !tt.expectClient && opts.ToolFormatterClient != nil {
				t.Error("WithToolResultFormatter() should not set ToolFormatterClient when not provided")
			}

			// Test that the formatter can be called
			handler := opts.ToolResultFormatter(nil, []byte("test input"))

			// Only test handler execution if it's not nil
			// (some formatters may intentionally return nil)
			if handler != nil {
				var buf bytes.Buffer
				req := calque.NewRequest(context.Background(), strings.NewReader("tool results"))
				res := calque.NewResponse(&buf)
				err := handler.ServeFlow(req, res)
				if err != nil {
					t.Errorf("Handler execution error = %v", err)
				}
			}
		})
	}
}

func TestWithToolResultFormatterIntegration(t *testing.T) {
	// Test that formatter receives original input and can use client
	originalInput := []byte("What is 2+2?")
	var capturedInput []byte
	var capturedClient Client

	mockClient := NewMockClient("4")

	formatter := func(client Client, input []byte) calque.Handler {
		capturedInput = input
		capturedClient = client
		return calque.HandlerFunc(func(_ *calque.Request, w *calque.Response) error {
			return calque.Write(w, []byte("custom formatted result"))
		})
	}

	option := WithToolResultFormatter(formatter, mockClient)
	opts := &AgentOptions{}
	option.Apply(opts)

	// Call the formatter
	handler := opts.ToolResultFormatter(mockClient, originalInput)

	// Verify captured values
	if !bytes.Equal(capturedInput, originalInput) {
		t.Errorf("Formatter received input = %v, want %v", capturedInput, originalInput)
	}

	if capturedClient != mockClient {
		t.Error("Formatter should receive the provided client")
	}

	// Execute handler
	var buf bytes.Buffer
	req := calque.NewRequest(context.Background(), strings.NewReader("tool result data"))
	res := calque.NewResponse(&buf)
	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Handler execution error = %v", err)
	}

	output := buf.String()
	if output != "custom formatted result" {
		t.Errorf("Handler output = %v, want %v", output, "custom formatted result")
	}
}

func TestWithTools(t *testing.T) {
	// Test WithTools (already exists, but verify it works with our new tests)
	tool1 := tools.Simple("tool1", "desc1", func(s string) string { return s })
	tool2 := tools.Simple("tool2", "desc2", func(s string) string { return s })

	tests := []struct {
		name        string
		tools       []tools.Tool
		expectedLen int
	}{
		{
			name:        "no tools",
			tools:       []tools.Tool{},
			expectedLen: 0,
		},
		{
			name:        "single tool",
			tools:       []tools.Tool{tool1},
			expectedLen: 1,
		},
		{
			name:        "multiple tools",
			tools:       []tools.Tool{tool1, tool2},
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithTools(tt.tools...)
			opts := &AgentOptions{}
			option.Apply(opts)

			if len(opts.Tools) != tt.expectedLen {
				t.Errorf("WithTools() len = %v, want %v", len(opts.Tools), tt.expectedLen)
			}
		})
	}
}

func TestWithToolsConfig(t *testing.T) {
	// Test WithToolsConfig (already exists, but verify it works)
	config := tools.Config{
		MaxConcurrentTools:    3,
		IncludeOriginalOutput: true,
	}

	option := WithToolsConfig(config)
	opts := &AgentOptions{}
	option.Apply(opts)

	if opts.ToolsConfig == nil {
		t.Error("WithToolsConfig() should set ToolsConfig")
		return
	}

	if opts.ToolsConfig.MaxConcurrentTools != 3 {
		t.Errorf("MaxConcurrentTools = %v, want 3", opts.ToolsConfig.MaxConcurrentTools)
	}

	if !opts.ToolsConfig.IncludeOriginalOutput {
		t.Error("IncludeOriginalOutput should be true")
	}
}

func TestOptionComposition(t *testing.T) {
	// Test that multiple options can be composed together
	tool := tools.Simple("test", "desc", func(s string) string { return s })
	schema := &ResponseFormat{Type: "json_object"}
	multimodal := &MultimodalInput{Parts: []ContentPart{Text("test")}}

	var usageCalled bool
	usageHandler := func(_ *UsageMetadata) {
		usageCalled = true
	}

	opts := &AgentOptions{}

	// Apply multiple options
	WithTools(tool).Apply(opts)
	WithSchema(schema).Apply(opts)
	WithMultimodalData(multimodal).Apply(opts)
	WithUsageHandler(usageHandler).Apply(opts)

	// Verify all were applied
	if len(opts.Tools) != 1 {
		t.Error("Tools should be set")
	}
	if opts.Schema == nil {
		t.Error("Schema should be set")
	}
	if opts.MultimodalData == nil {
		t.Error("MultimodalData should be set")
	}
	if opts.UsageHandler == nil {
		t.Error("UsageHandler should be set")
	}

	// Test usage handler
	opts.UsageHandler(&UsageMetadata{TotalTokens: 100})
	if !usageCalled {
		t.Error("UsageHandler should have been called")
	}
}
