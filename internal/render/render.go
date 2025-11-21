package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Renderer provides markdown rendering capabilities
type Renderer struct {
	md goldmark.Markdown
}

// RenderedContent represents the output of markdown rendering
type RenderedContent struct {
	HTML string `json:"html"`
}

// NewRenderer creates a new Renderer instance with Goldmark
func NewRenderer() *Renderer {
	// Configure Goldmark with extensions
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithASTTransformers(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
		// Add extensions for enhanced markdown support
		goldmark.WithExtensions(
			extension.GFM,           // GitHub Flavored Markdown
			extension.Table,         // Tables
			extension.Strikethrough, // Strikethrough
			extension.Linkify,       // Auto-linking
			extension.TaskList,      // Task lists
		),
	)

	return &Renderer{
		md: md,
	}
}

// RenderMarkdown converts markdown content to HTML
func (r *Renderer) RenderMarkdown(content string) (RenderedContent, error) {
	var buf bytes.Buffer

	// Parse and render the markdown
	if err := r.md.Convert([]byte(content), &buf); err != nil {
		return RenderedContent{}, fmt.Errorf("failed to render markdown: %w", err)
	}

	return RenderedContent{
		HTML: buf.String(),
	}, nil
}

// RenderMarkdownToString is a helper function that renders markdown and returns HTML as string
func (r *Renderer) RenderMarkdownToString(content string) (string, error) {
	result, err := r.RenderMarkdown(content)
	return result.HTML, err
}
