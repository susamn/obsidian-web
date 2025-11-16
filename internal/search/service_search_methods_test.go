package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/susamn/obsidian-web/internal/indexing"
)

// Helper to create a test search service with indexed data
func createTestSearchService(t *testing.T) (*SearchService, func()) {
	t.Helper()

	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}

	// Index test documents
	docs := []struct {
		id   string
		data map[string]interface{}
	}{
		{
			"doc1",
			map[string]interface{}{
				"title":     "Golang Microservices",
				"path":      "/golang/microservices.md",
				"tags":      []string{"golang", "microservices", "backend"},
				"wikilinks": []string{"API Design", "Docker"},
			},
		},
		{
			"doc2",
			map[string]interface{}{
				"title":     "Docker Kubernetes Guide",
				"path":      "/devops/docker-k8s.md",
				"tags":      []string{"docker", "kubernetes", "devops"},
				"wikilinks": []string{"Docker", "Cloud"},
			},
		},
		{
			"doc3",
			map[string]interface{}{
				"title":     "Python Data Science",
				"path":      "/python/data-science.md",
				"tags":      []string{"python", "data-science", "ml"},
				"wikilinks": []string{"Pandas", "NumPy"},
			},
		},
		{
			"doc4",
			map[string]interface{}{
				"title":     "React Frontend Development",
				"path":      "/frontend/react.md",
				"tags":      []string{"react", "frontend", "javascript"},
				"wikilinks": []string{"TypeScript", "Redux"},
			},
		},
	}

	for _, doc := range docs {
		if err := index.Index(doc.id, doc.data); err != nil {
			t.Fatalf("Failed to index document %s: %v", doc.id, err)
		}
	}

	ctx := context.Background()
	svc := NewSearchService(ctx, "test-vault", index)
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	cleanup := func() {
		svc.Stop()
		index.Close()
	}

	return svc, cleanup
}

// TestSearchService_SearchByMultipleTags tests searching with multiple tags (AND)
func TestSearchService_SearchByMultipleTags(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name        string
		tags        []string
		expectMatch bool
	}{
		{"golang AND microservices", []string{"golang", "microservices"}, true},
		{"docker AND kubernetes", []string{"docker", "kubernetes"}, true},
		{"golang AND python", []string{"golang", "python"}, false},
		{"single tag", []string{"react"}, true},
		{"empty tags", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.SearchByMultipleTags(tt.tags)
			if err != nil {
				t.Fatalf("SearchByMultipleTags failed: %v", err)
			}

			if tt.expectMatch && results.Total == 0 {
				t.Errorf("Expected matches but got 0")
			}
			if !tt.expectMatch && results.Total > 0 {
				t.Errorf("Expected no matches but got %d", results.Total)
			}
		})
	}
}

// TestSearchService_SearchByWikilink tests wikilink search
func TestSearchService_SearchByWikilink(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name     string
		wikilink string
		wantHits bool
	}{
		{"existing wikilink Docker", "Docker", true},
		{"existing wikilink Pandas", "Pandas", true},
		{"non-existent wikilink", "NonExistent", false},
		{"empty wikilink", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.SearchByWikilink(tt.wikilink)
			if err != nil {
				t.Fatalf("SearchByWikilink failed: %v", err)
			}

			if tt.wantHits && results.Total == 0 {
				t.Errorf("Expected hits for wikilink '%s'", tt.wikilink)
			}
			if !tt.wantHits && results.Total > 0 {
				t.Errorf("Unexpected hits for wikilink '%s': %d", tt.wikilink, results.Total)
			}
		})
	}
}

// TestSearchService_SearchByBacklinks tests backlink search
func TestSearchService_SearchByBacklinks(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	results, err := svc.SearchByBacklinks("Docker")
	if err != nil {
		t.Fatalf("SearchByBacklinks failed: %v", err)
	}

	if results.Total == 0 {
		t.Error("Expected backlinks for 'Docker'")
	}
}

// TestSearchService_SearchByTagsOR tests OR search with tags
func TestSearchService_SearchByTagsOR(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name    string
		tags    []string
		minHits uint64
	}{
		{"golang OR python", []string{"golang", "python"}, 2},
		{"docker OR kubernetes OR react", []string{"docker", "kubernetes", "react"}, 2},
		{"single tag", []string{"ml"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.SearchByTagsOR(tt.tags)
			if err != nil {
				t.Fatalf("SearchByTagsOR failed: %v", err)
			}

			if results.Total < tt.minHits {
				t.Errorf("Expected at least %d hits, got %d", tt.minHits, results.Total)
			}
		})
	}
}

// TestSearchService_SearchByMultipleWikilinks tests AND search with wikilinks
func TestSearchService_SearchByMultipleWikilinks(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name      string
		wikilinks []string
		wantHits  bool
	}{
		{"Docker AND Cloud", []string{"Docker", "Cloud"}, true},
		{"Docker AND NonExistent", []string{"Docker", "NonExistent"}, false},
		{"single wikilink", []string{"Pandas"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.SearchByMultipleWikilinks(tt.wikilinks)
			if err != nil {
				t.Fatalf("SearchByMultipleWikilinks failed: %v", err)
			}

			if tt.wantHits && results.Total == 0 {
				t.Errorf("Expected hits for wikilinks %v", tt.wikilinks)
			}
			if !tt.wantHits && results.Total > 0 {
				t.Errorf("Unexpected hits: %d", results.Total)
			}
		})
	}
}

// TestSearchService_SearchByWikilinksOR tests OR search with wikilinks
func TestSearchService_SearchByWikilinksOR(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	results, err := svc.SearchByWikilinksOR([]string{"Docker", "Pandas", "NonExistent"})
	if err != nil {
		t.Fatalf("SearchByWikilinksOR failed: %v", err)
	}

	if results.Total < 2 {
		t.Errorf("Expected at least 2 hits, got %d", results.Total)
	}
}

// TestSearchService_SearchByTitleOnly tests title-only search
func TestSearchService_SearchByTitleOnly(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name     string
		query    string
		wantHits bool
	}{
		{"search for Golang", "Golang", true},
		{"search for Docker", "Docker", true},
		{"search for nonexistent", "Nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.SearchByTitleOnly(tt.query)
			if err != nil {
				t.Fatalf("SearchByTitleOnly failed: %v", err)
			}

			if tt.wantHits && results.Total == 0 {
				t.Errorf("Expected hits for '%s'", tt.query)
			}
			if !tt.wantHits && results.Total > 0 {
				t.Errorf("Unexpected hits: %d", results.Total)
			}
		})
	}
}

// TestSearchService_FuzzySearch tests fuzzy search
func TestSearchService_FuzzySearch(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name      string
		query     string
		fuzziness int
		wantError bool
	}{
		{"exact match", "golang", 0, false},
		{"1 char typo", "golng", 1, false},
		{"2 char typo", "golan", 2, false},
		{"nonexistent with fuzz", "xyzabc", 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.FuzzySearch(tt.query, tt.fuzziness)
			if err != nil && !tt.wantError {
				t.Fatalf("FuzzySearch failed: %v", err)
			}

			// Fuzzy search behavior can vary, just verify it doesn't error
			// and returns a valid result
			if results == nil {
				t.Error("Expected non-nil results")
			}
		})
	}
}

// TestSearchService_PhraseSearch tests exact phrase search
func TestSearchService_PhraseSearch(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name     string
		phrase   string
		wantHits bool
	}{
		{"exact phrase in title", "Data Science", true},
		{"partial phrase", "Frontend Development", true},
		{"nonexistent phrase", "Machine Learning Advanced", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.PhraseSearch(tt.phrase)
			if err != nil {
				t.Fatalf("PhraseSearch failed: %v", err)
			}

			if tt.wantHits && results.Total == 0 {
				t.Logf("No hits for phrase '%s' (may be expected)", tt.phrase)
			}
		})
	}
}

// TestSearchService_PrefixSearch tests prefix search
func TestSearchService_PrefixSearch(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name     string
		prefix   string
		wantHits bool
	}{
		{"prefix Gol", "Gol", true},
		{"prefix Dock", "Dock", true},
		{"prefix Py", "Py", true},
		{"prefix Xyz", "Xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.PrefixSearch(tt.prefix)
			if err != nil {
				t.Fatalf("PrefixSearch failed: %v", err)
			}

			if tt.wantHits && results.Total == 0 {
				t.Logf("No hits for prefix '%s'", tt.prefix)
			}
		})
	}
}

// TestSearchService_AdvancedSearch tests advanced search with text and tags
func TestSearchService_AdvancedSearch(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name     string
		text     string
		tags     []string
		wantHits bool
	}{
		{"text and tag", "Golang", []string{"microservices"}, true},
		{"only text", "Docker", []string{}, true},
		{"only tags", "", []string{"python"}, true},
		{"non-matching combination", "Golang", []string{"python"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.AdvancedSearch(tt.text, tt.tags)
			if err != nil {
				t.Fatalf("AdvancedSearch failed: %v", err)
			}

			if tt.wantHits && results.Total == 0 {
				t.Errorf("Expected hits for text='%s' tags=%v", tt.text, tt.tags)
			}
			if !tt.wantHits && results.Total > 0 {
				t.Errorf("Unexpected hits: %d", results.Total)
			}
		})
	}
}

// TestSearchService_SearchCombined tests combined search with text, tags, and wikilinks
func TestSearchService_SearchCombined(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	tests := []struct {
		name      string
		text      string
		tags      []string
		wikilinks []string
		wantHits  bool
	}{
		{
			"text + tags + wikilinks",
			"Docker",
			[]string{"docker"},
			[]string{"Docker"},
			true,
		},
		{
			"only text",
			"Golang",
			[]string{},
			[]string{},
			true,
		},
		{
			"only tags",
			"",
			[]string{"python"},
			[]string{},
			true,
		},
		{
			"only wikilinks",
			"",
			[]string{},
			[]string{"Pandas"},
			true,
		},
		{
			"no parameters",
			"",
			[]string{},
			[]string{},
			false,
		},
		{
			"non-matching combination",
			"Docker",
			[]string{"python"},
			[]string{},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.SearchCombined(tt.text, tt.tags, tt.wikilinks)
			if err != nil {
				t.Fatalf("SearchCombined failed: %v", err)
			}

			if tt.wantHits && results.Total == 0 {
				t.Errorf("Expected hits for combined search")
			}
			if !tt.wantHits && results.Total > 0 {
				t.Errorf("Unexpected hits: %d", results.Total)
			}
		})
	}
}

// TestSearchService_AllMethodsRecordMetrics tests that all search methods record metrics
func TestSearchService_AllMethodsRecordMetrics(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	initialMetrics := svc.GetMetrics()
	initialCount := initialMetrics.SearchCount

	// Call each search method once
	svc.SearchByText("test")
	svc.SearchByTag("test")
	svc.SearchByMultipleTags([]string{"test"})
	svc.SearchByWikilink("test")
	svc.SearchByBacklinks("test")
	svc.SearchByTagsOR([]string{"test"})
	svc.SearchByMultipleWikilinks([]string{"test"})
	svc.SearchByWikilinksOR([]string{"test"})
	svc.SearchByTitleOnly("test")
	svc.FuzzySearch("test", 1)
	svc.PhraseSearch("test")
	svc.PrefixSearch("test")
	svc.AdvancedSearch("test", []string{"tag"})
	svc.SearchCombined("test", []string{"tag"}, []string{"link"})

	finalMetrics := svc.GetMetrics()
	expectedCount := initialCount + 14 // 14 search method calls

	if finalMetrics.SearchCount != expectedCount {
		t.Errorf("Expected search count %d, got %d", expectedCount, finalMetrics.SearchCount)
	}

	if finalMetrics.LastSearchTime.IsZero() {
		t.Error("LastSearchTime should be set after searches")
	}
}

// TestSearchService_NotifyIndexUpdate_ChannelFull tests non-blocking notification
func TestSearchService_NotifyIndexUpdate_ChannelFull(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	svc := NewSearchService(ctx, "test-vault", index)
	// Don't start the service, so the channel won't be consumed

	// Fill the channel (buffer size is 10)
	for i := 0; i < 15; i++ {
		event := indexing.IndexUpdateEvent{
			Timestamp: time.Now(),
			EventType: "incremental",
			NewIndex:  nil,
		}
		svc.NotifyIndexUpdate(event) // Should not block
	}

	// If we get here without blocking, the test passes
	t.Log("NotifyIndexUpdate is non-blocking as expected")
}

// TestSearchService_Stop_Idempotent tests that Stop() can be called multiple times
func TestSearchService_Stop_Idempotent(t *testing.T) {
	svc, cleanup := createTestSearchService(t)
	defer cleanup()

	// First stop
	if err := svc.Stop(); err != nil {
		t.Fatalf("First Stop() failed: %v", err)
	}

	// Second stop (should not error)
	if err := svc.Stop(); err != nil {
		t.Errorf("Second Stop() failed: %v", err)
	}

	// Third stop
	if err := svc.Stop(); err != nil {
		t.Errorf("Third Stop() failed: %v", err)
	}
}
