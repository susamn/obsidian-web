package render

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// FileContentResponse represents the structured response for file rendering
type FileContentResponse struct {
	RawMarkdown string                 `json:"raw_markdown"`
	Frontmatter map[string]interface{} `json:"frontmatter"`
	Headings    []Heading              `json:"headings"`
	Tags        []Tag                  `json:"tags"`
	WikiLinks   []WikiLink             `json:"wikilinks"`
	Backlinks   []Backlink             `json:"backlinks"`
	Embeds      []Embed                `json:"embeds"`
	Stats       Stats                  `json:"stats"`
}

// Heading represents a markdown heading
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id"`
	Line  int    `json:"line"`
}

// Tag represents a tag with metadata
type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// WikiLink represents a wikilink with resolution metadata
type WikiLink struct {
	Original string `json:"original"`
	Target   string `json:"target"`
	Display  string `json:"display"`
	Exists   bool   `json:"exists"`
	FileID   string `json:"file_id,omitempty"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line"`
}

// Backlink represents a link from another file to this file
type Backlink struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	Context  string `json:"context"`
}

// Embed represents an embedded note or media
type Embed struct {
	Original string `json:"original"`
	Type     string `json:"type"`
	Target   string `json:"target"`
	Display  string `json:"display,omitempty"` // For sizing like |500
	Content  string `json:"content,omitempty"`
	Exists   bool   `json:"exists"`
	FileID   string `json:"file_id,omitempty"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line"`
}

// Stats represents file statistics
type Stats struct {
	Words       int       `json:"words"`
	Characters  int       `json:"characters"`
	ReadingTime int       `json:"reading_time_minutes"`
	Created     time.Time `json:"created,omitempty"`
	Modified    time.Time `json:"modified,omitempty"`
}

// StructuredRenderer processes markdown files into structured data
type StructuredRenderer struct {
	// FileResolver is used to resolve wikilinks and backlinks
	FileResolver FileResolver
}

// FileResolver interface for resolving file paths and metadata
type FileResolver interface {
	// ResolveWikiLink resolves a wikilink to file metadata
	ResolveWikiLink(vaultID, linkTarget string) (exists bool, fileID, path string)
	// GetBacklinks finds all files linking to the given file
	GetBacklinks(vaultID, fileID string) []Backlink
	// GetTagCount returns the number of files with a given tag
	GetTagCount(vaultID, tag string) int
}

// NewStructuredRenderer creates a new structured renderer
func NewStructuredRenderer(resolver FileResolver) *StructuredRenderer {
	return &StructuredRenderer{
		FileResolver: resolver,
	}
}

// ProcessMarkdown processes markdown content and returns structured data
func (sr *StructuredRenderer) ProcessMarkdown(content string, vaultID string, fileID string, created, modified time.Time) (*FileContentResponse, error) {
	// Extract frontmatter
	frontmatter, cleanContent, err := sr.extractFrontmatter(content)
	if err != nil {
		// If frontmatter parsing fails, use content as-is
		cleanContent = content
	}

	// Extract tags from frontmatter
	frontmatterTags := sr.extractTagsFromFrontmatter(frontmatter)

	// Extract inline tags
	inlineTags := sr.extractInlineTags(cleanContent)

	// Combine and deduplicate tags
	allTags := sr.mergeTags(frontmatterTags, inlineTags, vaultID)

	// Extract headings
	headings := sr.extractHeadings(cleanContent)

	// Extract wikilinks
	wikilinks := sr.extractWikiLinks(cleanContent, vaultID)

	// Extract embeds
	embeds := sr.extractEmbeds(cleanContent, vaultID)

	// Get backlinks
	var backlinks []Backlink
	if sr.FileResolver != nil && fileID != "" {
		backlinks = sr.FileResolver.GetBacklinks(vaultID, fileID)
	}

	// Calculate stats
	stats := sr.calculateStats(cleanContent, created, modified)

	// Replace image/embed links with file IDs in raw markdown
	processedMarkdown := sr.replaceLinksWithIDs(cleanContent, wikilinks, embeds)

	return &FileContentResponse{
		RawMarkdown: processedMarkdown,
		Frontmatter: frontmatter,
		Headings:    headings,
		Tags:        allTags,
		WikiLinks:   wikilinks,
		Backlinks:   backlinks,
		Embeds:      embeds,
		Stats:       stats,
	}, nil
}

// replaceLinksWithIDs replaces wikilinks and embeds with their file IDs in the markdown content
func (sr *StructuredRenderer) replaceLinksWithIDs(content string, wikilinks []WikiLink, embeds []Embed) string {
	result := content

	// Replace embeds first (they start with !)
	for _, embed := range embeds {
		if embed.FileID != "" {
			// Replace ![[filename|display]] with ![[fileID|display]]
			// Or ![[filename]] with ![[fileID]]
			replacement := "![[" + embed.FileID
			if embed.Display != "" {
				replacement += "|" + embed.Display
			}
			replacement += "]]"
			result = strings.Replace(result, embed.Original, replacement, -1)
		}
	}

	// Replace wikilinks
	for _, wikilink := range wikilinks {
		if wikilink.FileID != "" {
			// Replace [[target|display]] with [[fileID|display]]
			// Or [[target]] with [[fileID]]
			replacement := "[[" + wikilink.FileID
			if wikilink.Display != wikilink.Target {
				replacement += "|" + wikilink.Display
			}
			replacement += "]]"
			result = strings.Replace(result, wikilink.Original, replacement, -1)
		}
	}

	return result
}

// extractFrontmatter extracts and parses YAML frontmatter
func (sr *StructuredRenderer) extractFrontmatter(content string) (map[string]interface{}, string, error) {
	// Regex to match frontmatter
	frontmatterRegex := regexp.MustCompile(`^---\s*\n([\s\S]*?)\n---\s*\n`)
	matches := frontmatterRegex.FindStringSubmatch(content)

	if len(matches) < 2 {
		return map[string]interface{}{}, content, nil
	}

	yamlContent := matches[1]
	remainingContent := content[len(matches[0]):]

	// Parse YAML
	var frontmatter map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return map[string]interface{}{}, content, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	return frontmatter, remainingContent, nil
}

// extractTagsFromFrontmatter extracts tags from frontmatter
func (sr *StructuredRenderer) extractTagsFromFrontmatter(frontmatter map[string]interface{}) []string {
	var tags []string

	// Check common tag fields
	tagFields := []string{"tags", "tag", "keywords"}
	for _, field := range tagFields {
		if value, exists := frontmatter[field]; exists {
			switch v := value.(type) {
			case string:
				// Single tag or comma-separated
				parts := strings.Split(v, ",")
				for _, part := range parts {
					tag := strings.TrimSpace(part)
					if tag != "" {
						tags = append(tags, tag)
					}
				}
			case []interface{}:
				// Array of tags
				for _, item := range v {
					if tagStr, ok := item.(string); ok {
						tag := strings.TrimSpace(tagStr)
						if tag != "" {
							tags = append(tags, tag)
						}
					}
				}
			}
		}
	}

	return tags
}

// extractInlineTags extracts hashtags from content
func (sr *StructuredRenderer) extractInlineTags(content string) []string {
	tagRegex := regexp.MustCompile(`#([\w\-_/]+)`)
	matches := tagRegex.FindAllStringSubmatch(content, -1)

	var tags []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			tag := match[1]
			if !seen[tag] {
				tags = append(tags, tag)
				seen[tag] = true
			}
		}
	}

	return tags
}

// mergeTags combines and deduplicates tags with counts
func (sr *StructuredRenderer) mergeTags(frontmatterTags, inlineTags []string, vaultID string) []Tag {
	seen := make(map[string]bool)
	var tags []Tag

	// Add all unique tags
	allTagNames := append(frontmatterTags, inlineTags...)
	for _, tagName := range allTagNames {
		if !seen[tagName] {
			count := 1
			if sr.FileResolver != nil {
				count = sr.FileResolver.GetTagCount(vaultID, tagName)
			}

			tags = append(tags, Tag{
				Name:  tagName,
				Count: count,
			})
			seen[tagName] = true
		}
	}

	return tags
}

// extractHeadings extracts headings from markdown content
func (sr *StructuredRenderer) extractHeadings(content string) []Heading {
	headingRegex := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := headingRegex.FindAllStringSubmatch(content, -1)

	var headings []Heading
	lines := strings.Split(content, "\n")
	lineNum := 0

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		level := len(match[1])
		text := strings.TrimSpace(match[2])

		// Find line number
		for i, line := range lines[lineNum:] {
			if strings.Contains(line, match[0]) {
				lineNum += i
				break
			}
		}

		headings = append(headings, Heading{
			Level: level,
			Text:  text,
			ID:    slugify(text),
			Line:  lineNum + 1,
		})
	}

	return headings
}

// extractWikiLinks extracts wikilinks from content
func (sr *StructuredRenderer) extractWikiLinks(content string, vaultID string) []WikiLink {
	// Regex for wikilinks: [[target]] or [[target|display]]
	wikilinkRegex := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	matches := wikilinkRegex.FindAllStringSubmatch(content, -1)

	var wikilinks []WikiLink
	lines := strings.Split(content, "\n")

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		original := match[0]
		inner := match[1]

		// Split by pipe for display text
		parts := strings.SplitN(inner, "|", 2)
		target := strings.TrimSpace(parts[0])
		display := target
		if len(parts) > 1 {
			display = strings.TrimSpace(parts[1])
		}

		// Find line number
		lineNum := 0
		for i, line := range lines {
			if strings.Contains(line, original) {
				lineNum = i + 1
				break
			}
		}

		// Resolve wikilink
		exists := false
		fileID := ""
		path := ""
		if sr.FileResolver != nil {
			exists, fileID, path = sr.FileResolver.ResolveWikiLink(vaultID, target)
		}

		wikilinks = append(wikilinks, WikiLink{
			Original: original,
			Target:   target,
			Display:  display,
			Exists:   exists,
			FileID:   fileID,
			Path:     path,
			Line:     lineNum,
		})
	}

	return wikilinks
}

// extractEmbeds extracts embedded notes and media
func (sr *StructuredRenderer) extractEmbeds(content string, vaultID string) []Embed {
	// Regex for embeds: ![[target]] or ![[target|display]]
	embedRegex := regexp.MustCompile(`!\[\[([^\]]+)\]\]`)
	matches := embedRegex.FindAllStringSubmatch(content, -1)

	var embeds []Embed
	lines := strings.Split(content, "\n")

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		original := match[0]
		inner := match[1]

		// Split by pipe for display text/sizing (e.g., |500)
		parts := strings.SplitN(inner, "|", 2)
		target := strings.TrimSpace(parts[0])
		display := ""
		if len(parts) > 1 {
			display = strings.TrimSpace(parts[1])
		}

		// Find line number
		lineNum := 0
		for i, line := range lines {
			if strings.Contains(line, original) {
				lineNum = i + 1
				break
			}
		}

		// Determine type based on extension
		embedType := "note"
		if strings.Contains(target, ".") {
			ext := strings.ToLower(target[strings.LastIndex(target, ".")+1:])
			switch ext {
			case "png", "jpg", "jpeg", "gif", "svg", "webp":
				embedType = "image"
			case "pdf":
				embedType = "pdf"
			case "mp4", "webm", "ogv":
				embedType = "video"
			case "mp3", "wav", "ogg":
				embedType = "audio"
			}
		}

		// Resolve embed
		exists := false
		fileID := ""
		path := ""
		if sr.FileResolver != nil {
			exists, fileID, path = sr.FileResolver.ResolveWikiLink(vaultID, target)
		}

		embeds = append(embeds, Embed{
			Original: original,
			Type:     embedType,
			Target:   target,
			Display:  display,
			Exists:   exists,
			FileID:   fileID,
			Path:     path,
			Line:     lineNum,
		})
	}

	return embeds
}

// calculateStats calculates word count and reading time
func (sr *StructuredRenderer) calculateStats(content string, created, modified time.Time) Stats {
	// Remove markdown syntax for accurate word count
	plainText := sr.stripMarkdown(content)

	// Count words
	words := countWords(plainText)

	// Count characters
	chars := len(plainText)

	// Calculate reading time (average 200 words per minute)
	readingTime := (words + 199) / 200

	return Stats{
		Words:       words,
		Characters:  chars,
		ReadingTime: readingTime,
		Created:     created,
		Modified:    modified,
	}
}

// stripMarkdown removes markdown syntax for word counting
func (sr *StructuredRenderer) stripMarkdown(content string) string {
	// Remove code blocks
	codeBlockRegex := regexp.MustCompile("(?s)```.*?```")
	content = codeBlockRegex.ReplaceAllString(content, "")

	// Remove inline code
	inlineCodeRegex := regexp.MustCompile("`[^`]+`")
	content = inlineCodeRegex.ReplaceAllString(content, "")

	// Remove headings
	headingRegex := regexp.MustCompile(`(?m)^#{1,6}\s+`)
	content = headingRegex.ReplaceAllString(content, "")

	// Remove bold/italic
	boldItalicRegex := regexp.MustCompile(`\*\*?([^*]+)\*\*?`)
	content = boldItalicRegex.ReplaceAllString(content, "$1")

	// Remove links
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	content = linkRegex.ReplaceAllString(content, "$1")

	// Remove wikilinks
	wikilinkRegex := regexp.MustCompile(`!?\[\[([^\]]+)\]\]`)
	content = wikilinkRegex.ReplaceAllString(content, "$1")

	return strings.TrimSpace(content)
}

// countWords counts words in text
func countWords(text string) int {
	if text == "" {
		return 0
	}

	words := 0
	inWord := false

	for _, r := range text {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			words++
			inWord = true
		}
	}

	return words
}

// slugify converts text to URL-friendly slug
func slugify(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Replace spaces with hyphens
	text = strings.ReplaceAll(text, " ", "-")

	// Remove special characters but keep unicode
	var result strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}

	// Remove multiple consecutive hyphens
	slug := result.String()
	multiHyphen := regexp.MustCompile(`-+`)
	slug = multiHyphen.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}
