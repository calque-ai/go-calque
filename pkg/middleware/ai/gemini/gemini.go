// Package gemini provides Google Gemini AI model integration for the calque framework.
// It implements the AI client interface to enable chat completions, tool calling,
// and streaming responses using Google's Gemini models including Pro and Flash variants.
package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"google.golang.org/genai"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/config"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

const (
	applicationJSON        = "application/json"
	responseTypeJSON       = "json_object"
	responseTypeJSONSchema = "json_schema"
	toolCallType           = "function"
	contentTypeText        = "text"
	errModelNameRequired   = "model name is required"
)

// Client implements the Client interface for Google Gemini.
//
// Provides streaming chat completions with tool calling support.
// Supports Gemini Pro, Flash, and other Google AI models.
//
// Example:
//
//	client, _ := gemini.New("gemini-3.6-flash")
//	agent := ai.Agent(client)
type Client struct {
	client    *genai.Client
	model     string
	config    *Config
	lastUsage *ai.UsageMetadata
}

// Config holds Gemini-specific configuration.
//
// Configures model behavior, safety settings, and response format.
// All fields are optional with sensible defaults.
//
// Example:
//
//	config := &gemini.Config{
//		Temperature: new(float32(0.8)),
//		MaxTokens: new(1000),
//	}
type Config struct {
	// Required. API key for Google AI Studio authentication.
	// Not used by NewVertex — Vertex AI authenticates via ADC instead.
	APIKey string

	// Optional. Controls randomness in token selection (0.0-2.0)
	// Lower values = more deterministic, higher values = more creative
	Temperature *float32

	// Optional. Nucleus sampling parameter (0.0-1.0)
	// Tokens are selected until their probabilities sum to this value
	TopP *float32

	// Optional. Top-k sampling - select from k highest probability tokens
	// Lower values = less random, higher values = more random
	TopK *float32

	// Optional. Maximum number of tokens in the response
	MaxTokens *int

	// Optional. Strings that stop text generation when encountered
	Stop []string

	// Optional. System instructions to steer model behavior
	// Example: "Answer as concisely as possible" or "Don't use technical terms"
	SystemInstruction string

	// Optional. Penalize tokens that already appear in generated text (-2.0 to 2.0)
	// Positive values increase content diversity
	PresencePenalty *float32

	// Optional. Penalize frequently repeated tokens (-2.0 to 2.0)
	// Positive values reduce repetition
	FrequencyPenalty *float32

	// Optional. Fixed seed for reproducible responses
	Seed *int32

	// Optional. Number of response variations to generate
	CandidateCount *int32

	// Optional. Response format configuration (JSON schema, etc.)
	ResponseFormat *ai.ResponseFormat

	// Optional. Safety settings to block unsafe content
	SafetySettings []*genai.SafetySetting

	// Optional. Enable/disable streaming of responses (disabled automatically when tools are present)
	// Default: true (streaming enabled), but tools force non-streaming regardless of this setting
	Stream *bool
}

// Option interface for functional options pattern
type Option interface {
	Apply(*Config)
}

// configOption implements Option
type configOption struct{ config *Config }

func (o configOption) Apply(opts *Config) {
	config.Merge(opts, o.config)
}

// WithConfig sets custom Gemini configuration.
//
// Input: *Config with Gemini settings
// Output: Option for client creation
// Behavior: Merges with default configuration (only non-zero/nil fields override defaults)
//
// Example:
//
//	config := &gemini.Config{Temperature: new(float32(0.9))}
//	client, _ := gemini.New("gemini-3.6-flash", gemini.WithConfig(config))
func WithConfig(config *Config) Option {
	return configOption{config: config}
}

// DefaultConfig returns sensible defaults for Gemini.
//
// Input: none
// Output: *Config with default settings
// Behavior: Creates config with GOOGLE_API_KEY from env
//
// Sets temperature to 0.7 and API key from environment.
//
// Example:
//
//	config := gemini.DefaultConfig()
//	config.MaxTokens = new(2000)
func DefaultConfig() *Config {
	return &Config{
		APIKey:      os.Getenv("GOOGLE_API_KEY"),
		Temperature: new(float32(0.7)),
	}
}

// New creates a new Gemini client with optional configuration.
//
// Input: model name string, optional config Options
// Output: *Client, error
// Behavior: Initializes authenticated Gemini client
//
// Requires GOOGLE_API_KEY environment variable or config.APIKey.
// Supports all Gemini models: gemini-3.6-flash, gemini-3.5-flash-lite, etc.
//
// Example:
//
//	client, err := gemini.New("gemini-3.6-flash")
//	if err != nil { log.Fatal(err) }
func New(model string, opts ...Option) (*Client, error) {
	ctx := context.Background()
	if model == "" {
		return nil, calque.NewErr(ctx, errModelNameRequired)
	}

	// Build config from options
	config := DefaultConfig()
	for _, opt := range opts {
		opt.Apply(config)
	}

	// Validate API key
	if config.APIKey == "" {
		return nil, calque.NewErr(ctx, "GOOGLE_API_KEY environment variable not set or provided in config")
	}

	// Configure the GenAI client
	clientConfig := &genai.ClientConfig{
		APIKey: config.APIKey,
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, calque.WrapErr(ctx, err, "failed to create genai client")
	}

	return &Client{
		client: client,
		model:  model,
		config: config,
	}, nil
}

// RequestConfig holds configuration for a Gemini request
type RequestConfig struct {
	GenaiConfig *genai.GenerateContentConfig
	Chat        *genai.Chat
	Parts       []genai.Part
	HasTools    bool
}

// Chat implements the Client interface with streaming support.
//
// Input: user prompt/query via calque.Request
// Output: streamed AI response via calque.Response
// Behavior: STREAMING - outputs tokens as they arrive
//
// Supports tool calling, JSON schema responses, and safety filtering.
// Automatically formats tool calls for agent framework compatibility.
//
// Example:
//
//	err := client.Chat(req, res, &ai.AgentOptions{Tools: tools})
func (g *Client) Chat(r *calque.Request, w *calque.Response, opts *ai.AgentOptions) error {
	// Which input type are we processing?
	input, err := ai.ClassifyInput(r, opts)
	if err != nil {
		return err
	}

	// Build request configuration based on input type
	config, err := g.buildRequestConfig(r.Context, input, ai.GetSchema(opts), ai.GetTools(opts), ai.GetHistory(opts), ai.GetToolsDisabled(opts))
	if err != nil {
		return err
	}

	// Execute the request with the configured chat
	return g.executeRequest(config, r, w, opts)
}

// buildGenerateConfig creates a Gemini GenerateContentConfig from provider config and optional schema override
func (g *Client) buildGenerateConfig(schemaOverride *ai.ResponseFormat) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}

	// Apply client configuration
	if g.config.Temperature != nil {
		config.Temperature = new(*g.config.Temperature)
	}
	if g.config.TopP != nil {
		config.TopP = new(*g.config.TopP)
	}
	if g.config.TopK != nil {
		config.TopK = new(*g.config.TopK)
	}
	if g.config.MaxTokens != nil {
		config.MaxOutputTokens = int32(*g.config.MaxTokens)
	}
	if len(g.config.Stop) > 0 {
		config.StopSequences = g.config.Stop
	}
	if g.config.SystemInstruction != "" {
		systemContent := genai.Text(g.config.SystemInstruction)
		if len(systemContent) > 0 {
			config.SystemInstruction = systemContent[0]
		}
	}
	if g.config.PresencePenalty != nil {
		config.PresencePenalty = new(*g.config.PresencePenalty)
	}
	if g.config.FrequencyPenalty != nil {
		config.FrequencyPenalty = new(*g.config.FrequencyPenalty)
	}
	if g.config.Seed != nil {
		config.Seed = new(*g.config.Seed)
	}
	if g.config.CandidateCount != nil {
		config.CandidateCount = *g.config.CandidateCount
	}
	if len(g.config.SafetySettings) > 0 {
		config.SafetySettings = g.config.SafetySettings
	}

	// Apply response format - request override takes priority
	var responseFormat *ai.ResponseFormat
	if schemaOverride != nil {
		responseFormat = schemaOverride
	} else {
		responseFormat = g.config.ResponseFormat
	}

	if responseFormat != nil {
		switch responseFormat.Type {
		case responseTypeJSON:
			config.ResponseMIMEType = applicationJSON
		case responseTypeJSONSchema:
			config.ResponseMIMEType = applicationJSON
			if responseFormat.Schema != nil {
				config.ResponseJsonSchema = responseFormat.Schema
			}
		}
	}

	return config
}

// Convert tools to Gemini format using internal schema
func convertToolsToGeminiFunctions(toolList []tools.Tool) []*genai.FunctionDeclaration {
	internalTools := tools.FormatToolsAsInternal(toolList)
	functions := make([]*genai.FunctionDeclaration, len(internalTools))

	for i, tool := range internalTools {
		// Convert internal schema to JSON for Gemini
		var parametersJSONSchema map[string]any
		if tool.Parameters != nil {
			if paramsBytes, err := json.Marshal(tool.Parameters); err == nil {
				_ = json.Unmarshal(paramsBytes, &parametersJSONSchema)
			}
		}

		functions[i] = &genai.FunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJsonSchema: parametersJSONSchema,
		}
	}

	return functions
}

// applyTools declares tools on genaiConfig and, if toolsDisabled, forces
// FunctionCallingConfigModeNone - Gemini's documented mechanism for "stop
// calling tools, answer in text" - instead of dropping the declarations
// outright. Gemini's newer models get confused by their own unresolved
// FunctionCall/FunctionResponse history when a later turn omits tools
// entirely, and echo back an empty FunctionCall instead of prose; keeping
// tools declared but disabled avoids that. Reports whether any tools were
// declared.
func applyTools(genaiConfig *genai.GenerateContentConfig, toolList []tools.Tool, toolsDisabled bool) bool {
	if len(toolList) == 0 {
		return false
	}

	genaiConfig.Tools = []*genai.Tool{{FunctionDeclarations: convertToolsToGeminiFunctions(toolList)}}

	if toolsDisabled {
		genaiConfig.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeNone,
			},
		}
	}

	return true
}

// buildRequestConfig creates configuration for the request.
// If history is non-empty, it is round-tripped as prior chat turns instead
// of building a single-message request from input. See applyTools for what
// toolsDisabled does.
func (g *Client) buildRequestConfig(ctx context.Context, input *ai.ClassifiedInput, schema *ai.ResponseFormat, tools []tools.Tool, history []ai.Message, toolsDisabled bool) (*RequestConfig, error) {
	// Build config once
	genaiConfig := g.buildGenerateConfig(schema)
	hasTools := applyTools(genaiConfig, tools, toolsDisabled)

	// Convert input (or history) to parts and prior chat history once
	priorHistory, parts, err := g.inputToContents(ctx, input, history)
	if err != nil {
		return nil, err
	}

	// Create chat, seeded with any prior turns
	chat, err := g.client.Chats.Create(ctx, g.model, genaiConfig, priorHistory)
	if err != nil {
		return nil, calque.WrapErr(ctx, err, "failed to create chat")
	}

	return &RequestConfig{
		GenaiConfig: genaiConfig,
		Chat:        chat,
		Parts:       parts,
		HasTools:    hasTools,
	}, nil
}

// executeRequest executes the configured request
func (g *Client) executeRequest(config *RequestConfig, r *calque.Request, w *calque.Response, opts *ai.AgentOptions) error {
	// Determine if we should stream
	// Tools force non-streaming to avoid mixing text with function call JSON
	shouldStream := !config.HasTools && (g.config.Stream == nil || *g.config.Stream)

	if shouldStream {
		return g.executeStreamingRequest(config, r, w, opts)
	}

	return g.executeNonStreamingRequest(config, r, w, opts)
}

// reportUsage invokes the usage handler if present
func (g *Client) reportUsage(opts *ai.AgentOptions) {
	if g.lastUsage != nil && opts != nil && opts.UsageHandler != nil {
		opts.UsageHandler(g.lastUsage)
	}
}

// executeNonStreamingRequest executes a non-streaming request using SendMessage
func (g *Client) executeNonStreamingRequest(config *RequestConfig, r *calque.Request, w *calque.Response, opts *ai.AgentOptions) error {
	// Use SendMessage for buffered response
	result, err := config.Chat.SendMessage(r.Context, config.Parts...)
	if err != nil {
		return calque.WrapErr(r.Context, err, "failed to get response")
	}

	// Capture usage metadata
	if result.UsageMetadata != nil {
		g.lastUsage = &ai.UsageMetadata{
			PromptTokens:     int(result.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(result.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(result.UsageMetadata.TotalTokenCount),
		}
	}

	// Report usage
	g.reportUsage(opts)

	// Check for function calls first
	functionCallParts := extractFunctionCallParts(result)
	if len(functionCallParts) > 0 {
		return g.writeFunctionCalls(functionCallParts, w)
	}

	// Write text response
	text := result.Text()
	if text != "" {
		_, err = w.Data.Write([]byte(text))
		return err
	}

	return nil
}

// executeStreamingRequest executes a streaming request using SendMessageStream
func (g *Client) executeStreamingRequest(config *RequestConfig, r *calque.Request, w *calque.Response, opts *ai.AgentOptions) error {
	// Stream response chunks directly
	for result, err := range config.Chat.SendMessageStream(r.Context, config.Parts...) {
		if err != nil {
			return calque.WrapErr(r.Context, err, "failed to get response")
		}

		// Capture usage metadata from stream chunks
		if result.UsageMetadata != nil {
			g.lastUsage = &ai.UsageMetadata{
				PromptTokens:     int(result.UsageMetadata.PromptTokenCount),
				CompletionTokens: int(result.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int(result.UsageMetadata.TotalTokenCount),
			}
		}

		// Get text from chunk and stream it
		text := result.Text()
		if text != "" {
			if _, writeErr := w.Data.Write([]byte(text)); writeErr != nil {
				return writeErr
			}
		}
	}

	// Report usage after stream completes
	g.reportUsage(opts)

	return nil
}

// extractFunctionCallParts returns the parts of the first candidate that
// carry a function call, preserving each part's ThoughtSignature - unlike
// GenerateContentResponse.FunctionCalls(), which discards it. Gemini's newer
// models require that signature echoed back verbatim if the call is replayed
// in later history (see assistantContent in this package).
func extractFunctionCallParts(result *genai.GenerateContentResponse) []*genai.Part {
	if len(result.Candidates) == 0 || result.Candidates[0].Content == nil {
		return nil
	}

	var parts []*genai.Part
	for _, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			parts = append(parts, part)
		}
	}
	return parts
}

// writeFunctionCalls formats Gemini function calls as OpenAI JSON format for the agent
func (g *Client) writeFunctionCalls(parts []*genai.Part, w *calque.Response) error {
	// Convert to OpenAI format
	toolCalls := make([]map[string]any, len(parts))

	for i, part := range parts {
		call := part.FunctionCall

		// Marshal ALL arguments from Gemini to JSON string
		var argsJSON string
		if call.Args != nil {
			argsBytes, err := json.Marshal(call.Args)
			if err == nil {
				argsJSON = string(argsBytes)
			} else {
				argsJSON = "{}"
			}
		} else {
			argsJSON = "{}"
		}

		// OpenAI format with type and function fields; ID and
		// thought_signature are included when Gemini supplies them - ID
		// otherwise falls back to tools.generateToolCallID() on the
		// receiving end, thought_signature has no fallback since it must
		// match what the model actually returned.
		toolCall := map[string]any{
			"type": toolCallType,
			"id":   call.ID,
			toolCallType: map[string]any{
				"name":      call.Name,
				"arguments": argsJSON,
			},
		}
		if len(part.ThoughtSignature) > 0 {
			toolCall["thought_signature"] = base64.StdEncoding.EncodeToString(part.ThoughtSignature)
		}
		toolCalls[i] = toolCall
	}

	// Use json.Marshal for proper JSON formatting
	result := map[string]any{
		"tool_calls": toolCalls,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = w.Data.Write(jsonBytes)
	return err
}

// inputToParts converts classified input to genai.Part array
func (g *Client) inputToParts(ctx context.Context, input *ai.ClassifiedInput) ([]genai.Part, error) {
	switch input.Type {
	case ai.TextInput:
		return []genai.Part{{Text: input.Text}}, nil

	case ai.MultimodalJSONInput, ai.MultimodalStreamingInput:
		return g.multimodalToParts(ctx, input.Multimodal)

	default:
		return nil, calque.NewErr(ctx, fmt.Sprintf("unsupported input type: %d", input.Type))
	}
}

// inputToContents converts classified input (or, once populated, agent
// history) into the two pieces genai.Chats.Create/SendMessage need: prior
// turns to seed the chat with, and the parts of the final turn to send.
//
// If history is non-empty, it is round-tripped in full instead of building
// a single turn from input - the raw request body is unused once History is
// populated, matching openai.Client.inputToMessages.
func (g *Client) inputToContents(ctx context.Context, input *ai.ClassifiedInput, history []ai.Message) (priorHistory []*genai.Content, finalParts []genai.Part, err error) {
	if len(history) > 0 {
		contents, err := g.historyToContents(ctx, history)
		if err != nil {
			return nil, nil, err
		}
		last := contents[len(contents)-1]
		parts := make([]genai.Part, len(last.Parts))
		for i, p := range last.Parts {
			parts[i] = *p
		}
		return contents[:len(contents)-1], parts, nil
	}

	parts, err := g.inputToParts(ctx, input)
	return nil, parts, err
}

// historyToContents converts agent conversation history to Gemini's native
// []*genai.Content turn representation. Gemini's Chat is stateful, but the
// loop in ai.runAgentLoop rebuilds the full history on every call, so
// history is converted fresh each time rather than relying on Chat's own
// tracking - this keeps the loop provider-agnostic.
//
// Consecutive ai.RoleTool messages are folded into a single Content, since
// Gemini requires the FunctionResponse parts answering a parallel-call turn
// to be batched into one turn matching the count of FunctionCall parts that
// requested them - unlike OpenAI, which accepts one ToolMessage per result.
// ai.runAgentLoop always appends a whole tool-result run contiguously right
// after the assistant turn that requested it, so a contiguous run here is
// guaranteed to belong to a single turn.
func (g *Client) historyToContents(ctx context.Context, history []ai.Message) ([]*genai.Content, error) {
	contents := make([]*genai.Content, 0, len(history))
	for i := 0; i < len(history); i++ {
		msg := history[i]
		if msg.Role == ai.RoleTool {
			j := i + 1
			for j < len(history) && history[j].Role == ai.RoleTool {
				j++
			}
			run := history[i:j]
			if err := validateToolResultRun(ctx, history, i, run); err != nil {
				return nil, err
			}
			contents = append(contents, toolResultsContent(run))
			i = j - 1
			continue
		}

		content, err := g.messageToContent(ctx, msg)
		if err != nil {
			return nil, err
		}
		contents = append(contents, content)
	}
	return contents, nil
}

// validateToolResultRun checks that a contiguous run of ai.RoleTool messages
// starting at history[runStart] answers exactly the FunctionCalls requested
// by the immediately preceding assistant turn - same IDs, same count. This
// never fires for history built by ai.runAgentLoop (its append site keeps
// the two in lockstep), but historyToContents also accepts arbitrary
// caller-built history, so a mismatched run - dropped, duplicated, or
// misordered tool results - is rejected here with a specific error instead
// of silently producing a malformed request that Gemini would reject later
// with a vaguer one.
func validateToolResultRun(ctx context.Context, history []ai.Message, runStart int, run []ai.Message) error {
	if runStart == 0 || history[runStart-1].Role != ai.RoleAssistant || len(history[runStart-1].ToolCalls) == 0 {
		return calque.NewErr(ctx, "tool result message has no preceding assistant tool call turn")
	}

	calls := history[runStart-1].ToolCalls
	if len(run) != len(calls) {
		return calque.NewErr(ctx, fmt.Sprintf("tool result count (%d) does not match prior tool call count (%d)", len(run), len(calls)))
	}

	wantIDs := make(map[string]bool, len(calls))
	for _, call := range calls {
		wantIDs[call.ID] = true
	}
	for _, msg := range run {
		if !wantIDs[msg.ToolCallID] {
			return calque.NewErr(ctx, fmt.Sprintf("tool result ID %q does not match any tool call in the prior turn", msg.ToolCallID))
		}
	}
	return nil
}

// messageToContent converts a single non-tool-result ai.Message to a
// genai.Content turn. Gemini's Content role is either "user" or "model" -
// RoleAssistant maps to "model", and RoleSystem (not produced by the current
// agent loop, but part of the shared Role enum) falls back to "user" since
// Gemini has no inline system turn; system instructions are configured
// separately via Config.SystemInstruction.
func (g *Client) messageToContent(ctx context.Context, msg ai.Message) (*genai.Content, error) {
	switch msg.Role {
	case ai.RoleUser, ai.RoleSystem:
		if msg.Multimodal != nil {
			parts, err := g.multimodalToParts(ctx, msg.Multimodal)
			if err != nil {
				return nil, err
			}
			return genai.NewContentFromParts(partPointers(parts), genai.RoleUser), nil
		}
		return genai.NewContentFromText(msg.Content, genai.RoleUser), nil

	case ai.RoleAssistant:
		return assistantContent(msg), nil

	default:
		return nil, calque.NewErr(ctx, fmt.Sprintf("unsupported message role: %s", msg.Role))
	}
}

// toolResultsContent folds a contiguous run of ai.RoleTool messages from a
// single turn into one "user" role Content, with one FunctionResponse part
// per message - matching the FunctionCall parts Gemini sent in that turn.
func toolResultsContent(msgs []ai.Message) *genai.Content {
	parts := make([]*genai.Part, len(msgs))
	for i, msg := range msgs {
		parts[i] = &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				ID:       msg.ToolCallID,
				Name:     msg.ToolName,
				Response: map[string]any{"output": msg.Content},
			},
		}
	}
	return genai.NewContentFromParts(parts, genai.RoleUser)
}

// assistantContent builds a "model" role Content, attaching function calls
// when present.
func assistantContent(msg ai.Message) *genai.Content {
	if len(msg.ToolCalls) == 0 {
		return genai.NewContentFromText(msg.Content, genai.RoleModel)
	}

	parts := make([]*genai.Part, len(msg.ToolCalls))
	for i, call := range msg.ToolCalls {
		var args map[string]any
		if call.Arguments != "" {
			_ = json.Unmarshal([]byte(call.Arguments), &args)
		}
		part := &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   call.ID,
				Name: call.Name,
				Args: args,
			},
		}
		// Gemini requires the exact ThoughtSignature it originally returned
		// to be echoed back when this call is replayed in history, or newer
		// models reject the request. Decoding failure just leaves it unset
		// rather than failing the whole turn.
		if call.ThoughtSignature != "" {
			if sig, err := base64.StdEncoding.DecodeString(call.ThoughtSignature); err == nil {
				part.ThoughtSignature = sig
			}
		}
		parts[i] = part
	}
	return genai.NewContentFromParts(parts, genai.RoleModel)
}

// partPointers converts a []genai.Part value slice to []*genai.Part, matching
// the pointer-element shape genai.Content.Parts expects.
func partPointers(parts []genai.Part) []*genai.Part {
	pointers := make([]*genai.Part, len(parts))
	for i := range parts {
		pointers[i] = &parts[i]
	}
	return pointers
}

// multimodalToParts converts multimodal input to genai.Part array
func (g *Client) multimodalToParts(ctx context.Context, multimodal *ai.MultimodalInput) ([]genai.Part, error) {
	if multimodal == nil {
		return nil, calque.NewErr(ctx, "multimodal input cannot be nil")
	}

	var parts []genai.Part

	for _, part := range multimodal.Parts {
		switch part.Type {
		case contentTypeText:
			if part.Text != "" {
				parts = append(parts, genai.Part{Text: part.Text})
			}
		case "image", "audio", "video":
			var data []byte
			var err error

			if part.Reader != nil {
				// Read stream data (streaming approach)
				data, err = io.ReadAll(part.Reader)
				if err != nil {
					return nil, calque.WrapErr(ctx, err, fmt.Sprintf("failed to read %s data", part.Type))
				}
			} else if part.Data != nil {
				// Use embedded data (simple approach)
				data = part.Data
			}

			if data != nil {
				parts = append(parts, genai.Part{
					InlineData: &genai.Blob{
						Data:     data,
						MIMEType: part.MimeType,
					},
				})
			}
		default:
			return nil, calque.NewErr(ctx, fmt.Sprintf("unsupported content part type: %s", part.Type))
		}
	}

	if len(parts) == 0 {
		return nil, calque.NewErr(ctx, "no valid content parts found in multimodal input")
	}

	return parts, nil
}
