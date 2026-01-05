// Package main demonstrates logging middleware capabilities with the calque framework.
// It showcases different logging backends (standard log, slog, zerolog) and various
// logging patterns for debugging and monitoring data flows in production systems.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/rs/zerolog"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

func main() {
	fmt.Print("=== Calque Flow - New Logger Example === \n\n")

	// Setup different loggers for demonstration
	setupSimpleExample()
	setupSlogExample()
	setupZerologExample()

}

func setupSimpleExample() {
	fmt.Println("1. Simple Logging Using Standard Library log Package:")

	flow := calque.NewFlow()

	flow.
		Use(inspect.Print("FULL_INPUT")).     // Log the complete input content (buffered)
		Use(inspect.Head("QUICK_DEBUG", 30)). // Define a prefix and the number of bytes to preview (streaming)
		Use(text.Transform(func(s string) string {
			return fmt.Sprintf("Processed: %s", s)
		})).
		Use(inspect.HeadTail("FINAL_CHECK", 20, 15)) // Log n bytes of the beginning and end of an input (buffered)

	input := "Quick debugging example"
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
		return
	}

	fmt.Printf("Simple Logging Result: %s\n\n", result)

}

func setupSlogExample() {
	fmt.Println("2. Slog Example:")

	// Create slog with JSON handler for structured output
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})
	slogLogger := slog.New(jsonHandler)

	// Create our logger instance
	log := inspect.New(inspect.NewSlogAdapter(slogLogger))

	// Build pipeline using slog structured logging
	flow := calque.NewFlow()

	flow.
		Use(log.Info().Head("INPUT", 40,
			inspect.Attr("framework", "calque-flow"),
			inspect.Attr("logger", "slog"),
		)).
		Use(text.Transform(func(s string) string {
			return fmt.Sprintf("Transformed: %s", strings.ToLower(s))
		})).
		Use(log.Warn().Sampling("STREAM_SAMPLING", 3, 20,
			inspect.Attr("sample_type", "output"),
		)).
		Use(log.Info().HeadTail("RESULT", 20, 10,
			inspect.Attr("stage", "final"),
			inspect.Attr("json_output", true),
		))

	input := "SLOG provides structured logging with JSON output by default."
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
		return
	}

	fmt.Printf("Slog Result: %s\n\n", result)
}

func setupZerologExample() {
	fmt.Println("3. Zerolog Console Writer Integration Example:")

	// Setup zerolog for structured logging
	zlog := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
		With().
		Timestamp().
		Str("component", "mixed-demo").
		Logger().
		Level(zerolog.DebugLevel) // Enable all levels

	log := inspect.New(inspect.NewZerologAdapter(zlog))

	flow := calque.NewFlow()

	flow.
		Use(log.Info().Head("INFO_START", 30)).
		Use(log.Debug().Chunks("DEBUG_CHUNKS", 20,
			inspect.Attr("chunk_processing", true),
		)).
		Use(log.Info().Timing("TRANSFORM_TIMING", text.Transform(func(s string) string {
			// Simulate some processing that might have issues
			if len(s) > 50 {
				return s + " [LARGE_INPUT_DETECTED]"
			}
			return s + " [NORMAL_PROCESSING]"
		}))).
		Use(log.Warn().HeadTail("WARN_ANALYSIS", 15, 10,
			inspect.Attr("analysis", "size_check"),
			inspect.Attr("threshold", 50),
		))

	// Test with longer input to trigger the warning path
	input := "This is a longer input text that will demonstrate different logging levels and how they work together in a streaming pipeline processing system."
	var result string

	err := flow.Run(context.Background(), input, &result)
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
		return
	}

	fmt.Printf("Mixed Logger Result: %s\n\n", result)
}
