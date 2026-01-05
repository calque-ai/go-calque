package inspect

import (
	"github.com/calque-ai/go-calque/pkg/calque"
)

// Package-level convenience functions for quick debugging using standard log.
// These are shortcuts to the full HandlerBuilder methods - see handlers.go for detailed documentation.
var defaultLogger = Default()

// Head logs a preview of the first N bytes using standard log.
//
// Convenience function for standard log debugging. Streams through data
// without consuming it, perfect for debugging pipeline starts.
//
// Quick debugging equivalent to: logger.Default().Print().Head(prefix, bytes)
func Head(prefix string, bytes int) calque.Handler {
	return defaultLogger.Print().Head(prefix, bytes)
}

// HeadTail logs first N and last M bytes using standard log.
//
// Convenience function for standard log debugging. Buffers data to capture
// both head and tail, useful for understanding data transformations.
//
// Quick debugging equivalent to: logger.Default().Print().HeadTail(prefix, headBytes, tailBytes)
func HeadTail(prefix string, headBytes, tailBytes int) calque.Handler {
	return defaultLogger.Print().HeadTail(prefix, headBytes, tailBytes)
}

// Chunks logs data in fixed-size chunks using standard log.
//
// Convenience function for standard log debugging. Monitors streaming data
// flow with chunk-by-chunk logging for large data debugging.
//
// Quick debugging equivalent to: logger.Default().Print().Chunks(prefix, chunkSize)
func Chunks(prefix string, chunkSize int) calque.Handler {
	return defaultLogger.Print().Chunks(prefix, chunkSize)
}

// Timing wraps handler with execution time measurement using standard log.
//
// Convenience function for standard log debugging. Measures handler performance
// with duration and throughput metrics for optimization analysis.
//
// Quick debugging equivalent to: logger.Default().Print().Timing(prefix, handler)
func Timing(prefix string, handler calque.Handler) calque.Handler {
	return defaultLogger.Print().Timing(prefix, handler)
}

// Sampling logs distributed samples throughout the stream using standard log.
//
// Convenience function for standard log debugging. Takes N samples evenly
// distributed across data for overview of large transformations.
//
// Quick debugging equivalent to: logger.Default().Print().Sampling(prefix, numSamples, sampleSize)
func Sampling(prefix string, numSamples int, sampleSize int) calque.Handler {
	return defaultLogger.Print().Sampling(prefix, numSamples, sampleSize)
}

// Print logs the complete input content using standard log.
//
// Convenience function for standard log debugging. Buffers entire input
// to log complete content, ideal for small to medium data inspection.
//
// Quick debugging equivalent to: logger.Default().Print().Print(prefix)
func Print(prefix string) calque.Handler {
	return defaultLogger.Print().Print(prefix)
}
