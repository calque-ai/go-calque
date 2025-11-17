package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// TestDocumentLoaderFromFiles tests loading documents from local files
//
//nolint:gocyclo // Table-driven test with many cases
func TestDocumentLoaderFromFiles(t *testing.T) {
	// Create temporary test directory with files
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		setupFn   func(t *testing.T) []string
		sources   []string
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, docs []Document)
	}{
		{
			name: "load single file successfully",
			setupFn: func(t *testing.T) []string {
				testFile := filepath.Join(tempDir, "test1.txt")
				if err := os.WriteFile(testFile, []byte("This is test content"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return []string{testFile}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 1 {
					t.Errorf("Expected 1 document, got %d", len(docs))
				}
				if len(docs) > 0 && docs[0].Content != "This is test content" {
					t.Errorf("Expected content 'This is test content', got %q", docs[0].Content)
				}
			},
		},
		{
			name: "load multiple files from glob pattern",
			setupFn: func(t *testing.T) []string {
				for i := 1; i <= 3; i++ {
					testFile := filepath.Join(tempDir, fmt.Sprintf("glob_test_%d.md", i))
					content := fmt.Sprintf("Content for file %d", i)
					if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
						t.Fatalf("Failed to create test file: %v", err)
					}
				}
				pattern := filepath.Join(tempDir, "glob_test_*.md")
				return []string{pattern}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 3 {
					t.Errorf("Expected 3 documents, got %d", len(docs))
				}
				for _, doc := range docs {
					if !strings.HasPrefix(doc.Content, "Content for file") {
						t.Errorf("Unexpected content: %q", doc.Content)
					}
				}
			},
		},
		{
			name: "load from multiple sources",
			setupFn: func(t *testing.T) []string {
				file1 := filepath.Join(tempDir, "multi1.txt")
				file2 := filepath.Join(tempDir, "multi2.txt")
				if err := os.WriteFile(file1, []byte("First file"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				if err := os.WriteFile(file2, []byte("Second file"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return []string{file1, file2}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 2 {
					t.Errorf("Expected 2 documents, got %d", len(docs))
				}
			},
		},
		{
			name: "documents include metadata",
			setupFn: func(t *testing.T) []string {
				testFile := filepath.Join(tempDir, "metadata_test.json")
				if err := os.WriteFile(testFile, []byte(`{"key": "value"}`), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return []string{testFile}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) == 0 {
					t.Fatal("Expected at least one document")
				}
				doc := docs[0]
				if doc.Metadata == nil {
					t.Fatal("Expected metadata to be non-nil")
				}
				if doc.Metadata["source"] == nil {
					t.Error("Expected source metadata")
				}
				if doc.Metadata["extension"] == nil {
					t.Error("Expected extension metadata")
				}
				if doc.Metadata["size"] == nil {
					t.Error("Expected size metadata")
				}
			},
		},
		{
			name: "empty sources list attempts to read from input and errors",
			setupFn: func(_ *testing.T) []string {
				return []string{}
			},
			expectErr: true,
			errMsg:    "no input sources provided",
		},
		{
			name: "non-existent file is skipped",
			setupFn: func(_ *testing.T) []string {
				return []string{filepath.Join(tempDir, "non_existent.txt")}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 0 {
					t.Errorf("Expected 0 documents for non-existent file, got %d", len(docs))
				}
			},
		},
		{
			name: "directory traversal is blocked",
			setupFn: func(_ *testing.T) []string {
				return []string{"../../etc/passwd"}
			},
			expectErr: true,
			errMsg:    "directory traversal",
		},
		{
			name: "glob pattern with no matches returns empty",
			setupFn: func(_ *testing.T) []string {
				pattern := filepath.Join(tempDir, "nomatch_*.xyz")
				return []string{pattern}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 0 {
					t.Errorf("Expected 0 documents for non-matching pattern, got %d", len(docs))
				}
			},
		},
		{
			name: "files have timestamps populated",
			setupFn: func(t *testing.T) []string {
				testFile := filepath.Join(tempDir, "timestamp_test.txt")
				if err := os.WriteFile(testFile, []byte("Test"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return []string{testFile}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) == 0 {
					t.Fatal("Expected at least one document")
				}
				doc := docs[0]
				if doc.Created.IsZero() {
					t.Error("Expected Created timestamp to be set")
				}
				if doc.Updated.IsZero() {
					t.Error("Expected Updated timestamp to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := tt.setupFn(t)

			// Create handler
			handler := DocumentLoader(sources...)

			// Create mock request/response
			req := calque.NewRequest(context.Background(), bytes.NewReader(nil))
			respBuf := &bytes.Buffer{}
			resp := calque.NewResponse(respBuf)

			// Execute handler
			err := handler.ServeFlow(req, resp)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Parse response
			var docs []Document
			if err := json.Unmarshal(respBuf.Bytes(), &docs); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, docs)
			}
		})
	}
}

// TestDocumentLoaderFromURL tests loading documents from HTTP URLs
func TestDocumentLoaderFromURL(t *testing.T) {
	tests := []struct {
		name      string
		setupFn   func() (*httptest.Server, []string)
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, docs []Document)
	}{
		{
			name: "load from HTTP URL successfully",
			setupFn: func() (*httptest.Server, []string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					w.Write([]byte("HTTP content"))
				}))
				return server, []string{server.URL}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 1 {
					t.Fatalf("Expected 1 document, got %d", len(docs))
				}
				if docs[0].Content != "HTTP content" {
					t.Errorf("Expected content 'HTTP content', got %q", docs[0].Content)
				}
				if docs[0].Metadata["content_type"] != "text/plain" {
					t.Errorf("Expected content_type 'text/plain', got %v", docs[0].Metadata["content_type"])
				}
			},
		},
		{
			name: "load JSON from URL",
			setupFn: func() (*httptest.Server, []string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"data": "json content"}`))
				}))
				return server, []string{server.URL}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 1 {
					t.Fatalf("Expected 1 document, got %d", len(docs))
				}
				if !strings.Contains(docs[0].Content, "json content") {
					t.Errorf("Expected JSON content, got %q", docs[0].Content)
				}
			},
		},
		{
			name: "HTTP error status returns error",
			setupFn: func() (*httptest.Server, []string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("Not found"))
				}))
				return server, []string{server.URL}
			},
			expectErr: true,
			errMsg:    "404",
		},
		{
			name: "multiple URLs can be loaded",
			setupFn: func() (*httptest.Server, []string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Write([]byte("URL content"))
				}))
				// Use same server URL twice
				return server, []string{server.URL + "/path1", server.URL + "/path2"}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 2 {
					t.Errorf("Expected 2 documents, got %d", len(docs))
				}
			},
		},
		{
			name: "URL document has metadata",
			setupFn: func() (*httptest.Server, []string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/xml")
					w.Write([]byte("<data>xml</data>"))
				}))
				return server, []string{server.URL}
			},
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) == 0 {
					t.Fatal("Expected at least one document")
				}
				doc := docs[0]
				if doc.Metadata["source"] == nil {
					t.Error("Expected source metadata")
				}
				if doc.Metadata["content_type"] != "application/xml" {
					t.Errorf("Expected content_type 'application/xml', got %v", doc.Metadata["content_type"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, sources := tt.setupFn()
			defer server.Close()

			// Create handler
			handler := DocumentLoader(sources...)

			// Create mock request/response
			req := calque.NewRequest(context.Background(), bytes.NewReader(nil))
			respBuf := &bytes.Buffer{}
			resp := calque.NewResponse(respBuf)

			// Execute handler
			err := handler.ServeFlow(req, resp)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Parse response
			var docs []Document
			if err := json.Unmarshal(respBuf.Bytes(), &docs); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, docs)
			}
		})
	}
}

// TestDocumentLoaderFromInput tests loading sources from request input
func TestDocumentLoaderFromInput(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "input_test.txt")
	if err := os.WriteFile(testFile, []byte("Input test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		input     string
		expectErr bool
		errMsg    string
		checkFn   func(t *testing.T, docs []Document)
	}{
		{
			name:      "load from single source string input",
			input:     testFile,
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 1 {
					t.Errorf("Expected 1 document, got %d", len(docs))
				}
				if len(docs) > 0 && docs[0].Content != "Input test content" {
					t.Errorf("Expected 'Input test content', got %q", docs[0].Content)
				}
			},
		},
		{
			name:      "load from JSON array input",
			input:     fmt.Sprintf(`["%s"]`, testFile),
			expectErr: false,
			checkFn: func(t *testing.T, docs []Document) {
				if len(docs) != 1 {
					t.Errorf("Expected 1 document, got %d", len(docs))
				}
			},
		},
		{
			name:      "empty input returns error",
			input:     "",
			expectErr: true,
			errMsg:    "no input sources provided",
		},
		{
			name:      "whitespace-only input returns error",
			input:     "   \t\n  ",
			expectErr: true,
			errMsg:    "no input sources provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler without sources (will use input)
			handler := DocumentLoader()

			// Create request with input
			req := calque.NewRequest(context.Background(), bytes.NewBufferString(tt.input))
			respBuf := &bytes.Buffer{}
			resp := calque.NewResponse(respBuf)

			// Execute handler
			err := handler.ServeFlow(req, resp)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Parse response
			var docs []Document
			if err := json.Unmarshal(respBuf.Bytes(), &docs); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, docs)
			}
		})
	}
}

// TestDocumentLoaderConcurrency tests concurrent document loading
func TestDocumentLoaderConcurrency(t *testing.T) {
	// Create multiple test files
	tempDir := t.TempDir()
	numFiles := 10
	sources := make([]string, numFiles)

	for i := 0; i < numFiles; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", i))
		content := fmt.Sprintf("Content for file %d", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		sources[i] = filename
	}

	// Load documents
	handler := DocumentLoader(sources...)
	req := calque.NewRequest(context.Background(), bytes.NewReader(nil))
	respBuf := &bytes.Buffer{}
	resp := calque.NewResponse(respBuf)

	if err := handler.ServeFlow(req, resp); err != nil {
		t.Fatalf("DocumentLoader failed: %v", err)
	}

	// Verify all documents loaded
	var docs []Document
	if err := json.Unmarshal(respBuf.Bytes(), &docs); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(docs) != numFiles {
		t.Errorf("Expected %d documents, got %d", numFiles, len(docs))
	}

	// Verify each document is unique
	seen := make(map[string]bool)
	for _, doc := range docs {
		if seen[doc.ID] {
			t.Errorf("Duplicate document ID: %s", doc.ID)
		}
		seen[doc.ID] = true
	}
}
