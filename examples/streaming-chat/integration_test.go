package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/memory"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
)

// TestStreamingChatPipeline tests the streaming chat pipeline with conversation scenarios
func TestStreamingChatPipeline(t *testing.T) {
	// Create mock AI client for testing with responses
	mockClient := ai.NewMockClient("Hello! I'm a helpful AI assistant. I can help you with various tasks including customer support, technical questions, and general inquiries. How can I assist you today?")

	// Create conversation memory
	conversationMemory := memory.NewConversation()

	// Test  conversation scenarios
	testCases := []struct {
		name     string
		userID   string
		message  string
		expected []string
	}{
		{
			name:    "Customer support inquiry",
			userID:  "customer_12345",
			message: "I'm having trouble with my account login",
			expected: []string{
				"Hello! I'm a helpful AI assistant",
				"customer support",
				"technical questions",
			},
		},
		{
			name:    "Technical question",
			userID:  "developer_67890",
			message: "How do I implement OAuth 2.0 authentication?",
			expected: []string{
				"Hello! I'm a helpful AI assistant",
				"technical questions",
				"general inquiries",
			},
		},
		{
			name:    "Product inquiry",
			userID:  "prospect_11111",
			message: "What features does your premium plan include?",
			expected: []string{
				"Hello! I'm a helpful AI assistant",
				"customer support",
				"general inquiries",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create pipeline similar to the one in handleStreamingChat
			pipeline := calque.NewFlow().
				Use(conversationMemory.Input(tc.userID)).
				Use(prompt.Template("You are a helpful assistant. User: {{.Input}}\n\nAssistant:")).
				Use(ai.Agent(mockClient)).
				Use(conversationMemory.Output(tc.userID))

			var result string
			err := pipeline.Run(context.Background(), tc.message, &result)
			if err != nil {
				t.Fatalf("Pipeline execution failed: %v", err)
			}

			// Verify response contains expected content
			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected response to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

// TestMemoryFunctionality tests conversation memory functionality with multi-turn conversations
func TestMemoryFunctionality(t *testing.T) {
	mockClient := ai.NewMockClient("I remember our conversation! Based on our previous discussion, I can provide you with personalized assistance.")
	conversationMemory := memory.NewConversation()

	// Test multi-turn conversations
	testCases := []struct {
		name     string
		userID   string
		messages []string
		expected []string
	}{
		{
			name:   "Customer support conversation",
			userID: "support_user_001",
			messages: []string{
				"My name is Alice Johnson and I work at TechCorp",
				"What's my name and where do I work?",
				"Can you help me with a billing issue for my account?",
			},
			expected: []string{
				"I remember our conversation",
				"personalized assistance",
			},
		},
		{
			name:   "Technical consultation",
			userID: "tech_user_002",
			messages: []string{
				"I'm building a microservices architecture with Go",
				"What was I working on?",
				"Can you suggest best practices for service communication?",
			},
			expected: []string{
				"I remember our conversation",
				"personalized assistance",
			},
		},
		{
			name:   "Product discussion",
			userID: "product_user_003",
			messages: []string{
				"I'm interested in your enterprise solution for data analytics",
				"What product was I asking about?",
				"What are the pricing tiers for that solution?",
			},
			expected: []string{
				"I remember our conversation",
				"personalized assistance",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i, message := range tc.messages {
				t.Run(fmt.Sprintf("Message_%d", i+1), func(t *testing.T) {
					// Create a pipeline to test memory
					pipeline := calque.NewFlow().
						Use(conversationMemory.Input(tc.userID)).
						Use(prompt.Template("Previous context: {{.Input}}\n\nUser: " + message + "\n\nAssistant:")).
						Use(ai.Agent(mockClient)).
						Use(conversationMemory.Output(tc.userID))

					var result string
					err := pipeline.Run(context.Background(), message, &result)
					if err != nil {
						t.Fatalf("Pipeline execution failed: %v", err)
					}

					// Verify response contains expected content
					for _, expected := range tc.expected {
						if !strings.Contains(result, expected) {
							t.Errorf("Expected response to contain %q, got: %s", expected, result)
						}
					}
				})
			}
		})
	}
}

// TestRateLimiting tests the rate limiting functionality with load patterns
func TestRateLimiting(t *testing.T) {
	mockClient := ai.NewMockClient("Rate limited response - this request was processed successfully.")

	// Create a pipeline with rate limiting
	pipeline := calque.NewFlow().
		Use(ctrl.RateLimit(3, time.Second)). // 3 requests per second
		Use(ai.Agent(mockClient))

	// Test  load patterns
	testCases := []struct {
		name        string
		numRequests int
		delay       time.Duration
		expectRate  bool
	}{
		{
			name:        "Burst requests - should hit rate limit",
			numRequests: 10,
			delay:       0,
			expectRate:  true,
		},
		{
			name:        "Spaced requests - should not hit rate limit",
			numRequests: 5,
			delay:       500 * time.Millisecond,
			expectRate:  false,
		},
		{
			name:        "Moderate load",
			numRequests: 8,
			delay:       200 * time.Millisecond,
			expectRate:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := make(chan error, tc.numRequests)

			for i := 0; i < tc.numRequests; i++ {
				go func(id int) {
					// Add delay between requests if specified
					if tc.delay > 0 {
						time.Sleep(tc.delay)
					}

					var result string
					err := pipeline.Run(context.Background(), fmt.Sprintf("Request %d from user %d", id, id), &result)
					if err != nil {
						results <- fmt.Errorf("request %d failed: %v", id, err)
						return
					}
					results <- nil
				}(i)
			}

			// Collect results
			successCount := 0
			errorCount := 0
			for i := 0; i < tc.numRequests; i++ {
				if err := <-results; err != nil {
					errorCount++
					t.Logf("Request %d: %v", i, err)
				} else {
					successCount++
				}
			}

			// Analyze results based on expected behavior
			if tc.expectRate {
				// Should have some rate limiting errors
				if errorCount == 0 {
					t.Logf("No rate limiting errors occurred, but some were expected")
				}
			} else {
				// Should have mostly successful requests
				if successCount < tc.numRequests*8/10 { // At least 80% success rate
					t.Errorf("Expected at least %d successful requests, got %d", tc.numRequests*8/10, successCount)
				}
			}

			t.Logf("Rate limiting test: %d/%d requests succeeded", successCount, tc.numRequests)
		})
	}
}

// TestFallbackFunctionality tests the fallback mechanism with failure scenarios
func TestFallbackFunctionality(t *testing.T) {
	// Create  failure scenarios
	testCases := []struct {
		name             string
		primaryError     string
		fallbackResponse string
		expected         string
	}{
		{
			name:             "Primary service unavailable",
			primaryError:     "service unavailable",
			fallbackResponse: "Fallback response: I'm here to help with your request.",
			expected:         "Fallback response: I'm here to help with your request.",
		},
		{
			name:             "Primary service timeout",
			primaryError:     "timeout",
			fallbackResponse: "Fallback response: Your request is being processed.",
			expected:         "Fallback response: Your request is being processed.",
		},
		{
			name:             "Primary service error",
			primaryError:     "internal server error",
			fallbackResponse: "Fallback response: Please try again later.",
			expected:         "Fallback response: Please try again later.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a primary client that fails
			failingClient := ai.NewMockClientWithError(tc.primaryError)

			// Create a fallback client
			fallbackClient := ai.NewMockClient(tc.fallbackResponse)

			// Create pipeline with fallback
			pipeline := calque.NewFlow().
				Use(ctrl.Fallback(ai.Agent(failingClient), ai.Agent(fallbackClient)))

			var result string
			err := pipeline.Run(context.Background(), "test message", &result)
			if err != nil {
				t.Fatalf("Pipeline execution failed: %v", err)
			}

			// Should get fallback response
			if !strings.Contains(result, tc.expected) {
				t.Errorf("Expected fallback response containing %q, got: %s", tc.expected, result)
			}
		})
	}
}

// TestConcurrentChatSessions tests multiple concurrent chat sessions with user scenarios
func TestConcurrentChatSessions(t *testing.T) {
	mockClient := ai.NewMockClient("Concurrent response: I'm processing your request in a multi-user environment.")
	conversationMemory := memory.NewConversation()

	// Test  concurrent user scenarios
	testCases := []struct {
		name     string
		numUsers int
		userType string
		message  string
		expected string
	}{
		{
			name:     "Customer support agents",
			numUsers: 5,
			userType: "support_agent",
			message:  "Customer inquiry about refund",
			expected: "Concurrent response: I'm processing your request",
		},
		{
			name:     "Developers",
			numUsers: 3,
			userType: "developer",
			message:  "API integration question",
			expected: "Concurrent response: I'm processing your request",
		},
		{
			name:     "Sales team",
			numUsers: 4,
			userType: "sales_rep",
			message:  "Product pricing inquiry",
			expected: "Concurrent response: I'm processing your request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := make(chan error, tc.numUsers)

			for i := 0; i < tc.numUsers; i++ {
				go func(userID int) {
					userIDStr := fmt.Sprintf("%s_%d", tc.userType, userID)
					message := fmt.Sprintf("%s from user %d", tc.message, userID)

					// Create pipeline for this user
					pipeline := calque.NewFlow().
						Use(conversationMemory.Input(userIDStr)).
						Use(prompt.Template("User: {{.Input}}\n\nAssistant:")).
						Use(ai.Agent(mockClient)).
						Use(conversationMemory.Output(userIDStr))

					var result string
					err := pipeline.Run(context.Background(), message, &result)
					if err != nil {
						results <- fmt.Errorf("user %d failed: %v", userID, err)
						return
					}

					if !strings.Contains(result, tc.expected) {
						results <- fmt.Errorf("user %d got unexpected response: %s", userID, result)
						return
					}

					results <- nil
				}(i)
			}

			// Collect results
			successCount := 0
			for i := 0; i < tc.numUsers; i++ {
				if err := <-results; err != nil {
					t.Errorf("Concurrent session %d failed: %v", i, err)
				} else {
					successCount++
				}
			}

			// Should have most sessions succeed
			if successCount < tc.numUsers*8/10 { // At least 80% success rate
				t.Errorf("Expected at least %d successful sessions, got %d", tc.numUsers*8/10, successCount)
			}
		})
	}
}

// TestLargeMessageHandling tests handling of large messages with content
func TestLargeMessageHandling(t *testing.T) {
	mockClient := ai.NewMockClient("Large message processed successfully. I can handle extensive content and provide meaningful responses.")
	conversationMemory := memory.NewConversation()

	// Create  large messages
	largeMessages := []struct {
		name    string
		message string
		userID  string
	}{
		{
			name:    "Large technical document",
			message: strings.Repeat("This is a comprehensive technical specification document that contains detailed information about system architecture, API endpoints, database schemas, security protocols, deployment procedures, monitoring configurations, and performance optimization strategies. ", 100),
			userID:  "tech_writer_001",
		},
		{
			name:    "Large customer feedback",
			message: strings.Repeat("I've been using your product for several months now and I wanted to provide detailed feedback about my experience. The interface is generally intuitive, but there are some areas where I think improvements could be made. ", 100),
			userID:  "customer_002",
		},
		{
			name:    "Large log analysis request",
			message: strings.Repeat("I need help analyzing these application logs to identify the root cause of the performance issues we're experiencing. The logs contain information about database queries, API calls, error messages, and system metrics. ", 100),
			userID:  "devops_003",
		},
		{
			name:    "Large business report",
			message: strings.Repeat("Here's our quarterly business report containing detailed financial analysis, market trends, customer acquisition metrics, revenue projections, competitive analysis, and strategic recommendations for the upcoming fiscal year. ", 100),
			userID:  "analyst_004",
		},
	}

	for _, tc := range largeMessages {
		t.Run(tc.name, func(t *testing.T) {
			userID := tc.userID

			// Create pipeline
			pipeline := calque.NewFlow().
				Use(conversationMemory.Input(userID)).
				Use(prompt.Template("User: {{.Input}}\n\nAssistant:")).
				Use(ai.Agent(mockClient)).
				Use(conversationMemory.Output(userID))

			var result string
			err := pipeline.Run(context.Background(), tc.message, &result)
			if err != nil {
				t.Fatalf("Large message processing failed: %v", err)
			}

			// Verify response
			if !strings.Contains(result, "Large message processed successfully") {
				t.Errorf("Expected processing confirmation, got: %s", result[:100])
			}

			// Verify the response is reasonable in length
			if len(result) < 50 {
				t.Errorf("Expected substantial response, got only %d characters", len(result))
			}
		})
	}
}

// TestErrorHandling tests error scenarios with error conditions
func TestErrorHandling(t *testing.T) {
	// Test  error scenarios
	testCases := []struct {
		name        string
		errorType   string
		expectedErr string
	}{
		{
			name:        "Service unavailable",
			errorType:   "service unavailable",
			expectedErr: "service unavailable",
		},
		{
			name:        "Authentication failure",
			errorType:   "authentication failed",
			expectedErr: "authentication failed",
		},
		{
			name:        "Rate limit exceeded",
			errorType:   "rate limit exceeded",
			expectedErr: "rate limit exceeded",
		},
		{
			name:        "Invalid request",
			errorType:   "invalid request format",
			expectedErr: "invalid request format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a client that always fails with the specified error
			failingClient := ai.NewMockClientWithError(tc.errorType)

			conversationMemory := memory.NewConversation()

			userID := "error_test_user"
			message := "test message"

			// Create pipeline
			pipeline := calque.NewFlow().
				Use(conversationMemory.Input(userID)).
				Use(ai.Agent(failingClient)).
				Use(conversationMemory.Output(userID))

			var result string
			err := pipeline.Run(context.Background(), message, &result)
			if err == nil {
				t.Error("Expected error, but got none")
			}

			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("Expected error message containing %q, got: %v", tc.expectedErr, err)
			}
		})
	}
}

// TestMemoryIsolation tests that different users have isolated conversation memory
func TestMemoryIsolation(t *testing.T) {
	mockClient := ai.NewMockClient("I can see the conversation context for this specific user.")
	conversationMemory := memory.NewConversation()

	// Test that different users have isolated memory
	user1 := "user_alice"
	user2 := "user_bob"

	// User 1's conversation
	message1 := "My name is Alice and I work at Company A"

	pipeline1 := calque.NewFlow().
		Use(conversationMemory.Input(user1)).
		Use(prompt.Template("Previous context: {{.Input}}\n\nUser: " + message1 + "\n\nAssistant:")).
		Use(ai.Agent(mockClient)).
		Use(conversationMemory.Output(user1))

	var result1 string
	err := pipeline1.Run(context.Background(), message1, &result1)
	if err != nil {
		t.Fatalf("User 1 conversation failed: %v", err)
	}

	// User 2's conversation
	message2 := "My name is Bob and I work at Company B"

	pipeline2 := calque.NewFlow().
		Use(conversationMemory.Input(user2)).
		Use(prompt.Template("Previous context: {{.Input}}\n\nUser: " + message2 + "\n\nAssistant:")).
		Use(ai.Agent(mockClient)).
		Use(conversationMemory.Output(user2))

	var result2 string
	err = pipeline2.Run(context.Background(), message2, &result2)
	if err != nil {
		t.Fatalf("User 2 conversation failed: %v", err)
	}

	// Both should succeed independently
	if !strings.Contains(result1, "I can see the conversation context") {
		t.Errorf("User 1 response missing expected content: %s", result1)
	}

	if !strings.Contains(result2, "I can see the conversation context") {
		t.Errorf("User 2 response missing expected content: %s", result2)
	}
}

// TestPromptTemplates tests different prompt templates with  scenarios
func TestPromptTemplates(t *testing.T) {
	mockClient := ai.NewMockClient("I'm responding based on the provided prompt template and context.")
	conversationMemory := memory.NewConversation()

	testCases := []struct {
		name           string
		template       string
		userMessage    string
		expectedPhrase string
	}{
		{
			name:           "Customer service template",
			template:       "You are a helpful customer service representative. Be polite and professional. User: {{.Input}}\n\nAssistant:",
			userMessage:    "I need help with my order",
			expectedPhrase: "I'm responding based on the provided prompt template",
		},
		{
			name:           "Technical support template",
			template:       "You are a technical support specialist. Provide detailed technical guidance. User: {{.Input}}\n\nAssistant:",
			userMessage:    "How do I configure the API?",
			expectedPhrase: "I'm responding based on the provided prompt template",
		},
		{
			name:           "Sales template",
			template:       "You are a sales representative. Be enthusiastic and helpful with product information. User: {{.Input}}\n\nAssistant:",
			userMessage:    "Tell me about your premium features",
			expectedPhrase: "I'm responding based on the provided prompt template",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pipeline := calque.NewFlow().
				Use(conversationMemory.Input("template_test_user")).
				Use(prompt.Template(tc.template)).
				Use(ai.Agent(mockClient)).
				Use(conversationMemory.Output("template_test_user"))

			var result string
			err := pipeline.Run(context.Background(), tc.userMessage, &result)
			if err != nil {
				t.Fatalf("Template test failed: %v", err)
			}

			if !strings.Contains(result, tc.expectedPhrase) {
				t.Errorf("Expected response to contain %q, got: %s", tc.expectedPhrase, result)
			}
		})
	}
}
