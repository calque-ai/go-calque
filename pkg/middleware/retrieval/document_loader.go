package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

const (
	maxConcurrency = 3
	httpTimeout    = 30 * time.Second
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
		inputSources, err := resolveInputSources(r, sources)
		if err != nil {
			return err
		}

		// Load documents from all sources
		allDocuments, err := loadDocuments(r.Context, inputSources)
		if err != nil {
			return err
		}

		// Write documents as JSON array
		result, err := json.Marshal(allDocuments)
		if err != nil {
			return err
		}

		return calque.Write(w, result)
	})
}

// loadDocuments loads documents from multiple sources using concurrent workers
func loadDocuments(ctx context.Context, sources []string) ([]Document, error) {
	var allDocuments []Document
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent workers to avoid overwhelming system
	semaphore := make(chan struct{}, maxConcurrency)
	errors := make(chan error, len(sources))

	for _, source := range sources {
		wg.Add(1)
		go func(src string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire worker slot
			defer func() { <-semaphore }() // Release worker slot

			docs, err := loadFromSource(ctx, src)
			if err != nil {
				errors <- calque.WrapErr(ctx, err, fmt.Sprintf("failed to load from source %s", src))
				return
			}

			mu.Lock()
			allDocuments = append(allDocuments, docs...)
			mu.Unlock()
		}(source)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	if err := <-errors; err != nil {
		return nil, err
	}

	return allDocuments, nil
}

// resolveInputSources determines the list of sources to load from, either from parameters or input
func resolveInputSources(r *calque.Request, paramSources []string) ([]string, error) {
	if len(paramSources) > 0 {
		return paramSources, nil
	}

	var input string
	err := calque.Read(r, &input)
	if err != nil {
		return nil, err
	}

	return parseSourceString(input)
}

// parseSourceString parses a string input into a list of sources
func parseSourceString(input string) ([]string, error) {
	ctx := context.Background()
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, calque.NewErr(ctx, "no input sources provided")
	}

	// Try to parse as JSON array first
	if strings.HasPrefix(input, "[") {
		var sources []string
		if err := json.Unmarshal([]byte(input), &sources); err == nil {
			return sources, nil
		}
	}

	// Fall back to single source
	return []string{input}, nil
}

// loadFromSource loads documents from a single source (file path or URL)
func loadFromSource(ctx context.Context, source string) ([]Document, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return loadFromURL(ctx, source)
	}
	return loadFromFilePattern(ctx, source)
}

// loadFromURL loads document from HTTP/HTTPS URL
func loadFromURL(ctx context.Context, url string) ([]Document, error) {
	client := &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			MaxIdleConns:      10,
			IdleConnTimeout:   30 * time.Second,
			DisableKeepAlives: false,
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			calque.LogWarn(ctx, "failed to close response body", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, calque.NewErr(ctx, fmt.Sprintf("HTTP error %d: %s", resp.StatusCode, resp.Status))
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
func loadFromFilePattern(ctx context.Context, pattern string) ([]Document, error) {
	// Path sanitization - prevent directory traversal attacks
	cleanPattern := filepath.Clean(pattern)
	if strings.Contains(cleanPattern, "..") {
		return nil, calque.NewErr(ctx, fmt.Sprintf("invalid path pattern: %s (contains directory traversal)", pattern))
	}

	// Handle glob patterns
	matches, err := filepath.Glob(cleanPattern)
	if err != nil {
		return nil, err
	}

	// If no matches found and pattern doesn't contain wildcards, try as direct path
	if len(matches) == 0 && !strings.ContainsAny(pattern, "*?[]") {
		matches = []string{pattern}
	}

	documents := make([]Document, 0, len(matches))
	for _, path := range matches {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Additional path sanitization for each match
		cleanPath := filepath.Clean(path)
		if strings.Contains(cleanPath, "..") {
			continue // Skip potentially dangerous paths
		}

		// Check if it's a file (skip directories)
		info, err := os.Stat(cleanPath)
		if err != nil {
			continue // Skip files that can't be accessed
		}
		if info.IsDir() {
			continue
		}

		// Load file content
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Create document from file
		doc := Document{
			ID:      cleanPath,
			Content: string(content),
			Metadata: map[string]any{
				"source":    cleanPath,
				"size":      info.Size(),
				"extension": filepath.Ext(cleanPath),
			},
			Created: info.ModTime(),
			Updated: info.ModTime(),
		}

		documents = append(documents, doc)
	}

	return documents, nil
}
