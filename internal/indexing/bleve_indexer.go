package indexing

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/mapping"
)

type MarkdownDoc struct {
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags"`
	Metadata string   `json:"metadata"` // YAML frontmatter
}

// Parse markdown file with frontmatter
func parseMarkdownFile(path string) (*MarkdownDoc, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	text := string(content)
	doc := &MarkdownDoc{
		Path: path,
		Tags: []string{},
	}

	// Extract YAML frontmatter (if exists)
	if strings.HasPrefix(text, "---") {
		parts := strings.SplitN(text, "---", 3)
		if len(parts) >= 3 {
			doc.Metadata = strings.TrimSpace(parts[1])
			text = strings.TrimSpace(parts[2])

			// Extract tags from frontmatter
			doc.Tags = extractTags(doc.Metadata)
		}
	}

	// Extract title (first # heading)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			doc.Title = strings.TrimPrefix(line, "# ")
			break
		}
	}

	// Extract inline tags from content (e.g., #tag, #nested/tag)
	inlineTags := extractInlineTags(text)

	// Combine frontmatter tags with inline tags (deduplicate)
	tagSet := make(map[string]bool)
	for _, tag := range doc.Tags {
		tagSet[tag] = true
	}
	for _, tag := range inlineTags {
		tagSet[tag] = true
	}

	// Convert back to slice
	allTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		allTags = append(allTags, tag)
	}
	doc.Tags = allTags

	doc.Content = text
	return doc, nil
}

// Extract tags from YAML frontmatter
func extractTags(metadata string) []string {
	tags := []string{}
	lines := strings.Split(metadata, "\n")

	inTagsSection := false
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "tags:") {
			inTagsSection = true
			// Handle inline tags: tags: [tag1, tag2]
			if strings.Contains(line, "[") {
				tagStr := strings.TrimPrefix(line, "tags:")
				tagStr = strings.Trim(tagStr, " []")
				for _, tag := range strings.Split(tagStr, ",") {
					tags = append(tags, strings.TrimSpace(tag))
				}
				inTagsSection = false
			}
			continue
		}

		if inTagsSection {
			if strings.HasPrefix(line, "-") {
				tag := strings.TrimPrefix(line, "-")
				tags = append(tags, strings.TrimSpace(tag))
			} else if line != "" && !strings.HasSuffix(line, ":") {
				inTagsSection = false
			}
		}
	}

	return tags
}

// Extract inline tags from markdown content (e.g., #tag, #nested/tag)
// Ignores tags in code blocks and inline code
func extractInlineTags(content string) []string {
	tags := []string{}
	tagSet := make(map[string]bool)

	lines := strings.Split(content, "\n")
	inCodeBlock := false

	for _, line := range lines {
		// Check for code block markers
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Skip lines inside code blocks
		if inCodeBlock {
			continue
		}

		// Process line for inline tags, avoiding inline code
		processedLine := removeInlineCode(line)

		// Find all tags in the line
		// Tags start with # followed by alphanumeric, -, _, or /
		i := 0
		for i < len(processedLine) {
			if processedLine[i] == '#' {
				// Check if this is a heading (# at start of line with space after)
				if i == 0 && i+1 < len(processedLine) && processedLine[i+1] == ' ' {
					// This is a heading, not a tag
					break
				}

				// Check if # is preceded by whitespace or is at start (valid tag position)
				if i == 0 || processedLine[i-1] == ' ' || processedLine[i-1] == '\t' || processedLine[i-1] == '(' || processedLine[i-1] == '[' {
					// Extract the tag
					tagStart := i + 1
					tagEnd := tagStart

					// Tag can contain: letters, numbers, -, _, /
					for tagEnd < len(processedLine) {
						c := processedLine[tagEnd]
						if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
							(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '/' {
							tagEnd++
						} else {
							break
						}
					}

					// Extract tag if it's not empty
					if tagEnd > tagStart {
						tag := processedLine[tagStart:tagEnd]
						if !tagSet[tag] {
							tags = append(tags, tag)
							tagSet[tag] = true
						}
					}

					i = tagEnd
					continue
				}
			}
			i++
		}
	}

	return tags
}

// Remove inline code from a line (content between backticks)
func removeInlineCode(line string) string {
	result := ""
	inInlineCode := false

	for i := 0; i < len(line); i++ {
		if line[i] == '`' {
			inInlineCode = !inInlineCode
			result += " " // Replace backtick with space to preserve tag boundaries
		} else if !inInlineCode {
			result += string(line[i])
		} else {
			result += " " // Replace inline code content with spaces
		}
	}

	return result
}

// Create index mapping for better search
func buildIndexMapping() mapping.IndexMapping {
	// Create document mapping
	docMapping := bleve.NewDocumentMapping()

	// Text fields - analyzed for full-text search
	textFieldMapping := bleve.NewTextFieldMapping()
	docMapping.AddFieldMappingsAt("title", textFieldMapping)
	docMapping.AddFieldMappingsAt("content", textFieldMapping)
	docMapping.AddFieldMappingsAt("metadata", textFieldMapping)

	// Keyword field for exact tag matching
	keywordFieldMapping := bleve.NewTextFieldMapping()
	keywordFieldMapping.Analyzer = keyword.Name
	docMapping.AddFieldMappingsAt("tags", keywordFieldMapping)

	// Path field - stored but not analyzed
	pathFieldMapping := bleve.NewTextFieldMapping()
	pathFieldMapping.Store = true
	pathFieldMapping.Index = false
	docMapping.AddFieldMappingsAt("path", pathFieldMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping("markdown", docMapping)
	indexMapping.DefaultMapping = docMapping

	return indexMapping
}

// IndexMarkdownFiles indexes all markdown files from docsPath into the bleve index
func IndexMarkdownFiles(indexPath, docsPath string) (bleve.Index, error) {
	var index bleve.Index
	var err error

	// Try to open existing index
	index, err = bleve.Open(indexPath)
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		// Create new index
		docMapping := buildIndexMapping()
		index, err = bleve.New(indexPath, docMapping)
		if err != nil {
			return nil, fmt.Errorf("failed to create index: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}

	// Walk through markdown files
	batch := index.NewBatch()
	count := 0

	err = filepath.WalkDir(docsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		doc, err := parseMarkdownFile(path)
		if err != nil {
			log.Printf("Error parsing %s: %v", path, err)
			return nil // Continue processing other files
		}

		// Use relative path as document ID
		relPath, _ := filepath.Rel(docsPath, path)
		batch.Index(relPath, doc)
		count++

		// Batch index every 100 documents
		if batch.Size() >= 100 {
			if err := index.Batch(batch); err != nil {
				return fmt.Errorf("batch index failed: %w", err)
			}
			fmt.Printf("Indexed %d documents...\n", count)
			batch = index.NewBatch()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Index remaining documents
	if batch.Size() > 0 {
		if err := index.Batch(batch); err != nil {
			return nil, fmt.Errorf("final batch index failed: %w", err)
		}
	}

	fmt.Printf("Successfully indexed %d documents\n", count)
	return index, nil
}
