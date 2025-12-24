package ai

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// InputType represents the type of input being processed
type InputType int

const (
	// TextInput is plain text input
	TextInput InputType = iota
	// MultimodalJSONInput is multimodal input via JSON structure
	MultimodalJSONInput
	// MultimodalStreamingInput is multimodal input via streaming (e.g., multipart)
	MultimodalStreamingInput
)

// ClassifiedInput represents input after classification
type ClassifiedInput struct {
	Type       InputType
	RawBytes   []byte
	Text       string
	Multimodal *MultimodalInput
}

// ClassifyInput reads and classifies the input type for any AI client
func ClassifyInput(r *calque.Request, opts *AgentOptions) (*ClassifiedInput, error) {
	// Read input once
	inputBytes, err := io.ReadAll(r.Data)
	if err != nil {
		return nil, calque.WrapErr(r.Context, err, "failed to read input")
	}

	// Check streaming multimodal first (most specific)
	if opts != nil && opts.MultimodalData != nil {
		return &ClassifiedInput{
			Type:       MultimodalStreamingInput,
			RawBytes:   inputBytes,
			Multimodal: opts.MultimodalData,
		}, nil
	}

	// Try JSON multimodal with fast pre-check
	if isMultimodalJSON(inputBytes) {
		var jsonMultimodal MultimodalInput
		if json.Unmarshal(inputBytes, &jsonMultimodal) == nil && len(jsonMultimodal.Parts) > 0 {
			if hasJSONData(jsonMultimodal) {
				return &ClassifiedInput{
					Type:       MultimodalJSONInput,
					RawBytes:   inputBytes,
					Multimodal: &jsonMultimodal,
				}, nil
			}
		}
	}

	// Default to text
	return &ClassifiedInput{
		Type:     TextInput,
		RawBytes: inputBytes,
		Text:     string(inputBytes),
	}, nil
}

// isMultimodalJSON performs fast detection before expensive unmarshaling
func isMultimodalJSON(data []byte) bool {
	if !json.Valid(data) {
		return false
	}

	// Check for MultimodalInput structure indicators
	if !bytes.Contains(data, []byte(`"parts"`)) {
		return false
	}

	// Check for ContentPart "type" field to reduce false positives
	if !bytes.Contains(data, []byte(`"type"`)) {
		return false
	}

	return true
}

// hasJSONData checks if multimodal input contains embedded data (simple approach)
func hasJSONData(multimodal MultimodalInput) bool {
	for _, part := range multimodal.Parts {
		if part.Type != "text" && part.Data != nil && len(part.Data) > 0 {
			return true
		}
	}
	return false
}

// GetSchema extracts schema from AgentOptions, returns nil if none
func GetSchema(opts *AgentOptions) *ResponseFormat {
	if opts != nil {
		return opts.Schema
	}
	return nil
}

// GetTools extracts tools from AgentOptions, returns nil if none
func GetTools(opts *AgentOptions) []tools.Tool {
	if opts != nil {
		return opts.Tools
	}
	return nil
}
