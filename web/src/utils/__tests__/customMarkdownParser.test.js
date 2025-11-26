import { describe, it, expect } from 'vitest'
import { parseMarkdown } from '../customMarkdownParser'

describe('customMarkdownParser', () => {
  describe('Headings', () => {
    it('should parse H1 heading', () => {
      const result = parseMarkdown('# Hello World')
      expect(result).toHaveLength(1)
      expect(result[0].type).toBe('heading')
      expect(result[0].level).toBe(1)
      expect(result[0].id).toBe('hello-world')
      expect(result[0].content).toEqual([{ type: 'text', content: 'Hello World' }])
    })

    it('should parse H2 through H6 headings', () => {
      const markdown = '## H2\n### H3\n#### H4\n##### H5\n###### H6'
      const result = parseMarkdown(markdown)
      expect(result).toHaveLength(5)
      expect(result.map((h) => h.level)).toEqual([2, 3, 4, 5, 6])
    })

    it('should parse heading with inline formatting', () => {
      const result = parseMarkdown('# Hello **bold** world')
      expect(result[0].content).toHaveLength(3)
      expect(result[0].content[0]).toEqual({ type: 'text', content: 'Hello ' })
      expect(result[0].content[1].type).toBe('bold')
      expect(result[0].content[2]).toEqual({ type: 'text', content: ' world' })
    })
  })

  describe('Paragraphs', () => {
    it('should parse simple paragraph', () => {
      const result = parseMarkdown('This is a paragraph.')
      expect(result).toHaveLength(1)
      expect(result[0].type).toBe('paragraph')
      expect(result[0].content).toEqual([{ type: 'text', content: 'This is a paragraph.' }])
    })

    it('should parse multi-line paragraph', () => {
      const markdown = 'Line one\nLine two\nLine three'
      const result = parseMarkdown(markdown)
      expect(result).toHaveLength(1)
      expect(result[0].type).toBe('paragraph')
    })

    it('should split paragraphs on empty lines', () => {
      const markdown = 'Paragraph one\n\nParagraph two'
      const result = parseMarkdown(markdown)
      expect(result).toHaveLength(2)
      expect(result[0].type).toBe('paragraph')
      expect(result[1].type).toBe('paragraph')
    })
  })

  describe('Inline Formatting', () => {
    it('should parse bold with **', () => {
      const result = parseMarkdown('This is **bold** text')
      const content = result[0].content
      expect(content[1].type).toBe('bold')
      expect(content[1].content).toEqual([{ type: 'text', content: 'bold' }])
    })

    it('should parse italic with *', () => {
      const result = parseMarkdown('This is *italic* text')
      const content = result[0].content
      expect(content[1].type).toBe('italic')
      expect(content[1].content).toEqual([{ type: 'text', content: 'italic' }])
    })

    it('should parse strikethrough with ~~', () => {
      const result = parseMarkdown('This is ~~strikethrough~~ text')
      const content = result[0].content
      expect(content[1].type).toBe('strikethrough')
    })

    it('should parse highlight with ==', () => {
      const result = parseMarkdown('This is ==highlighted== text')
      const content = result[0].content
      expect(content[1].type).toBe('highlight')
    })

    it('should parse inline code with `', () => {
      const result = parseMarkdown('This is `code` text')
      const content = result[0].content
      expect(content[1]).toEqual({ type: 'code', content: 'code' })
    })

    it('should parse nested formatting', () => {
      const result = parseMarkdown('This is **bold and *italic* text**')
      const content = result[0].content
      expect(content[1].type).toBe('bold')
      expect(content[1].content).toHaveLength(3)
      expect(content[1].content[0].type).toBe('text')
      expect(content[1].content[0].content).toBe('bold and ')
      expect(content[1].content[1].type).toBe('italic')
      expect(content[1].content[2].type).toBe('text')
      expect(content[1].content[2].content).toBe(' text')
    })
  })

  describe('Links', () => {
    it('should parse markdown link', () => {
      const result = parseMarkdown('Click [here](https://example.com)')
      const content = result[0].content
      expect(content[1]).toEqual({
        type: 'link',
        text: 'here',
        url: 'https://example.com',
      })
    })
  })

  describe('Wikilinks', () => {
    it('should parse simple wikilink', () => {
      const result = parseMarkdown('See [[Page Name]]')
      const content = result[0].content
      expect(content[1].type).toBe('wikilink')
      expect(content[1].target).toBe('Page Name')
      expect(content[1].display).toBe('Page Name')
    })

    it('should parse wikilink with display text', () => {
      const result = parseMarkdown('See [[Page Name|Custom Display]]')
      const content = result[0].content
      expect(content[1].type).toBe('wikilink')
      expect(content[1].target).toBe('Page Name')
      expect(content[1].display).toBe('Custom Display')
    })

    it('should inject wikilink metadata', () => {
      const wikilinks = [
        {
          original: '[[Page Name]]',
          target: 'Page Name',
          display: 'Page Name',
          exists: true,
          file_id: 'file-123',
        },
      ]
      const result = parseMarkdown('See [[Page Name]]', wikilinks)
      const content = result[0].content
      expect(content[1].exists).toBe(true)
      expect(content[1].file_id).toBe('file-123')
    })
  })

  describe('Tags', () => {
    it('should parse hashtag', () => {
      const result = parseMarkdown('This has #tag in it')
      const content = result[0].content
      expect(content[1]).toEqual({ type: 'tag', tag: 'tag' })
    })

    it('should parse tag with hyphens and slashes', () => {
      const result = parseMarkdown('#multi-word #nested/tag')
      const content = result[0].content
      expect(content[0].tag).toBe('multi-word')
      expect(content[2].tag).toBe('nested/tag')
    })
  })

  describe('Code Blocks', () => {
    it('should parse fenced code block', () => {
      const markdown = '```javascript\nconst x = 1;\n```'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('code_block')
      expect(result[0].language).toBe('javascript')
      expect(result[0].content).toBe('const x = 1;')
    })

    it('should parse code block without language', () => {
      const markdown = '```\nPlain code\n```'
      const result = parseMarkdown(markdown)
      expect(result[0].language).toBe('text')
    })

    it('should handle multi-line code blocks', () => {
      const markdown = '```\nLine 1\nLine 2\nLine 3\n```'
      const result = parseMarkdown(markdown)
      expect(result[0].content).toBe('Line 1\nLine 2\nLine 3')
    })
  })

  describe('Lists', () => {
    it('should parse unordered list with -', () => {
      const markdown = '- Item 1\n- Item 2\n- Item 3'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('ul')
      expect(result[0].items).toHaveLength(3)
      expect(result[0].items[0].content).toEqual([{ type: 'text', content: 'Item 1' }])
    })

    it('should parse unordered list with *', () => {
      const markdown = '* Item 1\n* Item 2'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('ul')
    })

    it('should parse ordered list', () => {
      const markdown = '1. First\n2. Second\n3. Third'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('ol')
      expect(result[0].items).toHaveLength(3)
    })

    it('should parse list with inline formatting', () => {
      const markdown = '- Item with **bold**\n- Item with [[wikilink]]'
      const result = parseMarkdown(markdown)
      expect(result[0].items[0].content[1].type).toBe('bold')
      expect(result[0].items[1].content[1].type).toBe('wikilink')
    })
  })

  describe('Blockquotes', () => {
    it('should parse simple blockquote', () => {
      const markdown = '> This is a quote'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('blockquote')
      expect(result[0].content).toEqual([{ type: 'text', content: 'This is a quote' }])
    })

    it('should parse multi-line blockquote', () => {
      const markdown = '> Line 1\n> Line 2\n> Line 3'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('blockquote')
    })
  })

  describe('Callouts', () => {
    it('should parse note callout', () => {
      const markdown = '> [!note] This is a note\n> Content here'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('callout')
      expect(result[0].calloutType).toBe('note')
      expect(result[0].title).toBe('This is a note')
    })

    it('should parse callout without title', () => {
      const markdown = '> [!warning]\n> Warning content'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('callout')
      expect(result[0].calloutType).toBe('warning')
      expect(result[0].title).toBe('Warning')
    })

    it('should parse various callout types', () => {
      const types = ['tip', 'important', 'danger', 'bug', 'example']
      types.forEach((type) => {
        const markdown = `> [!${type}] Title\n> Content`
        const result = parseMarkdown(markdown)
        expect(result[0].calloutType).toBe(type)
      })
    })
  })

  describe('Tables', () => {
    it('should parse simple table', () => {
      const markdown = '| Header 1 | Header 2 |\n| --- | --- |\n| Cell 1 | Cell 2 |'
      const result = parseMarkdown(markdown)
      expect(result[0].type).toBe('table')
      expect(result[0].rows).toHaveLength(2)
      expect(result[0].rows[0].type).toBe('header')
      expect(result[0].rows[1].type).toBe('row')
    })

    it('should parse table with inline formatting', () => {
      const markdown = '| **Bold** | `code` |\n| --- | --- |\n| [[link]] | normal |'
      const result = parseMarkdown(markdown)
      expect(result[0].rows[0].cells[0][0].type).toBe('bold')
      expect(result[0].rows[0].cells[1][0].type).toBe('code')
    })
  })

  describe('Horizontal Rule', () => {
    it('should parse horizontal rule with ---', () => {
      const result = parseMarkdown('---')
      expect(result[0].type).toBe('hr')
    })

    it('should parse horizontal rule with ***', () => {
      const result = parseMarkdown('***')
      expect(result[0].type).toBe('hr')
    })
  })

  describe('Embeds', () => {
    it('should parse embed syntax', () => {
      const result = parseMarkdown('See this ![[image.png]]')
      const content = result[0].content
      expect(content[1].type).toBe('embed')
      expect(content[1].target).toBe('image.png')
    })

    it('should inject embed metadata', () => {
      const embeds = [
        {
          target: 'image.png',
          exists: true,
          type: 'image',
        },
      ]
      const result = parseMarkdown('See ![[image.png]]', [], embeds)
      const content = result[0].content
      expect(content[1].exists).toBe(true)
      expect(content[1].type).toBe('embed')
    })
  })

  describe('Complex Documents', () => {
    it('should parse mixed content', () => {
      const markdown = `# Title

This is a paragraph with **bold** and [[wikilink]].

- List item 1
- List item 2

> [!note] Important
> This is a callout

\`\`\`javascript
const x = 1;
\`\`\``

      const result = parseMarkdown(markdown)
      expect(result).toHaveLength(5)
      expect(result[0].type).toBe('heading')
      expect(result[1].type).toBe('paragraph')
      expect(result[2].type).toBe('ul')
      expect(result[3].type).toBe('callout')
      expect(result[4].type).toBe('code_block')
    })
  })

  describe('Edge Cases', () => {
    it('should handle empty string', () => {
      const result = parseMarkdown('')
      expect(result).toEqual([])
    })

    it('should handle only whitespace', () => {
      const result = parseMarkdown('   \n  \n   ')
      expect(result).toEqual([])
    })

    it('should handle special characters', () => {
      const result = parseMarkdown('Text with & < > " \'')
      expect(result[0].content[0].content).toContain('&')
    })
  })
})
