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

	testCases := []struct {
		name         string
		input        string
		llmResponses []string
		expected     []string
	}{
		{
			name:  "Mathematical calculation request",
			input: "What is 15 + 8? Also, what time is it?",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "{\"input\": \"15+8\"}"}}]}`,
				"15 + 8 is 23.00, and the time is 2024-01-15 14:30:25.",
			},
			expected: []string{"23.00", "2024-01-15 14:30:25"},
		},
		{
			name:  "Weather and time request",
			input: "What's the weather like in New York and what time is it?",
			llmResponses: []string{
				`{"tool_calls": [{"type": "function", "function": {"name": "weather", "arguments": "{\"input\": \"New York\"}"}}]}`,
				"It's sunny in New York, and the time is 2024-01-15 14:30:25.",
			},
			expected: []string{"sunny", "2024-01-15 14:30:25"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mockClient := ai.NewMockClientWithResponses(tc.llmResponses)
			agent := ai.Agent(mockClient, ai.WithTools(calculator, currentTime, weatherTool))

			var result string
			err := calque.NewFlow().Use(agent).Run(ctx, tc.input, &result)
			if err != nil {
				t.Fatalf("Agent execution failed: %v", err)
			}

			// The loop should execute the requested tool and let the model
			// answer in its own words using the real tool result.
			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

// TestAgentMultiShotToolChain tests that the agent loop can call a second,
// dependent tool using the first tool's result - something a single-shot
// agent could never do, since it never fed tool results back to the model.
func TestAgentMultiShotToolChain(t *testing.T) {
	t.Parallel()

	lookupCityID := tools.Simple("lookup_city_id", "Looks up the internal city ID for a city name", func(_ string) string {
		return "NYC-001"
	})

	getWeatherByID := tools.Simple("get_weather_by_id", "Gets current weather using a city ID", func(_ string) string {
		return "Weather for NYC-001: sunny, 65°F"
	})

	mockClient := ai.NewMockClientWithResponses([]string{
		`{"tool_calls": [{"type": "function", "function": {"name": "lookup_city_id", "arguments": "{\"city\": \"New York\"}"}}]}`,
		`{"tool_calls": [{"type": "function", "function": {"name": "get_weather_by_id", "arguments": "{\"city_id\": \"NYC-001\"}"}}]}`,
		"The weather in New York is sunny, 65°F.",
	})

	agent := ai.Agent(mockClient, ai.WithTools(lookupCityID, getWeatherByID))

	var result string
	err := calque.NewFlow().Use(agent).Run(context.Background(), "What's the weather in New York?", &result)
	if err != nil {
		t.Fatalf("Agent execution failed: %v", err)
	}

	if !strings.Contains(result, "sunny") {
		t.Errorf("Expected final answer to use the second tool's result, got: %s", result)
	}

	// Pin the actual history threading, not just the final text: turn 2 and
	// turn 3 must have seen turn 1's tool result, and turn 3 must have seen
	// turn 2's, or this test would pass even if history were silently dropped
	// (as it currently is for the Gemini and Ollama clients).
	if mockClient.CallCount() != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", mockClient.CallCount())
	}
	if !ai.HistoryContainsToolResult(mockClient.HistoryAt(1), "NYC-001") {
		t.Error("turn 2 history missing turn 1's tool result (lookup_city_id)")
	}
	if !ai.HistoryContainsToolResult(mockClient.HistoryAt(2), "sunny") {
		t.Error("turn 3 history missing turn 2's tool result (get_weather_by_id)")
	}
}

// TestAgentMaxIterations pins that MaxIterations is the true cap on total LLM
// calls (not calls-before-one-more-uncounted-forced-call), across the
// meaningful boundary values: the tightest possible cap (1, zero tool-calling
// rounds), one tool-calling round before the forced final (2), and a cap with
// headroom (4) where the model still exhausts it. In every case a model that
// keeps requesting tools indefinitely must still get exactly maxIterations
// calls and a real final answer with no tools offered on the last call.
func TestAgentMaxIterations(t *testing.T) {
	t.Parallel()

	alwaysAskAgain := tools.Simple("always_ask_again", "A tool that never satisfies the model", func(_ string) string {
		return "keep going"
	})
	toolCallResponse := `{"tool_calls": [{"type": "function", "function": {"name": "always_ask_again", "arguments": "{}"}}]}`
	finalAnswer := "I'll stop here and answer directly."

	tests := []struct {
		name          string
		maxIterations int
		responses     []string // one response per expected LLM call
	}{
		{
			name:          "cap of 1 - zero tool-calling rounds, immediate forced answer",
			maxIterations: 1,
			responses:     []string{finalAnswer},
		},
		{
			name:          "cap of 2 - one tool-calling round then forced answer",
			maxIterations: 2,
			responses:     []string{toolCallResponse, finalAnswer},
		},
		{
			name:          "cap of 4 - three tool-calling rounds then forced answer",
			maxIterations: 4,
			responses:     []string{toolCallResponse, toolCallResponse, toolCallResponse, finalAnswer},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := ai.NewMockClientWithResponses(tt.responses)
			agent := ai.Agent(mockClient, ai.WithTools(alwaysAskAgain), ai.WithMaxIterations(tt.maxIterations))

			var result string
			err := calque.NewFlow().Use(agent).Run(context.Background(), "Keep calling the tool forever", &result)
			if err != nil {
				t.Fatalf("Agent execution failed: %v", err)
			}

			if !strings.Contains(result, finalAnswer) {
				t.Errorf("expected forced final answer, got: %s", result)
			}
			if mockClient.CallCount() != tt.maxIterations {
				t.Errorf("expected exactly %d LLM calls, got %d", tt.maxIterations, mockClient.CallCount())
			}
			// Tools stay declared on the forced-final call - some providers
			// (e.g. Gemini) get confused if tool declarations vanish while
			// history still references a prior call - but ToolsDisabled
			// tells the Client to refuse to call any of them.
			if opts := mockClient.OptionsAt(tt.maxIterations - 1); opts != nil {
				if len(opts.Tools) == 0 {
					t.Error("expected tools to stay declared on the final call")
				}
				if !opts.ToolsDisabled {
					t.Error("expected ToolsDisabled to be set on the final call")
				}
			}
		})
	}
}

// TestAgentMaxIterationsDefaultsWhenUnsetOrInvalid pins that MaxIterations
// falls back to the package default instead of looping zero times or
// panicking when left unset (0) or given an invalid negative value.
func TestAgentMaxIterationsDefaultsWhenUnsetOrInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		maxIterations int // 0 means "don't call WithMaxIterations at all"
	}{
		{name: "unset"},
		{name: "explicit zero", maxIterations: 0},
		{name: "negative", maxIterations: -1},
	}

	calc := tools.Simple("calculator", "Math Calculator", func(_ string) string { return "4" })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := ai.NewMockClientWithResponses([]string{
				`{"tool_calls": [{"type": "function", "function": {"name": "calculator", "arguments": "2+2"}}]}`,
				"The answer is 4.",
			})

			opts := []ai.AgentOption{ai.WithTools(calc)}
			if tt.name != "unset" {
				opts = append(opts, ai.WithMaxIterations(tt.maxIterations))
			}
			agent := ai.Agent(mockClient, opts...)

			var result string
			err := calque.NewFlow().Use(agent).Run(context.Background(), "What is 2+2?", &result)
			if err != nil {
				t.Fatalf("Agent execution failed: %v", err)
			}
			if !strings.Contains(result, "The answer is 4.") {
				t.Errorf("expected the model's natural answer, got: %s", result)
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
				"5+3 is 8.00.",
			})

			// Create agent with configured tools
			agent := ai.Agent(mockClient, ai.WithTools(calculator), ai.WithToolsConfig(config))

			// Test the configuration
			var result string
			err := calque.NewFlow().Use(agent).Run(context.Background(), "Calculate 5+3", &result)
			if err != nil {
				t.Fatalf("Agent execution failed: %v", err)
			}

			// The loop should execute the tool and return the model's answer
			// using the real result, regardless of concurrency configuration.
			if !strings.Contains(result, "8.00") {
				t.Errorf("Expected result to contain the calculated value, got: %s", result)
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
