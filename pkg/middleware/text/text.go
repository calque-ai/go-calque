package text

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/calque-ai/calque-pipe/pkg/calque"
)

// Transform applies a function to transform the entire input content.
//
// Input: string content (buffered - reads entire input into memory)
// Output: string (result of transformation function)
// Behavior: BUFFERED - must read entire input to apply transformation
//
// The transformation function receives the entire input as a string and
// returns the transformed string. Useful for text processing, formatting,
// or content modification that requires the complete input.
//
// Example:
//
//	upperCase := strings.Transform(strings.ToUpper)
//	reverse := strings.Transform(func(s string) string {
//	  runes := []rune(s)
//	  for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
//	    runes[i], runes[j] = runes[j], runes[i]
//	  }
//	  return string(runes)
//	})
func Transform(fn func(string) string) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		input, err := io.ReadAll(req.Data)
		if err != nil {
			return err
		}

		output := fn(string(input))
		_, err = res.Data.Write([]byte(output))
		return err
	})
}

// Branch creates conditional routing based on input content evaluation.
//
// Input: string content (buffered - reads entire input into memory)
// Output: depends on which handler is executed (ifHandler or elseHandler)
// Behavior: BUFFERED - must read entire input to evaluate condition
//
// The condition function receives the entire input as a string and returns a boolean.
// If true, ifHandler is executed; if false, elseHandler is executed.
// Both handlers receive the same original input.
//
// Example:
//
//	jsonBranch := strings.Branch(
//	  func(s string) bool { return strings.HasPrefix(s, "{") },
//	  jsonHandler,
//	  textHandler,
//	)
func Branch(condition func(string) bool, ifHandler calque.Handler, elseHandler calque.Handler) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		err := calque.Read(req, &input)
		if err != nil {
			return err
		}

		req.Data = strings.NewReader(input)

		if condition(string(input)) {
			return ifHandler.ServeFlow(req, res)
		}
		return elseHandler.ServeFlow(req, res)
	})
}

// Filter conditionally processes input based on content evaluation.
//
// Input: string content (buffered - reads entire input into memory)
// Output: depends on condition - either handler output or original input
// Behavior: BUFFERED - must read entire input to evaluate condition
//
// If the condition function returns true, the input is processed by the handler.
// If false, the original input passes through unchanged. The condition function
// receives the entire input as a string.
//
// Example:
//
//	jsonFilter := strings.Filter(
//	  func(s string) bool { return json.Valid([]byte(s)) },
//	  jsonProcessor,
//	)
//	// Only valid JSON gets processed, everything else passes through
func Filter(condition func(string) bool, handler calque.Handler) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		err := calque.Read(req, &input)
		if err != nil {
			return err
		}

		if condition(input) {
			req.Data = strings.NewReader(input)
			return handler.ServeFlow(req, res)
		}

		// Pass through unchanged
		err = calque.Write(res, input)
		return err
	})
}

// LineProcessor transforms input line-by-line using buffered scanning.
//
// Input: string content (streaming - uses bufio.Scanner for line-by-line processing)
// Output: string (processed lines separated by newlines)
// Behavior: STREAMING - processes each line as it's read, memory efficient
//
// Reads input line by line and applies the transformation function to each line.
// Output lines are written immediately, making this memory efficient for large
// inputs. Each output line ends with a newline character.
//
// Example:
//
//	addLineNumbers := strings.LineProcessor(func(line string) string {
//	  return fmt.Sprintf("%d: %s", lineNum, line)
//	})
//	csvProcessor := strings.LineProcessor(func(line string) string {
//	  return strings.ToUpper(line) // Convert CSV to uppercase
//	})
func LineProcessor(fn func(string) string) calque.Handler {
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		scanner := bufio.NewScanner(req.Data)

		for scanner.Scan() {
			line := scanner.Text()
			processed := fn(line)
			if _, err := fmt.Fprintln(res.Data, processed); err != nil {
				return err
			}
		}

		return scanner.Err()
	})
}
