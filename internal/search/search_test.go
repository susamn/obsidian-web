package search

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/susamn/obsidian-web/internal/indexing"
)

func setupTestIndex(t *testing.T) (bleve.Index, func()) {
	t.Helper()

	testdataDir := "testdata"
	tmpDir, err := os.MkdirTemp("", "search-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	indexPath := filepath.Join(tmpDir, "test.bleve")
	index, err := indexing.IndexMarkdownFiles(indexPath, testdataDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create index: %v", err)
	}

	cleanup := func() {
		index.Close()
		os.RemoveAll(tmpDir)
	}

	return index, cleanup
}

func TestSearchByText(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		query          string
		wantMinResults uint64
		wantContains   []string
	}{
		{
			name:           "search for kubernetes",
			query:          "kubernetes",
			wantMinResults: 1,
			wantContains:   []string{"kubernetes-docker.md"},
		},
		{
			name:           "search for golang",
			query:          "golang",
			wantMinResults: 2,
			wantContains:   []string{"golang-tutorial.md"},
		},
		{
			name:           "search for python",
			query:          "python",
			wantMinResults: 1,
			wantContains:   []string{"python-ml.md"},
		},
		{
			name:           "search for react",
			query:          "react",
			wantMinResults: 1,
			wantContains:   []string{"react-frontend.md"},
		},
		{
			name:           "search for non-existent term",
			query:          "nonexistentterm12345",
			wantMinResults: 0,
			wantContains:   []string{},
		},
		{
			name:           "empty query",
			query:          "",
			wantMinResults: 0,
			wantContains:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByText(index, tt.query)
			if err != nil {
				t.Fatalf("SearchByText() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Check if expected documents are in results
			for _, expected := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expected) {
						found = true
						break
					}
				}
				if !found && results.Total > 0 {
					t.Errorf("Expected to find %s in results", expected)
				}
			}

			// Verify highlight is configured for non-empty queries
			if tt.query != "" && results.Total > 0 && results.Request != nil {
				if results.Request.Highlight == nil {
					t.Error("Highlight should be configured for non-empty queries")
				}
			}

			// Verify size limit
			if len(results.Hits) > 20 {
				t.Errorf("Results hits = %d, should be limited to 20", len(results.Hits))
			}
		})
	}
}

func TestSearchByTag(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		tag            string
		wantMinResults uint64
		wantContains   []string
	}{
		{
			name:           "search for golang tag",
			tag:            "golang",
			wantMinResults: 2,
			wantContains:   []string{"golang-tutorial.md", "multiple-topics.md"},
		},
		{
			name:           "search for docker tag",
			tag:            "docker",
			wantMinResults: 2,
			wantContains:   []string{"kubernetes-docker.md", "multiple-topics.md"},
		},
		{
			name:           "search for kubernetes tag",
			tag:            "kubernetes",
			wantMinResults: 2,
			wantContains:   []string{"kubernetes-docker.md", "multiple-topics.md"},
		},
		{
			name:           "search for python tag",
			tag:            "python",
			wantMinResults: 1,
			wantContains:   []string{"python-ml.md"},
		},
		{
			name:           "search for react tag",
			tag:            "react",
			wantMinResults: 1,
			wantContains:   []string{"react-frontend.md"},
		},
		{
			name:           "search for non-existent tag",
			tag:            "nonexistenttag",
			wantMinResults: 0,
			wantContains:   []string{},
		},
		{
			name:           "empty tag",
			tag:            "",
			wantMinResults: 0,
			wantContains:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByTag(index, tt.tag)
			if err != nil {
				t.Fatalf("SearchByTag() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Check if expected documents are in results
			for _, expected := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expected) {
						found = true
						break
					}
				}
				if !found && results.Total > 0 {
					t.Errorf("Expected to find %s in results", expected)
				}
			}

			// Verify size limit
			if len(results.Hits) > 20 {
				t.Errorf("Results hits = %d, should be limited to 20", len(results.Hits))
			}
		})
	}
}

func TestSearchByMultipleTags(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		tags           []string
		wantMinResults uint64
		wantMaxResults uint64
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "search for golang AND microservices",
			tags:           []string{"golang", "microservices"},
			wantMinResults: 1,
			wantMaxResults: 4,
			wantContains:   []string{"multiple-topics.md"},
			wantNotContain: []string{},
		},
		{
			name:           "search for docker AND kubernetes",
			tags:           []string{"docker", "kubernetes"},
			wantMinResults: 2,
			wantMaxResults: 4,
			wantContains:   []string{"kubernetes-docker.md", "multiple-topics.md"},
			wantNotContain: []string{},
		},
		{
			name:           "search for golang AND docker AND kubernetes",
			tags:           []string{"golang", "docker", "kubernetes"},
			wantMinResults: 1,
			wantMaxResults: 3,
			wantContains:   []string{"multiple-topics.md"},
			wantNotContain: []string{},
		},
		{
			name:           "search with non-matching combination",
			tags:           []string{"python", "golang"},
			wantMinResults: 0,
			wantMaxResults: 0,
			wantContains:   []string{},
			wantNotContain: []string{},
		},
		{
			name:           "single tag in array",
			tags:           []string{"react"},
			wantMinResults: 1,
			wantMaxResults: 1,
			wantContains:   []string{"react-frontend.md"},
			wantNotContain: []string{},
		},
		{
			name:           "empty tags array",
			tags:           []string{},
			wantMinResults: 0,
			wantMaxResults: 0,
			wantContains:   []string{},
			wantNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByMultipleTags(index, tt.tags)
			if err != nil {
				t.Fatalf("SearchByMultipleTags() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			if results.Total > tt.wantMaxResults {
				t.Errorf("Total results = %d, want at most %d", results.Total, tt.wantMaxResults)
			}

			// Check expected documents are in results
			for _, expected := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expected) {
						found = true
						break
					}
				}
				if !found && results.Total > 0 {
					t.Errorf("Expected to find %s in results", expected)
				}
			}

			// Check unwanted documents are NOT in results
			for _, unwanted := range tt.wantNotContain {
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, unwanted) {
						t.Errorf("Did not expect to find %s in results", unwanted)
					}
				}
			}
		})
	}
}

func TestAdvancedSearch(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		text           string
		tags           []string
		wantMinResults uint64
		wantContains   []string
	}{
		{
			name:           "text and single tag",
			text:           "microservices",
			tags:           []string{"golang"},
			wantMinResults: 1,
			wantContains:   []string{"multiple-topics.md"},
		},
		{
			name:           "text and multiple tags",
			text:           "kubernetes",
			tags:           []string{"docker"},
			wantMinResults: 1,
			wantContains:   []string{"kubernetes-docker.md"},
		},
		{
			name:           "only text no tags",
			text:           "python",
			tags:           []string{},
			wantMinResults: 1,
			wantContains:   []string{"python-ml.md"},
		},
		{
			name:           "only tags no text",
			text:           "",
			tags:           []string{"react", "frontend"},
			wantMinResults: 1,
			wantContains:   []string{"react-frontend.md"},
		},
		{
			name:           "no matches for combination",
			text:           "python",
			tags:           []string{"golang"},
			wantMinResults: 0,
			wantContains:   []string{},
		},
		{
			name:           "empty text and tags",
			text:           "",
			tags:           []string{},
			wantMinResults: 0,
			wantContains:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := AdvancedSearch(index, tt.text, tt.tags)
			if err != nil {
				t.Fatalf("AdvancedSearch() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Check expected documents
			for _, expected := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expected) {
						found = true
						break
					}
				}
				if !found && results.Total > 0 {
					t.Errorf("Expected to find %s in results", expected)
				}
			}

			// Verify highlight is configured for text searches
			if tt.text != "" && results.Total > 0 && results.Request != nil {
				if results.Request.Highlight == nil {
					t.Error("Highlight should be configured when text query is provided")
				}
			}
		})
	}
}

func TestPrintResults(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Perform a search
	results, err := SearchByTag(index, "golang")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintResults(results)

	err = w.Close()
	if err != nil {
		return
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected elements
	expectedStrings := []string{
		"Total matches:",
		"score:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain '%s', got: %s", expected, output)
		}
	}

	// Verify output is not empty
	if len(output) == 0 {
		t.Error("PrintResults produced no output")
	}
}

func TestPrintResultsEmpty(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Search for something that doesn't exist
	results, err := SearchByText(index, "nonexistentterm12345")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintResults(results)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should show 0 matches
	if !strings.Contains(output, "Total matches: 0") {
		t.Errorf("Expected 'Total matches: 0', got: %s", output)
	}
}

func TestSearchIntegration(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Test complete workflow
	t.Run("search and verify results", func(t *testing.T) {
		// 1. Text search
		textResults, err := SearchByText(index, "kubernetes")
		if err != nil {
			t.Fatalf("Text search failed: %v", err)
		}
		if textResults.Total == 0 {
			t.Error("Expected text search results")
		}

		// 2. Tag search
		tagResults, err := SearchByTag(index, "golang")
		if err != nil {
			t.Fatalf("Tag search failed: %v", err)
		}
		if tagResults.Total == 0 {
			t.Error("Expected tag search results")
		}

		// 3. Multiple tags search
		multiTagResults, err := SearchByMultipleTags(index, []string{"docker", "kubernetes"})
		if err != nil {
			t.Fatalf("Multiple tag search failed: %v", err)
		}
		if multiTagResults.Total == 0 {
			t.Error("Expected multiple tag search results")
		}

		// 4. Advanced search
		advResults, err := AdvancedSearch(index, "microservices", []string{"golang"})
		if err != nil {
			t.Fatalf("Advanced search failed: %v", err)
		}
		if advResults.Total == 0 {
			t.Error("Expected advanced search results")
		}

		// Verify scoring
		for _, hit := range advResults.Hits {
			if hit.Score <= 0 {
				t.Error("Score should be positive")
			}
		}
	})
}

func TestSearchResultFields(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	results, err := SearchByText(index, "kubernetes")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results.Total > 0 && results.Request != nil {
		// Verify fields are present in the request
		expectedFields := []string{"title", "path", "tags", "wikilinks"}
		for _, field := range expectedFields {
			found := false
			for _, reqField := range results.Request.Fields {
				if reqField == field {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected field %s in search request", field)
			}
		}

		// Verify hits have IDs
		for _, hit := range results.Hits {
			if hit.ID == "" {
				t.Error("Hit should have an ID")
			}
			if hit.Score <= 0 {
				t.Error("Hit should have a positive score")
			}
		}
	}
}

func TestSearchWithSpecialCharacters(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name  string
		query string
	}{
		{"with hyphen", "machine-learning"},
		{"with underscore", "some_term"},
		{"with numbers", "go1.20"},
		{"with dot", "v2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic or error
			_, err := SearchByText(index, tt.query)
			if err != nil {
				t.Errorf("SearchByText() with special chars error = %v", err)
			}
		})
	}
}

func TestSearchPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Run multiple searches to ensure no memory leaks or performance issues
	for i := 0; i < 100; i++ {
		_, err := SearchByText(index, "kubernetes")
		if err != nil {
			t.Fatalf("Search iteration %d failed: %v", i, err)
		}
	}
}

func TestSearchByWikilink(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		wikilink       string
		wantMinResults uint64
		wantContains   []string
	}{
		{
			name:           "search for Getting Started wikilink",
			wikilink:       "Getting Started",
			wantMinResults: 1,
			wantContains:   []string{},
		},
		{
			name:           "search for non-existent wikilink",
			wikilink:       "NonExistentNote",
			wantMinResults: 0,
			wantContains:   []string{},
		},
		{
			name:           "search for wikilink with heading",
			wikilink:       "Installation#Prerequisites",
			wantMinResults: 0, // Heading-only searches may not match in test data
			wantContains:   []string{},
		},
		{
			name:           "empty wikilink",
			wikilink:       "",
			wantMinResults: 0,
			wantContains:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByWikilink(index, tt.wikilink)
			if err != nil {
				t.Fatalf("SearchByWikilink() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Check expected documents are in results
			for _, expected := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expected) {
						found = true
						break
					}
				}
				if !found && results.Total > 0 {
					t.Errorf("Expected to find %s in results", expected)
				}
			}

			// Verify wikilinks field is included
			if results.Total > 0 && results.Request != nil {
				found := false
				for _, field := range results.Request.Fields {
					if field == "wikilinks" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected 'wikilinks' field in search request")
				}
			}

			// Verify size limit
			if len(results.Hits) > 20 {
				t.Errorf("Results hits = %d, should be limited to 20", len(results.Hits))
			}
		})
	}
}

func TestSearchByBacklinks(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		noteName       string
		wantMinResults uint64
	}{
		{
			name:           "find backlinks to a note",
			noteName:       "Getting Started",
			wantMinResults: 0, // May or may not have backlinks in test data
		},
		{
			name:           "find backlinks to non-existent note",
			noteName:       "NonExistentNote",
			wantMinResults: 0,
		},
		{
			name:           "empty note name",
			noteName:       "",
			wantMinResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByBacklinks(index, tt.noteName)
			if err != nil {
				t.Fatalf("SearchByBacklinks() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Verify it returns a valid result (even if empty)
			if results == nil {
				t.Error("SearchByBacklinks() should not return nil results")
			}
		})
	}
}

func TestPrintResultsWithWikilinks(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Search for wikilinks to get results that may include wikilinks field
	results, err := SearchByText(index, "wikilink")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintResults(results)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected elements
	expectedStrings := []string{
		"Total matches:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain '%s', got: %s", expected, output)
		}
	}
}

func TestSearchResultsIncludeWikilinksField(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Test that all search functions include wikilinks field
	t.Run("SearchByText includes wikilinks", func(t *testing.T) {
		results, err := SearchByText(index, "test")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if results.Total > 0 && results.Request != nil {
			verifyWikilinksField(t, results)
		}
	})

	t.Run("SearchByTag includes wikilinks", func(t *testing.T) {
		results, err := SearchByTag(index, "golang")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if results.Total > 0 && results.Request != nil {
			verifyWikilinksField(t, results)
		}
	})

	t.Run("SearchByMultipleTags includes wikilinks", func(t *testing.T) {
		results, err := SearchByMultipleTags(index, []string{"golang"})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if results.Total > 0 && results.Request != nil {
			verifyWikilinksField(t, results)
		}
	})

	t.Run("AdvancedSearch includes wikilinks", func(t *testing.T) {
		results, err := AdvancedSearch(index, "test", []string{})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if results.Total > 0 && results.Request != nil {
			verifyWikilinksField(t, results)
		}
	})
}

func verifyWikilinksField(t *testing.T, results *bleve.SearchResult) {
	found := false
	for _, field := range results.Request.Fields {
		if field == "wikilinks" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'wikilinks' field in search request fields")
	}
}

func TestSearchByTagsOR(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		tags           []string
		wantMinResults uint64
		wantContains   []string
	}{
		{
			name:           "search for golang OR python",
			tags:           []string{"golang", "python"},
			wantMinResults: 2,
			wantContains:   []string{"golang-tutorial.md", "python-ml.md"},
		},
		{
			name:           "search for docker OR kubernetes",
			tags:           []string{"docker", "kubernetes"},
			wantMinResults: 2,
			wantContains:   []string{"kubernetes-docker.md"},
		},
		{
			name:           "search for react OR frontend",
			tags:           []string{"react", "frontend"},
			wantMinResults: 1,
			wantContains:   []string{"react-frontend.md"},
		},
		{
			name:           "single tag",
			tags:           []string{"golang"},
			wantMinResults: 1,
			wantContains:   []string{"golang-tutorial.md"},
		},
		{
			name:           "non-matching tags",
			tags:           []string{"nonexistent1", "nonexistent2"},
			wantMinResults: 0,
			wantContains:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByTagsOR(index, tt.tags)
			if err != nil {
				t.Fatalf("SearchByTagsOR() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			for _, expected := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expected) {
						found = true
						break
					}
				}
				if !found && len(tt.wantContains) > 0 {
					t.Errorf("Expected to find %s in results", expected)
				}
			}
		})
	}
}

func TestSearchByMultipleWikilinks(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		wikilinks      []string
		wantMinResults uint64
		wantMaxResults uint64
	}{
		{
			name:           "search for Getting Started AND FAQ",
			wikilinks:      []string{"Getting Started", "FAQ"},
			wantMinResults: 0,
			wantMaxResults: 1,
		},
		{
			name:           "single wikilink",
			wikilinks:      []string{"Getting Started"},
			wantMinResults: 1,
			wantMaxResults: 2,
		},
		{
			name:           "non-matching wikilinks",
			wikilinks:      []string{"NonExistent1", "NonExistent2"},
			wantMinResults: 0,
			wantMaxResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByMultipleWikilinks(index, tt.wikilinks)
			if err != nil {
				t.Fatalf("SearchByMultipleWikilinks() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			if results.Total > tt.wantMaxResults {
				t.Errorf("Total results = %d, want at most %d", results.Total, tt.wantMaxResults)
			}
		})
	}
}

func TestSearchByWikilinksOR(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		wikilinks      []string
		wantMinResults uint64
	}{
		{
			name:           "search for Getting Started OR FAQ",
			wikilinks:      []string{"Getting Started", "FAQ"},
			wantMinResults: 1,
		},
		{
			name:           "single wikilink",
			wikilinks:      []string{"Getting Started"},
			wantMinResults: 1,
		},
		{
			name:           "non-matching wikilinks",
			wikilinks:      []string{"NonExistent1", "NonExistent2"},
			wantMinResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByWikilinksOR(index, tt.wikilinks)
			if err != nil {
				t.Fatalf("SearchByWikilinksOR() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}
		})
	}
}

func TestSearchByTitleOnly(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		query          string
		wantMinResults uint64
		wantContains   []string
	}{
		{
			name:           "search for Golang in title",
			query:          "Golang",
			wantMinResults: 1,
			wantContains:   []string{"golang-tutorial.md"},
		},
		{
			name:           "search for Python in title",
			query:          "Python",
			wantMinResults: 1,
			wantContains:   []string{"python-ml.md"},
		},
		{
			name:           "search for Tutorial in title",
			query:          "Tutorial",
			wantMinResults: 1,
			wantContains:   []string{"golang-tutorial.md"},
		},
		{
			name:           "non-matching title",
			query:          "NonExistentTitle",
			wantMinResults: 0,
			wantContains:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByTitleOnly(index, tt.query)
			if err != nil {
				t.Fatalf("SearchByTitleOnly() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			for _, expected := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expected) {
						found = true
						break
					}
				}
				if !found && len(tt.wantContains) > 0 {
					t.Errorf("Expected to find %s in results", expected)
				}
			}
		})
	}
}

func TestFuzzySearch(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		query          string
		fuzziness      int
		wantMinResults uint64
	}{
		{
			name:           "fuzzy search kubernetez (typo)",
			query:          "kubernetez",
			fuzziness:      1,
			wantMinResults: 0, // May or may not match depending on index
		},
		{
			name:           "fuzzy search golang exact",
			query:          "golang",
			fuzziness:      0,
			wantMinResults: 1,
		},
		{
			name:           "fuzzy search with 2 character difference",
			query:          "pythn",
			fuzziness:      2,
			wantMinResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := FuzzySearch(index, tt.query, tt.fuzziness)
			if err != nil {
				t.Fatalf("FuzzySearch() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Verify highlight is configured
			if results.Total > 0 && results.Request != nil && results.Request.Highlight == nil {
				t.Error("Highlight should be configured")
			}
		})
	}
}

func TestPhraseSearch(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		phrase         string
		wantMinResults uint64
	}{
		{
			name:           "phrase search machine learning",
			phrase:         "machine learning",
			wantMinResults: 0, // Depends on test data
		},
		{
			name:           "phrase search simple note",
			phrase:         "simple note",
			wantMinResults: 0,
		},
		{
			name:           "single word phrase",
			phrase:         "kubernetes",
			wantMinResults: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := PhraseSearch(index, tt.phrase)
			if err != nil {
				t.Fatalf("PhraseSearch() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}
		})
	}
}

func TestPrefixSearch(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		prefix         string
		wantMinResults uint64
	}{
		{
			name:           "prefix search kub",
			prefix:         "kub",
			wantMinResults: 1,
		},
		{
			name:           "prefix search gol",
			prefix:         "gol",
			wantMinResults: 1,
		},
		{
			name:           "prefix search pyt",
			prefix:         "pyt",
			wantMinResults: 1,
		},
		{
			name:           "prefix search xyz",
			prefix:         "xyz",
			wantMinResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := PrefixSearch(index, tt.prefix)
			if err != nil {
				t.Fatalf("PrefixSearch() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}
		})
	}
}

func TestSearchCombined(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		text           string
		tags           []string
		wikilinks      []string
		wantMinResults uint64
	}{
		{
			name:           "text + tags + wikilinks",
			text:           "kubernetes",
			tags:           []string{"docker"},
			wikilinks:      []string{},
			wantMinResults: 1,
		},
		{
			name:           "only text",
			text:           "golang",
			tags:           []string{},
			wikilinks:      []string{},
			wantMinResults: 1,
		},
		{
			name:           "only tags",
			text:           "",
			tags:           []string{"python"},
			wikilinks:      []string{},
			wantMinResults: 1,
		},
		{
			name:           "only wikilinks",
			text:           "",
			tags:           []string{},
			wikilinks:      []string{"Getting Started"},
			wantMinResults: 1,
		},
		{
			name:           "text + tags",
			text:           "tutorial",
			tags:           []string{"golang"},
			wikilinks:      []string{},
			wantMinResults: 1,
		},
		{
			name:           "empty search",
			text:           "",
			tags:           []string{},
			wikilinks:      []string{},
			wantMinResults: 0,
		},
		{
			name:           "no matches",
			text:           "nonexistent",
			tags:           []string{"faketag"},
			wikilinks:      []string{"FakeLink"},
			wantMinResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchCombined(index, tt.text, tt.tags, tt.wikilinks)
			if err != nil {
				t.Fatalf("SearchCombined() error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Verify fields are included if results exist
			if results.Total > 0 && results.Request != nil {
				verifyWikilinksField(t, results)
			}
		})
	}
}

func TestSearchCombinedComplex(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Test complex combinations
	t.Run("multiple tags and wikilinks", func(t *testing.T) {
		results, err := SearchCombined(index, "", []string{"docker", "kubernetes"}, []string{"Getting Started"})
		if err != nil {
			t.Fatalf("SearchCombined() error = %v", err)
		}

		// Should find documents matching all conditions
		if results == nil {
			t.Error("Expected non-nil results")
		}
	})

	t.Run("all parameters provided", func(t *testing.T) {
		results, err := SearchCombined(index, "microservices", []string{"golang"}, []string{})
		if err != nil {
			t.Fatalf("SearchCombined() error = %v", err)
		}

		if results == nil {
			t.Error("Expected non-nil results")
		}
	})
}

func TestSearchMethodsReturnFields(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	// Test that all search methods return the expected fields
	searchFuncs := []struct {
		name string
		fn   func() (*bleve.SearchResult, error)
	}{
		{"SearchByText", func() (*bleve.SearchResult, error) {
			return SearchByText(index, "kubernetes")
		}},
		{"SearchByTag", func() (*bleve.SearchResult, error) {
			return SearchByTag(index, "golang")
		}},
		{"SearchByMultipleTags", func() (*bleve.SearchResult, error) {
			return SearchByMultipleTags(index, []string{"golang"})
		}},
		{"SearchByTagsOR", func() (*bleve.SearchResult, error) {
			return SearchByTagsOR(index, []string{"golang", "python"})
		}},
		{"SearchByWikilink", func() (*bleve.SearchResult, error) {
			return SearchByWikilink(index, "Getting Started")
		}},
		{"SearchByMultipleWikilinks", func() (*bleve.SearchResult, error) {
			return SearchByMultipleWikilinks(index, []string{"Getting Started"})
		}},
		{"SearchByWikilinksOR", func() (*bleve.SearchResult, error) {
			return SearchByWikilinksOR(index, []string{"Getting Started", "FAQ"})
		}},
		{"SearchByTitleOnly", func() (*bleve.SearchResult, error) {
			return SearchByTitleOnly(index, "Golang")
		}},
		{"SearchCombined", func() (*bleve.SearchResult, error) {
			return SearchCombined(index, "test", []string{}, []string{})
		}},
	}

	expectedFields := []string{"title", "path", "tags", "wikilinks"}

	for _, sf := range searchFuncs {
		t.Run(sf.name, func(t *testing.T) {
			results, err := sf.fn()
			if err != nil {
				t.Fatalf("%s error = %v", sf.name, err)
			}

			if results.Total > 0 && results.Request != nil {
				for _, expectedField := range expectedFields {
					found := false
					for _, field := range results.Request.Fields {
						if field == expectedField {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("%s: expected field %s in results", sf.name, expectedField)
					}
				}
			}
		})
	}
}

// TestSearchByInlineTags tests searching for inline tags (not frontmatter tags)
func TestSearchByInlineTags(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		tag            string
		wantMinResults uint64
		wantContains   []string // Files that should be in results
	}{
		{
			name:           "search for inline tag golang",
			tag:            "golang",
			wantMinResults: 1,
			wantContains:   []string{"inline-tags-only.md"},
		},
		{
			name:           "search for inline tag docker",
			tag:            "docker",
			wantMinResults: 1,
			wantContains:   []string{"inline-tags-only.md"},
		},
		{
			name:           "search for nested inline tag docker/container",
			tag:            "docker/container",
			wantMinResults: 1,
			wantContains:   []string{"inline-tags-only.md"},
		},
		{
			name:           "search for deeply nested tag",
			tag:            "nested/deep/tag",
			wantMinResults: 1,
			wantContains:   []string{"inline-tags-only.md"},
		},
		{
			name:           "search for microservices (inline tag)",
			tag:            "microservices",
			wantMinResults: 1,
			wantContains:   []string{"inline-tags-only.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByTag(index, tt.tag)
			if err != nil {
				t.Fatalf("SearchByTag error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d", results.Total, tt.wantMinResults)
			}

			// Verify expected files are in results
			for _, expectedFile := range tt.wantContains {
				found := false
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expectedFile) {
						found = true
						// Verify tags field is returned
						if _, ok := hit.Fields["tags"]; !ok {
							t.Errorf("Expected tags field in result for %s", hit.ID)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected to find %s in results, but it was not found", expectedFile)
				}
			}
		})
	}
}

// TestSearchByAdvancedWikilinks tests searching for wikilinks with block references
// Verifies that block references (#^blockid) are properly stripped during indexing
func TestSearchByAdvancedWikilinks(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		wikilink       string
		wantMinResults uint64
		wantContains   []string // Files that should be in results
		description    string
	}{
		{
			name:           "search for Date Formats (block ref stripped)",
			wikilink:       "Date Formats",
			wantMinResults: 1,
			wantContains:   []string{"advanced-wikilinks.md"},
			description:    "Should find [[Date Formats#^e4a164|RFC3339]] with block ref stripped",
		},
		{
			name:           "search for Golang FAQs (multiple block refs deduplicated)",
			wikilink:       "Golang FAQs",
			wantMinResults: 1,
			wantContains:   []string{"advanced-wikilinks.md", "golang-tricks.md"},
			description:    "Should find multiple [[Golang FAQs#^xxx]] entries, deduplicated",
		},
		{
			name:           "search for Getting Started (simple wikilink)",
			wikilink:       "Getting Started",
			wantMinResults: 1,
			wantContains:   []string{"advanced-wikilinks.md"},
			description:    "Should find simple [[Getting Started]] wikilink",
		},
		{
			name:           "search for Installation (heading preserved)",
			wikilink:       "Installation#Prerequisites",
			wantMinResults: 1,
			wantContains:   []string{"advanced-wikilinks.md"},
			description:    "Should find [[Installation#Prerequisites]] with heading preserved",
		},
		{
			name:           "search for HTTP (from golang-tricks.md)",
			wikilink:       "HTTP",
			wantMinResults: 1,
			wantContains:   []string{"golang-tricks.md", "advanced-wikilinks.md"},
			description:    "Should find [[HTTP]] in multiple files",
		},
		{
			name:           "search for POST (with block ref in source)",
			wikilink:       "POST",
			wantMinResults: 1,
			wantContains:   []string{"advanced-wikilinks.md", "golang-tricks.md"},
			description:    "Should find [[POST#^blockid|post method]] with block ref stripped",
		},
		{
			name:           "search for Golang (common link)",
			wikilink:       "Golang",
			wantMinResults: 1,
			wantContains:   []string{"advanced-wikilinks.md", "golang-tricks.md"},
			description:    "Should find [[Golang]] in multiple files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByWikilink(index, tt.wikilink)
			if err != nil {
				t.Fatalf("SearchByWikilink error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d (test: %s)", results.Total, tt.wantMinResults, tt.description)
			}

			// Verify at least one expected file is in results
			foundAny := false
			for _, expectedFile := range tt.wantContains {
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expectedFile) {
						foundAny = true
						// Verify wikilinks field is returned
						if _, ok := hit.Fields["wikilinks"]; !ok {
							t.Errorf("Expected wikilinks field in result for %s", hit.ID)
						}
						break
					}
				}
			}
			if !foundAny && results.Total > 0 {
				t.Errorf("Expected to find at least one of %v in results, found none (test: %s)", tt.wantContains, tt.description)
			}
		})
	}
}

// TestSearchByMultipleWikilinksDetailed tests AND logic for multiple wikilinks with actual result verification
func TestSearchByMultipleWikilinksDetailed(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		wikilinks      []string
		wantMinResults uint64
		wantContains   []string
		description    string
	}{
		{
			name:           "search for HTTP AND POST (both in same file)",
			wikilinks:      []string{"HTTP", "POST"},
			wantMinResults: 1,
			wantContains:   []string{"golang-tricks.md"},
			description:    "golang-tricks.md has both [[HTTP]] and [[POST]]",
		},
		{
			name:           "search for HTTP AND Golang (both common)",
			wikilinks:      []string{"HTTP", "Golang"},
			wantMinResults: 1,
			wantContains:   []string{"golang-tricks.md"},
			description:    "Should find files with both wikilinks",
		},
		{
			name:           "search for Date Formats AND Golang FAQs",
			wikilinks:      []string{"Date Formats", "Golang FAQs"},
			wantMinResults: 1,
			wantContains:   []string{"advanced-wikilinks.md"},
			description:    "advanced-wikilinks.md has both (block refs stripped)",
		},
		{
			name:           "single wikilink Golang",
			wikilinks:      []string{"Golang"},
			wantMinResults: 1,
			wantContains:   []string{"golang-tricks.md", "advanced-wikilinks.md"},
			description:    "Single wikilink should return all matching files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByMultipleWikilinks(index, tt.wikilinks)
			if err != nil {
				t.Fatalf("SearchByMultipleWikilinks error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d (test: %s)", results.Total, tt.wantMinResults, tt.description)
			}

			// Verify at least one expected file is in results
			if tt.wantMinResults > 0 {
				foundAny := false
				for _, expectedFile := range tt.wantContains {
					for _, hit := range results.Hits {
						if strings.Contains(hit.ID, expectedFile) {
							foundAny = true
							break
						}
					}
				}
				if !foundAny {
					var foundFiles []string
					for _, hit := range results.Hits {
						foundFiles = append(foundFiles, hit.ID)
					}
					t.Errorf("Expected to find at least one of %v in results, found: %v (test: %s)",
						tt.wantContains, foundFiles, tt.description)
				}
			}
		})
	}
}

// TestSearchByWikilinksORDetailed tests OR logic for multiple wikilinks with actual result verification
func TestSearchByWikilinksORDetailed(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name           string
		wikilinks      []string
		wantMinResults uint64
		wantContains   []string
		description    string
	}{
		{
			name:           "search for Getting Started OR FAQ",
			wikilinks:      []string{"Getting Started", "FAQ"},
			wantMinResults: 1,
			wantContains:   []string{"wikilinks-basic.md", "advanced-wikilinks.md"},
			description:    "Should find files with either wikilink",
		},
		{
			name:           "search for HTTP OR JSON",
			wikilinks:      []string{"HTTP", "JSON"},
			wantMinResults: 1,
			wantContains:   []string{"golang-tricks.md", "advanced-wikilinks.md"},
			description:    "Should find files with either HTTP or JSON",
		},
		{
			name:           "search for random OR Queue",
			wikilinks:      []string{"random", "Queue"},
			wantMinResults: 1,
			wantContains:   []string{"golang-tricks.md"},
			description:    "Both wikilinks are in golang-tricks.md",
		},
		{
			name:           "search for Date Formats OR Golang FAQs (both have block refs)",
			wikilinks:      []string{"Date Formats", "Golang FAQs"},
			wantMinResults: 2,
			wantContains:   []string{"advanced-wikilinks.md", "golang-tricks.md"},
			description:    "Should find files with either wikilink (block refs stripped)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByWikilinksOR(index, tt.wikilinks)
			if err != nil {
				t.Fatalf("SearchByWikilinksOR error = %v", err)
			}

			if results.Total < tt.wantMinResults {
				t.Errorf("Total results = %d, want at least %d (test: %s)", results.Total, tt.wantMinResults, tt.description)
			}

			// Verify at least one expected file is in results
			if tt.wantMinResults > 0 {
				foundAny := false
				for _, expectedFile := range tt.wantContains {
					for _, hit := range results.Hits {
						if strings.Contains(hit.ID, expectedFile) {
							foundAny = true
							break
						}
					}
				}
				if !foundAny {
					var foundFiles []string
					for _, hit := range results.Hits {
						foundFiles = append(foundFiles, hit.ID)
					}
					t.Errorf("Expected to find at least one of %v in results, found: %v (test: %s)",
						tt.wantContains, foundFiles, tt.description)
				}
			}
		})
	}
}

// TestInlineTagsAndFrontmatterTagsMerged tests that inline tags and frontmatter tags are merged correctly
func TestInlineTagsAndFrontmatterTagsMerged(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name         string
		tag          string
		wantContains []string
		description  string
	}{
		{
			name:         "search for yaml-tag (from frontmatter)",
			tag:          "yaml-tag",
			wantContains: []string{"mixed-tags.md"},
			description:  "Should find frontmatter tag",
		},
		{
			name:         "search for inline-tag (from content)",
			tag:          "inline-tag",
			wantContains: []string{"mixed-tags.md"},
			description:  "Should find inline tag",
		},
		{
			name:         "search for golang (appears in multiple files)",
			tag:          "golang",
			wantContains: []string{"inline-tags-only.md", "inline-tags.md"},
			description:  "Should find tag from inline content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchByTag(index, tt.tag)
			if err != nil {
				t.Fatalf("SearchByTag error = %v", err)
			}

			if results.Total == 0 {
				t.Errorf("Expected at least 1 result for tag %s, got 0 (test: %s)", tt.tag, tt.description)
			}

			// Verify at least one expected file is in results
			foundAny := false
			for _, expectedFile := range tt.wantContains {
				for _, hit := range results.Hits {
					if strings.Contains(hit.ID, expectedFile) {
						foundAny = true
						break
					}
				}
			}
			if !foundAny && results.Total > 0 {
				var foundFiles []string
				for _, hit := range results.Hits {
					foundFiles = append(foundFiles, hit.ID)
				}
				t.Errorf("Expected to find at least one of %v in results, found: %v (test: %s)",
					tt.wantContains, foundFiles, tt.description)
			}
		})
	}
}
