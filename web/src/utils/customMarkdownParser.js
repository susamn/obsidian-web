/**
 * Custom Markdown Parser for Obsidian-style markdown
 *
 * Parses raw markdown into structured nodes without using external libraries.
 * Handles: headings, paragraphs, lists, code blocks, blockquotes, callouts,
 * wikilinks, tags, bold, italic, code, links, and more.
 *
 * @module customMarkdownParser
 */

/**
 * Parse markdown content into structured nodes
 *
 * @param {string} markdown - Raw markdown content
 * @param {Array<Object>} wikilinks - Wikilink metadata from backend
 * @param {Array<Object>} embeds - Embed metadata from backend
 * @returns {Array<Object>} - Array of parsed nodes
 */
export function parseMarkdown(markdown, wikilinks = [], embeds = []) {
  if (!markdown) return [];

  const lines = markdown.split('\n');
  const nodes = [];
  let i = 0;
  const maxIterations = lines.length * 2; // Safety limit
  let iterations = 0;

  while (i < lines.length) {
    iterations++;
    if (iterations > maxIterations) {
      console.error('Parser stuck in infinite loop at line', i, ':', lines[i]);
      break;
    }

    const line = lines[i];

    // Skip empty lines (but track them for paragraph breaks)
    if (line.trim() === '') {
      i++;
      continue;
    }

    // Code blocks (fenced with ```)
    if (line.trim().startsWith('```')) {
      const result = parseCodeBlock(lines, i);
      nodes.push(result.node);
      i = result.nextIndex;
      continue;
    }

    // Headings
    if (line.match(/^#{1,6}\s/)) {
      nodes.push(parseHeading(line));
      i++;
      continue;
    }

    // Blockquotes (including callouts)
    if (line.trim().startsWith('>')) {
      const result = parseBlockquote(lines, i);
      nodes.push(result.node);
      i = result.nextIndex;
      continue;
    }

    // Unordered lists
    if (line.match(/^\s*[-*+]\s/)) {
      const result = parseList(lines, i, 'ul');
      nodes.push(result.node);
      i = result.nextIndex;
      continue;
    }

    // Ordered lists
    if (line.match(/^\s*\d+\.\s/)) {
      const result = parseList(lines, i, 'ol');
      nodes.push(result.node);
      i = result.nextIndex;
      continue;
    }

    // Horizontal rule
    if (line.match(/^[-*_]{3,}$/)) {
      nodes.push({ type: 'hr' });
      i++;
      continue;
    }

    // Tables
    if (line.includes('|') && i < lines.length - 1 && lines[i + 1].match(/^\|?\s*:?-+:?\s*\|/)) {
      const result = parseTable(lines, i);
      nodes.push(result.node);
      i = result.nextIndex;
      continue;
    }

    // Default: paragraph
    const result = parseParagraph(lines, i);
    nodes.push(result.node);
    i = result.nextIndex;
  }

  // Post-process: inject wikilink and embed data
  return postProcessNodes(nodes, wikilinks, embeds);
}

/**
 * Parse a heading
 */
function parseHeading(line) {
  const match = line.match(/^(#{1,6})\s+(.+)$/);
  if (!match) return { type: 'paragraph', content: line };

  const level = match[1].length;
  const text = match[2].trim();
  const id = slugify(text);

  return {
    type: 'heading',
    level,
    id,
    content: parseInline(text)
  };
}

/**
 * Parse a code block
 */
function parseCodeBlock(lines, startIndex) {
  const firstLine = lines[startIndex].trim();
  const language = firstLine.slice(3).trim() || 'text';
  let i = startIndex + 1;
  const codeLines = [];

  while (i < lines.length && !lines[i].trim().startsWith('```')) {
    codeLines.push(lines[i]);
    i++;
  }

  return {
    node: {
      type: 'code_block',
      language,
      content: codeLines.join('\n')
    },
    nextIndex: i + 1
  };
}

/**
 * Parse a blockquote (or callout)
 */
function parseBlockquote(lines, startIndex) {
  const quoteLines = [];
  let i = startIndex;

  // Collect all consecutive quote lines
  while (i < lines.length && lines[i].trim().startsWith('>')) {
    const line = lines[i].trim().slice(1).trim();
    quoteLines.push(line);
    i++;
  }

  const fullContent = quoteLines.join('\n');

  // Check if it's a callout: [!TYPE] Title
  const calloutMatch = fullContent.match(/^\[!(\w+)\]\s*(.*)$/m);

  if (calloutMatch) {
    const type = calloutMatch[1].toLowerCase();
    const title = calloutMatch[2].trim();
    const content = fullContent.replace(/^\[!(\w+)\]\s*.*$/m, '').trim();

    return {
      node: {
        type: 'callout',
        calloutType: type,
        title: title || getCalloutLabel(type),
        content: parseInline(content)
      },
      nextIndex: i
    };
  }

  return {
    node: {
      type: 'blockquote',
      content: parseInline(fullContent)
    },
    nextIndex: i
  };
}

/**
 * Parse a list (ul or ol)
 */
function parseList(lines, startIndex, listType) {
  const items = [];
  let i = startIndex;
  const baseIndent = lines[i].match(/^\s*/)[0].length;

  while (i < lines.length) {
    const line = lines[i];
    const indent = line.match(/^\s*/)[0].length;

    // Check if this is a list item
    const match = listType === 'ul'
      ? line.match(/^\s*[-*+]\s+(.+)$/)
      : line.match(/^\s*\d+\.\s+(.+)$/);

    if (!match) break;
    if (indent < baseIndent) break;

    items.push({
      content: parseInline(match[1].trim())
    });

    i++;
  }

  return {
    node: {
      type: listType,
      items
    },
    nextIndex: i
  };
}

/**
 * Parse a table
 */
function parseTable(lines, startIndex) {
  const rows = [];
  let i = startIndex;

  // Parse header
  const headerCells = lines[i].split('|').map(c => c.trim()).filter(c => c);
  rows.push({
    type: 'header',
    cells: headerCells.map(c => parseInline(c))
  });

  i++; // Skip separator line
  i++; // Move to first data row

  // Parse data rows
  while (i < lines.length && lines[i].includes('|')) {
    const cells = lines[i].split('|').map(c => c.trim()).filter(c => c);
    rows.push({
      type: 'row',
      cells: cells.map(c => parseInline(c))
    });
    i++;
  }

  return {
    node: {
      type: 'table',
      rows
    },
    nextIndex: i
  };
}

/**
 * Parse a paragraph
 */
function parseParagraph(lines, startIndex) {
  const paragraphLines = [];
  let i = startIndex;

  while (i < lines.length) {
    const line = lines[i];

    // Stop at empty line
    if (line.trim() === '') break;

    // Stop at block elements
    if (line.match(/^#{1,6}\s/) ||
        line.trim().startsWith('>') ||
        line.match(/^\s*[-*+]\s/) ||
        line.match(/^\s*\d+\.\s/) ||
        line.trim().startsWith('```')) {
      break;
    }

    paragraphLines.push(line);
    i++;
  }

  return {
    node: {
      type: 'paragraph',
      content: parseInline(paragraphLines.join('\n'))
    },
    nextIndex: i
  };
}

/**
 * Parse inline elements (bold, italic, code, links, wikilinks, tags)
 * Returns an array of text and inline nodes
 */
function parseInline(text) {
  if (!text) return [];

  const tokens = [];
  let remaining = text;
  let pos = 0;
  const maxIterations = text.length * 2; // Safety limit
  let iterations = 0;

  while (remaining.length > 0) {
    iterations++;
    if (iterations > maxIterations) {
      console.error('Inline parser stuck in loop at position', pos, 'remaining:', remaining.substring(0, 50));
      // Add remaining text as plain text and break
      tokens.push({ type: 'text', content: remaining });
      break;
    }

    // Try to match inline patterns
    let matched = false;

    // Wikilinks [[link]]
    const wikilinkMatch = remaining.match(/^\[\[([^\]]+)\]\]/);
    if (wikilinkMatch) {
      const fullMatch = wikilinkMatch[0];
      const content = wikilinkMatch[1];
      const parts = content.split('|');
      const target = parts[0].trim();
      const display = parts[1] ? parts[1].trim() : target;

      tokens.push({
        type: 'wikilink',
        target,
        display,
        original: fullMatch
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Embeds ![[embed]]
    const embedMatch = remaining.match(/^!\[\[([^\]]+)\]\]/);
    if (embedMatch) {
      const fullMatch = embedMatch[0];
      const target = embedMatch[1].trim();

      tokens.push({
        type: 'embed',
        target,
        original: fullMatch
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Tags #tag
    const tagMatch = remaining.match(/^#([\w-/]+)/);
    if (tagMatch) {
      const fullMatch = tagMatch[0];
      const tag = tagMatch[1];

      tokens.push({
        type: 'tag',
        tag
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Code `code`
    const codeMatch = remaining.match(/^`([^`]+)`/);
    if (codeMatch) {
      const fullMatch = codeMatch[0];
      const code = codeMatch[1];

      tokens.push({
        type: 'code',
        content: code
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Bold **text** or __text__
    const boldMatch = remaining.match(/^(\*\*|__)(.+?)\1/);
    if (boldMatch) {
      const fullMatch = boldMatch[0];
      const content = boldMatch[2];

      tokens.push({
        type: 'bold',
        content: parseInline(content)
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Italic *text* or _text_
    const italicMatch = remaining.match(/^(\*|_)(.+?)\1/);
    if (italicMatch) {
      const fullMatch = italicMatch[0];
      const content = italicMatch[2];

      tokens.push({
        type: 'italic',
        content: parseInline(content)
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Strikethrough ~~text~~
    const strikeMatch = remaining.match(/^~~(.+?)~~/);
    if (strikeMatch) {
      const fullMatch = strikeMatch[0];
      const content = strikeMatch[1];

      tokens.push({
        type: 'strikethrough',
        content: parseInline(content)
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Highlight ==text==
    const highlightMatch = remaining.match(/^==(.+?)==/);
    if (highlightMatch) {
      const fullMatch = highlightMatch[0];
      const content = highlightMatch[1];

      tokens.push({
        type: 'highlight',
        content: parseInline(content)
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // Links [text](url)
    const linkMatch = remaining.match(/^\[([^\]]+)\]\(([^)]+)\)/);
    if (linkMatch) {
      const fullMatch = linkMatch[0];
      const text = linkMatch[1];
      const url = linkMatch[2];

      tokens.push({
        type: 'link',
        text,
        url
      });

      remaining = remaining.slice(fullMatch.length);
      pos += fullMatch.length;
      matched = true;
      continue;
    }

    // No match, consume one character as text
    if (!matched) {
      const char = remaining[0];

      // Merge consecutive text
      if (tokens.length > 0 && tokens[tokens.length - 1].type === 'text') {
        tokens[tokens.length - 1].content += char;
      } else {
        tokens.push({
          type: 'text',
          content: char
        });
      }

      remaining = remaining.slice(1);
      pos++;
    }
  }

  return tokens;
}

/**
 * Post-process nodes to inject wikilink and embed metadata
 */
function postProcessNodes(nodes, wikilinks, embeds) {
  return nodes.map(node => {
    if (node.content && Array.isArray(node.content)) {
      node.content = node.content.map(inline => {
        if (inline.type === 'wikilink') {
          const metadata = wikilinks.find(wl => wl.original === inline.original);
          return { ...inline, ...metadata };
        }
        if (inline.type === 'embed') {
          const metadata = embeds.find(em => em.target === inline.target);
          return { ...inline, ...metadata };
        }
        if (inline.content && Array.isArray(inline.content)) {
          inline.content = postProcessInline(inline.content, wikilinks, embeds);
        }
        return inline;
      });
    }

    if (node.type === 'ul' || node.type === 'ol') {
      node.items = node.items.map(item => ({
        ...item,
        content: postProcessInline(item.content, wikilinks, embeds)
      }));
    }

    if (node.type === 'table') {
      node.rows = node.rows.map(row => ({
        ...row,
        cells: row.cells.map(cell => postProcessInline(cell, wikilinks, embeds))
      }));
    }

    return node;
  });
}

function postProcessInline(inlineNodes, wikilinks, embeds) {
  return inlineNodes.map(inline => {
    if (inline.type === 'wikilink') {
      const metadata = wikilinks.find(wl => wl.original === inline.original);
      return { ...inline, ...metadata };
    }
    if (inline.type === 'embed') {
      const metadata = embeds.find(em => em.target === inline.target);
      return { ...inline, ...metadata };
    }
    if (inline.content && Array.isArray(inline.content)) {
      inline.content = postProcessInline(inline.content, wikilinks, embeds);
    }
    return inline;
  });
}

/**
 * Get callout label for a type
 */
function getCalloutLabel(type) {
  const labels = {
    note: 'Note',
    abstract: 'Abstract',
    summary: 'Summary',
    tldr: 'TLDR',
    info: 'Info',
    tip: 'Tip',
    hint: 'Hint',
    important: 'Important',
    warning: 'Warning',
    caution: 'Caution',
    attention: 'Attention',
    danger: 'Danger',
    error: 'Error',
    failure: 'Failure',
    bug: 'Bug',
    example: 'Example',
    quote: 'Quote'
  };

  return labels[type] || 'Note';
}

/**
 * Create a slug from text (for heading IDs)
 */
function slugify(text) {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');
}

export default {
  parseMarkdown
};
