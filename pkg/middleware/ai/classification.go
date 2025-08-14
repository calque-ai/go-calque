package ai

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// InputType represents the type of input being processed
type InputType int

const (
	TextInput InputType = iota
	MultimodalJSONInput
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
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	// Check streaming multimodal first (most specific)
	if opts != nil && opts.MultimodalData != nil {
		return &ClassifiedInput{
			Type:       MultimodalStreamingInput,
			RawBytes:   inputBytes,
			Multimodal: opts.MultimodalData,
		}, nil
	}

	// Try JSON multimodal
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

	// Default to text
	return &ClassifiedInput{
		Type:     TextInput,
		RawBytes: inputBytes,
		Text:     string(inputBytes),
	}, nil
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

// Helper functions for extracting options
func GetSchema(opts *AgentOptions) *ResponseFormat {
	if opts != nil {
		return opts.Schema
	}
	return nil
}

func GetTools(opts *AgentOptions) []tools.Tool {
	if opts != nil {
		return opts.Tools
	}
	return nil
}