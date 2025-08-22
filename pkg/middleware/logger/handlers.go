package logger

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Head logs a preview of the first N bytes of the input stream.
//
// Input: any data type (streaming - uses bufio.Reader.Peek for preview)
// Output: same as input (pass-through)
// Behavior: STREAMING - peeks at first N bytes without consuming, then streams
//
// Logs a preview of the input with the specified prefix, then passes the
// complete input through unchanged. Uses buffered peeking to avoid consuming
// the input stream. Perfect for debugging the beginning of data flows.
//
// Example:
//
//	handler := log.Info().Head("INPUT_PREVIEW", 50)
//	pipe.Use(handler) // Logs: [INPUT_PREVIEW]: Hello, world!...
func (hb *HandlerBuilder) Head(prefix string, headBytes int, attrs ...Attribute) calque.Handler {
	return hb.createHandler(func(req *calque.Request, res *calque.Response, logFunc func(string, ...Attribute)) error {
		bufReader := bufio.NewReader(req.Data)

		// Peek at first N bytes for logging without consuming
		firstBytes, err := bufReader.Peek(headBytes)
		if err != nil && err != io.EOF {
			return err
		}

		// Log preview with structured attributes
		preview := formatPreview(firstBytes)
		allAttrs := make([]Attribute, len(attrs), len(attrs)+1)
		copy(allAttrs, attrs)
		allAttrs = append(allAttrs, Attribute{"preview", preview})
		logFunc(fmt.Sprintf("[%s]", prefix), allAttrs...)

		// Pass through unchanged
		_, err = io.Copy(res.Data, bufReader)
		return err
	})
}

// Chunks logs data in fixed-size chunks as it flows through the stream.
//
// Input: any data type (streaming - uses io.TeeReader for non-intrusive monitoring)
// Output: same as input (pass-through)
// Behavior: STREAMING - logs each chunk as data flows, no buffering
//
// Monitors data flow by logging fixed-size chunks as they stream through.
// Uses TeeReader to observe data without affecting the stream flow.
// Each chunk generates a separate log entry with chunk number and data preview.
// Perfect for monitoring large streams or debugging data processing.
//
// Example:
//
//	handler := log.Debug().Chunks("STREAM_MONITOR", 1024)
//	pipe.Use(handler) // Logs: [STREAM_MONITOR] Chunk 1, Chunk 2, etc.
func (hb *HandlerBuilder) Chunks(prefix string, chunkSize int, attrs ...Attribute) calque.Handler {
	return hb.createHandler(func(req *calque.Request, res *calque.Response, logFunc func(string, ...Attribute)) error {
		// Use TeeReader to capture chunks as they flow through
		var chunkBuffer bytes.Buffer
		teeReader := io.TeeReader(req.Data, &chunkBuffer)

		buf := make([]byte, chunkSize)
		chunkNum := 0
		totalBytes := 0

		for {
			n, err := teeReader.Read(buf)
			if n > 0 {
				chunkNum++
				totalBytes += n

				// Log this chunk
				allAttrs := make([]Attribute, len(attrs), len(attrs)+4)
				copy(allAttrs, attrs)
				allAttrs = append(allAttrs,
					Attribute{"chunk_num", chunkNum},
					Attribute{"chunk_size", n},
					Attribute{"total_bytes", totalBytes},
					Attribute{"data", formatPreview(buf[:n])},
				)
				logFunc(fmt.Sprintf("[%s] Chunk %d", prefix, chunkNum), attrs...)

				// Write chunk to response (data from teeReader)
				if _, writeErr := res.Data.Write(buf[:n]); writeErr != nil {
					return writeErr
				}
			}

			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// Timing wraps another handler to measure its execution time and throughput.
//
// Input: any data type (wraps another handler)
// Output: same as wrapped handler's output
// Behavior: WRAPPING - measures wrapped handler's execution time and data throughput
//
// Wraps any handler to measure its performance. Uses TeeReader to track bytes
// processed and times the handler execution. Logs timing with smart duration
// units (μs, ms, s) and throughput (bytes/sec) when meaningful.
//
// Example:
//
//	timedHandler := log.Info().Timing("AI_PROCESSING", ai.Agent(client))
//	pipe.Use(timedHandler) // Logs: [AI_PROCESSING] completed duration_ms=150 bytes=1024
func (hb *HandlerBuilder) Timing(prefix string, handler calque.Handler, attrs ...Attribute) calque.Handler {
	return hb.createHandler(func(req *calque.Request, res *calque.Response, logFunc func(string, ...Attribute)) error {
		start := time.Now()

		// Use TeeReader to capture bytes as they flow through the handler
		var bytesBuffer bytes.Buffer
		teeReader := io.TeeReader(req.Data, &bytesBuffer)
		wrappedReq := &calque.Request{
			Data:    teeReader,
			Context: req.Context,
		}

		// Execute the wrapped handler
		err := handler.ServeFlow(wrappedReq, res)

		duration := time.Since(start)
		bytesRead := int64(bytesBuffer.Len())

		// Log timing with duration formatting and throughput
		durationField, durationValue := formatDuration(duration)
		allAttrs := make([]Attribute, len(attrs), len(attrs)+2)
		copy(allAttrs, attrs)
		allAttrs = append(allAttrs,
			Attribute{durationField, durationValue},
			Attribute{"bytes", bytesRead},
		)

		// Add throughput if we have meaningful data and time
		if bytesRead > 0 && duration.Seconds() > 0 {
			attrs = append(attrs, Attribute{"bytes_per_sec", float64(bytesRead) / duration.Seconds()})
		}

		// Log the completion message
		logFunc(fmt.Sprintf("[%s] completed", prefix), attrs...)

		return err
	})
}

// Sampling takes distributed samples throughout the stream and logs them in a single entry.
//
// Input: any data type (buffered - reads entire input to calculate sample positions)
// Output: same as input (pass-through)
// Behavior: BUFFERED - reads entire input to distribute samples evenly across data
//
// Takes N samples of specified size distributed evenly throughout the input stream.
// Like HeadTail but with multiple samples across the entire data. All samples are
// logged in a single entry with positions and data previews. Perfect for getting
// an overview of large data transformations or streaming content.
//
// Example:
//
//	handler := log.Info().Sampling("DATA_OVERVIEW", 5, 30)
//	pipe.Use(handler) // Logs: 5 samples from 1024 bytes with positions and previews
func (hb *HandlerBuilder) Sampling(prefix string, numSamples int, sampleSize int, attrs ...Attribute) calque.Handler {
	return hb.createHandler(func(req *calque.Request, res *calque.Response, logFunc func(string, ...Attribute)) error {
		// Read all data to analyze and sample
		var allData []byte
		err := calque.Read(req, &allData)
		if err != nil {
			return err
		}

		totalBytes := len(allData)
		if totalBytes == 0 {
			// Log empty stream
			allAttrs := make([]Attribute, len(attrs), len(attrs)+1)
			copy(allAttrs, attrs)
			allAttrs = append(allAttrs, Attribute{"total_bytes", 0})
			logFunc(fmt.Sprintf("[%s] Empty stream", prefix), allAttrs...)
			return nil
		}

		// Calculate sample positions distributed throughout the data
		samples := make([]string, 0, numSamples)
		samplePositions := make([]int, 0, numSamples)

		if numSamples <= 0 || totalBytes <= sampleSize {
			// If we can't sample properly, just take one sample from the beginning
			samples = append(samples, formatPreview(allData))
			samplePositions = append(samplePositions, 0)
		} else {
			// Distribute samples evenly throughout the data
			for i := range numSamples {
				// Calculate position for this sample
				position := (i * totalBytes) / numSamples
				if position+sampleSize > totalBytes {
					sampleSize = totalBytes - position
				}
				if sampleSize <= 0 {
					break
				}

				sampleData := allData[position : position+sampleSize]
				samples = append(samples, formatPreview(sampleData))
				samplePositions = append(samplePositions, position)
			}
		}

		// Create single log entry with all samples
		allAttrs := make([]Attribute, len(attrs), len(attrs)+5)
		copy(allAttrs, attrs)
		allAttrs = append(allAttrs,
			Attribute{"total_bytes", totalBytes},
			Attribute{"num_samples", len(samples)},
			Attribute{"sample_size", sampleSize},
			Attribute{"sample_positions", samplePositions},
			Attribute{"samples", samples},
		)
		logFunc(fmt.Sprintf("[%s] %d samples from %d bytes", prefix, len(samples), totalBytes), attrs...)

		// Write all data to response
		return calque.Write(res, allData)
	})
}

// Print logs the complete input content as a string.
//
// Input: any data type (buffered - reads entire input into memory)
// Output: same as input (pass-through)
// Behavior: BUFFERED - reads entire input to log complete content
//
// Logs the full input content as a string with total byte count. Perfect for
// debugging small to medium inputs where you need to see the complete data.
// Use carefully with large inputs as it buffers everything in memory.
//
// Example:
//
//	handler := log.Debug().Print("FULL_INPUT")
//	pipe.Use(handler) // Logs: [FULL_INPUT] content="Hello world" total_bytes=11
func (hb *HandlerBuilder) Print(prefix string, attrs ...Attribute) calque.Handler {
	return hb.createHandler(func(req *calque.Request, res *calque.Response, logFunc func(string, ...Attribute)) error {
		// Read all data into buffer
		var allData []byte
		err := calque.Read(req, &allData)
		if err != nil {
			return err
		}

		// Log the complete content
		allAttrs := make([]Attribute, len(attrs), len(attrs)+2)
		copy(allAttrs, attrs)
		allAttrs = append(allAttrs,
			Attribute{"total_bytes", len(allData)},
			Attribute{"content", string(allData)}, // Full content as string
		)
		logFunc(fmt.Sprintf("[%s]", prefix), attrs...)

		// Write all data to response
		return calque.Write(res, allData)
	})
}

// headTailCapture captures head and tail data during streaming
type headTailCapture struct {
	headBuf    []byte
	tailBuf    []byte
	totalBytes int
	headSize   int
	tailSize   int
}

func newHeadTailCapture(headSize, tailSize int) *headTailCapture {
	return &headTailCapture{
		headBuf:  make([]byte, 0, headSize),
		tailBuf:  make([]byte, tailSize),
		headSize: headSize,
		tailSize: tailSize,
	}
}

func (h *headTailCapture) Write(p []byte) (n int, err error) {
	// Head: append until full
	if len(h.headBuf) < h.headSize {
		needed := min(h.headSize-len(h.headBuf), len(p))
		h.headBuf = append(h.headBuf, p[:needed]...)
	}

	// Tail: overwrite until end of data stream
	if len(p) >= h.tailSize {
		// Large write - just take the end
		copy(h.tailBuf, p[len(p)-h.tailSize:])
	} else {
		// Small write
		copy(h.tailBuf, h.tailBuf[len(p):])    // shift existing data left
		copy(h.tailBuf[h.tailSize-len(p):], p) // append new data at end
	}

	h.totalBytes += len(p)
	return len(p), nil
}

// HeadTail logs the first N bytes and last M bytes of the stream.
//
// Input: any data type (streaming - uses io.TeeReader for efficient monitoring)
// Output: same as input (pass-through)
// Behavior: STREAMING - captures head and tail while data flows, constant memory usage
//
// Logs both the beginning and end of the data stream in a single log entry.
// Uses optimized streaming approach with fixed-size buffers for constant memory usage.
// Perfect for large data streams where memory efficiency is important.
//
// Example:
//
//	handler := log.Info().HeadTail("TRANSFORM_RESULT", 30, 20)
//	pipe.Use(handler) // Logs head=first 30 bytes, tail=last 20 bytes
func (hb *HandlerBuilder) HeadTail(prefix string, headBytes, tailBytes int, attrs ...Attribute) calque.Handler {
	return hb.createHandler(func(req *calque.Request, res *calque.Response, logFunc func(string, ...Attribute)) error {
		capture := newHeadTailCapture(headBytes, tailBytes)

		// TeeReader streams to both response and capture
		teeReader := io.TeeReader(req.Data, capture)
		_, err := io.Copy(res.Data, teeReader)
		if err != nil {
			return err
		}

		// Log head and tail
		allAttrs := make([]Attribute, len(attrs), len(attrs)+3)
		copy(allAttrs, attrs)
		allAttrs = append(allAttrs,
			Attribute{"head", formatPreview(capture.headBuf)},
			Attribute{"tail", formatPreview(capture.tailBuf)},
			Attribute{"total_bytes", capture.totalBytes},
		)
		logFunc(fmt.Sprintf("[%s]", prefix), attrs...)

		return nil
	})
}

// formatDuration returns the appropriate field name and value based on duration with improved precision
func formatDuration(d time.Duration) (string, float64) {
	microseconds := float64(d.Microseconds())
	milliseconds := float64(d.Milliseconds())
	seconds := d.Seconds()

	switch {
	case milliseconds < 10: // Use microseconds for better precision under 10ms
		return "duration_µs", microseconds
	case milliseconds >= 1000:
		return "duration_s", seconds
	default:
		return "duration_ms", milliseconds
	}
}

// createHandler is a helper that creates a handler with the appropriate logging function
func (hb *HandlerBuilder) createHandler(handlerFunc func(*calque.Request, *calque.Response, func(string, ...Attribute)) error) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		logFunc := func(msg string, attrs ...Attribute) {

			// context handling: prefer explicit context, then request context, then background
			var finalCtx context.Context

			switch {
			case hb.ctx != nil:
				finalCtx = hb.ctx // 1. Use explicit context (highest priority)
			case req.Context != nil:
				finalCtx = req.Context // 2. Use request context (fallback)
			default:
				finalCtx = context.Background() // 3. Use background context (last resort)
			}

			hb.printer.Print(finalCtx, msg, attrs...)
		}
		return handlerFunc(req, res, logFunc)
	})
}

// formatPreview creates a readable preview of data, handling both text and binary content
func formatPreview(data []byte) string {
	if len(data) == 0 {
		return "<empty>"
	}

	// Try to detect if printable text
	if isPrintable(data) {
		return string(data)
	}

	// For binary data, show hex summary
	if len(data) > 20 {
		return fmt.Sprintf("binary data (%d bytes): %x...", len(data), data[:20])
	}
	return fmt.Sprintf("binary data: %x", data)
}

// isPrintable checks if all bytes are printable unicode characters
func isPrintable(data []byte) bool {
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError {
			return false // Invalid UTF-8
		}
		if !unicode.IsPrint(r) && !isWhitespace(r) {
			return false
		}
		data = data[size:]
	}
	return true
}

func isWhitespace(r rune) bool {
	return r == '\t' || r == '\n' || r == '\r' || r == ' '
}
