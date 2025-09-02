package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/logger"
	"github.com/calque-ai/go-calque/pkg/middleware/prompt"
)

// TestMapReducePipeline tests the basic map-reduce pipeline
func TestMapReducePipeline(t *testing.T) {
	t.Parallel()

	// Create mock AI client for testing
	mockClient := ai.NewMockClient(`{
		"candidate_name": "John Doe",
		"qualifies": true,
		"reasons": ["Has Bachelor of Science in Computer Science", "Has 5+ years of software engineering experience"]
	}`)

	// Create AI evaluation pipeline
	evaluationPipeline := calque.NewFlow().
		Use(logger.Head("resume evaluation", 200)).
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(logger.Head("prompt with data", 1000)).
		Use(logger.Timing("AI Response Time", ai.Agent(mockClient))).
		Use(logger.Head("llm response", 200))

	// Test input
	input := "John Doe\nComputer Science\nMIT\nSoftware Engineer\n5 years experience"
	var result string

	err := evaluationPipeline.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Map-reduce pipeline failed: %v", err)
	}

	// Verify the result contains expected content
	if !strings.Contains(result, "candidate_name") {
		t.Error("Expected candidate name in result")
	}
	if !strings.Contains(result, "qualifies") {
		t.Error("Expected qualification status in result")
	}
	if !strings.Contains(result, "reasons") {
		t.Error("Expected reasons in result")
	}
}

// TestResumeEvaluation tests individual resume evaluation
func TestResumeEvaluation(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient(`{
		"candidate_name": "Alice Johnson",
		"qualifies": true,
		"reasons": ["Has Bachelor of Science in Computer Science from MIT", "Has 5+ years of software engineering experience"]
	}`)

	// Create evaluation pipeline
	pipeline := calque.NewFlow().
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(ai.Agent(mockClient))

	// Test resume evaluation
	resumeContent := "Alice Johnson\nComputer Science\nMIT\nSoftware Engineer\n5 years experience"
	var result string

	err := pipeline.Run(context.Background(), resumeContent, &result)
	if err != nil {
		t.Fatalf("Resume evaluation failed: %v", err)
	}

	// Verify JSON response structure
	if !strings.Contains(result, "Alice Johnson") {
		t.Error("Expected candidate name in result")
	}
	if !strings.Contains(result, "true") {
		t.Error("Expected qualification status in result")
	}
}

// TestMultipleResumes tests processing multiple resumes
func TestMultipleResumes(t *testing.T) {
	t.Parallel()

	// Create mock AI client that returns different responses
	mockClient := ai.NewMockClient(`{
		"candidate_name": "Bob Smith",
		"qualifies": false,
		"reasons": ["Only 2 years of experience", "No relevant degree"]
	}`)

	// Create evaluation pipeline
	pipeline := calque.NewFlow().
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(ai.Agent(mockClient))

	// Test multiple resumes
	resumes := []string{
		"Bob Smith\nComputer Science\nState University\nJunior Developer\n2 years experience",
		"Carol Davis\nEngineering\nTech Institute\nSoftware Engineer\n4 years experience",
		"David Wilson\nComputer Science\nUniversity of Technology\nSenior Developer\n7 years experience",
	}

	results := make([]string, len(resumes))
	for i, resume := range resumes {
		err := pipeline.Run(context.Background(), resume, &results[i])
		if err != nil {
			t.Fatalf("Resume %d evaluation failed: %v", i+1, err)
		}
	}

	// Verify all results were processed
	for i, result := range results {
		if len(result) == 0 {
			t.Errorf("Expected non-empty result for resume %d", i+1)
		}
		if !strings.Contains(result, "candidate_name") {
			t.Errorf("Expected candidate name in result %d", i+1)
		}
	}
}

// TestQualificationCriteria tests different qualification scenarios
func TestQualificationCriteria(t *testing.T) {
	t.Parallel()

	// Test qualified candidate
	qualifiedClient := ai.NewMockClient(`{
		"candidate_name": "Qualified Candidate",
		"qualifies": true,
		"reasons": ["Has Bachelor of Science in Computer Science", "Has 5+ years of software engineering experience"]
	}`)

	qualifiedPipeline := calque.NewFlow().
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(ai.Agent(qualifiedClient))

	qualifiedResume := "Qualified Candidate\nComputer Science\nMIT\nSoftware Engineer\n5 years experience"
	var qualifiedResult string

	err := qualifiedPipeline.Run(context.Background(), qualifiedResume, &qualifiedResult)
	if err != nil {
		t.Fatalf("Qualified candidate evaluation failed: %v", err)
	}

	if !strings.Contains(qualifiedResult, "true") {
		t.Error("Expected qualified candidate to be marked as qualified")
	}

	// Test unqualified candidate
	unqualifiedClient := ai.NewMockClient(`{
		"candidate_name": "Unqualified Candidate",
		"qualifies": false,
		"reasons": ["Only 1 year of experience", "No relevant degree"]
	}`)

	unqualifiedPipeline := calque.NewFlow().
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(ai.Agent(unqualifiedClient))

	unqualifiedResume := "Unqualified Candidate\nLiberal Arts\nCommunity College\nIntern\n1 year experience"
	var unqualifiedResult string

	err = unqualifiedPipeline.Run(context.Background(), unqualifiedResume, &unqualifiedResult)
	if err != nil {
		t.Fatalf("Unqualified candidate evaluation failed: %v", err)
	}

	if !strings.Contains(unqualifiedResult, "false") {
		t.Error("Expected unqualified candidate to be marked as unqualified")
	}
}

// TestResumeParsing tests resume content parsing
func TestResumeParsing(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient(`{
		"candidate_name": "Test Candidate",
		"qualifies": true,
		"reasons": ["Has relevant degree", "Has sufficient experience"]
	}`)

	// Create pipeline
	pipeline := calque.NewFlow().
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(ai.Agent(mockClient))

	// Test different resume formats
	testCases := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Simple format",
			content:  "Name: Test Candidate\nDegree: Computer Science\nExperience: 5 years",
			expected: "Test Candidate",
		},
		{
			name:     "Detailed format",
			content:  "CANDIDATE: Test Candidate\nEDUCATION: Bachelor of Science in Computer Science\nWORK EXPERIENCE: 5+ years as Software Engineer",
			expected: "Test Candidate",
		},
		{
			name:     "Minimal format",
			content:  "Test Candidate\nCS Degree\n5 years exp",
			expected: "Test Candidate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := pipeline.Run(context.Background(), tc.content, &result)
			if err != nil {
				t.Fatalf("Resume parsing failed for %s: %v", tc.name, err)
			}

			if !strings.Contains(result, tc.expected) {
				t.Errorf("Expected to find '%s' in result for %s", tc.expected, tc.name)
			}
		})
	}
}

// TestPipelineLogging tests logging functionality in the pipeline
func TestPipelineLogging(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient(`{
		"candidate_name": "Logged Candidate",
		"qualifies": true,
		"reasons": ["Has relevant qualifications"]
	}`)

	// Create pipeline with comprehensive logging
	pipeline := calque.NewFlow().
		Use(logger.Print("START_EVALUATION")).
		Use(logger.Head("INPUT_RESUME", 100)).
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(logger.Head("PROMPT_DATA", 200)).
		Use(logger.Timing("AI_EVALUATION_TIME", ai.Agent(mockClient))).
		Use(logger.Head("AI_RESPONSE", 200)).
		Use(logger.Print("END_EVALUATION"))

	// Test input
	input := "Logged Candidate\nComputer Science\nUniversity\nSoftware Engineer\n4 years experience"
	var result string

	err := pipeline.Run(context.Background(), input, &result)
	if err != nil {
		t.Fatalf("Pipeline logging test failed: %v", err)
	}

	// Verify the result
	if !strings.Contains(result, "Logged Candidate") {
		t.Error("Expected candidate name in result")
	}
}

// TestConcurrentEvaluation tests concurrent resume evaluation
func TestConcurrentEvaluation(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient(`{
		"candidate_name": "Concurrent Candidate",
		"qualifies": true,
		"reasons": ["Has relevant qualifications"]
	}`)

	// Create pipeline
	pipeline := calque.NewFlow().
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(ai.Agent(mockClient))

	// Test concurrent processing
	const numResumes = 5
	results := make(chan string, numResumes)
	errors := make(chan error, numResumes)

	for i := 0; i < numResumes; i++ {
		go func(id int) {
			resume := fmt.Sprintf("Concurrent Candidate %d\nComputer Science\nUniversity\nSoftware Engineer\n4 years experience", id)
			var result string
			err := pipeline.Run(context.Background(), resume, &result)
			if err != nil {
				errors <- fmt.Errorf("resume %d failed: %v", id, err)
			} else {
				results <- result
			}
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numResumes; i++ {
		select {
		case result := <-results:
			if strings.Contains(result, "candidate_name") {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent evaluation failed: %v", err)
		}
	}

	// Should have successful results
	if successCount == 0 {
		t.Error("Expected at least one successful concurrent evaluation")
	}
}

// TestErrorHandling tests error handling in the pipeline
func TestErrorHandling(t *testing.T) {
	t.Parallel()

	// Create mock AI client that returns invalid JSON
	errorClient := ai.NewMockClient("Invalid JSON response")

	// Create pipeline
	pipeline := calque.NewFlow().
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(ai.Agent(errorClient))

	// Test input
	input := "Error Test Candidate\nComputer Science\nUniversity\nSoftware Engineer\n4 years experience"
	var result string

	err := pipeline.Run(context.Background(), input, &result)
	// Should handle the error gracefully
	if err != nil {
		t.Logf("Expected error handling: %v", err)
	}

	// Result should still contain some content
	if len(result) == 0 {
		t.Error("Expected some result content despite error")
	}
}
