package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/calque-ai/calque-pipe/convert"
	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	"github.com/calque-ai/calque-pipe/middleware/llm"
	"github.com/calque-ai/calque-pipe/middleware/prompt"
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

const systemPrompt = `You are an expert HR evaluator. Evaluate resumes for advanced technical roles.

Criteria for qualification:
- At least a bachelor's degree in a relevant field  
- At least 3 years of relevant work experience
- Strong technical skills relevant to the position

IMPORTANT: Extract the actual candidate name from the resume content. Do not use placeholder text.

Respond ONLY with valid JSON in this exact format:
{
  "candidate_name": "actual name from resume",
  "qualifies": true,
  "reasons": ["specific reason based on resume", "another specific reason"]
}

Example response:
{
  "candidate_name": "Alice Johnson",
  "qualifies": true,
  "reasons": ["Has Bachelor of Science in Computer Science from MIT", "Has 5+ years of software engineering experience"]
}`

// Uses framework for AI pipeline, Go for data processing
func main() {
	fmt.Println("Map-Reduce Resume Processing Example")

	// Create Ollama provider (connects to localhost:11434 by default)
	provider, err := llm.NewOllamaProvider("", "llama3.2:1b", llm.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to create provider:", err)
	}

	// Step 1: Read in all resumes from files. Files are in yaml format
	resumes, err := readResumes()
	if err != nil {
		log.Fatal("Failed to read resumes:", err)
	}

	// Create AI evaluation pipeline
	evaluationPipeline := core.New().
		Use(flow.Logger("resume evaluation", 200)).
		Use(prompt.Template("System: {{.System}}\n\nResume data to evaluate: {{.Input}}", map[string]any{
			"System": systemPrompt,
		})).
		Use(flow.Logger("prompt with data", 1000)).
		Use(llm.Chat(provider)).
		Use(flow.Logger("llm response", 200))

	// list of evaluation results
	evaluations := make([]Evaluation, 0, len(resumes))

	// Map phase, use regular Go loop with framework pipeline for each item
	for i, resume := range resumes {
		fmt.Printf("Processing resume %d/%d: %s\n", i+1, len(resumes), resume.Filename)

		var evaluation Evaluation
		err := evaluationPipeline.Run(context.Background(), resume.Content, convert.FromJson(&evaluation))
		if err != nil {
			log.Printf("Failed to evaluate %s: %v", resume.Filename, err)
			continue
		}

		evaluation.Filename = resume.Filename // Ensure filename is set
		evaluations = append(evaluations, evaluation)
	}

	// Reduce phase: no need for framework
	summary := reduceResults(evaluations)

	printSummary(summary)
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

// readResumes reads all resume files
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
