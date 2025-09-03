package retrieval

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// DocumentLoader creates a document loading middleware.
//
// Input: string or []string of source paths/URLs
// Output: []Document JSON array with loaded documents
// Behavior: BUFFERED - reads input sources and loads all documents
//
// Loads and streams documents from various sources including files and URLs.
// Supports glob patterns for file paths and handles common document formats.
//
// Supported sources:
// - File paths: "./docs/*.md", "/path/to/file.txt"
// - URLs: "https://api.example.com/docs"
//
// Example:
//
//	flow := calque.NewFlow().
//	    Use(retrieval.DocumentLoader(
//	        "./docs/*.md",                    // Local markdown files
//	        "https://api.company.com/kb",     // Knowledge base API
//	    ))
func DocumentLoader(sources ...string) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var inputSources []string

		// If no sources provided as parameters, read from input
		if len(sources) == 0 {
			var input string
			err := calque.Read(r, &input)
			if err != nil {
				return err
			}

			// Try to parse as JSON array first
			if strings.HasPrefix(strings.TrimSpace(input), "[") {
				if err := json.Unmarshal([]byte(input), &inputSources); err != nil {
					// If JSON parsing fails, treat as single source
					inputSources = []string{input}
				}
			} else {
				inputSources = []string{input}
			}
		} else {
			inputSources = sources
		}

		// Load documents from all sources
		var allDocuments []Document
		for _, source := range inputSources {
			docs, err := loadFromSource(source)
			if err != nil {
				return fmt.Errorf("failed to load from source %s: %w", source, err)
			}
			allDocuments = append(allDocuments, docs...)
		}

		// Write documents as JSON array
		result, err := json.Marshal(allDocuments)
		if err != nil {
			return err
		}

		return calque.Write(w, result)
	})
}

// loadFromSource loads documents from a single source (file path or URL)
func loadFromSource(source string) ([]Document, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return loadFromURL(source)
	}
	return loadFromFilePattern(source)
}

// loadFromURL loads document from HTTP/HTTPS URL
func loadFromURL(url string) ([]Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Create document from URL content
	doc := Document{
		ID:      url,
		Content: string(content),
		Metadata: map[string]any{
			"source":       url,
			"content_type": resp.Header.Get("Content-Type"),
		},
		Created: time.Now(),
		Updated: time.Now(),
	}

	return []Document{doc}, nil
}

// loadFromFilePattern loads documents from file path (supports glob patterns)
func loadFromFilePattern(pattern string) ([]Document, error) {
	// Handle glob patterns
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	// If no matches found and pattern doesn't contain wildcards, try as direct path
	if len(matches) == 0 && !strings.ContainsAny(pattern, "*?[]") {
		matches = []string{pattern}
	}

	documents := make([]Document, 0, len(matches))
	for _, path := range matches {
		// Check if it's a file (skip directories)
		info, err := os.Stat(path)
		if err != nil {
			continue // Skip files that can't be accessed
		}
		if info.IsDir() {
			continue
		}

		// Load file content
		content, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Create document from file
		doc := Document{
			ID:      path,
			Content: string(content),
			Metadata: map[string]any{
				"source":    path,
				"size":      info.Size(),
				"extension": filepath.Ext(path),
			},
			Created: info.ModTime(),
			Updated: info.ModTime(),
		}

		documents = append(documents, doc)
	}

	return documents, nil
}
