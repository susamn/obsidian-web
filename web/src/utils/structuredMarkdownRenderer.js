/**
 * Structured Markdown Renderer
 *
 * Renders markdown from structured data provided by the backend.
 * The backend handles parsing, extraction, and metadata generation.
 * This renderer focuses solely on presentation.
 *
 * @module structuredMarkdownRenderer
 */

/**
 * Callout/Admonition type configuration
 * Maps callout types to icons and CSS classes (Obsidian-style)
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
 * Create a markdown-it instance for rendering with callout support
 */
const createMarkdownInstance = () => {
  const md = new MarkdownIt({
    html: false, // Don't allow raw HTML for security
    linkify: true,
    typographer: true,
    breaks: false,
  })

  // Add callout rendering support
  enhanceMarkdownWithCallouts(md)

  return md
}

/**
 * Enhance markdown-it instance with Obsidian-style callout rendering
 */
const enhanceMarkdownWithCallouts = (md) => {
  // Enhanced blockquote renderer for callouts
  md.renderer.rules.blockquote_open = (tokens, idx, options, env, self) => {
    const token = tokens[idx]

    // Look for the first inline token in this blockquote
    let contentToken = null
    for (let i = idx + 1; i < tokens.length; i++) {
      if (tokens[i].type === 'blockquote_close') break
      if (tokens[i].type === 'inline') {
        contentToken = tokens[i]
        break
      }
    }

    // Check if this is a callout (> [!TYPE] Title format)
    if (contentToken) {
      const match = contentToken.content.match(/^\[!(\w+)\]\s*(.*)$/)
      if (match) {
        const type = match[1].toLowerCase()
        const title = match[2] || ''
        const calloutConfig = CALLOUT_TYPES[type] || CALLOUT_TYPES.note

        token.meta = token.meta || {}
        token.meta.isCallout = true
        token.meta.calloutType = type
        token.meta.calloutTitle = title
        token.meta.calloutConfig = calloutConfig

        // Remove the callout marker from content
        contentToken.content = contentToken.content.replace(/^\[!(\w+)\]\s*(.*)$/, '')

        // If the content is now empty, mark it
        if (contentToken.content.trim() === '') {
          contentToken.content = ''
        }

        const { icon, class: calloutClass, label } = calloutConfig
        const displayTitle = title || label

        return `<div class="md-callout ${calloutClass}">
        <div class="md-callout-header">
          <span class="md-callout-icon">${icon}</span>
          <span class="md-callout-title">${escapeHtml(displayTitle)}</span>
        </div>
        <div class="md-callout-content">`
      }
    }

    return '<blockquote class="md-blockquote">'
  }

  md.renderer.rules.blockquote_close = (tokens, idx, options, env, self) => {
    // Find the matching opening token
    let openToken = null
    let nestLevel = 0

    for (let i = idx - 1; i >= 0; i--) {
      if (tokens[i].type === 'blockquote_close') {
        nestLevel++
      } else if (tokens[i].type === 'blockquote_open') {
        if (nestLevel === 0) {
          openToken = tokens[i]
          break
        }
        nestLevel--
      }
    }

    if (openToken && openToken.meta && openToken.meta.isCallout) {
      return '</div></div>'
    }

    return '</blockquote>'
  }
}

// Cached markdown instance
let cachedMdInstance = null

const getMdInstance = () => {
  if (!cachedMdInstance) {
    cachedMdInstance = createMarkdownInstance()
  }
  return cachedMdInstance
}

/**
 * Render markdown content with resolved wikilinks and embeds
 *
 * @param {string} rawMarkdown - Raw markdown content
 * @param {Array<Object>} wikilinks - Resolved wikilinks from backend
 * @param {Array<Object>} embeds - Resolved embeds from backend
 * @returns {string} - Rendered HTML
 */
export const renderStructuredMarkdown = (rawMarkdown, wikilinks = [], embeds = []) => {
  if (!rawMarkdown) {
    return ''
  }

  let content = rawMarkdown

  // First, replace embeds with appropriate HTML (these need to be in markdown)
  content = replaceEmbeds(content, embeds)

  // Render markdown to HTML first
  const md = getMdInstance()
  let html = md.render(content)

  // Then replace wikilinks in the rendered HTML
  html = replaceWikiLinksInHTML(html, wikilinks)

  return html
}

/**
 * Replace wikilinks in rendered HTML
 *
 * @param {string} html - Rendered HTML content
 * @param {Array<Object>} wikilinks - Wikilink metadata
 * @returns {string} - HTML with replaced wikilinks
 */
const replaceWikiLinksInHTML = (html, wikilinks) => {
  if (!wikilinks || wikilinks.length === 0) {
    return html
  }

  let result = html

  for (const link of wikilinks) {
    const { original, display, exists, file_id } = link

    if (!original) continue

    // Create pill-shaped wiki link with "Backlink" label
    const linkClass = exists ? 'md-wikilink-pill' : 'md-wikilink-pill-broken'
    const href = exists && file_id ? `#file-${file_id}` : '#'
    const title = exists ? `Open ${display}` : `File not found: ${display}`

    // Create a pill with two parts: label and content
    const replacement = `<span class="${linkClass}" title="${escapeHtml(title)}"><a href="${escapeHtml(href)}" class="md-wikilink-pill-link" data-file-id="${escapeHtml(file_id || '')}"><span class="md-wikilink-label">B</span><span class="md-wikilink-content">${escapeHtml(display)}</span></a></span>`

    // Escape regex special characters in original wikilink syntax
    const escapedOriginal = escapeRegex(original)

    // Replace the wikilink pattern in the HTML (it appears as plain text in paragraphs)
    result = result.replace(new RegExp(escapedOriginal, 'g'), replacement)
  }

  return result
}

/**
 * Escape regex special characters
 *
 * @param {string} str - String to escape
 * @returns {string} - Escaped string
 */
const escapeRegex = (str) => {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Replace embeds with HTML
 *
 * @param {string} content - Markdown content
 * @param {Array<Object>} embeds - Embed metadata
 * @returns {string} - Content with replaced embeds
 */
const replaceEmbeds = (content, embeds) => {
  if (!embeds || embeds.length === 0) {
    return content
  }

  let result = content

  for (const embed of embeds) {
    const { type, target, exists } = embed
    const original = `![[${target}]]`

    let replacement

    if (!exists) {
      // Embed not found
      replacement = `<div class="md-embed-not-found">
        <i class="fas fa-exclamation-triangle"></i>
        <span>Embed not found: ${escapeHtml(target)}</span>
      </div>`
    } else if (type === 'image') {
      // Image embed - will be handled later when image URLs are implemented
      replacement = `<div class="md-embed-image">
        <img src="#" alt="${escapeHtml(target)}" data-embed="${escapeHtml(target)}" />
        <p class="md-embed-caption">${escapeHtml(target)}</p>
      </div>`
    } else if (type === 'note') {
      // Note embed - placeholder for now
      replacement = `<div class="md-embed-note" data-embed="${escapeHtml(target)}">
        <div class="md-embed-note-header">
          <i class="fas fa-file-alt"></i>
          <span>${escapeHtml(target)}</span>
        </div>
        <div class="md-embed-note-placeholder">
          <p>Embedded note content will appear here</p>
        </div>
      </div>`
    } else {
      // Other embeds (PDF, video, audio)
      replacement = `<div class="md-embed-${type}" data-embed="${escapeHtml(target)}">
        <i class="fas fa-file"></i>
        <span>${escapeHtml(target)}</span>
      </div>`
    }

    // Escape regex special characters
    const escapedOriginal = original.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    result = result.replace(new RegExp(escapedOriginal, 'g'), replacement)
  }

  return result
}

/**
 * Render tags as clickable elements
 *
 * @param {Array<Object>} tags - Tag metadata with counts
 * @returns {Array<Object>} - Processed tags
 */
export const processTags = (tags) => {
  if (!tags || tags.length === 0) {
    return []
  }

  return tags.map((tag) => ({
    name: tag.name,
    count: tag.count,
    display: `#${tag.name}`,
    clickable: true,
  }))
}

/**
 * Build outline data structure from headings
 *
 * @param {Array<Object>} headings - Heading metadata
 * @returns {Array<Object>} - Outline structure
 */
export const buildOutline = (headings) => {
  if (!headings || headings.length === 0) {
    return []
  }

  return headings.map((heading) => ({
    id: heading.id,
    text: heading.text,
    level: heading.level,
    line: heading.line,
    indentClass: `outline-level-${heading.level}`,
  }))
}

/**
 * Format reading time
 *
 * @param {number} minutes - Reading time in minutes
 * @returns {string} - Formatted reading time
 */
export const formatReadingTime = (minutes) => {
  if (!minutes || minutes === 0) {
    return 'Less than a minute'
  }

  if (minutes === 1) {
    return '1 minute'
  }

  return `${minutes} minutes`
}

/**
 * Format file statistics
 *
 * @param {Object} stats - File statistics
 * @returns {Object} - Formatted statistics
 */
export const formatStats = (stats) => {
  if (!stats) {
    return {
      words: 0,
      characters: 0,
      readingTime: 'Less than a minute',
    }
  }

  return {
    words: stats.words || 0,
    characters: stats.characters || 0,
    readingTime: formatReadingTime(stats.reading_time_minutes),
  }
}

/**
 * Escape HTML special characters
 *
 * @param {string} text - Text to escape
 * @returns {string} - Escaped text
 */
const escapeHtml = (text) => {
  if (!text) return ''

  const htmlEscapeMap = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }

  return String(text).replace(/[&<>"']/g, (char) => htmlEscapeMap[char])
}

/**
 * Process backlinks for display
 *
 * @param {Array<Object>} backlinks - Backlink metadata
 * @returns {Array<Object>} - Processed backlinks
 */
export const processBacklinks = (backlinks) => {
  if (!backlinks || backlinks.length === 0) {
    return []
  }

  return backlinks.map((backlink) => ({
    id: backlink.file_id,
    name: backlink.file_name,
    path: backlink.file_path,
    context: backlink.context || '',
    clickable: true,
  }))
}

export default {
  renderStructuredMarkdown,
  processTags,
  buildOutline,
  formatStats,
  formatReadingTime,
  processBacklinks,
}
