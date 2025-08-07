// Package main demonstrates JSON Schema usage with Calque-Pipe AI agents.
//
// This package contains 3 focused examples showing different approaches:
// 1. Basic WithSchema usage for structured AI output
// 2. JSON Schema converters for schema-embedded input/output
// 3. Advanced combined usage in multi-stage pipelines
//
// Each example demonstrates when and how to use different JSON Schema features
// for type-safe, validated AI interactions.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/convert"
	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/ai"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/prompt"
	"github.com/invopop/jsonschema"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("Calque-Pipe JSON Schema Examples")
	fmt.Println("=====================================")
	fmt.Println("Demonstrating structured AI interactions with JSON Schema validation")

	// Initialize Ollama client for Examples 1 & 2
	ollamaClient, err := ai.NewOllama("llama3.2:1b")
	if err != nil {
		log.Printf("Could not connect to Ollama: %v", err)
		log.Println("To run examples 1-2:")
		log.Println("  1. Install Ollama: https://ollama.ai")
		log.Println("  2. Run: ollama pull llama3.2:1b")
		log.Println("  3. Ensure Ollama is running (ollama serve)")
		return
	}

	fmt.Println("Connected to Ollama and Gemini")
	fmt.Println()

	// Run the 3 focused examples
	runExample1WithSchema(ollamaClient)
	fmt.Println()

	runExample2JsonSchemaConverters(ollamaClient)
	fmt.Println()

	// Load env file
	err = godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize Gemini client for Example 3
	geminiClient, err := ai.NewGemini("gemini-2.0-flash")
	if err != nil {
		log.Printf("Could not connect to Gemini: %v", err)
		log.Println("To run example 3:")
		log.Println("  1. Get API key from: https://aistudio.google.com/app/apikey")
		log.Println("  2. Set: export GOOGLE_API_KEY=your_api_key")
		return
	}

	runExample3AdvancedCombined(geminiClient)
	fmt.Println()

	fmt.Println("All examples completed!")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Println("• Example 1: ai.WithSchema() ensures AI generates valid structured JSON")
	fmt.Println("• Example 2: JSON Schema converters embed schema context and validate output")
	fmt.Println("• Example 3: Combined multi-stage pipeline with context passing (Gemini)")
}

// TaskAnalysisEx1 represents structured analysis of a development task
type TaskAnalysisEx1 struct {
	TaskType       string   `json:"task_type" jsonschema:"required,enum=bug_fix,enum=feature,enum=refactor,enum=documentation,description=Type of development task"`
	Priority       string   `json:"priority" jsonschema:"required,enum=low,enum=medium,enum=high,enum=critical,description=Task priority level"`
	EstimatedHours int      `json:"estimated_hours" jsonschema:"minimum=1,maximum=40,description=Estimated hours to complete"`
	Dependencies   []string `json:"dependencies" jsonschema:"description=List of dependencies or blockers"`
	Skills         []string `json:"skills" jsonschema:"description=Required skills or technologies"`
	Confidence     float64  `json:"confidence" jsonschema:"minimum=0,maximum=1,description=Confidence in the analysis (0-1)"`
}

const promptTemplateEx1 = `Analyze this development task and provide structured analysis:

Task: {{.Input}}

Analyze the task and provide:
- task_type: classify as bug_fix, feature, refactor, or documentation
- priority: assess as low, medium, high, or critical
- estimated_hours: realistic estimate (1-40 hours)
- dependencies: any blocking dependencies
- skills: required technologies/skills
- confidence: your confidence in this analysis (0.0-1.0)

Return valid JSON only.`

func runExample1WithSchema(client ai.Client) {
	fmt.Println("=== Example 1: Basic WithSchema Usage ===")
	fmt.Println("This example shows how to use ai.WithSchema() for structured AI output")

	// Task description - plain text input
	taskDescription := "Build a real-time chat application with WebSocket support, user authentication, message history, and emoji reactions. The backend should use Go with a PostgreSQL database."

	// Create JSON schema for the expected response structure
	reflector := jsonschema.Reflector{}
	schema := reflector.Reflect(&TaskAnalysisEx1{})

	responseFormat := &ai.ResponseFormat{
		Type:   "json_schema",
		Schema: schema,
	}

	// Pipeline: Text input → AI analysis → Structured JSON output
	pipe := core.New()
	pipe.
		Use(flow.Logger("INPUT", 200)).
		Use(prompt.Template(promptTemplateEx1)).
		Use(flow.Logger("PROMPT", 500)).
		Use(ai.Agent(client, ai.WithSchema(responseFormat))). // ← Key: WithSchema tells AI to follow the schema
		Use(flow.Logger("RESPONSE", 500))                     // Add this to see what AI is returning

	// Use standard JSON converters since AI generates structured JSON
	var analysis TaskAnalysisEx1
	err := pipe.Run(context.Background(), taskDescription, convert.FromJson(&analysis))
	if err != nil {
		log.Printf("Analysis failed: %v", err)
		return
	}

	// Display structured results
	fmt.Printf("\n📋 TASK ANALYSIS RESULTS:\n")
	fmt.Printf("Type: %s\n", analysis.TaskType)
	fmt.Printf("Priority: %s\n", analysis.Priority)
	fmt.Printf("Estimated Hours: %d\n", analysis.EstimatedHours)
	fmt.Printf("Confidence: %.0f%%\n", analysis.Confidence*100)

	if len(analysis.Skills) > 0 {
		fmt.Printf("Required Skills: %v\n", analysis.Skills)
	}

	if len(analysis.Dependencies) > 0 {
		fmt.Printf("Dependencies: %v\n", analysis.Dependencies)
	}

	fmt.Println("\n✅ WithSchema ensured the AI response matches our TaskAnalysis structure")
}

// UserProfileEx2 represents user input data
type UserProfileEx2 struct {
	Name       string   `json:"name" jsonschema:"required,description=Full name"`
	Role       string   `json:"role" jsonschema:"required,description=Job role or title"`
	Experience int      `json:"experience" jsonschema:"minimum=0,maximum=50,description=Years of experience"`
	Skills     []string `json:"skills" jsonschema:"description=List of technical skills"`
	Location   string   `json:"location" jsonschema:"description=Work location"`
}

// CareerAdvice represents AI-generated career guidance
type CareerAdvice struct {
	SuggestedRole   string   `json:"suggested_role" jsonschema:"required,description=Recommended job role"`
	CareerPath      string   `json:"career_path" jsonschema:"required,description=Career development advice"`
	SkillsToLearn   []string `json:"skills_to_learn" jsonschema:"description=Skills to develop"`
	SalaryRange     string   `json:"salary_range" jsonschema:"description=Expected salary range"`
	NextSteps       []string `json:"next_steps" jsonschema:"description=Actionable next steps"`
	ConfidenceScore float64  `json:"confidence_score" jsonschema:"minimum=0,maximum=1,description=Confidence in advice (0-1)"`
}

const promptTemplateEx2 = `Based on the user profile with embedded schema, provide career advice:

{{.Input}}

The input includes both the user data and the JSON schema structure. Please provide career advice that matches the schema format shown. Pay attention to the schema definitions and required fields.

Do not include any explanatory text, only the JSON response.`

func runExample2JsonSchemaConverters(client ai.Client) {
	fmt.Println("=== Example 2: JSON Schema Converters Usage ===")
	fmt.Println("This example shows convert.ToJsonSchema() and convert.FromJsonSchema()")

	// Sample user profile data
	userProfile := UserProfileEx2{
		Name:       "Jordan Smith",
		Role:       "Junior Go Developer",
		Experience: 2,
		Skills:     []string{"Go", "Docker", "REST APIs", "Git", "PostgreSQL"},
		Location:   "Remote",
	}

	fmt.Printf("Input Profile: %s (%s with %d years experience)\n",
		userProfile.Name, userProfile.Role, userProfile.Experience)

	// Create JSON schema for the expected response structure
	reflector := jsonschema.Reflector{}
	schema := reflector.Reflect(&CareerAdvice{})

	responseFormat := &ai.ResponseFormat{
		Type:   "json_schema",
		Schema: schema,
	}

	// Pipeline using JSON Schema converters WITH WithSchema for reliability
	pipe := core.New()
	pipe.
		Use(flow.Logger("PROFILE_WITH_SCHEMA", 400)). // This will show the embedded schema
		Use(prompt.Template(promptTemplateEx2)).
		Use(flow.Logger("AI_PROMPT", 500)).
		Use(ai.Agent(client, ai.WithSchema(responseFormat))). // WithSchema ensures JSON mode for model
		Use(flow.Logger("RESPONSE", 600))

	// Key difference: Using JSON Schema converters
	var advice CareerAdvice
	err := pipe.Run(
		context.Background(),
		convert.ToJsonSchema(userProfile),             // ← Embeds schema with data into input
		convert.FromJsonSchema[CareerAdvice](&advice), // ← Validates output against schema
	)
	if err != nil {
		log.Printf("Career advice generation failed: %v", err)
		return
	}

	// Display results
	fmt.Printf("\n💼 CAREER ADVICE FOR %s:\n", userProfile.Name)
	fmt.Printf("Suggested Role: %s\n", advice.SuggestedRole)
	fmt.Printf("Salary Range: %s\n", advice.SalaryRange)
	fmt.Printf("Confidence: %.0f%%\n", advice.ConfidenceScore*100)

	fmt.Printf("\nCareer Path:\n%s\n", advice.CareerPath)

	if len(advice.SkillsToLearn) > 0 {
		fmt.Printf("\nSkills to Learn:\n")
		for _, skill := range advice.SkillsToLearn {
			fmt.Printf("  • %s\n", skill)
		}
	}

	if len(advice.NextSteps) > 0 {
		fmt.Printf("\nNext Steps:\n")
		for i, step := range advice.NextSteps {
			fmt.Printf("  %d. %s\n", i+1, step)
		}
	}

	fmt.Println("\n✅ JSON Schema converters handled schema embedding and validation")
}

// UserProfile represents initial user data (Stage 1 output)
type UserProfile struct {
	Name       string   `json:"name" jsonschema:"required,description=Full name"`
	Role       string   `json:"role" jsonschema:"required,description=Current role"`
	Experience int      `json:"experience" jsonschema:"minimum=0,description=Years of experience"`
	Skills     []string `json:"skills" jsonschema:"description=Technical skills"`
}

// EnhancedProfile represents enriched data (Stage 2 output)
type EnhancedProfile struct {
	BasicInfo   UserProfile `json:"basic_info" jsonschema:"required,description=Basic user information"`
	CareerLevel string      `json:"career_level" jsonschema:"required,enum=junior,enum=mid,enum=senior,enum=lead,description=Career level assessment"`
	Strengths   []string    `json:"strengths" jsonschema:"description=Identified strengths"`
	NextRole    string      `json:"next_role" jsonschema:"description=Suggested next role"`
}

const promptTemplateEx3 = `Enhance this profile with career insights:

{{.Input}}

The input includes the basic profile with embedded schema. Analyze experience level, identify strengths, and suggest next career role. Return valid JSON only.`

func runExample3AdvancedCombined(client ai.Client) {
	fmt.Println("=== Example 3: Multi-Stage Pipeline with Context Passing ===")
	fmt.Println("Shows chaining converters and passing structured data between stages")

	// Simple input data
	inputText := "Alex Johnson, Senior Go Developer, 5 years experience, skills: Go, Kubernetes, PostgreSQL, gRPC"
	fmt.Printf("Input: %s\n", inputText)

	// Stage 1: Extract basic profile using WithSchema (like Example 1)
	fmt.Println("\n🔄 Stage 1: Extract structured data (WithSchema + FromJson)")

	reflector := jsonschema.Reflector{}
	profileSchema := reflector.Reflect(&UserProfile{})
	profileFormat := &ai.ResponseFormat{Type: "json_schema", Schema: profileSchema}

	stage1Pipe := core.New()
	stage1Pipe.
		Use(prompt.Template("Extract user profile from: {{.Input}}\nReturn valid JSON only.")).
		Use(flow.Logger("Prompt", 500)).
		Use(ai.Agent(client, ai.WithSchema(profileFormat))).
		Use(flow.Logger("Output", 500))

	var profile UserProfile
	err := stage1Pipe.Run(context.Background(), inputText, convert.FromJson(&profile))
	if err != nil {
		log.Printf("Stage 1 failed: %v", err)
		return
	}

	fmt.Printf("✓ Extracted: %s (%s, %d years)\n", profile.Name, profile.Role, profile.Experience)

	// Stage 2: Enhance profile using context from Stage 1
	fmt.Println("\n🔄 Stage 2: Enhance with context (ToJsonSchema + FromJsonSchema)")

	enhancedSchema := reflector.Reflect(&EnhancedProfile{})
	enhancedFormat := &ai.ResponseFormat{Type: "json_schema", Schema: enhancedSchema}

	stage2Pipe := core.New()
	stage2Pipe.
		Use(prompt.Template(promptTemplateEx3)).
		Use(ai.Agent(client, ai.WithSchema(enhancedFormat)))

	var enhanced EnhancedProfile
	err = stage2Pipe.Run(
		context.Background(),
		convert.ToJsonSchema(profile),                      // ← Stage 1 output as schema-embedded input
		convert.FromJsonSchema[EnhancedProfile](&enhanced), // ← Stage 2 output with validation
	)
	if err != nil {
		log.Printf("Stage 2 failed: %v", err)
		return
	}

	// Display results
	fmt.Printf("\n📋 ENHANCED PROFILE:\n")
	fmt.Printf("Name: %s | Level: %s\n", enhanced.BasicInfo.Name, enhanced.CareerLevel)
	fmt.Printf("Next Role: %s\n", enhanced.NextRole)
	if len(enhanced.Strengths) > 0 {
		fmt.Printf("Strengths: %v\n", enhanced.Strengths)
	}

	fmt.Println("\n✅ Multi-stage pipeline completed:")
	fmt.Println("  Stage 1: WithSchema → FromJson (structured extraction)")
	fmt.Println("  Stage 2: ToJsonSchema → FromJsonSchema (context passing)")
	fmt.Println("  Key: Stage 1 output becomes Stage 2 input with embedded schema")
}
