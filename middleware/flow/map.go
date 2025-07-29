package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/calque-ai/calque-pipe/core"
)

// Map applies a handler to each item in a slice, transforming []T to []U.
//
// Input: JSON array of type T (buffered - reads entire slice into memory)  
// Output: JSON array of type U (results from handler applied to each item)
// Behavior: BUFFERED - reads entire slice to process each item sequentially
//
// The handler receives each item individually as JSON and produces a single
// result as JSON. All items are processed sequentially, maintaining order.
//
// Usage with convert utilities:
//   pipeline.Use(flow.Map[Resume, Evaluation](evaluationHandler))
//   pipeline.Run(ctx, convert.Json(resumes), convert.JsonOutput(&results))
//
// The conversion to/from other formats (YAML, etc) happens at pipeline
// boundaries using convert.Json(), convert.StructuredYAML(), etc.
//
// If any item processing fails, the entire operation fails.
//
// Example:
//
//	evaluator := createEvaluationHandler(llmProvider) 
//	pipeline := core.New().Use(flow.Map[Resume, Evaluation](evaluator))
//	pipeline.Run(ctx, convert.Json(resumes), convert.JsonOutput(&evaluations))
func Map[T, U any](handler core.Handler) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Read input as JSON array of T
		inputBytes, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		var input []T
		if err := json.Unmarshal(inputBytes, &input); err != nil {
			return err
		}

		// Process each item through the handler
		var results []U
		for _, item := range input {
			// Convert item to JSON for handler input
			itemBytes, err := json.Marshal(item)
			if err != nil {
				return err
			}

			// Run the item through the handler
			var resultBuffer bytes.Buffer
			if err := handler.ServeFlow(ctx, bytes.NewReader(itemBytes), &resultBuffer); err != nil {
				return err
			}

			// Parse handler output as U
			var result U
			if err := json.Unmarshal(resultBuffer.Bytes(), &result); err != nil {
				return err
			}

			results = append(results, result)
		}

		// Write results as JSON array
		outputBytes, err := json.Marshal(results)
		if err != nil {
			return err
		}

		_, err = w.Write(outputBytes)
		return err
	})
}