package indexing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

func TestParseMarkdownFile(t *testing.T) {
	testdataDir := "testdata"

	tests := []struct {
		name            string
		filename        string
		wantTitle       string
		wantTagsCount   int
		wantHasMetadata bool
		wantErr         bool
	}{
		{
			name:            "with frontmatter list tags",
			filename:        "go-microservices.md",
			wantTitle:       "Go Microservices Architecture",
			wantTagsCount:   3,
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "with frontmatter inline tags",
			filename:        "kubernetes-docker.md",
			wantTitle:       "Kubernetes with Docker",
			wantTagsCount:   3,
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "without frontmatter",
			filename:        "no-frontmatter.md",
			wantTitle:       "Simple Note",
			wantTagsCount:   0,
			wantHasMetadata: false,
			wantErr:         false,
		},
		{
			name:            "multiple headings (takes first)",
			filename:        "multiple-headings.md",
			wantTitle:       "Main Title",
			wantTagsCount:   2,
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "empty file",
			filename:        "empty-file.md",
			wantTitle:       "",
			wantTagsCount:   0,
			wantHasMetadata: false,
			wantErr:         false,
		},
		{
			name:            "only frontmatter no content",
			filename:        "only-frontmatter.md",
			wantTitle:       "",
			wantTagsCount:   1,
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "no title heading",
			filename:        "no-title.md",
			wantTitle:       "",
			wantTagsCount:   1,
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "malformed frontmatter",
			filename:        "malformed-frontmatter.md",
			wantTitle:       "Malformed Frontmatter",
			wantTagsCount:   1, // Extracts what it can
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "with code blocks",
			filename:        "with-code.md",
			wantTitle:       "Code Examples",
			wantTagsCount:   2,
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "inline tags only",
			filename:        "inline-tags.md",
			wantTitle:       "Document with Inline Tags",
			wantTagsCount:   10, // golang, microservices, docker, kubernetes, devops, nested/tag, multi-level/deep/tag, tag1, tag2, tag3
			wantHasMetadata: false,
			wantErr:         false,
		},
		{
			name:            "mixed frontmatter and inline tags",
			filename:        "mixed-tags.md",
			wantTitle:       "Mixed Tags Document",
			wantTagsCount:   4, // yaml-tag, frontmatter-tag, inline-tag, another-tag (no duplicates)
			wantHasMetadata: true,
			wantErr:         false,
		},
		{
			name:            "tags in code blocks ignored",
			filename:        "tags-in-code.md",
			wantTitle:       "Tags in Code Blocks",
			wantTagsCount:   2, // real-tag, actual-tag (fake-tag and not-a-tag ignored)
			wantHasMetadata: false,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(testdataDir, tt.filename)

			doc, err := parseMarkdownFile(filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMarkdownFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if doc.Title != tt.wantTitle {
					t.Errorf("Title = %v, want %v", doc.Title, tt.wantTitle)
				}

				if len(doc.Tags) != tt.wantTagsCount {
					t.Errorf("Tags count = %v, want %v (tags: %v)", len(doc.Tags), tt.wantTagsCount, doc.Tags)
				}

				if doc.Path != filePath {
					t.Errorf("Path = %v, want %v", doc.Path, filePath)
				}

				if tt.wantHasMetadata && len(doc.Metadata) == 0 {
					t.Errorf("Expected metadata but got none")
				}

				if !tt.wantHasMetadata && len(doc.Metadata) > 0 {
					t.Errorf("Expected no metadata but got some")
				}
			}
		})
	}
}

func TestParseMarkdownFileErrors(t *testing.T) {
	// Test with non-existent file
	_, err := parseMarkdownFile("/nonexistent/file.md")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test with directory instead of file
	_, err = parseMarkdownFile("testdata")
	if err == nil {
		t.Error("Expected error when reading directory")
	}
}

func TestExtractInlineTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single tag",
			content: "This is about #golang programming.",
			want:    []string{"golang"},
		},
		{
			name:    "multiple tags",
			content: "Topics: #golang #docker #kubernetes",
			want:    []string{"golang", "docker", "kubernetes"},
		},
		{
			name:    "nested tag",
			content: "Using #nested/tag format.",
			want:    []string{"nested/tag"},
		},
		{
			name:    "multi-level nested tag",
			content: "Deep nesting: #level1/level2/level3",
			want:    []string{"level1/level2/level3"},
		},
		{
			name:    "tag with hyphen",
			content: "Using #my-tag format.",
			want:    []string{"my-tag"},
		},
		{
			name:    "tag with underscore",
			content: "Using #my_tag format.",
			want:    []string{"my_tag"},
		},
		{
			name:    "tag at start of line",
			content: "#tag at the beginning",
			want:    []string{"tag"},
		},
		{
			name:    "heading should not be tag",
			content: "# This is a heading\nSome content with #real-tag",
			want:    []string{"real-tag"},
		},
		{
			name:    "tag in inline code ignored",
			content: "Use `#not-a-tag` in code but #real-tag outside.",
			want:    []string{"real-tag"},
		},
		{
			name:    "tag in code block ignored",
			content: "```\n# comment with #fake-tag\n```\nBut #real-tag here.",
			want:    []string{"real-tag"},
		},
		{
			name:    "no tags",
			content: "Just plain text without any tags.",
			want:    []string{},
		},
		{
			name:    "hash in middle of word",
			content: "Variable like var#123 should not extract tag.",
			want:    []string{},
		},
		{
			name:    "duplicate tags",
			content: "Multiple #tag mentions of the same #tag",
			want:    []string{"tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInlineTags(tt.content)

			// Sort both slices for comparison
			if len(got) != len(tt.want) {
				t.Errorf("extractInlineTags() length = %v, want %v\nGot: %v\nWant: %v",
					len(got), len(tt.want), got, tt.want)
				return
			}

			// Check each expected tag is present
			gotMap := make(map[string]bool)
			for _, tag := range got {
				gotMap[tag] = true
			}

			for _, wantTag := range tt.want {
				if !gotMap[wantTag] {
					t.Errorf("extractInlineTags() missing tag %v\nGot: %v\nWant: %v",
						wantTag, got, tt.want)
				}
			}
		})
	}
}

func TestExtractTags(t *testing.T) {
	tests := []struct {
		name     string
		metadata string
		want     []string
	}{
		{
			name: "list format",
			metadata: `title: Test
tags:
  - golang
  - testing
  - ci-cd`,
			want: []string{"golang", "testing", "ci-cd"},
		},
		{
			name: "inline format",
			metadata: `title: Test
tags: [golang, testing, ci-cd]`,
			want: []string{"golang", "testing", "ci-cd"},
		},
		{
			name:     "inline with spaces",
			metadata: `tags: [ golang , testing , ci-cd ]`,
			want:     []string{"golang", "testing", "ci-cd"},
		},
		{
			name:     "no tags",
			metadata: `title: Test\nauthor: John`,
			want:     []string{},
		},
		{
			name:     "empty metadata",
			metadata: ``,
			want:     []string{},
		},
		{
			name: "tags field but empty",
			metadata: `title: Test
tags:
author: John`,
			want: []string{},
		},
		{
			name:     "tags with special characters",
			metadata: `tags: [go-lang, test_case, tag.name]`,
			want:     []string{"go-lang", "test_case", "tag.name"},
		},
		{
			name:     "single tag inline",
			metadata: `tags: [single]`,
			want:     []string{"single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTags(tt.metadata)
			if len(got) != len(tt.want) {
				t.Errorf("extractTags() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i, tag := range got {
				if tag != tt.want[i] {
					t.Errorf("extractTags()[%d] = %v, want %v", i, tag, tt.want[i])
				}
			}
		})
	}
}

func TestBuildIndexMapping(t *testing.T) {
	mapping := buildIndexMapping()
	if mapping == nil {
		t.Fatal("buildIndexMapping() returned nil")
	}

	// Verify we can use the mapping to create an index
	tmpDir, err := os.MkdirTemp("", "mapping-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create index with mapping: %v", err)
	}
	defer index.Close()

	// Test indexing a document
	testDoc := &MarkdownDoc{
		Path:      "test.md",
		Title:     "Test",
		Content:   "Test content",
		Tags:      []string{"test"},
		Wikilinks: []string{"TestNote"},
		Metadata:  "title: Test",
	}

	if err := index.Index("test", testDoc); err != nil {
		t.Fatalf("Failed to index document: %v", err)
	}

	// Verify we can search
	query := bleve.NewMatchQuery("test")
	search := bleve.NewSearchRequest(query)
	results, err := index.Search(search)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if results.Total == 0 {
		t.Error("Expected to find indexed document")
	}
}

func TestIndexMarkdownFiles(t *testing.T) {
	testdataDir := "testdata"
	tmpDir, err := os.MkdirTemp("", "index-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")

	// Test creating new index
	index, err := IndexMarkdownFiles(indexPath, testdataDir)
	if err != nil {
		t.Fatalf("IndexMarkdownFiles() error = %v", err)
	}

	// Verify index was created
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("Index directory was not created")
	}

	// Verify document count (all .md files in testdata + subdir)
	count, err := index.DocCount()
	if err != nil {
		t.Fatalf("Failed to get doc count: %v", err)
	}
	expectedCount := uint64(20) // All .md files including subdir
	if count != expectedCount {
		t.Errorf("DocCount = %d, want %d", count, expectedCount)
	}

	// Close index before reopening
	index.Close()

	// Test opening existing index
	index2, err := IndexMarkdownFiles(indexPath, testdataDir)
	if err != nil {
		t.Fatalf("Failed to open existing index: %v", err)
	}
	defer index2.Close()

	// Should have more documents (reopening adds again)
	count2, err := index2.DocCount()
	if err != nil {
		t.Fatalf("Failed to get doc count: %v", err)
	}
	if count2 < count {
		t.Errorf("DocCount after reopen = %d, should be >= %d", count2, count)
	}
}

func TestIndexMarkdownFilesErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "error-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with non-existent docs path
	indexPath := filepath.Join(tmpDir, "test.bleve")
	_, err = IndexMarkdownFiles(indexPath, "/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent docs path")
	}
}

func TestIndexMarkdownFilesEmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "empty-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty docs dir
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.Mkdir(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}

	indexPath := filepath.Join(tmpDir, "test.bleve")
	index, err := IndexMarkdownFiles(indexPath, docsDir)
	if err != nil {
		t.Fatalf("IndexMarkdownFiles() error = %v", err)
	}
	defer index.Close()

	count, err := index.DocCount()
	if err != nil {
		t.Fatalf("Failed to get doc count: %v", err)
	}
	if count != 0 {
		t.Errorf("DocCount = %d, want 0 for empty directory", count)
	}
}

func TestMarkdownDocStruct(t *testing.T) {
	doc := &MarkdownDoc{
		Path:      "/path/to/note.md",
		Title:     "Test Note",
		Content:   "Test content",
		Tags:      []string{"tag1", "tag2"},
		Wikilinks: []string{"Note1", "Note2"},
		Metadata:  "title: Test",
	}

	if doc.Path != "/path/to/note.md" {
		t.Errorf("Path = %v, want /path/to/note.md", doc.Path)
	}
	if doc.Title != "Test Note" {
		t.Errorf("Title = %v, want Test Note", doc.Title)
	}
	if doc.Content != "Test content" {
		t.Errorf("Content = %v, want Test content", doc.Content)
	}
	if len(doc.Tags) != 2 {
		t.Errorf("Tags length = %v, want 2", len(doc.Tags))
	}
	if len(doc.Wikilinks) != 2 {
		t.Errorf("Wikilinks length = %v, want 2", len(doc.Wikilinks))
	}
	if doc.Metadata != "title: Test" {
		t.Errorf("Metadata = %v, want 'title: Test'", doc.Metadata)
	}
}

func TestIndexingIntegration(t *testing.T) {
	testdataDir := "testdata"
	tmpDir, err := os.MkdirTemp("", "integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")
	index, err := IndexMarkdownFiles(indexPath, testdataDir)
	if err != nil {
		t.Fatalf("IndexMarkdownFiles() error = %v", err)
	}
	defer index.Close()

	// Verify we can search the indexed content
	query := bleve.NewMatchQuery("kubernetes")
	search := bleve.NewSearchRequest(query)
	results, err := index.Search(search)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results.Total == 0 {
		t.Error("Expected search results but got none")
	}

	// Test tag search
	tagQuery := bleve.NewMatchQuery("golang")
	tagQuery.SetField("tags")
	tagSearch := bleve.NewSearchRequest(tagQuery)
	tagResults, err := index.Search(tagSearch)
	if err != nil {
		t.Fatalf("Tag search failed: %v", err)
	}
	if tagResults.Total == 0 {
		t.Error("Expected tag search results but got none")
	}
}

func TestIndexWithSubdirectories(t *testing.T) {
	testdataDir := "testdata"
	tmpDir, err := os.MkdirTemp("", "subdir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")
	index, err := IndexMarkdownFiles(indexPath, testdataDir)
	if err != nil {
		t.Fatalf("IndexMarkdownFiles() error = %v", err)
	}
	defer index.Close()

	// Verify subdirectory files are indexed
	query := bleve.NewMatchQuery("nested")
	search := bleve.NewSearchRequest(query)
	results, err := index.Search(search)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results.Total == 0 {
		t.Error("Expected to find nested file in subdirectory")
	}
}

func TestIndexIgnoresNonMarkdownFiles(t *testing.T) {
	testdataDir := "testdata"
	tmpDir, err := os.MkdirTemp("", "ignore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")
	index, err := IndexMarkdownFiles(indexPath, testdataDir)
	if err != nil {
		t.Fatalf("IndexMarkdownFiles() error = %v", err)
	}
	defer index.Close()

	// Search for content from .txt file (should not be found)
	query := bleve.NewMatchQuery("ignored")
	search := bleve.NewSearchRequest(query)
	results, err := index.Search(search)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	// readme.txt should not be indexed
	for _, hit := range results.Hits {
		if filepath.Ext(hit.ID) == ".txt" {
			t.Error("Non-markdown file was indexed")
		}
	}
}

func TestExtractWikilinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single basic wikilink",
			content: "See [[Getting Started]] for more info.",
			want:    []string{"Getting Started"},
		},
		{
			name:    "multiple wikilinks",
			content: "Read [[Introduction]], [[Guide]], and [[FAQ]].",
			want:    []string{"Introduction", "Guide", "FAQ"},
		},
		{
			name:    "wikilink with alias",
			content: "Check [[Configuration Options|config]] for settings.",
			want:    []string{"Configuration Options"},
		},
		{
			name:    "wikilink with heading",
			content: "See [[Installation#Prerequisites]] first.",
			want:    []string{"Installation#Prerequisites"},
		},
		{
			name:    "wikilink with heading and alias",
			content: "Read [[API Docs#Auth|authentication guide]] for details.",
			want:    []string{"API Docs#Auth"},
		},
		{
			name:    "multiple aliases same link",
			content: "See [[Note|alias1]] and [[Note|alias2]].",
			want:    []string{"Note"}, // Deduplicated
		},
		{
			name:    "wikilink in code block ignored",
			content: "Valid: [[Note1]]\n```\n[[Ignored]]\n```\nValid: [[Note2]]",
			want:    []string{"Note1", "Note2"},
		},
		{
			name:    "wikilink in inline code ignored",
			content: "Valid [[Note1]] but `[[Ignored]]` in code. Also [[Note2]].",
			want:    []string{"Note1", "Note2"},
		},
		{
			name:    "no wikilinks",
			content: "Just plain text with [regular links](url).",
			want:    []string{},
		},
		{
			name:    "empty wikilink",
			content: "Empty [[ ]] should be ignored.",
			want:    []string{},
		},
		{
			name:    "nested wikilinks",
			content: "Links: [[path/to/note]] and [[another/nested/note]]",
			want:    []string{"path/to/note", "another/nested/note"},
		},
		{
			name:    "wikilink with special characters",
			content: "Link to [[Note-with-dashes]] and [[Note_with_underscores]]",
			want:    []string{"Note-with-dashes", "Note_with_underscores"},
		},
		{
			name:    "embed without block reference",
			content: "Image: ![[parse-int-golang.png]]",
			want:    []string{"parse-int-golang.png"},
		},
		{
			name:    "embed with block reference",
			content: "See ![[Date Formats#^e4a164|RFC3339]] for details.",
			want:    []string{"Date Formats"},
		},
		{
			name:    "embed with block reference no alias",
			content: "Check ![[Golang FAQs#^46e652]] here.",
			want:    []string{"Golang FAQs"},
		},
		{
			name:    "mixed wikilinks and embeds",
			content: "Link [[Note1]] and embed ![[Note2#^ref123|content]]",
			want:    []string{"Note1", "Note2"},
		},
		{
			name:    "duplicate links with different block refs",
			content: "![[Note#^abc]] and ![[Note#^def]]",
			want:    []string{"Note"}, // Deduplicated after stripping block refs
		},
		{
			name:    "multiple embeds",
			content: "![[img1.png]] and ![[img2.png]]",
			want:    []string{"img1.png", "img2.png"},
		},
		{
			name:    "embed in code block ignored",
			content: "Valid: ![[Note1]]\n```\n![[Ignored]]\n```\nValid: ![[Note2]]",
			want:    []string{"Note1", "Note2"},
		},
		{
			name:    "embed in inline code ignored",
			content: "Valid ![[Note1]] but `![[Ignored]]` in code. Also ![[Note2]].",
			want:    []string{"Note1", "Note2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWikilinks(tt.content)

			if len(got) != len(tt.want) {
				t.Errorf("extractWikilinks() length = %v, want %v\nGot: %v\nWant: %v",
					len(got), len(tt.want), got, tt.want)
				return
			}

			// Check each expected wikilink is present
			gotMap := make(map[string]bool)
			for _, link := range got {
				gotMap[link] = true
			}

			for _, wantLink := range tt.want {
				if !gotMap[wantLink] {
					t.Errorf("extractWikilinks() missing link %v\nGot: %v\nWant: %v",
						wantLink, got, tt.want)
				}
			}
		})
	}
}

func TestExtractLinkFromWikilink(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "basic note",
			content: "Note Name",
			want:    "Note Name",
		},
		{
			name:    "note with alias",
			content: "Note Name|alias text",
			want:    "Note Name",
		},
		{
			name:    "note with heading",
			content: "Note#Heading",
			want:    "Note#Heading",
		},
		{
			name:    "note with heading and alias",
			content: "Note#Heading|alias",
			want:    "Note#Heading",
		},
		{
			name:    "note with spaces",
			content: "  Note With Spaces  ",
			want:    "Note With Spaces",
		},
		{
			name:    "note with spaces and alias",
			content: "  Note  |  alias  ",
			want:    "Note",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "only alias separator",
			content: "|",
			want:    "",
		},
		{
			name:    "note with block reference",
			content: "Date Formats#^e4a164",
			want:    "Date Formats",
		},
		{
			name:    "note with block reference and alias",
			content: "Date Formats#^e4a164|RFC3339",
			want:    "Date Formats",
		},
		{
			name:    "note with block reference no spaces",
			content: "Golang FAQs#^46e652",
			want:    "Golang FAQs",
		},
		{
			name:    "note with heading (not block ref)",
			content: "Note#Heading",
			want:    "Note#Heading",
		},
		{
			name:    "note with heading and alias (not block ref)",
			content: "Note#Heading|alias",
			want:    "Note#Heading",
		},
		{
			name:    "image file",
			content: "parse-int-golang.png",
			want:    "parse-int-golang.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLinkFromWikilink(tt.content)
			if got != tt.want {
				t.Errorf("extractLinkFromWikilink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMarkdownFileWithWikilinks(t *testing.T) {
	testdataDir := "testdata"

	tests := []struct {
		name               string
		filename           string
		wantWikilinksCount int
	}{
		{
			name:               "basic wikilinks",
			filename:           "wikilinks-basic.md",
			wantWikilinksCount: 6, // Getting Started, Advanced Topics, FAQ, Tutorials, Examples, API Reference
		},
		{
			name:               "wikilinks with aliases",
			filename:           "wikilinks-with-aliases.md",
			wantWikilinksCount: 5, // Introduction, Configuration Options, Troubleshooting Guide, API Documentation, Best Practices
		},
		{
			name:               "wikilinks with headings",
			filename:           "wikilinks-with-headings.md",
			wantWikilinksCount: 6, // Installation#Prerequisites, Configuration#Basic Setup, Advanced Topics#Performance Tuning, Getting Started#Installation, API Reference#Authentication, Troubleshooting#Common Errors
		},
		{
			name:               "mixed wikilinks",
			filename:           "wikilinks-mixed.md",
			wantWikilinksCount: 8, // Note1-Note8 (deduplicated)
		},
		{
			name:               "wikilinks in code ignored",
			filename:           "wikilinks-in-code.md",
			wantWikilinksCount: 3, // Valid Note, Another Valid Note, Final Note
		},
		{
			name:               "no wikilinks",
			filename:           "no-wikilinks.md",
			wantWikilinksCount: 0,
		},
		{
			name:               "golang tricks with embeds and block refs",
			filename:           "golang-tricks.md",
			wantWikilinksCount: 16, // parse-int-golang.png, parse-float-go.png, random, indexing, Interpolation, Date Formats, JSON, HTTP, GET, Golang, Golang FAQs (deduplicated from 2 block refs), POST, Deamon, while{}, for(), Queue, Graph
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(testdataDir, tt.filename)
			doc, err := parseMarkdownFile(filePath)
			if err != nil {
				t.Fatalf("parseMarkdownFile() error = %v", err)
			}

			if len(doc.Wikilinks) != tt.wantWikilinksCount {
				t.Errorf("Wikilinks count = %v, want %v (wikilinks: %v)",
					len(doc.Wikilinks), tt.wantWikilinksCount, doc.Wikilinks)
			}
		})
	}
}
