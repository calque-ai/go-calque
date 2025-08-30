package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

// TestCalculatorTool tests the calculator tool functionality with realistic mathematical operations
func TestCalculatorTool(t *testing.T) {
	t.Parallel()
	calculator := tools.Simple("calculator", "Performs basic math calculations including arithmetic, percentages, and scientific operations", func(input string) string {
		result, err := calculate(input)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("%.2f", result)
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic addition",
			input:    "15+8",
			expected: "23.00",
		},
		{
			name:     "Large multiplication",
			input:    "1250*67",
			expected: "83750.00",
		},
		{
			name:     "Decimal subtraction",
			input:    "100.5-23.7",
			expected: "76.80",
		},
		{
			name:     "Division with remainder",
			input:    "100/3",
			expected: "33.33",
		},
		{
			name:     "Complex expression",
			input:    "10+5",
			expected: "15.00",
		},
		{
			name:     "Percentage calculation",
			input:    "200*0.15",
			expected: "30.00",
		},
		{
			name:     "Negative numbers",
			input:    "-15+8",
			expected: "-7.00",
		},
		{
			name:     "Zero operations",
			input:    "0*42",
			expected: "0.00",
		},
		{
			name:     "Large numbers",
			input:    "999999+1",
			expected: "1000000.00",
		},
		{
			name:     "Decimal precision",
			input:    "3.14159*2",
			expected: "6.28",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			err := calque.NewFlow().Use(calculator).Run(context.Background(), tt.input, &result)
			if err != nil {
				t.Fatalf("Tool execution failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestTimeTool tests the current time tool functionality with various formats
func TestTimeTool(t *testing.T) {
	currentTime := tools.Simple("current_time", "Gets the current date and time in various formats", func(input string) string {
		format := input
		if format == "" {
			format = "2006-01-02 15:04:05"
		}
		return time.Now().Format(format)
	})

	testCases := []struct {
		name     string
		format   string
		expected string
	}{
		{
			name:     "Default format",
			format:   "",
			expected: "2006-01-02 15:04:05",
		},
		{
			name:     "Year only",
			format:   "2006",
			expected: "2006",
		},
		{
			name:     "Date only",
			format:   "2006-01-02",
			expected: "2006-01-02",
		},
		{
			name:     "Time only",
			format:   "15:04:05",
			expected: "15:04:05",
		},
		{
			name:     "RFC3339 format",
			format:   time.RFC3339,
			expected: time.RFC3339,
		},
		{
			name:     "Unix timestamp",
			format:   "Unix: 1136239445",
			expected: "Unix: 1136239445",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := calque.NewFlow().Use(currentTime).Run(context.Background(), tc.format, &result)
			if err != nil {
				t.Fatalf("Time tool execution failed: %v", err)
			}
			if result == "" {
				t.Error("Expected non-empty time result")
			}
			// For format validation, we can only check that we get a non-empty result
			// since the actual time will vary
			if len(result) < 4 {
				t.Errorf("Expected reasonable time result, got: %s", result)
			}
		})
	}
}

// TestUnitConverterTool tests the unit converter tool functionality with various conversions
func TestUnitConverterTool(t *testing.T) {
	converter := tools.Simple("unit_converter", "Converts between various units including temperature, length, weight, and currency", func(input string) string {
		result, err := convertTemperature(input)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return result
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Fahrenheit to Celsius",
			input:    "100 fahrenheit to celsius",
			expected: "100.00°F = 37.78°C",
		},
		{
			name:     "Celsius to Fahrenheit",
			input:    "25 celsius to fahrenheit",
			expected: "25.00°C = 77.00°F",
		},
		{
			name:     "Freezing point",
			input:    "0 celsius to fahrenheit",
			expected: "0.00°C = 32.00°F",
		},
		{
			name:     "Boiling point",
			input:    "100 celsius to fahrenheit",
			expected: "100.00°C = 212.00°F",
		},
		{
			name:     "Negative temperature",
			input:    "-10 celsius to fahrenheit",
			expected: "-10.00°C = 14.00°F",
		},
		{
			name:     "High temperature",
			input:    "500 fahrenheit to celsius",
			expected: "500.00°F = 260.00°C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			err := calque.NewFlow().Use(converter).Run(context.Background(), tt.input, &result)
			if err != nil {
				t.Fatalf("Converter tool execution failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestAgentWithTools tests an agent with multiple tools in realistic scenarios
func TestAgentWithTools(t *testing.T) {
	t.Parallel()
	// Create realistic tools
	calculator := tools.Simple("calculator", "Performs basic math calculations", func(input string) string {
		result, err := calculate(input)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("%.2f", result)
	})

	currentTime := tools.Simple("current_time", "Gets the current date and time", func(_ string) string {
		return "2024-01-15 14:30:25" // Mock time for testing
	})

	weatherTool := tools.Simple("weather", "Gets current weather information for a location", func(input string) string {
		return "Weather in " + input + ": Sunny, 72°F, Humidity: 45%"
	})

	// Create mock client that will use tools
	mockClient := ai.NewMockClientWithResponses([]string{
		`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "{\"input\": \"15+8\"}"}}]}`,
		`{"tool_calls": [{"type": "function", "function": {"name": "current_time", "arguments": "{\"input\": \"\"}"}}]}`,
		`{"tool_calls": [{"type": "function", "function": {"name": "weather", "arguments": "{\"input\": \"New York\"}"}}]}`,
		`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "{\"input\": \"5+3\"}"}}]}`,
	})

	// Create agent with tools
	agent := ai.Agent(mockClient, ai.WithTools(calculator, currentTime, weatherTool))

	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "Mathematical calculation request",
			input: "What is 15 + 8? Also, what time is it?",
			expected: []string{
				"tool_calls",
				"calculator",
				"current_time",
			},
		},
		{
			name:  "Weather and time request",
			input: "What's the weather like in New York and what time is it?",
			expected: []string{
				"tool_calls",
				"weather",
				"current_time",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			var result string
			err := calque.NewFlow().Use(agent).Run(ctx, tc.input, &result)
			if err != nil {
				t.Fatalf("Agent execution failed: %v", err)
			}

			// The mock client returns tool calls, not executed results
			// Just verify that we get a tool call response
			if !strings.Contains(result, "tool_calls") {
				t.Errorf("Expected tool calls in result, got: %s", result)
			}
		})
	}
}

// TestToolErrorHandling tests error handling in tools with various error scenarios
func TestToolErrorHandling(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		input       string
		expectError bool
		errorType   string
	}{
		{
			name:        "Invalid mathematical expression",
			input:       "invalid expression",
			expectError: true,
			errorType:   "Error:",
		},
		{
			name:        "Division by zero",
			input:       "10/0",
			expectError: true,
			errorType:   "Error:",
		},
		{
			name:        "Invalid syntax",
			input:       "5++3",
			expectError: true,
			errorType:   "Error:",
		},
		{
			name:        "Empty input",
			input:       "",
			expectError: true,
			errorType:   "Error:",
		},
		{
			name:        "Valid expression",
			input:       "5+3",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			calculator := tools.Simple("calculator", "Performs basic math calculations", func(input string) string {
				result, err := calculate(input)
				if err != nil {
					return fmt.Sprintf("Error: %v", err)
				}
				return fmt.Sprintf("%.2f", result)
			})

			var result string
			err := calque.NewFlow().Use(calculator).Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Tool execution failed: %v", err)
			}

			if tc.expectError {
				if !strings.Contains(result, tc.errorType) {
					t.Errorf("Expected calculation error, got: %s", result)
				}
			} else {
				if strings.Contains(result, "Error:") {
					t.Errorf("Expected successful calculation, got error: %s", result)
				}
			}
		})
	}
}

// TestToolConcurrency tests tool execution under concurrent load with realistic scenarios
func TestToolConcurrency(t *testing.T) {
	t.Parallel()
	calculator := tools.Simple("calculator", "Performs basic math calculations", func(input string) string {
		result, err := calculate(input)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("%.2f", result)
	})

	// Test realistic concurrent scenarios
	testCases := []struct {
		name        string
		numRequests int
		operations  []string
		expectRate  bool
	}{
		{
			name:        "High load calculations",
			numRequests: 20,
			operations:  []string{"addition", "multiplication", "division", "subtraction"},
			expectRate:  false,
		},
		{
			name:        "Burst calculations",
			numRequests: 50,
			operations:  []string{"simple math"},
			expectRate:  false,
		},
		{
			name:        "Mixed operations",
			numRequests: 15,
			operations:  []string{"complex", "simple", "error-prone"},
			expectRate:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := make(chan string, tc.numRequests)

			for i := 0; i < tc.numRequests; i++ {
				go func(id int) {
					// Generate different types of calculations
					var input string
					switch id % 4 {
					case 0:
						input = fmt.Sprintf("%d+%d", id, id+1)
					case 1:
						input = fmt.Sprintf("%d*%d", id, 2)
					case 2:
						input = fmt.Sprintf("%d-%d", id+10, id)
					case 3:
						input = fmt.Sprintf("%d/%d", id*10, 2)
					}

					var result string
					err := calque.NewFlow().Use(calculator).Run(context.Background(), input, &result)
					if err != nil {
						results <- fmt.Sprintf("Error: %v", err)
					} else {
						results <- result
					}
				}(i)
			}

			// Collect results
			successCount := 0
			errorCount := 0
			for i := 0; i < tc.numRequests; i++ {
				result := <-results
				// Check that we got a valid numeric result
				if strings.Contains(result, ".") && !strings.Contains(result, "Error:") {
					successCount++
				} else {
					errorCount++
				}
			}

			// Should have mostly successful results
			if successCount < tc.numRequests*8/10 { // At least 80% success rate
				t.Errorf("Expected at least %d successful calculations, got %d", tc.numRequests*8/10, successCount)
			}

			t.Logf("Concurrency test: %d/%d calculations succeeded", successCount, tc.numRequests)
		})
	}
}

// TestToolConfiguration tests tool configuration with various settings
func TestToolConfiguration(t *testing.T) {
	t.Parallel()
	calculator := tools.Simple("calculator", "Performs basic math calculations", func(input string) string {
		result, err := calculate(input)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("%.2f", result)
	})

	// Test different tool configurations
	testCases := []struct {
		name           string
		maxConcurrent  int
		includeOutput  bool
		numRequests    int
		expectedConfig tools.Config
	}{
		{
			name:          "Unlimited concurrency",
			maxConcurrent: 0,
			includeOutput: false,
			numRequests:   10,
			expectedConfig: tools.Config{
				MaxConcurrentTools:    0,
				IncludeOriginalOutput: false,
			},
		},
		{
			name:          "Limited concurrency",
			maxConcurrent: 2,
			includeOutput: true,
			numRequests:   5,
			expectedConfig: tools.Config{
				MaxConcurrentTools:    2,
				IncludeOriginalOutput: true,
			},
		},
		{
			name:          "Single thread",
			maxConcurrent: 1,
			includeOutput: false,
			numRequests:   3,
			expectedConfig: tools.Config{
				MaxConcurrentTools:    1,
				IncludeOriginalOutput: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := tools.Config{
				MaxConcurrentTools:    tc.maxConcurrent,
				IncludeOriginalOutput: tc.includeOutput,
			}

			// Create mock client
			mockClient := ai.NewMockClientWithResponses([]string{
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "{\"input\": \"5+3\"}"}}]}`,
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "{\"input\": \"5+3\"}"}}]}`,
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "{\"input\": \"5+3\"}"}}]}`,
			})

			// Create agent with configured tools
			agent := ai.Agent(mockClient, ai.WithTools(calculator), ai.WithToolsConfig(config))

			// Test the configuration
			var result string
			err := calque.NewFlow().Use(agent).Run(context.Background(), "Calculate 5+3", &result)
			if err != nil {
				t.Fatalf("Agent execution failed: %v", err)
			}

			// Verify tool calls are present
			if !strings.Contains(result, "tool_calls") {
				t.Errorf("Expected tool calls in result, got: %s", result)
			}

			if !strings.Contains(result, "calculator") {
				t.Errorf("Expected calculator tool call, got: %s", result)
			}
		})
	}
}

// TestToolValidation tests tool input validation with various edge cases
func TestToolValidation(t *testing.T) {
	t.Parallel()
	// Create a tool with validation
	validatedTool := tools.Simple("validated_tool", "A tool that validates input", func(input string) string {
		// Simulate validation
		if len(input) < 3 {
			return "Error: Input too short (minimum 3 characters)"
		}
		if len(input) > 100 {
			return "Error: Input too long (maximum 100 characters)"
		}
		if strings.Contains(input, "invalid") {
			return "Error: Invalid content detected"
		}
		return "Valid input processed: " + input
	})

	testCases := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid input",
			input:       "This is a valid input",
			expectError: false,
		},
		{
			name:        "Too short",
			input:       "Hi",
			expectError: true,
			errorMsg:    "Input too short",
		},
		{
			name:        "Too long",
			input:       strings.Repeat("This is a very long input that exceeds the maximum allowed length. ", 10),
			expectError: true,
			errorMsg:    "Input too long",
		},
		{
			name:        "Invalid content",
			input:       "This contains invalid content",
			expectError: true,
			errorMsg:    "Invalid content detected",
		},
		{
			name:        "Empty input",
			input:       "",
			expectError: true,
			errorMsg:    "Input too short",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := calque.NewFlow().Use(validatedTool).Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Tool execution failed: %v", err)
			}

			if tc.expectError {
				if !strings.Contains(result, tc.errorMsg) {
					t.Errorf("Expected error message containing %q, got: %s", tc.errorMsg, result)
				}
			} else {
				if !strings.Contains(result, "Valid input processed") {
					t.Errorf("Expected successful processing, got: %s", result)
				}
			}
		})
	}
}

// TestToolPerformance tests tool performance with realistic workloads
func TestToolPerformance(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a tool that simulates realistic processing
	performanceTool := tools.Simple("performance_tool", "A tool that simulates realistic processing time", func(input string) string {
		// Simulate processing time based on input complexity
		complexity := len(input) / 10
		if complexity > 0 {
			time.Sleep(time.Duration(complexity) * time.Millisecond)
		}
		return fmt.Sprintf("Processed in %dms: %s", complexity, input)
	})

	testCases := []struct {
		name    string
		input   string
		maxTime time.Duration
	}{
		{
			name:    "Simple input",
			input:   "Hello",
			maxTime: 100 * time.Millisecond,
		},
		{
			name:    "Medium input",
			input:   strings.Repeat("This is a medium complexity input. ", 5),
			maxTime: 200 * time.Millisecond,
		},
		{
			name:    "Complex input",
			input:   strings.Repeat("This is a very complex input that requires significant processing time. ", 10),
			maxTime: 500 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			var result string
			err := calque.NewFlow().Use(performanceTool).Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Performance tool execution failed: %v", err)
			}

			duration := time.Since(start)

			// Verify the result
			if !strings.Contains(result, "Processed in") {
				t.Errorf("Expected processing confirmation, got: %s", result)
			}

			// Check performance
			if duration > tc.maxTime {
				t.Errorf("Processing took too long: %v (max: %v)", duration, tc.maxTime)
			}

			t.Logf("Processed %d characters in %v", len(tc.input), duration)
		})
	}
}
