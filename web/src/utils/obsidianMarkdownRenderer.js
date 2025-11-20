/**
 * Obsidian-Style Markdown Renderer
 *
 * A comprehensive markdown rendering utility that provides Obsidian-like
 * features including YAML frontmatter parsing, wikilink support, tag extraction,
 * and theme-aware rendering.
 *
 * @module obsidianMarkdownRenderer
 */

import MarkdownIt from 'markdown-it';

// ============================================================================
// CONFIGURATION & INITIALIZATION
// ============================================================================

/**
 * Create a markdown-it instance with Obsidian-like settings
 */
const createMarkdownInstance = () => {
  const md = new MarkdownIt({
    html: true,
    linkify: true,
    typographer: true,
    breaks: false,
    highlight: (str, lang) => {
      const langClass = lang ? `language-${escapeHtml(lang)}` : '';
      return `<pre class="md-code-block ${langClass}"><code>${escapeHtml(str)}</code></pre>`;
    }
  });

  addCustomRules(md);

  return md;
};

// Cache the markdown instance for performance
let cachedMdInstance = null;

const getMdInstance = () => {
  if (!cachedMdInstance) {
    cachedMdInstance = createMarkdownInstance();
  }
  return cachedMdInstance;
};

// ============================================================================
// YAML FRONTMATTER PARSING
// ============================================================================

/**
 * Extracts YAML frontmatter from markdown content
 *
 * @param {string} content - Raw markdown content
 * @returns {Object} - Parsed frontmatter and remaining content
 */
export const extractFrontmatter = (content) => {
  const frontmatterRegex = /^---\s*\n([\s\S]*?)\n---\s*\n/;
  const match = content.match(frontmatterRegex);

  if (!match) {
    return {
      frontmatter: {},
      content: content,
      tags: [],
      rawFrontmatter: null
    };
  }

  const yamlContent = match[1];
  const remainingContent = content.slice(match[0].length);

  const frontmatter = parseSimpleYAML(yamlContent);
  const tags = extractTagsFromFrontmatter(frontmatter);

  return {
    frontmatter,
    content: remainingContent,
    tags,
    rawFrontmatter: yamlContent
  };
};

/**
 * Simple YAML parser for frontmatter
 * Supports: strings, arrays, simple nested values
 *
 * @private
 */
const parseSimpleYAML = (yaml) => {
  const result = {};
  const lines = yaml.split('\n');

  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;

    const colonIndex = trimmed.indexOf(':');
    if (colonIndex === -1) continue;

    const key = trimmed.substring(0, colonIndex).trim();
    let value = trimmed.substring(colonIndex + 1).trim();

    if (!value) continue;

    // Parse array format: [item1, item2, item3]
    if (value.startsWith('[') && value.endsWith(']')) {
      value = value
        .slice(1, -1)
        .split(',')
        .map(v => v.trim().replace(/^["']|["']$/g, ''))
        .filter(v => v.length > 0);
    } else {
      // Remove quotes from strings
      value = value.replace(/^["']|["']$/g, '');
    }

    result[key] = value;
  }

  return result;
};

/**
 * Extracts tags from frontmatter object
 * Supports: tags, tag, keywords fields
 *
 * @private
 */
const extractTagsFromFrontmatter = (frontmatter) => {
  const tagFields = ['tags', 'tag', 'keywords'];
  const tags = [];

  for (const field of tagFields) {
    if (frontmatter[field]) {
      const value = frontmatter[field];
      if (Array.isArray(value)) {
        tags.push(...value);
      } else if (typeof value === 'string') {
        tags.push(...value.split(',').map(t => t.trim()));
      }
    }
  }

  return [...new Set(tags)];
};

// ============================================================================
// INLINE TAG EXTRACTION
// ============================================================================

/**
 * Extracts inline hashtags from markdown content
 *
 * @param {string} content - Markdown content
 * @returns {Array<string>} - Array of tags (without # symbol)
 */
export const extractInlineTags = (content) => {
  const tagRegex = /#([\w\-_/]+)/g;
  const tags = [];
  let match;

  while ((match = tagRegex.exec(content)) !== null) {
    tags.push(match[1]);
  }

  return [...new Set(tags)];
};

/**
 * Combines frontmatter tags and inline tags
 *
 * @param {Array<string>} frontmatterTags - Tags from YAML frontmatter
 * @param {Array<string>} inlineTags - Tags from markdown content
 * @returns {Array<string>} - Combined unique tags
 */
export const getAllTags = (frontmatterTags = [], inlineTags = []) => {
  return [...new Set([...frontmatterTags, ...inlineTags])];
};

// ============================================================================
// CUSTOM RENDERING RULES
// ============================================================================

/**
 * Adds custom rendering rules for Obsidian-like features
 *
 * @private
 */
const addCustomRules = (md) => {
  // Custom rule for wikilinks [[link]]
  md.inline.ruler.before('link', 'wikilink', wikilinkRule);

  // Custom rule for tags #tag (with CSS class)
  md.inline.ruler.after('emphasis', 'hashtag', hashtagRule);

  // Custom rule for block references ^block-id
  md.inline.ruler.after('text', 'blockref', blockRefRule);

  enhanceDefaultRenderers(md);
};

/**
 * Wikilink parsing rule [[Page Name|Display Text]]
 *
 * @private
 */
const wikilinkRule = (state, silent) => {
  const start = state.pos;
  const max = state.posMax;

  if (state.src.charCodeAt(start) !== 0x5B /* [ */ ||
      state.src.charCodeAt(start + 1) !== 0x5B /* [ */) {
    return false;
  }

  let pos = start + 2;
  while (pos < max) {
    if (state.src.charCodeAt(pos) === 0x5D /* ] */ &&
        state.src.charCodeAt(pos + 1) === 0x5D /* ] */) {
      break;
    }
    pos++;
  }

  if (pos >= max) return false;

  const content = state.src.slice(start + 2, pos);

  if (!silent) {
    const parts = content.split('|');
    const pageName = parts[0].trim();
    const displayText = parts[1] ? parts[1].trim() : pageName;

    const token = state.push('wikilink', '', 0);
    token.content = displayText;
    token.meta = { page: pageName };
  }

  state.pos = pos + 2;
  return true;
};

/**
 * Hashtag parsing rule with CSS class application
 *
 * @private
 */
const hashtagRule = (state, silent) => {
  const start = state.pos;
  const max = state.posMax;

  if (state.src.charCodeAt(start) !== 0x23 /* # */) {
    return false;
  }

  if (start > 0) {
    const prev = state.src.charCodeAt(start - 1);
    if (prev !== 0x20 && prev !== 0x0A && prev !== 0x09) {
      return false;
    }
  }

  let pos = start + 1;
  while (pos < max) {
    const code = state.src.charCodeAt(pos);
    if (!((code >= 0x41 && code <= 0x5A) ||
          (code >= 0x61 && code <= 0x7A) ||
          (code >= 0x30 && code <= 0x39) ||
          code === 0x2D || code === 0x5F || code === 0x2F)) {
      break;
    }
    pos++;
  }

  if (pos === start + 1) return false;

  if (!silent) {
    const tagName = state.src.slice(start + 1, pos);
    const token = state.push('hashtag', '', 0);
    token.content = tagName;
  }

  state.pos = pos;
  return true;
};

/**
 * Block reference parsing rule ^block-id
 *
 * @private
 */
const blockRefRule = (state, silent) => {
  const start = state.pos;
  const max = state.posMax;

  if (state.src.charCodeAt(start) !== 0x5E /* ^ */) {
    return false;
  }

  let pos = start + 1;
  while (pos < max) {
    const code = state.src.charCodeAt(pos);
    if (code === 0x20 || code === 0x0A) break;
    pos++;
  }

  if (pos === start + 1) return false;

  if (!silent) {
    const blockId = state.src.slice(start + 1, pos);
    const token = state.push('blockref', '', 0);
    token.content = blockId;
  }

  state.pos = pos;
  return true;
};

/**
 * Enhance default markdown-it renderers with custom CSS classes
 *
 * @private
 */
const enhanceDefaultRenderers = (md) => {
  // Store original renderers
  const defaultHeadingOpen = md.renderer.rules.heading_open;

  // Custom renderer for wikilinks
  md.renderer.rules.wikilink = (tokens, idx) => {
    const token = tokens[idx];
    const page = token.meta.page;
    const display = escapeHtml(token.content);

    return `<a href="#" class="md-wikilink" data-page="${escapeHtml(page)}" title="Navigate to ${escapeHtml(page)}">${display}</a>`;
  };

  // Custom renderer for hashtags
  md.renderer.rules.hashtag = (tokens, idx) => {
    const token = tokens[idx];
    const tag = escapeHtml(token.content);

    return `<span class="md-tag" data-tag="${tag}">#${tag}</span>`;
  };

  // Custom renderer for block references
  md.renderer.rules.blockref = (tokens, idx) => {
    const token = tokens[idx];
    const blockId = escapeHtml(token.content);

    return `<span class="md-blockref" id="block-${blockId}">^${blockId}</span>`;
  };

  // Enhance heading renderer with anchor links
  md.renderer.rules.heading_open = (tokens, idx, options, env, self) => {
    const token = tokens[idx];
    const level = token.tag.slice(1);
    const nextToken = tokens[idx + 1];

    if (nextToken && nextToken.type === 'inline' && nextToken.content) {
      const id = slugify(nextToken.content);
      token.attrSet('id', id);
      token.attrSet('class', `md-heading md-heading-${level}`);
    }

    return defaultHeadingOpen
      ? defaultHeadingOpen(tokens, idx, options, env, self)
      : self.renderToken(tokens, idx, options);
  };

  // Enhance code blocks with language indicator
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx];
    const lang = token.info ? token.info.trim() : '';
    const langClass = lang ? `language-${escapeHtml(lang)}` : '';
    const code = escapeHtml(token.content);

    return `<pre class="${langClass}"><code>${code}</code></pre>`;
  };

  // Enhance table with wrapper for responsive scrolling
  md.renderer.rules.table_open = () => {
    return '<div class="md-table-wrapper"><table class="md-table">';
  };

  md.renderer.rules.table_close = () => {
    return '</table></div>';
  };
};

// ============================================================================
// METADATA EXTRACTION UTILITIES
// ============================================================================

/**
 * Extracts headings from markdown content
 *
 * @private
 */
const extractHeadings = (content) => {
  const headingRegex = /^(#{1,6})\s+(.+)$/gm;
  const headings = [];
  let match;

  while ((match = headingRegex.exec(content)) !== null) {
    const level = match[1].length;
    const text = match[2].trim();
    const id = slugify(text);

    headings.push({ level, text, id });
  }

  return headings;
};

/**
 * Extracts wikilinks from content
 *
 * @private
 */
const extractWikilinks = (content) => {
  const wikilinkRegex = /\[\[([^\]]+)\]\]/g;
  const wikilinks = [];
  let match;

  while ((match = wikilinkRegex.exec(content)) !== null) {
    wikilinks.push(match[1]);
  }

  return wikilinks;
};

/**
 * Calculates content statistics
 *
 * @private
 */
const calculateStats = (content) => {
  const plainText = content
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/\*\*(.+?)\*\*/g, '$1')
    .replace(/\*(.+?)\*/g, '$1')
    .replace(/`(.+?)`/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/\[\[([^\]]+)\]\]/g, '$1')
    .trim();

  const words = plainText.split(/\s+/).filter(w => w.length > 0).length;
  const chars = plainText.length;
  const readingTime = Math.ceil(words / 200);

  return { words, chars, readingTime };
};

// ============================================================================
// MAIN RENDERING FUNCTION
// ============================================================================

/**
 * Main rendering function that processes markdown content
 *
 * @param {string} rawContent - Raw markdown content with optional frontmatter
 * @returns {Object} - Rendered HTML and extracted metadata
 */
export const renderObsidianMarkdown = (rawContent) => {
  if (!rawContent || typeof rawContent !== 'string') {
    return {
      html: '',
      tags: [],
      frontmatter: {},
      headings: [],
      wikilinks: [],
      stats: { words: 0, chars: 0, readingTime: 0 }
    };
  }

  const md = getMdInstance();

  const { frontmatter, content, tags: frontmatterTags } = extractFrontmatter(rawContent);
  const inlineTags = extractInlineTags(content);
  const allTags = getAllTags(frontmatterTags, inlineTags);
  const html = md.render(content);
  const headings = extractHeadings(content);
  const wikilinks = extractWikilinks(content);
  const stats = calculateStats(content);

  return {
    html,
    tags: allTags,
    frontmatter,
    headings,
    wikilinks,
    stats
  };
};

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

/**
 * Converts text to URL-friendly slug
 *
 * @private
 */
const slugify = (text) => {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');
};

/**
 * Escapes HTML special characters
 *
 * @private
 */
const escapeHtml = (text) => {
  const htmlEscapeMap = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  };

  return text.replace(/[&<>"']/g, char => htmlEscapeMap[char]);
};

// ============================================================================
// EXPORTS
// ============================================================================

export default {
  renderObsidianMarkdown,
  extractFrontmatter,
  extractInlineTags,
  getAllTags
};
