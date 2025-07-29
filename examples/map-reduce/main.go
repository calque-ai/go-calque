package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/calque-ai/calque-pipe/convert"
	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/examples/providers/mock"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
)

// Resume represents a candidate's resume
type Resume struct {
	Filename string `yaml:"filename" json:"filename" desc:"Name of the resume file"`
	Content  string `yaml:"content" json:"content" desc:"Full text content of the resume"`
}

// Evaluation represents the qualification assessment
type Evaluation struct {
	CandidateName string   `yaml:"candidate_name" json:"candidate_name" desc:"Name of the candidate"`
	Qualifies     bool     `yaml:"qualifies" json:"qualifies" desc:"Whether the candidate qualifies"`
	Reasons       []string `yaml:"reasons" json:"reasons" desc:"List of reasons for qualification decision"`
	Filename      string   `yaml:"filename" json:"filename" desc:"Source resume filename"`
}

// Summary represents the final results
type Summary struct {
	TotalCandidates     int      `yaml:"total_candidates" json:"total_candidates" desc:"Total number of candidates evaluated"`
	QualifiedCount      int      `yaml:"qualified_count" json:"qualified_count" desc:"Number of qualified candidates"`
	QualifiedPercentage float64  `yaml:"qualified_percentage" json:"qualified_percentage" desc:"Percentage of qualified candidates"`
	QualifiedNames      []string `yaml:"qualified_names" json:"qualified_names" desc:"Names of qualified candidates"`
}

func main() {
	fmt.Println("Map-Reduce Resume Processing Example")
	fmt.Println("====================================")

	// Create mock provider
	provider := mock.NewMockProvider("").WithStreamDelay(50)

	// Step 1: Read all resumes (Map phase)
	fmt.Println("\n1. MAP PHASE: Reading resumes...")
	resumes, err := readResumes()
	if err != nil {
		log.Fatal("Failed to read resumes:", err)
	}
	fmt.Printf("Found %d resumes\n", len(resumes))

	// Step 2: Process resumes using framework-native batching
	fmt.Println("\n2. BATCH PROCESSING: Evaluating resumes...")
	
	// Create evaluation pipeline with batching for efficiency
	evaluationPipeline := core.New().
		Use(flow.Logger("EVALUATION_INPUT", 200)).
		Use(flow.Batch[Resume](createEvaluationHandler(provider), 2, 1*time.Second)). // Process 2 at a time
		Use(flow.Logger("EVALUATION_OUTPUT", 200))

	var evaluations []Evaluation
	
	// Process each resume through the batched pipeline
	for _, resume := range resumes {
		var result Evaluation
		err := evaluationPipeline.Run(context.Background(), convert.StructuredYAML(resume), convert.StructuredYAMLOutput[Evaluation](&result))
		if err != nil {
			log.Printf("Failed to evaluate resume %s: %v", resume.Filename, err)
			continue
		}
		result.Filename = resume.Filename
		evaluations = append(evaluations, result)
	}

	// Step 3: Reduce phase using framework handlers
	fmt.Println("\n3. REDUCE PHASE: Aggregating results...")
	
	// Create reduce pipeline
	reducePipeline := core.New().
		Use(flow.Logger("REDUCE_INPUT", 200)).
		Use(createReduceHandler()).
		Use(flow.Logger("REDUCE_OUTPUT", 500))

	// Run reduce phase using JSON for slice compatibility
	var summary Summary
	err = reducePipeline.Run(context.Background(), convert.Json(evaluations), convert.StructuredYAMLOutput[Summary](&summary))
	if err != nil {
		log.Fatal("Reduce phase failed:", err)
	}

	// Print final summary
	printSummary(summary)
}

// createResumeReader creates a handler that reads all resumes and outputs them as structured YAML
func createResumeReader() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		resumes, err := readResumes()
		if err != nil {
			return err
		}

		// Use a sub-pipeline to convert to structured YAML
		pipe := core.New()
		var yamlResult string
		err = pipe.Run(ctx, convert.StructuredYAML(resumes), &yamlResult)
		if err != nil {
			return err
		}

		_, err = w.Write([]byte(yamlResult))
		return err
	})
}

// readResumes implements the Map phase - reads all resume files
func readResumes() ([]Resume, error) {
	dataDir := filepath.Join(".", "data")
	var resumes []Resume

	// Create some sample data if directory doesn't exist
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := createSampleData(dataDir); err != nil {
			return nil, err
		}
	}

	files, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".txt") {
			content, err := os.ReadFile(filepath.Join(dataDir, file.Name()))
			if err != nil {
				continue
			}

			resumes = append(resumes, Resume{
				Filename: file.Name(),
				Content:  string(content),
			})
		}
	}

	return resumes, nil
}

// evaluateResumes implements batch processing with your framework
func evaluateResumes(resumes []Resume) ([]Evaluation, error) {
	// Create mock LLM provider
	provider := mock.NewMockProvider("").WithStreamDelay(50)

	// Create evaluation pipeline with batching
	evaluationPipeline := core.New().
		Use(flow.Logger("BATCH_INPUT", 200)).
		Use(flow.Batch[Resume](createEvaluationHandler(provider), 3, 2*time.Second)). // Batch of 3 or 2 seconds
		Use(flow.Logger("BATCH_OUTPUT", 200))

	var evaluations []Evaluation

	// Process each resume through the batched pipeline
	for _, resume := range resumes {
		var result Evaluation
		err := evaluationPipeline.Run(context.Background(), convert.Json(resume), convert.JsonOutput(&result))
		if err != nil {
			log.Printf("Failed to evaluate resume %s: %v", resume.Filename, err)
			continue
		}
		result.Filename = resume.Filename
		evaluations = append(evaluations, result)
	}

	return evaluations, nil
}

// createEvaluationHandler creates the LLM evaluation handler that processes YAML resume data
func createEvaluationHandler(provider llm.LLMProvider) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Create a pipeline for evaluation
		pipeline := core.New().
			Use(llm.SystemPrompt(`You are an expert HR evaluator. Evaluate resumes for advanced technical roles.

Criteria for qualification:
- At least a bachelor's degree in a relevant field  
- At least 3 years of relevant work experience
- Strong technical skills relevant to the position

Respond in YAML format:
candidate_name: [Name]
qualifies: [true/false] 
reasons:
  - [reason 1]
  - [reason 2]`)).
			Use(llm.Prompt("Resume data to evaluate: {{.Input}}")).
			Use(llm.Chat(provider))

		// Process and return result as YAML string
		var result string
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		
		err = pipeline.Run(ctx, string(input), &result)
		if err != nil {
			return err
		}

		_, err = w.Write([]byte(result))
		return err
	})
}

// createReduceHandler creates a handler that aggregates evaluation results
func createReduceHandler() core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		// Parse input as a slice of evaluations using structured YAML
		var evaluations []Evaluation
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		// Parse the JSON input (since slice YAML not supported)
		pipe := core.New()
		err = pipe.Run(ctx, string(input), convert.JsonOutput(&evaluations))
		if err != nil {
			return err
		}

		// Reduce logic
		summary := reduceResults(evaluations)

		// Output as structured YAML
		resultPipe := core.New()
		var yamlResult string
		err = resultPipe.Run(ctx, convert.StructuredYAML(summary), &yamlResult)
		if err != nil {
			return err
		}

		_, err = w.Write([]byte(yamlResult))
		return err
	})
}

// reduceResults implements the Reduce phase - aggregates all evaluations
func reduceResults(evaluations []Evaluation) Summary {
	totalCount := len(evaluations)
	qualifiedCount := 0
	var qualifiedNames []string

	for _, eval := range evaluations {
		if eval.Qualifies {
			qualifiedCount++
			qualifiedNames = append(qualifiedNames, eval.CandidateName)
		}
	}

	percentage := 0.0
	if totalCount > 0 {
		percentage = float64(qualifiedCount) / float64(totalCount) * 100
	}

	return Summary{
		TotalCandidates:     totalCount,
		QualifiedCount:      qualifiedCount,
		QualifiedPercentage: percentage,
		QualifiedNames:      qualifiedNames,
	}
}

// printSummary displays the final results
func printSummary(summary Summary) {
	fmt.Println("\n===== Resume Qualification Summary =====")
	fmt.Printf("Total candidates evaluated: %d\n", summary.TotalCandidates)
	fmt.Printf("Qualified candidates: %d (%.1f%%)\n", summary.QualifiedCount, summary.QualifiedPercentage)

	if len(summary.QualifiedNames) > 0 {
		fmt.Println("\nQualified candidates:")
		for _, name := range summary.QualifiedNames {
			fmt.Printf("- %s\n", name)
		}
	}

	// Show detailed results in JSON
	fmt.Println("\nDetailed Results (JSON):")
	jsonData, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(jsonData))
}

// createSampleData creates sample resume files for demonstration
func createSampleData(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	sampleResumes := map[string]string{
		"alice_johnson.txt": `Alice Johnson
Email: alice@example.com
Phone: (555) 123-4567

EDUCATION:
Bachelor of Science in Computer Science
MIT, 2019

EXPERIENCE:
Senior Software Engineer - Tech Corp (2020-2024)
- Led development of microservices architecture
- Implemented CI/CD pipelines
- Mentored junior developers

Software Engineer - StartupXYZ (2019-2020)
- Built React frontend applications
- Developed REST APIs in Node.js

SKILLS:
- Programming: Go, Python, JavaScript, Java
- Cloud: AWS, Docker, Kubernetes
- Databases: PostgreSQL, MongoDB`,

		"bob_smith.txt": `Bob Smith
Email: bob@email.com
Phone: (555) 987-6543

EDUCATION:
High School Diploma, 2020

EXPERIENCE:
Junior Developer - Small Company (2021-2024)
- Fixed bugs in legacy code
- Basic HTML/CSS modifications

Intern - Another Company (2020-2021)
- Data entry tasks
- Simple scripting

SKILLS:
- HTML, CSS
- Basic JavaScript
- Excel`,

		"carol_davis.txt": `Carol Davis, PhD
Email: carol@research.edu
Phone: (555) 456-7890

EDUCATION:
PhD in Computer Science - Stanford University, 2018
MS in Computer Science - UC Berkeley, 2015
BS in Mathematics - Harvard University, 2013

EXPERIENCE:
Principal Research Scientist - Google Research (2022-2024)
- Led machine learning research team
- Published 15+ papers in top-tier venues
- Developed novel deep learning architectures

Senior Research Engineer - Microsoft Research (2018-2022)
- Research in computer vision and NLP
- Shipped ML features to millions of users

SKILLS:
- Programming: Python, C++, CUDA, TensorFlow, PyTorch
- Research: Machine Learning, Computer Vision, NLP
- Leadership: Team management, research strategy`,

		"david_wilson.txt": `David Wilson
Email: david@freelance.com
Phone: (555) 234-5678

EDUCATION:
Associate Degree in Web Design, 2022

EXPERIENCE:
Freelance Web Developer (2022-2024)
- Created 5+ WordPress websites
- Basic e-commerce setup

Part-time Web Assistant (2021-2022)
- Updated website content
- Social media management

SKILLS:
- WordPress
- Basic PHP
- Photoshop`,
	}

	for filename, content := range sampleResumes {
		filePath := filepath.Join(dataDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	fmt.Printf("Created %d sample resume files in %s\n", len(sampleResumes), dataDir)
	return nil
}
