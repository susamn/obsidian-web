/**
 * Obsidian-Style Markdown Renderer
 *
 * A comprehensive markdown rendering utility that provides Obsidian-like
 * features including YAML frontmatter parsing, wikilink support, tag extraction,
 * and theme-aware rendering.
 *
 * @module obsidianMarkdownRenderer
 */

import MarkdownIt from 'markdown-it'

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
      const langClass = lang ? `language-${escapeHtml(lang)}` : ''
      return `<pre class="md-code-block ${langClass}"><code>${escapeHtml(str)}</code></pre>`
    },
  })

  addCustomRules(md)

  return md
}

// Cache the markdown instance for performance
let cachedMdInstance = null

const getMdInstance = () => {
  if (!cachedMdInstance) {
    cachedMdInstance = createMarkdownInstance()
  }
  return cachedMdInstance
}

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
  const frontmatterRegex = /^---\s*\n([\s\S]*?)\n---\s*\n/
  const match = content.match(frontmatterRegex)

  if (!match) {
    return {
      frontmatter: {},
      content: content,
      tags: [],
      rawFrontmatter: null,
    }
  }

  const yamlContent = match[1]
  const remainingContent = content.slice(match[0].length)

  const frontmatter = parseSimpleYAML(yamlContent)
  const tags = extractTagsFromFrontmatter(frontmatter)

  return {
    frontmatter,
    content: remainingContent,
    tags,
    rawFrontmatter: yamlContent,
  }
}

/**
 * Simple YAML parser for frontmatter
 * Supports: strings, arrays, simple nested values
 *
 * @private
 */
const parseSimpleYAML = (yaml) => {
  const result = {}
  const lines = yaml.split('\n')

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue

    const colonIndex = trimmed.indexOf(':')
    if (colonIndex === -1) continue

    const key = trimmed.substring(0, colonIndex).trim()
    let value = trimmed.substring(colonIndex + 1).trim()

    if (!value) continue

    // Parse array format: [item1, item2, item3]
    if (value.startsWith('[') && value.endsWith(']')) {
      value = value
        .slice(1, -1)
        .split(',')
        .map((v) => v.trim().replace(/^["']|["']$/g, ''))
        .filter((v) => v.length > 0)
    } else {
      // Remove quotes from strings
      value = value.replace(/^["']|["']$/g, '')
    }

    result[key] = value
  }

  return result
}

/**
 * Extracts tags from frontmatter object
 * Supports: tags, tag, keywords fields
 *
 * @private
 */
const extractTagsFromFrontmatter = (frontmatter) => {
  const tagFields = ['tags', 'tag', 'keywords']
  const tags = []

  for (const field of tagFields) {
    if (frontmatter[field]) {
      const value = frontmatter[field]
      if (Array.isArray(value)) {
        tags.push(...value)
      } else if (typeof value === 'string') {
        tags.push(...value.split(',').map((t) => t.trim()))
      }
    }
  }

  return [...new Set(tags)]
}

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
  const tagRegex = /#([\w\-_/]+)/g
  const tags = []
  let match

  while ((match = tagRegex.exec(content)) !== null) {
    tags.push(match[1])
  }

  return [...new Set(tags)]
}

/**
 * Combines frontmatter tags and inline tags
 *
 * @param {Array<string>} frontmatterTags - Tags from YAML frontmatter
 * @param {Array<string>} inlineTags - Tags from markdown content
 * @returns {Array<string>} - Combined unique tags
 */
export const getAllTags = (frontmatterTags = [], inlineTags = []) => {
  return [...new Set([...frontmatterTags, ...inlineTags])]
}

// ============================================================================
// CUSTOM RENDERING RULES
// ============================================================================

/**
 * Inline math parsing rule $...$
 *
 * @private
 */
const mathInlineRule = (state, silent) => {
  const start = state.pos
  const max = state.posMax

  if (state.src.charCodeAt(start) !== 0x24 /* $ */) {
    return false
  }

  let pos = start + 1
  while (pos < max) {
    if (state.src.charCodeAt(pos) === 0x24 /* $ */) {
      const content = state.src.slice(start + 1, pos)
      if (content.length > 0) {
        if (!silent) {
          const token = state.push('math_inline', '', 0)
          token.content = content
          token.markup = '$'
        }
        state.pos = pos + 1
        return true
      }
      break
    }
    if (state.src.charCodeAt(pos) === 0x5c /* \ */) {
      pos += 2
    } else {
      pos++
    }
  }

  return false
}

/**
 * Block math parsing rule $$...$$
 *
 * @private
 */
const mathBlockRule = (state, startLine, endLine, silent) => {
  const pos = state.bMarks[startLine] + state.tShift[startLine]
  const maximum = state.eMarks[startLine]

  if (pos + 2 > maximum) return false
  if (state.src.charCodeAt(pos) !== 0x24 /* $ */) return false
  if (state.src.charCodeAt(pos + 1) !== 0x24 /* $ */) return false

  let firstLine = state.src.slice(pos + 2, maximum)

  if (firstLine.includes('$$')) {
    // Single line math block
    const token = state.push('math_block', 'div', 0)
    token.content = firstLine.slice(0, firstLine.indexOf('$$'))
    token.markup = '$$'
    token.map = [startLine, startLine + 1]
    state.line = startLine + 1
    return true
  }

  let nextLine = startLine
  let auto = false

  while (nextLine < endLine) {
    nextLine++
    if (nextLine >= endLine) break

    pos = state.bMarks[nextLine] + state.tShift[nextLine]
    maximum = state.eMarks[nextLine]

    if (pos < maximum && state.src.charCodeAt(pos) === 0x24 /* $ */) {
      if (state.src.charCodeAt(pos + 1) === 0x24 /* $ */) {
        auto = true
        break
      }
    }
  }

  const oldParent = state.parentType
  const oldLineMax = state.lineMax
  state.parentType = 'math'

  const firstLineContent = firstLine
  const lastLine = auto ? state.src.slice(state.bMarks[nextLine], state.eMarks[nextLine]) : ''

  if (auto) {
    const content = state
      .getLines(startLine + 1, nextLine, state.tShift[startLine + 1], true)
      .trim()
      .slice(0, -2)

    const token = state.push('math_block', 'div', 0)
    token.content = firstLineContent + '\n' + content
    token.markup = '$$'
    token.map = [startLine, nextLine + 1]
  }

  state.parentType = oldParent
  state.line = nextLine + 1

  return true
}

/**
 * Adds custom rendering rules for Obsidian-like features
 *
 * @private
 */
const addCustomRules = (md) => {
  // Custom rule for inline math $...$
  md.inline.ruler.before('text', 'math_inline', mathInlineRule)

  // Custom rule for block math $$...$$
  md.block.ruler.before('fence', 'math_block', mathBlockRule)

  // Custom rule for wikilinks [[link]]
  md.inline.ruler.before('link', 'wikilink', wikilinkRule)

  // Custom rule for tags #tag (with CSS class)
  md.inline.ruler.after('emphasis', 'hashtag', hashtagRule)

  // Custom rule for block references ^block-id
  md.inline.ruler.after('text', 'blockref', blockRefRule)

  enhanceDefaultRenderers(md)
}

/**
 * Wikilink parsing rule [[Page Name|Display Text]]
 *
 * @private
 */
const wikilinkRule = (state, silent) => {
  const start = state.pos
  const max = state.posMax

  if (
    state.src.charCodeAt(start) !== 0x5b /* [ */ ||
    state.src.charCodeAt(start + 1) !== 0x5b /* [ */
  ) {
    return false
  }

  let pos = start + 2
  while (pos < max) {
    if (
      state.src.charCodeAt(pos) === 0x5d /* ] */ &&
      state.src.charCodeAt(pos + 1) === 0x5d /* ] */
    ) {
      break
    }
    pos++
  }

  if (pos >= max) return false

  const content = state.src.slice(start + 2, pos)

  if (!silent) {
    const parts = content.split('|')
    const pageName = parts[0].trim()
    const displayText = parts[1] ? parts[1].trim() : pageName

    const token = state.push('wikilink', '', 0)
    token.content = displayText
    token.meta = { page: pageName }
  }

  state.pos = pos + 2
  return true
}

/**
 * Hashtag parsing rule with CSS class application
 *
 * @private
 */
const hashtagRule = (state, silent) => {
  const start = state.pos
  const max = state.posMax

  if (state.src.charCodeAt(start) !== 0x23 /* # */) {
    return false
  }

  if (start > 0) {
    const prev = state.src.charCodeAt(start - 1)
    if (prev !== 0x20 && prev !== 0x0a && prev !== 0x09) {
      return false
    }
  }

  let pos = start + 1
  while (pos < max) {
    const code = state.src.charCodeAt(pos)
    if (
      !(
        (code >= 0x41 && code <= 0x5a) ||
        (code >= 0x61 && code <= 0x7a) ||
        (code >= 0x30 && code <= 0x39) ||
        code === 0x2d ||
        code === 0x5f ||
        code === 0x2f
      )
    ) {
      break
    }
    pos++
  }

  if (pos === start + 1) return false

  if (!silent) {
    const tagName = state.src.slice(start + 1, pos)
    const token = state.push('hashtag', '', 0)
    token.content = tagName
  }

  state.pos = pos
  return true
}

/**
 * Block reference parsing rule ^block-id
 *
 * @private
 */
const blockRefRule = (state, silent) => {
  const start = state.pos
  const max = state.posMax

  if (state.src.charCodeAt(start) !== 0x5e /* ^ */) {
    return false
  }

  let pos = start + 1
  while (pos < max) {
    const code = state.src.charCodeAt(pos)
    if (code === 0x20 || code === 0x0a) break
    pos++
  }

  if (pos === start + 1) return false

  if (!silent) {
    const blockId = state.src.slice(start + 1, pos)
    const token = state.push('blockref', '', 0)
    token.content = blockId
  }

  state.pos = pos
  return true
}

/**
 * Extract YouTube ID from various YouTube URL formats
 *
 * @private
 */
const extractYouTubeId = (url) => {
  const patterns = [
    /(?:https?:\/\/)?(?:www\.)?youtube\.com\/watch\?v=([a-zA-Z0-9_-]{11})/,
    /(?:https?:\/\/)?(?:www\.)?youtu\.be\/([a-zA-Z0-9_-]{11})/,
    /(?:https?:\/\/)?(?:www\.)?youtube\.com\/embed\/([a-zA-Z0-9_-]{11})/,
  ]

  for (const pattern of patterns) {
    const match = url.match(pattern)
    if (match) {
      return match[1]
    }
  }

  return null
}

/**
 * Callout/Admonition type configuration
 * Maps callout types to icons and CSS classes
 */
const CALLOUT_TYPES = {
  note: { icon: '📝', class: 'md-callout-note', label: 'Note' },
  abstract: { icon: '📋', class: 'md-callout-abstract', label: 'Abstract' },
  summary: { icon: '📋', class: 'md-callout-summary', label: 'Summary' },
  tldr: { icon: '📋', class: 'md-callout-tldr', label: 'TLDR' },
  info: { icon: 'ℹ️', class: 'md-callout-info', label: 'Info' },
  tip: { icon: '💡', class: 'md-callout-tip', label: 'Tip' },
  hint: { icon: '💡', class: 'md-callout-hint', label: 'Hint' },
  important: { icon: '❗', class: 'md-callout-important', label: 'Important' },
  warning: { icon: '⚠️', class: 'md-callout-warning', label: 'Warning' },
  caution: { icon: '⚠️', class: 'md-callout-caution', label: 'Caution' },
  attention: { icon: '⚠️', class: 'md-callout-attention', label: 'Attention' },
  danger: { icon: '🔥', class: 'md-callout-danger', label: 'Danger' },
  error: { icon: '❌', class: 'md-callout-error', label: 'Error' },
  failure: { icon: '❌', class: 'md-callout-failure', label: 'Failure' },
  bug: { icon: '🐛', class: 'md-callout-bug', label: 'Bug' },
  example: { icon: '📊', class: 'md-callout-example', label: 'Example' },
  quote: { icon: '💬', class: 'md-callout-quote', label: 'Quote' },
}

/**
 * Enhance default markdown-it renderers with custom CSS classes
 *
 * @private
 */
const enhanceDefaultRenderers = (md) => {
  // Store original renderers
  const defaultHeadingOpen = md.renderer.rules.heading_open
  const defaultBlockquoteOpen = md.renderer.rules.blockquote_open
  const defaultBlockquoteClose = md.renderer.rules.blockquote_close

  // Custom renderer for wikilinks
  md.renderer.rules.wikilink = (tokens, idx) => {
    const token = tokens[idx]
    const page = token.meta.page
    const display = escapeHtml(token.content)

    return `<a href="#" class="md-wikilink" data-page="${escapeHtml(page)}" title="Navigate to ${escapeHtml(page)}">${display}</a>`
  }

  // Custom renderer for hashtags
  md.renderer.rules.hashtag = (tokens, idx) => {
    const token = tokens[idx]
    const tag = escapeHtml(token.content)

    return `<span class="md-tag" data-tag="${tag}">#${tag}</span>`
  }

  // Custom renderer for block references
  md.renderer.rules.blockref = (tokens, idx) => {
    const token = tokens[idx]
    const blockId = escapeHtml(token.content)

    return `<span class="md-blockref" id="block-${blockId}">^${blockId}</span>`
  }

  // Enhance heading renderer with anchor links
  md.renderer.rules.heading_open = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    const level = token.tag.slice(1)
    const nextToken = tokens[idx + 1]

    if (nextToken && nextToken.type === 'inline' && nextToken.content) {
      const id = slugify(nextToken.content)
      token.attrSet('id', id)
      token.attrSet('class', `md-heading md-heading-${level}`)
    }

    return defaultHeadingOpen
      ? defaultHeadingOpen(tokens, idx, options, env, self)
      : self.renderToken(tokens, idx, options)
  }

  // Enhanced blockquote renderer for callouts
  md.renderer.rules.blockquote_open = (tokens, idx) => {
    const token = tokens[idx]
    const nextToken = tokens[idx + 1]

    // Check if this is a callout (> [!TYPE] Title format)
    if (nextToken && nextToken.type === 'paragraph_open') {
      const contentToken = tokens[idx + 2]
      if (contentToken && contentToken.type === 'inline') {
        const match = contentToken.content.match(/^\[!(\w+)\]\s*(.*)$/)
        if (match) {
          const type = match[1].toLowerCase()
          const title = match[2]
          const calloutConfig = CALLOUT_TYPES[type] || CALLOUT_TYPES.note

          token.meta = token.meta || {}
          token.meta.isCallout = true
          token.meta.calloutType = type
          token.meta.calloutTitle = title
          token.meta.calloutConfig = calloutConfig
        }
      }
    }

    if (token.meta && token.meta.isCallout) {
      const { icon, class: calloutClass, label } = token.meta.calloutConfig
      const title = token.meta.calloutTitle || token.meta.calloutConfig.label

      return `<div class="md-callout ${calloutClass}">
        <div class="md-callout-header">
          <span class="md-callout-icon">${icon}</span>
          <span class="md-callout-title">${escapeHtml(title)}</span>
        </div>
        <div class="md-callout-content">`
    }

    return '<blockquote class="md-blockquote">'
  }

  md.renderer.rules.blockquote_close = (tokens, idx) => {
    const openToken = tokens.find(
      (t, i) => i < idx && t.nesting === 1 && t.type === 'blockquote_open'
    )

    if (openToken && openToken.meta && openToken.meta.isCallout) {
      return '</div></div>'
    }

    return '</blockquote>'
  }

  // Enhance code blocks with language indicator and better styling
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    const lang = token.info ? token.info.trim() : ''
    const langClass = lang ? `language-${escapeHtml(lang)}` : ''
    const code = token.content

    return `<div class="md-code-block-wrapper">
      ${lang ? `<div class="md-code-lang">${escapeHtml(lang)}</div>` : ''}
      <pre class="md-code-block ${langClass}"><code>${code}</code></pre>
    </div>`
  }

  // Enhance inline code
  const defaultCodeInline = md.renderer.rules.code_inline
  md.renderer.rules.code_inline = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    const code = escapeHtml(token.content)
    return `<code class="md-inline-code">${code}</code>`
  }

  // Enhance table with wrapper for responsive scrolling
  md.renderer.rules.table_open = () => {
    return '<div class="md-table-wrapper"><table class="md-table">'
  }

  md.renderer.rules.table_close = () => {
    return '</table></div>'
  }

  // Enhance list rendering
  md.renderer.rules.bullet_list_open = (tokens, idx) => {
    const token = tokens[idx]
    const level = getListLevel(tokens, idx)
    return `<ul class="md-list md-list-level-${level}">`
  }

  md.renderer.rules.ordered_list_open = (tokens, idx) => {
    const token = tokens[idx]
    const level = getListLevel(tokens, idx)
    const start = token.attrGet('start') || '1'
    return `<ol class="md-list md-ordered-list md-list-level-${level}" start="${start}">`
  }

  // Enhanced emphasis (italic) with better styling
  md.renderer.rules.em_open = () => `<em class="md-emphasis">`
  md.renderer.rules.strong_open = () => `<strong class="md-strong">`

  // Math renderers
  md.renderer.rules.math_inline = (tokens, idx) => {
    const token = tokens[idx]
    const content = escapeHtml(token.content)
    return `<span class="md-math-inline" data-math="${content}"><script type="math/tex">${content}</script></span>`
  }

  md.renderer.rules.math_block = (tokens, idx) => {
    const token = tokens[idx]
    const content = escapeHtml(token.content)
    return `<div class="md-math-block"><script type="math/tex; mode=display">${content}</script></div>`
  }
}

// ============================================================================
// METADATA EXTRACTION UTILITIES
// ============================================================================

/**
 * Extracts headings from markdown content
 *
 * @private
 */
const extractHeadings = (content) => {
  const headingRegex = /^(#{1,6})\s+(.+)$/gm
  const headings = []
  let match

  while ((match = headingRegex.exec(content)) !== null) {
    const level = match[1].length
    const text = match[2].trim()
    const id = slugify(text)

    headings.push({ level, text, id })
  }

  return headings
}

/**
 * Extracts wikilinks from content
 *
 * @private
 */
const extractWikilinks = (content) => {
  const wikilinkRegex = /\[\[([^\]]+)\]\]/g
  const wikilinks = []
  let match

  while ((match = wikilinkRegex.exec(content)) !== null) {
    wikilinks.push(match[1])
  }

  return wikilinks
}

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
    .trim()

  const words = plainText.split(/\s+/).filter((w) => w.length > 0).length
  const chars = plainText.length
  const readingTime = Math.ceil(words / 200)

  return { words, chars, readingTime }
}

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
      stats: { words: 0, chars: 0, readingTime: 0 },
    }
  }

  const md = getMdInstance()

  const { frontmatter, content, tags: frontmatterTags } = extractFrontmatter(rawContent)
  const inlineTags = extractInlineTags(content)
  const allTags = getAllTags(frontmatterTags, inlineTags)
  let html = md.render(content)

  // Post-process HTML to detect and embed YouTube videos
  html = embedYouTubeVideos(html)

  const headings = extractHeadings(content)
  const wikilinks = extractWikilinks(content)
  const stats = calculateStats(content)

  return {
    html,
    tags: allTags,
    frontmatter,
    headings,
    wikilinks,
    stats,
  }
}

/**
 * Post-processes HTML to detect YouTube links and replace with embedded players
 *
 * @private
 */
const embedYouTubeVideos = (html) => {
  // Match YouTube links in href attributes and replace with embedded player
  const youtubeRegex =
    /<a[^>]+href=["']([^"']*(?:youtube\.com|youtu\.be)[^"']*)["'][^>]*>[^<]*<\/a>/gi

  return html.replace(youtubeRegex, (match, url) => {
    const videoId = extractYouTubeId(url)

    if (videoId) {
      return `<div class="md-youtube-embed">
        <iframe
          width="100%"
          height="400"
          src="https://www.youtube.com/embed/${escapeHtml(videoId)}"
          frameborder="0"
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowfullscreen>
        </iframe>
      </div>`
    }

    return match
  })
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

/**
 * Determines the nesting level of a list
 *
 * @private
 */
const getListLevel = (tokens, idx) => {
  let level = 1
  for (let i = idx - 1; i >= 0; i--) {
    const token = tokens[i]
    if (token.type === 'bullet_list_open' || token.type === 'ordered_list_open') {
      level++
    } else if (token.type === 'bullet_list_close' || token.type === 'ordered_list_close') {
      level--
    }
  }
  return Math.min(level, 3) // Cap at level 3 for CSS classes
}

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
    .replace(/-+/g, '-')
}

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
    "'": '&#39;',
  }

  return text.replace(/[&<>"']/g, (char) => htmlEscapeMap[char])
}

// ============================================================================
// EXPORTS
// ============================================================================

export default {
  renderObsidianMarkdown,
  extractFrontmatter,
  extractInlineTags,
  getAllTags,
}
