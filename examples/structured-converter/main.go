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
	Email string `yaml:"email_address"` // No desc
}

// JobAnalysis represents an analysis of someone's career
type JobAnalysis struct {
	SuitableRoles   []string `yaml:"suitable_roles" desc:"List of job roles that would suit this person"`
	SkillGaps       []string `yaml:"skill_gaps" desc:"Areas where the person might need improvement"`
	CareerAdvice    string   `yaml:"career_advice" desc:"Personalized career advice"`
	SalaryEstimate  int      `yaml:"salary_estimate" desc:"Estimated salary range (in thousands)"`
	RecommendedCity string   `yaml:"recommended_city" desc:"Best city for their career goals"`
}

// PersonXML uses XML tags instead
type PersonXML struct {
	Name  string `xml:"name" desc:"The person's full name"`
	Age   int    `xml:"age" desc:"The person's age in years"`
	City  string `xml:"city" desc:"The city where the person lives"`
	Email string `xml:"email_address"` // Auto-generated: "The person's email address"
}

func main() {
	fmt.Println("Structured Converter Example")
	fmt.Println("================================")

	// Example 1: Basic YAML Conversion
	fmt.Println("\nExample 1: YAML Struct to String")
	yamlInputExample()

	// Example 2: XML Conversion
	fmt.Println("\nExample 2: XML Struct to String")
	xmlInputExample()

	// Example 3: Full Pipeline with LLM Simulation
	fmt.Println("\nExample 3: Complete LLM Pipeline")
	llmPipelineExample()

	// Example 4: Output Parsing
	fmt.Println("\nExample 4: Parse LLM Response")
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
	var result Person
	err := pipe.Run(context.Background(), convert.StructuredYAML(originalPerson), convert.StructuredYAMLOutput[Person](&result))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Parsed back to struct: %+v\n", result)
	fmt.Printf("Round-trip successful: %t\n", originalPerson == result)

}

func xmlInputExample() {
	person := PersonXML{
		Name:  "Bob Smith",
		Age:   35,
		City:  "Seattle",
		Email: "bob@tech.com",
	}

	pipe := core.New()
	pipe.Use(flow.Logger("INPUT", 500)) // Use 500 byte peek

	// Run the pipe with a Structured XML converter and PersonXML schema
	var result PersonXML
	err := pipe.Run(context.Background(), convert.StructuredXML(person), convert.StructuredXMLOutput[PersonXML](&result))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// This should show the parsed struct, not the JSON string
	fmt.Printf("Parsed back to struct: %+v\n", result)
	fmt.Printf("Round-trip successful: %t\n", person == result)

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
	var output JobAnalysis
	err := pipeline.Run(context.Background(), convert.StructuredYAML(person), convert.StructuredYAMLOutput[JobAnalysis](&output))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("LLM processed the structured input and returned:\n")
	fmt.Printf("  Suitable Roles: %v\n", output.SuitableRoles)
	fmt.Printf("  Skill Gaps: %v\n", output.SkillGaps)
	fmt.Printf("  Career Advice: %s\n", output.CareerAdvice)
	fmt.Printf("  Salary Estimate: $%dk\n", output.SalaryEstimate)
	fmt.Printf("  Recommended City: %s\n", output.RecommendedCity)

}

func outputParsingExample() {
	// Mock LLM response in YAML format
	mockLLMResponse := `
  suitable_roles: ["Senior Engineer", "Tech Lead", "Engineering Manager"]
  skill_gaps: ["Leadership", "System Design"]
  career_advice: "Focus on developing leadership and system design skills for senior roles"
  salary_estimate: 150
  recommended_city: "San Francisco"`

	// Demonstrate direct parsing with converter
	pipeline := core.New().
		Use(mockResponseHandler(mockLLMResponse)).
		Use(flow.Logger("OUTPUT", 500))

	var output JobAnalysis
	err := pipeline.Run(context.Background(), "", convert.StructuredYAMLOutput[JobAnalysis](&output))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Parsed Job Analysis:\n")
	fmt.Printf("  Suitable Roles: %v\n", output.SuitableRoles)
	fmt.Printf("  Skill Gaps: %v\n", output.SkillGaps)
	fmt.Printf("  Career Advice: %s\n", output.CareerAdvice)
	fmt.Printf("  Salary Estimate: $%dk\n", output.SalaryEstimate)
	fmt.Printf("  Recommended City: %s\n", output.RecommendedCity)

}

// Mock handlers for demonstration
func mockCareerAnalysisLLM() core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		// Read the structured input (we'll ignore it for this demo)
		input, _ := io.ReadAll(r.Data)

		// Log what we received
		fmt.Printf("LLM received structured input:\n%s\n", string(input))

		// Mock LLM response
		response := `
  suitable_roles: ["Senior Software Engineer", "Tech Lead", "Product Manager"]
  skill_gaps: ["Leadership", "Product Strategy"]
  career_advice: "Consider developing leadership skills and understanding product strategy"
  salary_estimate: 120
  recommended_city: "Austin"`

		_, err := w.Data.Write([]byte(response))
		return err
	})
}

func mockResponseHandler(response string) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		_, err := w.Data.Write([]byte(response))
		return err
	})
}
