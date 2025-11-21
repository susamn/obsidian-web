package render

import (
	"testing"
	"time"
)

// MockFileResolver is a mock implementation for testing
type MockFileResolver struct {
	wikilinks map[string]struct {
		exists bool
		fileID string
		path   string
	}
	backlinks map[string][]Backlink
	tagCounts map[string]int
}

func NewMockFileResolver() *MockFileResolver {
	return &MockFileResolver{
		wikilinks: make(map[string]struct {
			exists bool
			fileID string
			path   string
		}),
		backlinks: make(map[string][]Backlink),
		tagCounts: make(map[string]int),
	}
}

func (m *MockFileResolver) ResolveWikiLink(vaultID, linkTarget string) (exists bool, fileID, path string) {
	if link, ok := m.wikilinks[linkTarget]; ok {
		return link.exists, link.fileID, link.path
	}
	return false, "", ""
}

func (m *MockFileResolver) GetBacklinks(vaultID, fileID string) []Backlink {
	return m.backlinks[fileID]
}

func (m *MockFileResolver) GetTagCount(vaultID, tag string) int {
	if count, ok := m.tagCounts[tag]; ok {
		return count
	}
	return 1
}

func TestExtractFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		expectedFM      int // number of frontmatter keys
		expectedContent string
		expectError     bool
	}{
		{
			name: "valid frontmatter",
			content: `---
title: Test Note
tags: [golang, testing]
---

# Content here`,
			expectedFM:      2,
			expectedContent: "# Content here",
			expectError:     false,
		},
		{
			name: "no frontmatter",
			content: `# Just a heading

Some content`,
			expectedFM:      0,
			expectedContent: "# Just a heading\n\nSome content",
			expectError:     false,
		},
		{
			name: "frontmatter with nested values",
			content: `---
title: Complex Note
author: John Doe
metadata:
  created: 2024-01-01
  tags: [test]
---

Content`,
			expectedFM:      3,
			expectedContent: "Content",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewStructuredRenderer(nil)
			fm, content, err := sr.extractFrontmatter(tt.content)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(fm) != tt.expectedFM {
				t.Errorf("Expected %d frontmatter keys, got %d", tt.expectedFM, len(fm))
			}

			if content != tt.expectedContent {
				t.Errorf("Expected content %q, got %q", tt.expectedContent, content)
			}
		})
	}
}

func TestExtractHeadings(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name: "multiple headings",
			content: `# Heading 1
## Heading 2
### Heading 3
#### Heading 4`,
			expected: 4,
		},
		{
			name: "headings with special characters",
			content: `# Hello World!
## Test-123
### Unicode: 你好`,
			expected: 3,
		},
		{
			name:     "no headings",
			content:  "Just plain text\nNo headings here",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewStructuredRenderer(nil)
			headings := sr.extractHeadings(tt.content)

			if len(headings) != tt.expected {
				t.Errorf("Expected %d headings, got %d", tt.expected, len(headings))
			}

			// Check that IDs are generated
			for _, h := range headings {
				if h.ID == "" {
					t.Errorf("Heading %q has empty ID", h.Text)
				}
				if h.Level < 1 || h.Level > 6 {
					t.Errorf("Heading %q has invalid level %d", h.Text, h.Level)
				}
			}
		})
	}
}

func TestExtractInlineTags(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "single tag",
			content:  "This is a note with #golang",
			expected: []string{"golang"},
		},
		{
			name:     "multiple tags",
			content:  "#golang #testing #backend",
			expected: []string{"golang", "testing", "backend"},
		},
		{
			name:     "tags with hyphens and underscores",
			content:  "#test-tag #another_tag #nested/tag",
			expected: []string{"test-tag", "another_tag", "nested/tag"},
		},
		{
			name:     "no tags",
			content:  "Just plain text without tags",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewStructuredRenderer(nil)
			tags := sr.extractInlineTags(tt.content)

			if len(tags) != len(tt.expected) {
				t.Errorf("Expected %d tags, got %d", len(tt.expected), len(tags))
			}

			tagMap := make(map[string]bool)
			for _, tag := range tags {
				tagMap[tag] = true
			}

			for _, expected := range tt.expected {
				if !tagMap[expected] {
					t.Errorf("Expected tag %q not found", expected)
				}
			}
		})
	}
}

func TestExtractWikiLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "simple wikilink",
			content:  "Link to [[Other Note]]",
			expected: 1,
		},
		{
			name:     "wikilink with display text",
			content:  "Link to [[Other Note|Display Text]]",
			expected: 1,
		},
		{
			name:     "multiple wikilinks",
			content:  "[[Note 1]] and [[Note 2]] and [[Note 3]]",
			expected: 3,
		},
		{
			name:     "no wikilinks",
			content:  "Just plain text",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewMockFileResolver()
			sr := NewStructuredRenderer(resolver)
			wikilinks := sr.extractWikiLinks(tt.content, "test-vault")

			if len(wikilinks) != tt.expected {
				t.Errorf("Expected %d wikilinks, got %d", tt.expected, len(wikilinks))
			}

			// Check wikilink structure
			for _, wl := range wikilinks {
				if wl.Original == "" {
					t.Error("Wikilink has empty original text")
				}
				if wl.Target == "" {
					t.Error("Wikilink has empty target")
				}
				if wl.Display == "" {
					t.Error("Wikilink has empty display text")
				}
			}
		})
	}
}

func TestExtractEmbeds(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedType string
		expectedNum  int
	}{
		{
			name:         "image embed",
			content:      "![[image.png]]",
			expectedType: "image",
			expectedNum:  1,
		},
		{
			name:         "note embed",
			content:      "![[Another Note]]",
			expectedType: "note",
			expectedNum:  1,
		},
		{
			name:         "pdf embed",
			content:      "![[document.pdf]]",
			expectedType: "pdf",
			expectedNum:  1,
		},
		{
			name:        "no embeds",
			content:     "Just text",
			expectedNum: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewStructuredRenderer(nil)
			embeds := sr.extractEmbeds(tt.content, "test-vault")

			if len(embeds) != tt.expectedNum {
				t.Errorf("Expected %d embeds, got %d", tt.expectedNum, len(embeds))
			}

			if tt.expectedNum > 0 && embeds[0].Type != tt.expectedType {
				t.Errorf("Expected embed type %q, got %q", tt.expectedType, embeds[0].Type)
			}
		})
	}
}

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedWords int
		minWords      int
		maxWords      int
	}{
		{
			name:          "simple text",
			content:       "Hello world this is a test",
			expectedWords: 6,
			minWords:      6,
			maxWords:      6,
		},
		{
			name: "text with markdown",
			content: `# Heading
**Bold text** and *italic text*
[Link](http://example.com)
` + "```" + `
code block
` + "```",
			expectedWords: 6, // Bold, text, and, italic, text, Link
			minWords:      5,
			maxWords:      7,
		},
		{
			name:          "empty content",
			content:       "",
			expectedWords: 0,
			minWords:      0,
			maxWords:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewStructuredRenderer(nil)
			stats := sr.calculateStats(tt.content, time.Now(), time.Now())

			if stats.Words < tt.minWords || stats.Words > tt.maxWords {
				t.Errorf("Expected words between %d and %d, got %d", tt.minWords, tt.maxWords, stats.Words)
			}

			if stats.Words > 0 && stats.ReadingTime == 0 {
				t.Error("Expected reading time > 0 for non-empty content")
			}

			// Character count should be reasonable (markdown is stripped)
			// Don't check exact count as it depends on markdown stripping
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Test-123", "test-123"},
		{"Unicode: 你好", "unicode-你好"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"Special!@#$%Characters", "specialcharacters"},
		{"hyphens---multiple", "hyphens-multiple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProcessMarkdown(t *testing.T) {
	content := `---
title: Test Note
tags: [golang, testing]
---

# Main Heading

This is a note about #golang and #testing.

Link to [[Other Note]] and embed ![[image.png]].

## Subheading

More content here.`

	resolver := NewMockFileResolver()
	resolver.wikilinks["Other Note"] = struct {
		exists bool
		fileID string
		path   string
	}{true, "note-123", "/notes/other.md"}

	sr := NewStructuredRenderer(resolver)
	result, err := sr.ProcessMarkdown(content, "test-vault", "file-123", time.Now(), time.Now())

	if err != nil {
		t.Fatalf("ProcessMarkdown failed: %v", err)
	}

	// Check frontmatter
	if len(result.Frontmatter) == 0 {
		t.Error("Expected frontmatter to be extracted")
	}

	// Check headings
	if len(result.Headings) != 2 {
		t.Errorf("Expected 2 headings, got %d", len(result.Headings))
	}

	// Check tags
	if len(result.Tags) < 2 {
		t.Errorf("Expected at least 2 tags, got %d", len(result.Tags))
	}

	// Check wikilinks (there might be 2 if "Other Note" appears in frontmatter as well)
	if len(result.WikiLinks) < 1 {
		t.Errorf("Expected at least 1 wikilink, got %d", len(result.WikiLinks))
	}
	if len(result.WikiLinks) > 0 && !result.WikiLinks[0].Exists {
		t.Error("Expected wikilink to be resolved as existing")
	}

	// Check embeds
	if len(result.Embeds) != 1 {
		t.Errorf("Expected 1 embed, got %d", len(result.Embeds))
	}
	if len(result.Embeds) > 0 && result.Embeds[0].Type != "image" {
		t.Errorf("Expected embed type 'image', got %q", result.Embeds[0].Type)
	}

	// Check stats
	if result.Stats.Words == 0 {
		t.Error("Expected word count > 0")
	}
	if result.Stats.ReadingTime == 0 {
		t.Error("Expected reading time > 0")
	}

	// Check raw markdown doesn't include frontmatter
	if len(result.RawMarkdown) >= len(content) {
		t.Error("Expected raw markdown to be shorter than original (frontmatter removed)")
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"Hello", 1},
		{"Hello World", 2},
		{"  Multiple   spaces  ", 2},
		{"Line1\nLine2\nLine3", 3},
		{"Hyphens-are-one-word", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := countWords(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %d words, got %d", tt.expected, result)
			}
		})
	}
}
