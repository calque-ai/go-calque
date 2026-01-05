package main

import (
	"context"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/convert"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/inspect"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

// TestConverterBasics tests basic converter functionality
func TestConverterBasics(t *testing.T) {
	t.Parallel()

	// Test JSON string conversion pipeline
	jsonString := `{
		"name": "Smart Widget",
		"category": "Electronics", 
		"price": 99.99,
		"features": ["WiFi", "Bluetooth", "Voice Control"],
		"description": "A widget that calques ideas from competing products."
	}`

	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Print("JSON_INPUT")).
		Use(text.Transform(strings.ToUpper)).
		Use(inspect.Print("UPPERCASE_JSON"))

	var jsonResult ProductInfo
	err := pipe.Run(context.Background(), convert.ToJSON(jsonString), convert.FromJSON(&jsonResult))
	if err != nil {
		t.Fatalf("JSON conversion failed: %v", err)
	}

	// Verify the conversion worked correctly
	if jsonResult.Name != "SMART WIDGET" {
		t.Errorf("Expected name 'SMART WIDGET', got '%s'", jsonResult.Name)
	}
	if jsonResult.Category != "ELECTRONICS" {
		t.Errorf("Expected category 'ELECTRONICS', got '%s'", jsonResult.Category)
	}
	if jsonResult.Price != 99.99 {
		t.Errorf("Expected price 99.99, got %f", jsonResult.Price)
	}
	if len(jsonResult.Features) != 3 {
		t.Errorf("Expected 3 features, got %d", len(jsonResult.Features))
	}
}

// TestAIConverterExample tests AI integration with converters
func TestAIConverterExample(t *testing.T) {
	t.Parallel()

	// Create mock AI client
	mockClient := ai.NewMockClient("This is a neural interface product with advanced AI capabilities. Consider adding wireless charging and improved neural mapping algorithms.")
	agent := ai.Agent(mockClient)

	// Input product data
	product := ProductInfo{
		Name:        "Neural Interface",
		Category:    "AI Hardware",
		Price:       2499.99,
		Features:    []string{"Brain-Computer Interface", "AI Processing", "Neural Mapping"},
		Description: "This device calques neural patterns and transforms them into digital commands.",
	}

	// Pipeline: struct -> YAML -> AI analysis -> result string
	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Print("STRUCT_INPUT")).
		Use(agent)

	var result string
	err := pipe.Run(context.Background(), convert.ToYAML(product), &result)
	if err != nil {
		t.Fatalf("AI converter pipeline failed: %v", err)
	}

	// Verify AI response
	if !strings.Contains(result, "neural interface") {
		t.Errorf("Expected AI response to mention neural interface, got: %s", result)
	}
}

// TestYAMLConversion tests YAML struct conversion
func TestYAMLConversion(t *testing.T) {
	t.Parallel()

	product := ProductInfo{
		Name:        "Test Product",
		Category:    "Test Category",
		Price:       19.99,
		Features:    []string{"Feature 1", "Feature 2"},
		Description: "Test description",
	}

	// Convert struct to YAML
	yamlData := convert.ToYAML(product)
	if yamlData == nil {
		t.Fatal("Failed to convert struct to YAML")
	}

	// Convert YAML back to struct using pipeline
	var result ProductInfo
	pipe := calque.NewFlow()
	err := pipe.Run(context.Background(), yamlData, convert.FromYAML(&result))
	if err != nil {
		t.Fatalf("Failed to convert YAML back to struct: %v", err)
	}

	// Verify the conversion preserved all data
	if result.Name != product.Name {
		t.Errorf("Expected name '%s', got '%s'", product.Name, result.Name)
	}
	if result.Category != product.Category {
		t.Errorf("Expected category '%s', got '%s'", product.Category, result.Category)
	}
	if result.Price != product.Price {
		t.Errorf("Expected price %f, got %f", product.Price, result.Price)
	}
	if len(result.Features) != len(product.Features) {
		t.Errorf("Expected %d features, got %d", len(product.Features), len(result.Features))
	}
}

// TestJSONRoundTrip tests JSON conversion round trip
func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := ProductInfo{
		Name:        "Round Trip Test",
		Category:    "Testing",
		Price:       42.00,
		Features:    []string{"Test", "Round", "Trip"},
		Description: "Testing JSON round trip conversion",
	}

	// Convert to JSON
	jsonData := convert.ToJSON(original)
	if jsonData == nil {
		t.Fatal("Failed to convert struct to JSON")
	}

	// Convert back to struct using pipeline
	var result ProductInfo
	pipe := calque.NewFlow()
	err := pipe.Run(context.Background(), jsonData, convert.FromJSON(&result))
	if err != nil {
		t.Fatalf("Failed to convert JSON back to struct: %v", err)
	}

	// Verify all fields match
	if result.Name != original.Name {
		t.Errorf("Expected name '%s', got '%s'", original.Name, result.Name)
	}
	if result.Category != original.Category {
		t.Errorf("Expected category '%s', got '%s'", original.Category, result.Category)
	}
	if result.Price != original.Price {
		t.Errorf("Expected price %f, got %f", original.Price, result.Price)
	}
	if len(result.Features) != len(original.Features) {
		t.Errorf("Expected %d features, got %d", len(original.Features), len(result.Features))
	}
}

// TestConverterPipeline tests a complete converter pipeline
func TestConverterPipeline(t *testing.T) {
	t.Parallel()

	// Create a complex pipeline with multiple converters
	pipe := calque.NewFlow()
	pipe.
		Use(inspect.Print("INPUT")).
		Use(text.Transform(strings.ToUpper)).
		Use(inspect.Print("TRANSFORMED")).
		Use(text.Transform(func(s string) string {
			return "Processed: " + s
		}))

	// Test with different input types
	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "String input",
			input:    "hello world",
			expected: "Processed: HELLO WORLD",
		},
		{
			name:     "JSON input",
			input:    convert.ToJSON(ProductInfo{Name: "test", Category: "test"}),
			expected: "Processed:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := pipe.Run(context.Background(), tc.input, &result)
			if err != nil {
				t.Fatalf("Pipeline failed: %v", err)
			}

			if !strings.Contains(result, tc.expected) {
				t.Errorf("Expected result to contain '%s', got '%s'", tc.expected, result)
			}
		})
	}
}
