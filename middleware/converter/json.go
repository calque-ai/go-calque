package converter

import (
	"context"
	"encoding/json"
	"io"

	"github.com/calque-ai/calque-pipe/core"
)

// JsonInput creates a Handler that processes JSON input from the pipeline stream
// It reads the stream data and ensures it's in a JSON format for further processing
func JsonInput() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Read the input stream
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		// Try to parse as JSON to validate/normalize it
		var jsonData any
		if err := json.Unmarshal(data, &jsonData); err != nil {
			// If it's not valid JSON, pass through as-is
			_, err = w.Write(data)
			return err
		}

		// Re-marshal to ensure consistent JSON formatting
		normalizedJSON, err := json.Marshal(jsonData)
		if err != nil {
			return err
		}

		_, err = w.Write(normalizedJSON)
		return err
	})
}

// JsonOutput creates a Handler that processes JSON output for the pipeline
// It reads the stream data, parses it as JSON, and formats it appropriately
func JsonOutput() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Read the pipeline data
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		// Parse as JSON
		var result any
		if err := json.Unmarshal(data, &result); err != nil {
			// If not valid JSON, pass through as string
			_, err = w.Write(data)
			return err
		}

		// Format the JSON nicely for output
		prettyJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}

		_, err = w.Write(prettyJSON)
		return err
	})
}