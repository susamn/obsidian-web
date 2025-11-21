<template>
  <div class="structured-renderer-wrapper">
    <!-- Loading State -->
    <div v-if="loading" class="sr-loading">
      <i class="fas fa-spinner fa-spin"></i>
      <p>Loading structured data...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="sr-error">
      <i class="fas fa-exclamation-circle"></i>
      <p>Error loading file</p>
      <p class="error-details">{{ error }}</p>
    </div>

    <!-- Rendered Content -->
    <div v-else-if="structuredData" class="sr-content-container">
      <!-- Outline Toggle Button -->
      <button
        v-if="outline.length > 0"
        class="outline-toggle"
        @click="showOutline = !showOutline"
        title="Toggle outline"
      >
        <i class="fas fa-list"></i>
      </button>

      <!-- Outline Panel -->
      <div v-if="showOutline && outline.length > 0" class="outline-panel">
        <div class="outline-header">Outline</div>
        <nav class="outline-list">
          <a
            v-for="heading in outline"
            :key="heading.id"
            :href="`#${heading.id}`"
            :class="['outline-item', heading.indentClass]"
            @click.prevent="scrollToHeading(heading.id)"
          >
            {{ heading.text }}
          </a>
        </nav>
      </div>

      <!-- Main Content -->
      <div class="sr-markdown-content" ref="markdownContentRef" v-html="renderedHTML"></div>

      <!-- Metadata Panels (optional) -->
      <div v-if="showMetadata" class="sr-metadata-panels">
        <!-- Tags -->
        <div v-if="tags.length > 0" class="sr-metadata-panel">
          <h3 class="metadata-title">
            <i class="fas fa-tags"></i>
            Tags
          </h3>
          <div class="tag-list">
            <span
              v-for="tag in tags"
              :key="tag.name"
              class="md-tag"
              :title="`${tag.count} file(s)`"
            >
              {{ tag.display }}
            </span>
          </div>
        </div>

        <!-- Backlinks -->
        <div v-if="backlinks.length > 0" class="sr-metadata-panel">
          <h3 class="metadata-title">
            <i class="fas fa-link"></i>
            Backlinks ({{ backlinks.length }})
          </h3>
          <div class="backlink-list">
            <a
              v-for="backlink in backlinks"
              :key="backlink.id"
              :href="`#file-${backlink.id}`"
              class="backlink-item"
              :title="backlink.path"
            >
              <i class="fas fa-file-alt"></i>
              <span>{{ backlink.name }}</span>
            </a>
          </div>
        </div>

        <!-- Stats -->
        <div v-if="stats" class="sr-metadata-panel sr-stats">
          <h3 class="metadata-title">
            <i class="fas fa-chart-bar"></i>
            Statistics
          </h3>
          <div class="stats-grid">
            <div class="stat-item">
              <span class="stat-label">Words</span>
              <span class="stat-value">{{ stats.words.toLocaleString() }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Characters</span>
              <span class="stat-value">{{ stats.characters.toLocaleString() }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Reading Time</span>
              <span class="stat-value">{{ stats.readingTime }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="sr-placeholder">
      <i class="fas fa-file-alt"></i>
      <p>Structured renderer ready</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue';
import {
  renderStructuredMarkdown,
  processTags,
  buildOutline,
  formatStats,
  processBacklinks
} from '../utils/structuredMarkdownRenderer';

const props = defineProps({
  vaultId: {
    type: String,
    required: true
  },
  fileId: {
    type: String,
    required: true
  }
});

const emit = defineEmits(['update:markdownResult']);

const loading = ref(false);
const error = ref(null);
const structuredData = ref(null);
const markdownContentRef = ref(null);
const showOutline = ref(false);
const showMetadata = ref(true);

// Computed properties
const renderedHTML = computed(() => {
  if (!structuredData.value) return '';

  return renderStructuredMarkdown(
    structuredData.value.raw_markdown,
    structuredData.value.wikilinks,
    structuredData.value.embeds
  );
});

const tags = computed(() => {
  if (!structuredData.value?.tags) return [];
  return processTags(structuredData.value.tags);
});

const outline = computed(() => {
  if (!structuredData.value?.headings) return [];
  return buildOutline(structuredData.value.headings);
});

const backlinks = computed(() => {
  if (!structuredData.value?.backlinks) return [];
  return processBacklinks(structuredData.value.backlinks);
});

const stats = computed(() => {
  if (!structuredData.value?.stats) return null;
  return formatStats(structuredData.value.stats);
});

// Fetch structured data from backend
const fetchStructuredData = async () => {
  if (!props.vaultId || !props.fileId) {
    error.value = null;
    structuredData.value = null;
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    const response = await fetch(
      `/api/v1/files/sr/by-id/${props.vaultId}/${props.fileId}`
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to load file');
    }

    const data = await response.json();

    // Backend returns {data: {...}} structure
    if (data.data) {
      structuredData.value = data.data;

      // Emit markdown result for parent component
      emit('update:markdownResult', {
        html: renderedHTML.value,
        tags: data.data.tags || [],
        frontmatter: data.data.frontmatter || {},
        headings: data.data.headings || [],
        wikilinks: data.data.wikilinks || [],
        stats: data.data.stats || {}
      });
    } else {
      throw new Error('Invalid response format');
    }
  } catch (err) {
    console.error('[StructuredRenderer] Error fetching data:', err);
    error.value = err.message || 'Failed to load structured data';
    structuredData.value = null;
  } finally {
    loading.value = false;
  }
};

// Scroll to heading
const scrollToHeading = (headingId) => {
  nextTick(() => {
    const element = document.getElementById(headingId);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  });
};

// Watch for file ID changes
watch(() => props.fileId, () => {
  fetchStructuredData();
}, { immediate: true });

// Watch for vault ID changes
watch(() => props.vaultId, () => {
  structuredData.value = null;
  error.value = null;
});
</script>

<style scoped>
.structured-renderer-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 0;
  min-width: 0;
  position: relative;
}

.sr-loading,
.sr-error,
.sr-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 3rem;
  color: var(--text-color-secondary);
  min-height: 400px;
}

.sr-loading i,
.sr-error i,
.sr-placeholder i {
  font-size: 3rem;
  margin-bottom: 1rem;
  opacity: 0.5;
}

.sr-loading i {
  color: #3b82f6;
}

.sr-error i {
  color: #ef4444;
}

.error-details {
  font-size: 0.85rem;
  color: #ef4444;
  margin-top: 0.5rem;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
}

.sr-content-container {
  position: relative;
  width: 100%;
}

/* Outline Toggle Button */
.outline-toggle {
  position: absolute;
  top: 1.5rem;
  right: 2rem;
  background: transparent;
  border: 1px solid var(--border-color);
  color: var(--text-color);
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: all 0.2s ease;
  z-index: 100;
}

.outline-toggle:hover {
  background-color: var(--background-color-light);
}

/* Outline Panel */
.outline-panel {
  position: absolute;
  top: 3.5rem;
  right: 2rem;
  background-color: var(--background-color-light);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  width: 250px;
  max-height: 400px;
  overflow-y: auto;
  z-index: 99;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.outline-header {
  font-weight: 600;
  font-size: 0.9rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--border-color);
  color: var(--text-color);
}

.outline-list {
  display: flex;
  flex-direction: column;
  padding: 0.5rem;
}

.outline-item {
  padding: 0.5rem 0.75rem;
  color: var(--md-link-color, #3b82f6);
  text-decoration: none;
  font-size: 0.85rem;
  border-radius: 3px;
  transition: all 0.2s ease;
  border-left: 2px solid transparent;
  cursor: pointer;
}

.outline-item:hover {
  background-color: var(--background-color);
  color: var(--md-link-hover, #2563eb);
}

.outline-level-1 {
  padding-left: 0.75rem;
}

.outline-level-2 {
  padding-left: 1.25rem;
}

.outline-level-3 {
  padding-left: 1.75rem;
}

.outline-level-4 {
  padding-left: 2.25rem;
}

.outline-level-5 {
  padding-left: 2.75rem;
}

.outline-level-6 {
  padding-left: 3.25rem;
}

/* Markdown Content */
.sr-markdown-content {
  color: var(--text-color);
  background-color: var(--background-color);
  line-height: 1.8;
  padding: 2rem;
  margin: 0;
  text-align: left;
  border: 1px solid var(--border-color);
  border-top: none;
  font-size: 15px;
  letter-spacing: 0.3px;
}

/* Import all markdown styles from MarkdownRenderer */
/* Headings */
.sr-markdown-content :deep(h1),
.sr-markdown-content :deep(h2),
.sr-markdown-content :deep(h3),
.sr-markdown-content :deep(h4),
.sr-markdown-content :deep(h5),
.sr-markdown-content :deep(h6) {
  color: var(--md-heading-color);
  line-height: 1.3;
  font-weight: 600;
  margin-top: 1.5em;
  margin-bottom: 0.5em;
}

.sr-markdown-content :deep(h1) {
  font-size: 2.15em;
  font-weight: 700;
}

.sr-markdown-content :deep(h2) {
  font-size: 1.75em;
}

.sr-markdown-content :deep(h3) {
  font-size: 1.4em;
}

.sr-markdown-content :deep(h4) {
  font-size: 1.2em;
}

.sr-markdown-content :deep(h5) {
  font-size: 1.05em;
}

.sr-markdown-content :deep(h6) {
  font-size: 1em;
}

/* Paragraphs */
.sr-markdown-content :deep(p) {
  margin: 0.8em 0;
  line-height: 1.8;
}

/* Links */
.sr-markdown-content :deep(a) {
  color: #3b82f6;
  text-decoration: none;
  transition: all 0.2s ease;
  font-weight: 500;
}

.sr-markdown-content :deep(a:hover) {
  color: #2563eb;
  text-decoration: underline;
}

/* Wikilinks - Pill Style */
.sr-markdown-content :deep(.md-wikilink-pill),
.sr-markdown-content :deep(.md-wikilink-pill-broken) {
  display: inline-flex;
  align-items: center;
  vertical-align: baseline;
  font-size: 1em;
  line-height: 1;
}

.sr-markdown-content :deep(.md-wikilink-pill-link) {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
  border-radius: 12px;
  overflow: hidden;
  font-size: 0.9em;
  transition: all 0.2s ease;
  border: 1px solid rgba(59, 130, 246, 0.3);
  vertical-align: baseline;
}

.sr-markdown-content :deep(.md-wikilink-pill-broken .md-wikilink-pill-link) {
  border-color: rgba(239, 68, 68, 0.3);
}

.sr-markdown-content :deep(.md-wikilink-label) {
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: rgba(59, 130, 246, 0.15);
  color: #3b82f6;
  padding: 0.25em 0.45em;
  font-weight: 700;
  font-size: 0.85em;
  border-right: 1px solid rgba(59, 130, 246, 0.3);
  white-space: nowrap;
  line-height: 1;
}

.sr-markdown-content :deep(.md-wikilink-pill-broken .md-wikilink-label) {
  background-color: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border-right-color: rgba(239, 68, 68, 0.3);
}

.sr-markdown-content :deep(.md-wikilink-content) {
  display: flex;
  align-items: center;
  background-color: rgba(59, 130, 246, 0.05);
  color: #3b82f6;
  padding: 0.25em 0.6em;
  font-weight: 500;
  white-space: nowrap;
  line-height: 1.2;
}

.sr-markdown-content :deep(.md-wikilink-pill-broken .md-wikilink-content) {
  background-color: rgba(239, 68, 68, 0.05);
  color: #ef4444;
}

.sr-markdown-content :deep(.md-wikilink-pill-link:hover) {
  border-color: rgba(59, 130, 246, 0.5);
  box-shadow: 0 1px 3px rgba(59, 130, 246, 0.2);
}

.sr-markdown-content :deep(.md-wikilink-pill-broken .md-wikilink-pill-link:hover) {
  border-color: rgba(239, 68, 68, 0.5);
  box-shadow: 0 1px 3px rgba(239, 68, 68, 0.2);
}

.sr-markdown-content :deep(.md-wikilink-pill-link:hover .md-wikilink-label) {
  background-color: rgba(59, 130, 246, 0.25);
}

.sr-markdown-content :deep(.md-wikilink-pill-broken .md-wikilink-pill-link:hover .md-wikilink-label) {
  background-color: rgba(239, 68, 68, 0.25);
}

.sr-markdown-content :deep(.md-wikilink-pill-link:hover .md-wikilink-content) {
  background-color: rgba(59, 130, 246, 0.1);
}

.sr-markdown-content :deep(.md-wikilink-pill-broken .md-wikilink-pill-link:hover .md-wikilink-content) {
  background-color: rgba(239, 68, 68, 0.1);
}

/* Code */
.sr-markdown-content :deep(code) {
  background-color: rgba(59, 130, 246, 0.15);
  color: #3b82f6;
  border-radius: 3px;
  padding: 0.2em 0.4em;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.9em;
  font-weight: 500;
}

.sr-markdown-content :deep(pre) {
  background-color: var(--md-pre-bg);
  border: 1px solid var(--md-pre-border);
  border-radius: 6px;
  padding: 1.2em;
  overflow-x: auto;
  margin: 1.2em 0;
}

.sr-markdown-content :deep(pre code) {
  background: none;
  color: var(--md-code-text);
  padding: 0;
}

/* Lists */
.sr-markdown-content :deep(ul),
.sr-markdown-content :deep(ol) {
  margin: 0.8em 0;
  padding-left: 2.2em;
}

.sr-markdown-content :deep(li) {
  margin-bottom: 0.45em;
  line-height: 1.75;
}

/* Tables */
.sr-markdown-content :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 1em 0;
}

.sr-markdown-content :deep(th),
.sr-markdown-content :deep(td) {
  border: 1px solid var(--md-table-border);
  padding: 0.75em 1em;
  text-align: left;
}

.sr-markdown-content :deep(th) {
  background-color: var(--md-table-header-bg);
  font-weight: bold;
}

/* Callouts / Admonitions (Obsidian-style) */
.sr-markdown-content :deep(.md-callout) {
  border-radius: 6px;
  margin: 1.2em 0;
  padding: 0;
  border: 1px solid;
  overflow: hidden;
  background-color: var(--background-color);
}

.sr-markdown-content :deep(.md-callout-header) {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 0.75em 1em;
  font-weight: 600;
  border-bottom: 1px solid;
}

.sr-markdown-content :deep(.md-callout-icon) {
  font-size: 1.2em;
  line-height: 1;
  display: flex;
  align-items: center;
}

.sr-markdown-content :deep(.md-callout-title) {
  font-size: 0.95em;
  line-height: 1.3;
}

.sr-markdown-content :deep(.md-callout-content) {
  padding: 1em;
  line-height: 1.6;
}

.sr-markdown-content :deep(.md-callout-content > :first-child) {
  margin-top: 0;
}

.sr-markdown-content :deep(.md-callout-content > :last-child) {
  margin-bottom: 0;
}

/* Callout Type Styles */
.sr-markdown-content :deep(.md-callout-note),
.sr-markdown-content :deep(.md-callout-abstract),
.sr-markdown-content :deep(.md-callout-summary),
.sr-markdown-content :deep(.md-callout-tldr) {
  border-color: rgba(59, 130, 246, 0.3);
  background-color: rgba(59, 130, 246, 0.05);
}

.sr-markdown-content :deep(.md-callout-note .md-callout-header),
.sr-markdown-content :deep(.md-callout-abstract .md-callout-header),
.sr-markdown-content :deep(.md-callout-summary .md-callout-header),
.sr-markdown-content :deep(.md-callout-tldr .md-callout-header) {
  background-color: rgba(59, 130, 246, 0.1);
  border-bottom-color: rgba(59, 130, 246, 0.2);
  color: #3b82f6;
}

.sr-markdown-content :deep(.md-callout-info) {
  border-color: rgba(14, 165, 233, 0.3);
  background-color: rgba(14, 165, 233, 0.05);
}

.sr-markdown-content :deep(.md-callout-info .md-callout-header) {
  background-color: rgba(14, 165, 233, 0.1);
  border-bottom-color: rgba(14, 165, 233, 0.2);
  color: #0ea5e9;
}

.sr-markdown-content :deep(.md-callout-tip),
.sr-markdown-content :deep(.md-callout-hint) {
  border-color: rgba(16, 185, 129, 0.3);
  background-color: rgba(16, 185, 129, 0.05);
}

.sr-markdown-content :deep(.md-callout-tip .md-callout-header),
.sr-markdown-content :deep(.md-callout-hint .md-callout-header) {
  background-color: rgba(16, 185, 129, 0.1);
  border-bottom-color: rgba(16, 185, 129, 0.2);
  color: #10b981;
}

.sr-markdown-content :deep(.md-callout-important) {
  border-color: rgba(168, 85, 247, 0.3);
  background-color: rgba(168, 85, 247, 0.05);
}

.sr-markdown-content :deep(.md-callout-important .md-callout-header) {
  background-color: rgba(168, 85, 247, 0.1);
  border-bottom-color: rgba(168, 85, 247, 0.2);
  color: #a855f7;
}

.sr-markdown-content :deep(.md-callout-warning),
.sr-markdown-content :deep(.md-callout-caution),
.sr-markdown-content :deep(.md-callout-attention) {
  border-color: rgba(251, 146, 60, 0.3);
  background-color: rgba(251, 146, 60, 0.05);
}

.sr-markdown-content :deep(.md-callout-warning .md-callout-header),
.sr-markdown-content :deep(.md-callout-caution .md-callout-header),
.sr-markdown-content :deep(.md-callout-attention .md-callout-header) {
  background-color: rgba(251, 146, 60, 0.1);
  border-bottom-color: rgba(251, 146, 60, 0.2);
  color: #fb923c;
}

.sr-markdown-content :deep(.md-callout-danger),
.sr-markdown-content :deep(.md-callout-error),
.sr-markdown-content :deep(.md-callout-failure),
.sr-markdown-content :deep(.md-callout-bug) {
  border-color: rgba(239, 68, 68, 0.3);
  background-color: rgba(239, 68, 68, 0.05);
}

.sr-markdown-content :deep(.md-callout-danger .md-callout-header),
.sr-markdown-content :deep(.md-callout-error .md-callout-header),
.sr-markdown-content :deep(.md-callout-failure .md-callout-header),
.sr-markdown-content :deep(.md-callout-bug .md-callout-header) {
  background-color: rgba(239, 68, 68, 0.1);
  border-bottom-color: rgba(239, 68, 68, 0.2);
  color: #ef4444;
}

.sr-markdown-content :deep(.md-callout-example) {
  border-color: rgba(139, 92, 246, 0.3);
  background-color: rgba(139, 92, 246, 0.05);
}

.sr-markdown-content :deep(.md-callout-example .md-callout-header) {
  background-color: rgba(139, 92, 246, 0.1);
  border-bottom-color: rgba(139, 92, 246, 0.2);
  color: #8b5cf6;
}

.sr-markdown-content :deep(.md-callout-quote) {
  border-color: rgba(148, 163, 184, 0.3);
  background-color: rgba(148, 163, 184, 0.05);
}

.sr-markdown-content :deep(.md-callout-quote .md-callout-header) {
  background-color: rgba(148, 163, 184, 0.1);
  border-bottom-color: rgba(148, 163, 184, 0.2);
  color: #94a3b8;
}

/* Blockquote (regular, non-callout) */
.sr-markdown-content :deep(.md-blockquote) {
  border-left: 3px solid var(--border-color);
  padding-left: 1em;
  margin: 1em 0;
  color: var(--text-color-secondary);
  font-style: italic;
}

/* Embeds */
.sr-markdown-content :deep(.md-embed-not-found) {
  background-color: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 6px;
  padding: 1em;
  margin: 1em 0;
  color: #ef4444;
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.sr-markdown-content :deep(.md-embed-image) {
  margin: 1.5em 0;
  text-align: center;
}

.sr-markdown-content :deep(.md-embed-image img) {
  max-width: 100%;
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

.sr-markdown-content :deep(.md-embed-caption) {
  font-size: 0.85em;
  color: var(--text-color-secondary);
  margin-top: 0.5em;
}

.sr-markdown-content :deep(.md-embed-note) {
  border: 1px solid var(--border-color);
  border-left: 3px solid #3b82f6;
  border-radius: 6px;
  margin: 1em 0;
  padding: 1em;
  background-color: var(--background-color-light);
}

.sr-markdown-content :deep(.md-embed-note-header) {
  font-weight: 600;
  margin-bottom: 0.5em;
  display: flex;
  align-items: center;
  gap: 0.5em;
  color: #3b82f6;
}

/* Metadata Panels */
.sr-metadata-panels {
  padding: 2rem;
  border-top: 1px solid var(--border-color);
  background-color: var(--background-color-light);
}

.sr-metadata-panel {
  margin-bottom: 2rem;
}

.sr-metadata-panel:last-child {
  margin-bottom: 0;
}

.metadata-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-color);
  margin-bottom: 1rem;
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.metadata-title i {
  color: #3b82f6;
}

/* Tags */
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.md-tag {
  color: #8b5cf6;
  font-weight: 500;
  background-color: rgba(139, 92, 246, 0.1);
  padding: 0.3em 0.6em;
  border-radius: 4px;
  font-size: 0.9em;
  cursor: pointer;
  transition: all 0.2s ease;
}

.md-tag:hover {
  background-color: rgba(139, 92, 246, 0.2);
}

/* Backlinks */
.backlink-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.backlink-item {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 0.5em 0.75em;
  background-color: var(--background-color);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  color: var(--text-color);
  text-decoration: none;
  transition: all 0.2s ease;
}

.backlink-item:hover {
  background-color: rgba(59, 130, 246, 0.1);
  border-color: #3b82f6;
}

.backlink-item i {
  color: #3b82f6;
  font-size: 0.9em;
}

/* Statistics */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 1rem;
}

.stat-item {
  background-color: var(--background-color);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 1em;
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.stat-label {
  font-size: 0.85em;
  color: var(--text-color-secondary);
  font-weight: 500;
}

.stat-value {
  font-size: 1.5em;
  font-weight: 700;
  color: #3b82f6;
}
</style>
