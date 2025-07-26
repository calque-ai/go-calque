package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/calque-ai/calque-pipe/convert"
	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
)

// Person represents a person with basic information
type Person struct {
	Name  string `yaml:"name" desc:"The person's full name"`
	Age   int    `yaml:"age" desc:"The person's age in years"`
	City  string `yaml:"city" desc:"The city where the person lives"`
	Email string `yaml:"email_address"` // No desc - will be auto-generated
}

// JobAnalysis represents an analysis of someone's career
type JobAnalysis struct {
	SuitableRoles   []string `yaml:"suitable_roles" desc:"List of job roles that would suit this person"`
	SkillGaps       []string `yaml:"skill_gaps" desc:"Areas where the person might need improvement"`
	CareerAdvice    string   `yaml:"career_advice" desc:"Personalized career advice"`
	SalaryEstimate  int      `yaml:"salary_estimate" desc:"Estimated salary range (in thousands)"`
	RecommendedCity string   `yaml:"recommended_city" desc:"Best city for their career goals"`
}

// PersonJSON uses JSON tags instead
type PersonJSON struct {
	Name  string `json:"name" desc:"The person's full name"`
	Age   int    `json:"age" desc:"The person's age in years"`
	City  string `json:"city" desc:"The city where the person lives"`
	Email string `json:"email_address"` // Auto-generated: "The person's email address"
}

func main() {
	fmt.Println("🔧 Structured Converter Example")
	fmt.Println("================================")

	// Example 1: Basic YAML Conversion
	fmt.Println("\n📝 Example 1: YAML Struct to String")
	yamlInputExample()

	// Example 2: JSON Conversion
	fmt.Println("\n📊 Example 2: JSON Struct to String")
	jsonInputExample()

	// Example 3: Full Pipeline with LLM Simulation
	fmt.Println("\n🤖 Example 3: Complete LLM Pipeline")
	llmPipelineExample()

	// Example 4: Output Parsing
	fmt.Println("\n📥 Example 4: Parse LLM Response")
	outputParsingExample()
}

// yamlInputExample demonstrates struct to YAML conversion with descriptions and round-trip parsing
func yamlInputExample() {
	originalPerson := Person{
		Name:  "Alice Johnson",
		Age:   28,
		City:  "Austin",
		Email: "alice@example.com",
	}

	pipe := core.New()
	pipe.Use(flow.Logger("INPUT", 500))
	// Run the pipe with a Structured YAML converter and Person schema
	result, err := pipe.Run(context.Background(), originalPerson, convert.StructuredYAML[Person]())
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if parsedPerson, ok := result.(Person); ok {
		fmt.Printf("Parsed back to struct: %+v\n", parsedPerson)
		fmt.Printf("Round-trip successful: %t\n", originalPerson == parsedPerson)
	} else {
		fmt.Printf("Unexpected result type: %T\n", result)
	}
}

func jsonInputExample() {
	person := PersonJSON{
		Name:  "Bob Smith",
		Age:   35,
		City:  "Seattle",
		Email: "bob@tech.com",
	}

	pipe := core.New()
	pipe.Use(flow.Logger("INPUT", 500)) // Use 500 byte peek
	// Run the pipe with a Structured JSON converter and PersonJson schema
	result, err := pipe.Run(context.Background(), person, convert.StructuredJSON[PersonJSON]())
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// This should show the parsed struct, not the JSON string
	if parsedPerson, ok := result.(PersonJSON); ok {
		fmt.Printf("Parsed back to struct: %+v\n", parsedPerson)
		fmt.Printf("Round-trip successful: %t\n", person == parsedPerson)
	} else {
		fmt.Printf("Unexpected result type: %T, value: %v\n", result, result)
	}
}

func llmPipelineExample() {
	person := Person{
		Name:  "Carol Davis",
		Age:   42,
		City:  "Denver",
		Email: "carol@startup.com",
	}

	// Create pipeline: Struct → YAML → Mock LLM → Parse Result
	pipeline := core.New().
		Use(mockCareerAnalysisLLM()). // Mock LLM processing
		Use(flow.Logger("OUTPUT", 500))

	// Run with structured input, but parse result as JobAnalysis
	result, err := pipeline.Run(context.Background(), person, convert.StructuredYAML[JobAnalysis]())
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if jobAnalysis, ok := result.(JobAnalysis); ok {
		fmt.Printf("LLM processed the structured input and returned:\n")
		fmt.Printf("  Suitable Roles: %v\n", jobAnalysis.SuitableRoles)
		fmt.Printf("  Skill Gaps: %v\n", jobAnalysis.SkillGaps)
		fmt.Printf("  Career Advice: %s\n", jobAnalysis.CareerAdvice)
		fmt.Printf("  Salary Estimate: $%dk\n", jobAnalysis.SalaryEstimate)
		fmt.Printf("  Recommended City: %s\n", jobAnalysis.RecommendedCity)
	} else {
		fmt.Printf("Unexpected result type: %T, value: %v\n", result, result)
	}
}

func outputParsingExample() {
	// Mock LLM response in YAML format
	mockLLMResponse := `jobanalysis:
  suitable_roles: ["Senior Engineer", "Tech Lead", "Engineering Manager"]
  skill_gaps: ["Leadership", "System Design"]
  career_advice: "Focus on developing leadership and system design skills for senior roles"
  salary_estimate: 150
  recommended_city: "San Francisco"`

	// Demonstrate direct parsing with converter
	pipeline := core.New().
		Use(mockResponseHandler(mockLLMResponse)).
		Use(flow.Logger("OUTPUT", 500))

	result, err := pipeline.Run(context.Background(), "", convert.StructuredYAML[JobAnalysis]())
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Type assertion to get the structured result
	if jobAnalysis, ok := result.(JobAnalysis); ok {
		fmt.Printf("Parsed Job Analysis:\n")
		fmt.Printf("  Suitable Roles: %v\n", jobAnalysis.SuitableRoles)
		fmt.Printf("  Skill Gaps: %v\n", jobAnalysis.SkillGaps)
		fmt.Printf("  Career Advice: %s\n", jobAnalysis.CareerAdvice)
		fmt.Printf("  Salary Estimate: $%dk\n", jobAnalysis.SalaryEstimate)
		fmt.Printf("  Recommended City: %s\n", jobAnalysis.RecommendedCity)
	} else {
		fmt.Printf("Unexpected result type: %T\n", result)
	}
}

// Mock handlers for demonstration
func mockCareerAnalysisLLM() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Read the structured input (we'll ignore it for this demo)
		input, _ := io.ReadAll(r)

		// Log what we received
		fmt.Printf("LLM received structured input:\n%s\n", string(input))

		// Mock LLM response
		response := `jobanalysis:
  suitable_roles: ["Senior Software Engineer", "Tech Lead", "Product Manager"]
  skill_gaps: ["Leadership", "Product Strategy"]
  career_advice: "Consider developing leadership skills and understanding product strategy"
  salary_estimate: 120
  recommended_city: "Austin"`

		_, err := w.Write([]byte(response))
		return err
	})
}

func mockResponseHandler(response string) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		_, err := w.Write([]byte(response))
		return err
	})
}
