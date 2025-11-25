<template>
  <div class="markdown-content-wrapper">
    <!-- Outline Toggle Button -->
    <button class="outline-toggle" @click="showOutline = !showOutline" title="Toggle outline">
      <i class="fas fa-list"></i>
    </button>

    <!-- Outline Panel -->
    <div v-if="showOutline" class="outline-panel">
      <div class="outline-header">Outline</div>
      <div v-if="markdownResult.headings.length === 0" class="outline-empty">No headings</div>
      <nav v-else class="outline-list">
        <a
          v-for="heading in markdownResult.headings"
          :key="heading.id"
          :href="`#${heading.id}`"
          :class="['outline-item', `outline-level-${heading.level}`]"
          @click.prevent="scrollToHeading(heading.id)"
        >
          {{ heading.text }}
        </a>
      </nav>
    </div>

    <!-- Rendered Markdown Content with Collapsible Sections -->
    <div class="markdown-content" ref="markdownContentRef" v-html="renderedMarkdown"></div>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted } from 'vue';
import { renderObsidianMarkdown } from '../utils/obsidianMarkdownRenderer';

const props = defineProps({
  content: {
    type: String,
    default: '',
  },
});

const emit = defineEmits(['update:markdownResult']);

const markdownContentRef = ref(null);
const showOutline = ref(false);

// Markdown rendering state
const markdownResult = ref({
  html: '',
  tags: [],
  frontmatter: {},
  headings: [],
  wikilinks: [],
  stats: { words: 0, chars: 0, readingTime: 0 }
});

const renderedMarkdown = computed(() => {
  if (!props.content) {
    markdownResult.value = {
      html: '',
      tags: [],
      frontmatter: {},
      headings: [],
      wikilinks: [],
      stats: { words: 0, chars: 0, readingTime: 0 }
    };
    return '';
  }

  // Render markdown with Obsidian features
  markdownResult.value = renderObsidianMarkdown(props.content);

  // Emit updated markdown result to parent
  emit('update:markdownResult', markdownResult.value);

  // Make collapsible sections after rendering
  nextTick(() => {
    makeHeadingsCollapsible();
    renderMathContent();
  });

  return markdownResult.value.html;
});

// Scroll to heading by ID
const scrollToHeading = (headingId) => {
  nextTick(() => {
    const element = document.getElementById(headingId);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  });
};

// Make headings collapsible
const makeHeadingsCollapsible = () => {
  if (!markdownContentRef.value) return;

  nextTick(() => {
    const headings = markdownContentRef.value.querySelectorAll('h2, h3, h4, h5, h6');

    headings.forEach((heading) => {
      // Add toggle button to heading
      const toggleBtn = document.createElement('button');
      toggleBtn.className = 'heading-toggle';
      toggleBtn.innerHTML = '<i class="fas fa-chevron-down"></i>';
      toggleBtn.setAttribute('aria-expanded', 'true');
      heading.insertBefore(toggleBtn, heading.firstChild);

      // Collect all elements until next heading of same or higher level
      const headingLevel = parseInt(heading.tagName[1]);
      const contentElements = [];
      let nextElement = heading.nextElementSibling;

      while (nextElement) {
        if (nextElement.tagName && /^H[1-6]$/.test(nextElement.tagName)) {
          const nextLevel = parseInt(nextElement.tagName[1]);
          if (nextLevel <= headingLevel) break;
        }
        contentElements.push(nextElement);
        nextElement = nextElement.nextElementSibling;
      }

      // Create wrapper for collapsible content
      const contentWrapper = document.createElement('div');
      contentWrapper.className = 'collapsible-content';
      contentWrapper.style.display = 'block';

      contentElements.forEach((el) => {
        contentWrapper.appendChild(el.cloneNode(true));
      });

      heading.parentNode.insertBefore(contentWrapper, heading.nextSibling);

      // Toggle handler
      toggleBtn.addEventListener('click', (e) => {
        e.preventDefault();
        const isExpanded = contentWrapper.style.display !== 'none';
        contentWrapper.style.display = isExpanded ? 'none' : 'block';
        toggleBtn.setAttribute('aria-expanded', isExpanded ? 'false' : 'true');
        toggleBtn.classList.toggle('collapsed', isExpanded);
      });
    });

    // Remove original content elements to avoid duplicates
    headings.forEach((heading) => {
      const headingLevel = parseInt(heading.tagName[1]);
      let nextElement = heading.nextElementSibling;

      while (nextElement) {
        if (nextElement.tagName && /^H[1-6]$/.test(nextElement.tagName)) {
          const nextLevel = parseInt(nextElement.tagName[1]);
          if (nextLevel <= headingLevel) break;
        }
        const toRemove = nextElement;
        nextElement = nextElement.nextElementSibling;

        // Only remove if not already in a collapsible-content wrapper
        if (!toRemove.classList.contains('collapsible-content') &&
            toRemove.parentNode &&
            !toRemove.parentNode.classList.contains('collapsible-content')) {
          toRemove.remove();
        }
      }
    });
  });
};

// Render math content using KaTeX
const renderMathContent = () => {
  if (!markdownContentRef.value) return;

  nextTick(() => {
    const mathElements = markdownContentRef.value.querySelectorAll('.md-math-inline, .md-math-block');

    if (mathElements.length === 0) return;

    // Load KaTeX dynamically if not already loaded
    if (!window.katex) {
      // Load KaTeX CSS
      const katexCss = document.createElement('link');
      katexCss.rel = 'stylesheet';
      katexCss.href = 'https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.css';
      document.head.appendChild(katexCss);

      // Load KaTeX JS
      const katexJs = document.createElement('script');
      katexJs.src = 'https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.js';
      katexJs.async = true;
      katexJs.onload = () => renderMathElements(mathElements);
      document.head.appendChild(katexJs);
    } else {
      renderMathElements(mathElements);
    }
  });
};

// Helper to render individual math elements
const renderMathElements = (mathElements) => {
  mathElements.forEach((el) => {
    const script = el.querySelector('script');
    if (!script) return;

    const content = script.textContent;
    const isBlock = script.type === 'math/tex; mode=display';

    try {
      window.katex.render(content, el, {
        throwOnError: false,
        displayMode: isBlock
      });
    } catch (error) {
      console.warn('[Math Render Error]', error);
    }
  });
};

// Expose markdownResult for parent component
onMounted(() => {
  emit('update:markdownResult', markdownResult.value);
});
</script>

<style scoped>
.markdown-content-wrapper {
  flex: 1;
  height: 100%;
  overflow-y: auto;
  padding: 0;
  min-width: 0;
  position: relative;
  min-height: 0;
}

.markdown-content {
  color: var(--text-color);
  background-color: var(--background-color);
  line-height: 1.65;
  padding: clamp(1.5rem, 4vw, 2.5rem);
  margin: 0;
  text-align: left;
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-top: none;
  border-radius: 0 0 8px 8px;
  font-size: clamp(0.875rem, 2vw, 0.9375rem);
  letter-spacing: 0.01em;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

/* HEADING STYLES - OBSIDIAN LIKE */

.markdown-content h1 {
  font-size: clamp(1.5em, 3vw, 1.75em);
  font-weight: 700;
  margin-top: clamp(1.2em, 2.5vw, 1.5em);
  margin-bottom: clamp(0.4em, 1vw, 0.5em);
  color: var(--md-heading-color);
  line-height: 1.25;
}

.markdown-content h2 {
  font-size: clamp(1.3em, 2.5vw, 1.5em);
  font-weight: 700;
  margin-top: clamp(1em, 2vw, 1.3em);
  margin-bottom: clamp(0.35em, 0.8vw, 0.4em);
  color: var(--md-heading-color);
  line-height: 1.25;
  display: flex;
  align-items: center;
  gap: 0.4em;
}

.markdown-content h3 {
  font-size: clamp(1.15em, 2.2vw, 1.3em);
  font-weight: 600;
  margin-top: clamp(0.9em, 1.8vw, 1.1em);
  margin-bottom: clamp(0.3em, 0.7vw, 0.35em);
  color: var(--md-heading-color);
  line-height: 1.3;
  display: flex;
  align-items: center;
  gap: 0.4em;
}

.markdown-content h4 {
  font-size: clamp(1.05em, 2vw, 1.15em);
  font-weight: 600;
  margin-top: clamp(0.8em, 1.5vw, 1em);
  margin-bottom: clamp(0.25em, 0.6vw, 0.3em);
  color: var(--md-heading-color);
  line-height: 1.3;
  display: flex;
  align-items: center;
  gap: 0.4em;
}

.markdown-content h5 {
  font-size: clamp(0.975em, 1.8vw, 1.05em);
  font-weight: 600;
  margin-top: clamp(0.7em, 1.3vw, 0.85em);
  margin-bottom: clamp(0.2em, 0.5vw, 0.25em);
  color: var(--md-heading-color);
  line-height: 1.3;
}

.markdown-content h6 {
  font-size: clamp(0.925em, 1.6vw, 1em);
  font-weight: 600;
  margin-top: clamp(0.65em, 1.2vw, 0.8em);
  margin-bottom: clamp(0.2em, 0.5vw, 0.25em);
  color: var(--md-heading-color);
  line-height: 1.3;
}

.heading-toggle {
  background: none;
  border: none;
  color: var(--md-heading-color);
  cursor: pointer;
  padding: 0 0.3em;
  margin: 0;
  font-size: 0.75em;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.2s ease;
  flex-shrink: 0;
  opacity: 0.6;
}

.heading-toggle:hover {
  opacity: 1;
}

.heading-toggle.collapsed {
  transform: rotate(-90deg);
}

.collapsible-content {
  transition: max-height 0.3s ease;
}

/* ============================================================================ */
/* CALLOUT / ADMONITION STYLING (Obsidian-like) */
/* ============================================================================ */

.md-callout {
  margin: clamp(0.75em, 1.5vw, 1em) 0;
  border-left: 3px solid;
  border-radius: 8px;
  padding: clamp(0.75em, 1.5vw, 0.875em) clamp(0.875em, 2vw, 1em);
  background-color: var(--background-color-light);
  display: flex;
  flex-direction: column;
}

.md-callout-header {
  display: flex;
  align-items: center;
  gap: clamp(0.5em, 1.2vw, 0.625em);
  font-weight: 600;
  font-size: clamp(0.85em, 1.8vw, 0.9em);
  margin-bottom: clamp(0.4em, 1vw, 0.5em);
  text-transform: none;
}

.md-callout-icon {
  font-size: clamp(1.1em, 2vw, 1.2em);
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.md-callout-title {
  color: var(--text-color);
  font-weight: 600;
}

.md-callout-content {
  padding-left: clamp(1.75em, 3vw, 2em);
  color: var(--text-color);
  font-size: clamp(0.9em, 1.8vw, 0.95em);
  line-height: 1.65;
}

.md-callout-content > :first-child {
  margin-top: 0;
}

.md-callout-content > :last-child {
  margin-bottom: 0;
}

/* Note - Blue */
.md-callout-note,
.md-callout-abstract,
.md-callout-summary,
.md-callout-tldr {
  border-left-color: #0ea5e9;
  background-color: rgba(14, 165, 233, 0.08);
}

.md-callout-note .md-callout-title,
.md-callout-abstract .md-callout-title,
.md-callout-summary .md-callout-title,
.md-callout-tldr .md-callout-title {
  color: #0ea5e9;
}

/* Info - Cyan */
.md-callout-info {
  border-left-color: #06b6d4;
  background-color: rgba(6, 182, 212, 0.08);
}

.md-callout-info .md-callout-title {
  color: #06b6d4;
}

/* Tip/Hint - Green */
.md-callout-tip,
.md-callout-hint {
  border-left-color: #10b981;
  background-color: rgba(16, 185, 129, 0.08);
}

.md-callout-tip .md-callout-title,
.md-callout-hint .md-callout-title {
  color: #10b981;
}

/* Important - Purple */
.md-callout-important {
  border-left-color: #a855f7;
  background-color: rgba(168, 85, 247, 0.08);
}

.md-callout-important .md-callout-title {
  color: #a855f7;
}

/* Warning/Caution - Yellow/Orange */
.md-callout-warning,
.md-callout-caution,
.md-callout-attention {
  border-left-color: #f59e0b;
  background-color: rgba(245, 158, 11, 0.08);
}

.md-callout-warning .md-callout-title,
.md-callout-caution .md-callout-title,
.md-callout-attention .md-callout-title {
  color: #f59e0b;
}

/* Danger/Error/Failure - Red */
.md-callout-danger,
.md-callout-error,
.md-callout-failure,
.md-callout-bug {
  border-left-color: #ef4444;
  background-color: rgba(239, 68, 68, 0.08);
}

.md-callout-danger .md-callout-title,
.md-callout-error .md-callout-title,
.md-callout-failure .md-callout-title,
.md-callout-bug .md-callout-title {
  color: #ef4444;
}

/* Example - Pink */
.md-callout-example {
  border-left-color: #ec4899;
  background-color: rgba(236, 72, 153, 0.08);
}

.md-callout-example .md-callout-title {
  color: #ec4899;
}

/* Quote - Gray */
.md-callout-quote {
  border-left-color: #6b7280;
  background-color: rgba(107, 114, 128, 0.08);
}

.md-callout-quote .md-callout-title {
  color: #6b7280;
}

/* ============================================================================ */
/* BLOCKQUOTE (regular, non-callout) */
/* ============================================================================ */

.md-blockquote {
  border-left: 4px solid var(--md-blockquote-border);
  padding-left: 1em;
  margin: 0.75em 0;
  color: var(--md-blockquote-text);
}

/* ============================================================================ */
/* CODE STYLING - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content .md-inline-code {
  background-color: rgba(59, 130, 246, 0.15);
  color: #3b82f6;
  border-radius: 3px;
  padding: 0.2em 0.4em;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.9em;
  border: none;
  white-space: nowrap;
  font-weight: 500;
}

.markdown-content .md-code-block-wrapper {
  margin: clamp(0.75em, 1.5vw, 1em) 0;
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 8px;
  overflow: hidden;
  background-color: var(--md-pre-bg);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.08);
}

.markdown-content .md-code-lang {
  background-color: rgba(0, 0, 0, 0.3);
  padding: clamp(0.375em, 1vw, 0.4375em) clamp(0.75em, 1.8vw, 0.875em);
  font-size: clamp(0.7em, 1.6vw, 0.75em);
  color: var(--md-code-text);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
  font-family: 'Fira Code', monospace;
}

.markdown-content .md-code-block {
  padding: clamp(0.875em, 2vw, 1em);
  overflow-x: auto;
  margin: 0;
  border-radius: 0;
  border: none;
  background-color: transparent;
  color: var(--md-code-text);
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: clamp(0.8em, 1.8vw, 0.85em);
  line-height: 1.55;
}

.markdown-content pre code {
  background-color: transparent;
  color: var(--md-code-text);
  padding: 0;
  font-size: 1em;
  border: none;
}

/* ============================================================================ */
/* LIST STYLING - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content ul,
.markdown-content ol {
  margin: clamp(0.5em, 1.2vw, 0.65em) 0;
  padding-left: clamp(1.5em, 3vw, 2em);
}

.markdown-content ul li,
.markdown-content ol li {
  margin-bottom: clamp(0.25em, 0.8vw, 0.35em);
  line-height: 1.65;
}

.markdown-content .md-list {
  margin: clamp(0.5em, 1.2vw, 0.65em) 0;
  padding-left: clamp(1.5em, 3vw, 2em);
}

.markdown-content .md-list li {
  margin-bottom: clamp(0.25em, 0.8vw, 0.35em);
  line-height: 1.65;
}

.markdown-content .md-list ul,
.markdown-content .md-list ol,
.markdown-content .md-list .md-list {
  margin: 0.3em 0;
  padding-left: 1.8em;
}

.markdown-content .md-list-level-2 {
  padding-left: 1.8em;
}

.markdown-content .md-list-level-3 {
  padding-left: 1.8em;
}

/* Ordered list styling */
.markdown-content .md-ordered-list {
  list-style-type: decimal;
}

.markdown-content .md-ordered-list .md-ordered-list {
  list-style-type: lower-alpha;
}

.markdown-content .md-ordered-list .md-ordered-list .md-ordered-list {
  list-style-type: lower-roman;
}

/* Bullet styling for nested lists */
.markdown-content ul {
  list-style-type: disc;
}

.markdown-content ul ul {
  list-style-type: circle;
}

.markdown-content ul ul ul {
  list-style-type: square;
}

/* ============================================================================ */
/* EMPHASIS AND STRONG TEXT - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content strong,
.markdown-content .md-strong {
  font-weight: 600;
  color: var(--text-color);
}

.markdown-content em,
.markdown-content .md-emphasis {
  font-style: italic;
  color: var(--text-color);
}

.markdown-content p {
  margin: clamp(0.5em, 1vw, 0.65em) 0;
  line-height: 1.65;
}

/* ============================================================================ */
/* TABLE STYLING */
/* ============================================================================ */

.markdown-content table {
  width: 100%;
  border-collapse: collapse;
  margin: clamp(0.75em, 1.5vw, 1em) 0;
  border-radius: 8px;
  overflow: hidden;
}

.markdown-content th, .markdown-content td {
  border: 1px solid rgba(128, 128, 128, 0.15);
  padding: clamp(0.5em, 1.2vw, 0.625em) clamp(0.75em, 1.8vw, 0.875em);
  text-align: left;
  font-size: clamp(0.875em, 1.8vw, 0.9em);
}

.markdown-content th {
  background-color: var(--md-table-header-bg);
  font-weight: bold;
  color: var(--text-color);
}

.markdown-content td {
  background-color: var(--background-color);
}

.markdown-content tbody tr:hover {
  background-color: var(--background-color-light);
}

/* ============================================================================ */
/* IMAGE STYLING */
/* ============================================================================ */

.markdown-content img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: clamp(0.75em, 1.5vw, 1em) auto;
  border-radius: 8px;
  border: 1px solid rgba(128, 128, 128, 0.15);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.08);
}

/* ============================================================================ */
/* HORIZONTAL RULE */
/* ============================================================================ */

.markdown-content hr {
  border: none;
  border-top: 2px solid var(--md-hr-color);
  margin: 2em 0;
}

/* ============================================================================ */
/* SPACING AND TYPOGRAPHY - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content {
  font-size: clamp(0.875rem, 2vw, 0.9375rem);
  line-height: 1.65;
}

/* Better spacing between different element types */
.markdown-content h1 + p,
.markdown-content h2 + p,
.markdown-content h3 + p,
.markdown-content h4 + p,
.markdown-content h5 + p,
.markdown-content h6 + p {
  margin-top: clamp(0.2em, 0.6vw, 0.25em);
}

/* Extra spacing before sections */
.markdown-content h2 {
  margin-top: clamp(1.2em, 2.5vw, 1.5em);
}

.markdown-content h3 {
  margin-top: clamp(1em, 2vw, 1.25em);
}

/* ============================================================================ */
/* LINK STYLING - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content a {
  color: #3b82f6;
  text-decoration: none;
  transition: all 0.2s ease;
  font-weight: 500;
}

.markdown-content a:hover {
  color: #2563eb;
  text-decoration: underline;
}

.markdown-content .md-wikilink {
  color: #3b82f6;
  font-weight: 500;
  background-color: rgba(59, 130, 246, 0.1);
  padding: 0.15em 0.35em;
  border-radius: 3px;
  transition: all 0.2s ease;
}

.markdown-content .md-wikilink:hover {
  background-color: rgba(59, 130, 246, 0.2);
  text-decoration: underline;
}

.markdown-content .md-tag {
  color: #8b5cf6;
  font-weight: 500;
  background-color: rgba(139, 92, 246, 0.1);
  padding: 0.15em 0.35em;
  border-radius: 3px;
  font-size: 0.95em;
  transition: all 0.2s ease;
}

.markdown-content .md-tag:hover {
  background-color: rgba(139, 92, 246, 0.2);
}

.markdown-content .md-blockref {
  color: #3b82f6;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.9em;
  background-color: rgba(59, 130, 246, 0.08);
  padding: 0.15em 0.3em;
  border-radius: 3px;
}

/* ============================================================================ */
/* MATH / LATEX STYLING */
/* ============================================================================ */

.markdown-content .md-math-inline {
  display: inline;
  font-family: 'KaTeX_Main', serif;
}

.markdown-content .md-math-inline .katex {
  margin: 0 0.1em;
}

.markdown-content .md-math-block {
  margin: 1.2em 0;
  padding: 1em;
  background-color: rgba(0, 0, 0, 0.03);
  border-left: 3px solid #3b82f6;
  border-radius: 4px;
  overflow-x: auto;
}

.markdown-content .md-math-block .katex-display {
  margin: 0;
}

/* KaTeX styles override for consistency */
.markdown-content .katex {
  color: var(--text-color);
}

/* ============================================================================ */
/* YOUTUBE VIDEO EMBEDDING */
/* ============================================================================ */

.markdown-content .md-youtube-embed {
  margin: 1.5em 0;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  background-color: #000;
}

.markdown-content .md-youtube-embed iframe {
  display: block;
  border: none;
}

/* ============================================================================ */
/* OUTLINE PANEL STYLES */
/* ============================================================================ */

.outline-toggle {
  position: absolute;
  top: clamp(0.75rem, 2vw, 1rem);
  right: clamp(1rem, 3vw, 1.5rem);
  background: transparent;
  border: 1px solid rgba(128, 128, 128, 0.2);
  color: var(--text-color);
  padding: clamp(0.375rem, 1vw, 0.5rem) clamp(0.5rem, 1.5vw, 0.625rem);
  border-radius: 6px;
  cursor: pointer;
  font-size: clamp(0.8rem, 1.8vw, 0.875rem);
  transition: all 0.2s ease;
  z-index: 100;
}

.outline-toggle:hover {
  background-color: rgba(var(--primary-color-rgb, 59, 130, 246), 0.1);
  border-color: rgba(var(--primary-color-rgb, 59, 130, 246), 0.3);
}

.outline-panel {
  position: absolute;
  top: clamp(2.5rem, 5vw, 3rem);
  right: clamp(1rem, 3vw, 1.5rem);
  background-color: var(--background-color-light);
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 8px;
  width: clamp(200px, 30vw, 250px);
  max-height: clamp(300px, 50vh, 400px);
  overflow-y: auto;
  z-index: 99;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.outline-header {
  font-weight: 600;
  font-size: clamp(0.8rem, 1.8vw, 0.875rem);
  padding: clamp(0.5rem, 1.5vw, 0.625rem) clamp(0.75rem, 2vw, 1rem);
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
  color: var(--text-color);
}

.outline-empty {
  padding: 1rem;
  color: var(--text-color-secondary);
  font-size: 0.85rem;
  text-align: center;
}

.outline-list {
  display: flex;
  flex-direction: column;
  padding: clamp(0.375rem, 1vw, 0.5rem);
  gap: clamp(0.125rem, 0.4vw, 0.15rem);
}

.outline-item {
  padding: clamp(0.25rem, 0.8vw, 0.35rem) clamp(0.5rem, 1.2vw, 0.625rem);
  color: var(--md-link-color);
  text-decoration: none;
  font-size: clamp(0.775rem, 1.6vw, 0.8rem);
  border-radius: 4px;
  transition: all 0.2s ease;
  cursor: pointer;
  display: flex;
  align-items: baseline;
  gap: clamp(0.3em, 0.8vw, 0.375em);
  text-align: left;
}

.outline-item::before {
  content: attr(data-marker);
  color: var(--text-color-secondary);
  opacity: 0.6;
  font-weight: 500;
  flex-shrink: 0;
  min-width: 1.2em;
}

.outline-item:hover {
  background-color: rgba(59, 130, 246, 0.08);
  color: var(--md-link-hover);
}

.outline-level-1 {
  padding-left: clamp(0.5rem, 1.2vw, 0.625rem);
}

.outline-level-1::before {
  content: "•";
}

.outline-level-2 {
  padding-left: clamp(1rem, 2vw, 1.25rem);
}

.outline-level-2::before {
  content: "*";
}

.outline-level-3 {
  padding-left: clamp(1.5rem, 3vw, 1.875rem);
}

.outline-level-3::before {
  content: "**";
}

.outline-level-4 {
  padding-left: clamp(2rem, 4vw, 2.5rem);
}

.outline-level-4::before {
  content: "***";
}

.outline-level-5 {
  padding-left: clamp(2.5rem, 5vw, 3.125rem);
}

.outline-level-5::before {
  content: "–";
}

.outline-level-6 {
  padding-left: clamp(3rem, 6vw, 3.75rem);
}

.outline-level-6::before {
  content: "·";
}
</style>
