/**
 * Structured Markdown Renderer
 *
 * Renders markdown from structured data provided by the backend.
 * The backend handles parsing, extraction, and metadata generation.
 * This renderer focuses solely on presentation.
 *
 * @module structuredMarkdownRenderer
 */

import MarkdownIt from 'markdown-it';

/**
 * Create a markdown-it instance for rendering
 */
const createMarkdownInstance = () => {
  return new MarkdownIt({
    html: false, // Don't allow raw HTML for security
    linkify: true,
    typographer: true,
    breaks: false,
  });
};

// Cached markdown instance
let cachedMdInstance = null;

const getMdInstance = () => {
  if (!cachedMdInstance) {
    cachedMdInstance = createMarkdownInstance();
  }
  return cachedMdInstance;
};

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
    return '';
  }

  let content = rawMarkdown;

  // Replace wikilinks with proper links
  content = replaceWikiLinks(content, wikilinks);

  // Replace embeds with appropriate HTML
  content = replaceEmbeds(content, embeds);

  // Render markdown to HTML
  const md = getMdInstance();
  const html = md.render(content);

  return html;
};

/**
 * Replace wikilinks with HTML links
 *
 * @param {string} content - Markdown content
 * @param {Array<Object>} wikilinks - Wikilink metadata
 * @returns {string} - Content with replaced wikilinks
 */
const replaceWikiLinks = (content, wikilinks) => {
  if (!wikilinks || wikilinks.length === 0) {
    return content;
  }

  let result = content;

  for (const link of wikilinks) {
    const { original, display, exists, file_id } = link;

    if (!original) continue;

    // Create link HTML
    const linkClass = exists ? 'md-wikilink' : 'md-wikilink-broken';
    const href = exists && file_id ? `#file-${file_id}` : '#';
    const title = exists ? `Open ${display}` : `File not found: ${display}`;

    const replacement = `<a href="${escapeHtml(href)}" class="${linkClass}" title="${escapeHtml(title)}" data-file-id="${escapeHtml(file_id || '')}">${escapeHtml(display)}</a>`;

    // Escape regex special characters in original
    const escapedOriginal = original.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    result = result.replace(new RegExp(escapedOriginal, 'g'), replacement);
  }

  return result;
};

/**
 * Replace embeds with HTML
 *
 * @param {string} content - Markdown content
 * @param {Array<Object>} embeds - Embed metadata
 * @returns {string} - Content with replaced embeds
 */
const replaceEmbeds = (content, embeds) => {
  if (!embeds || embeds.length === 0) {
    return content;
  }

  let result = content;

  for (const embed of embeds) {
    const { type, target, exists } = embed;
    const original = `![[${target}]]`;

    let replacement;

    if (!exists) {
      // Embed not found
      replacement = `<div class="md-embed-not-found">
        <i class="fas fa-exclamation-triangle"></i>
        <span>Embed not found: ${escapeHtml(target)}</span>
      </div>`;
    } else if (type === 'image') {
      // Image embed - will be handled later when image URLs are implemented
      replacement = `<div class="md-embed-image">
        <img src="#" alt="${escapeHtml(target)}" data-embed="${escapeHtml(target)}" />
        <p class="md-embed-caption">${escapeHtml(target)}</p>
      </div>`;
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
      </div>`;
    } else {
      // Other embeds (PDF, video, audio)
      replacement = `<div class="md-embed-${type}" data-embed="${escapeHtml(target)}">
        <i class="fas fa-file"></i>
        <span>${escapeHtml(target)}</span>
      </div>`;
    }

    // Escape regex special characters
    const escapedOriginal = original.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    result = result.replace(new RegExp(escapedOriginal, 'g'), replacement);
  }

  return result;
};

/**
 * Render tags as clickable elements
 *
 * @param {Array<Object>} tags - Tag metadata with counts
 * @returns {Array<Object>} - Processed tags
 */
export const processTags = (tags) => {
  if (!tags || tags.length === 0) {
    return [];
  }

  return tags.map(tag => ({
    name: tag.name,
    count: tag.count,
    display: `#${tag.name}`,
    clickable: true
  }));
};

/**
 * Build outline data structure from headings
 *
 * @param {Array<Object>} headings - Heading metadata
 * @returns {Array<Object>} - Outline structure
 */
export const buildOutline = (headings) => {
  if (!headings || headings.length === 0) {
    return [];
  }

  return headings.map(heading => ({
    id: heading.id,
    text: heading.text,
    level: heading.level,
    line: heading.line,
    indentClass: `outline-level-${heading.level}`
  }));
};

/**
 * Format reading time
 *
 * @param {number} minutes - Reading time in minutes
 * @returns {string} - Formatted reading time
 */
export const formatReadingTime = (minutes) => {
  if (!minutes || minutes === 0) {
    return 'Less than a minute';
  }

  if (minutes === 1) {
    return '1 minute';
  }

  return `${minutes} minutes`;
};

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
      readingTime: 'Less than a minute'
    };
  }

  return {
    words: stats.words || 0,
    characters: stats.characters || 0,
    readingTime: formatReadingTime(stats.reading_time_minutes)
  };
};

/**
 * Escape HTML special characters
 *
 * @param {string} text - Text to escape
 * @returns {string} - Escaped text
 */
const escapeHtml = (text) => {
  if (!text) return '';

  const htmlEscapeMap = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  };

  return String(text).replace(/[&<>"']/g, char => htmlEscapeMap[char]);
};

/**
 * Process backlinks for display
 *
 * @param {Array<Object>} backlinks - Backlink metadata
 * @returns {Array<Object>} - Processed backlinks
 */
export const processBacklinks = (backlinks) => {
  if (!backlinks || backlinks.length === 0) {
    return [];
  }

  return backlinks.map(backlink => ({
    id: backlink.file_id,
    name: backlink.file_name,
    path: backlink.file_path,
    context: backlink.context || '',
    clickable: true
  }));
};

export default {
  renderStructuredMarkdown,
  processTags,
  buildOutline,
  formatStats,
  formatReadingTime,
  processBacklinks
};
