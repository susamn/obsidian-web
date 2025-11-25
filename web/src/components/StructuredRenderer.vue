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
      <div class="sr-markdown-content" ref="markdownContentRef">
        <CustomMarkdownRenderer
          :nodes="parsedNodes"
          @wikilink-click="handleWikilinkClick"
        />
      </div>

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
import { parseMarkdown } from '../utils/customMarkdownParser';
import CustomMarkdownRenderer from './CustomMarkdownRenderer.vue';
import {
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

const emit = defineEmits(['update:markdownResult', 'wikilink-click']);

const loading = ref(false);
const error = ref(null);
const structuredData = ref(null);
const markdownContentRef = ref(null);
const showOutline = ref(false);
const showMetadata = ref(true);

// Computed properties
const parsedNodes = computed(() => {
  if (!structuredData.value) return [];

  try {
    const nodes = parseMarkdown(
      structuredData.value.raw_markdown,
      structuredData.value.wikilinks,
      structuredData.value.embeds,
      {
        vaultId: props.vaultId,
        images: structuredData.value.embeds || [] // Pass embeds as images metadata
      }
    );
    console.log('[StructuredRenderer] Parsed nodes:', nodes);
    return nodes;
  } catch (err) {
    console.error('[StructuredRenderer] Parse error:', err);
    return [];
  }
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

  console.log('[StructuredRenderer] Fetching file:', props.vaultId, props.fileId);
  loading.value = true;
  error.value = null;

  try {
    const url = `/api/v1/files/sr/by-id/${props.vaultId}/${props.fileId}`;
    console.log('[StructuredRenderer] Fetching URL:', url);

    const response = await fetch(url);
    console.log('[StructuredRenderer] Response status:', response.status);

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to load file');
    }

    const data = await response.json();
    console.log('[StructuredRenderer] Received data:', data);

    // Backend returns {data: {...}} structure
    if (data.data) {
      structuredData.value = data.data;

      // Emit markdown result for parent component
      emit('update:markdownResult', {
        nodes: parsedNodes.value,
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

// Handle wikilink clicks
const handleWikilinkClick = (event) => {
  emit('wikilink-click', event);
};

// Watch for file ID changes
watch(() => props.fileId, (newFileId) => {
  if (newFileId) {
    fetchStructuredData();
  }
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

/* Outline Panel */
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

.outline-list {
  display: flex;
  flex-direction: column;
  padding: clamp(0.375rem, 1vw, 0.5rem);
  gap: clamp(0.125rem, 0.4vw, 0.15rem);
}

.outline-item {
  padding: clamp(0.25rem, 0.8vw, 0.35rem) clamp(0.5rem, 1.2vw, 0.625rem);
  color: var(--md-link-color, #3b82f6);
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
  color: var(--md-link-hover, #2563eb);
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

/* Markdown Content Container */
.sr-markdown-content {
  color: var(--text-color);
  background-color: var(--background-color);
  padding: clamp(1rem, 3vw, 1.5rem);
  margin: 0;
  text-align: left;
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-top: none;
  border-radius: 0 0 8px 8px;
  font-size: clamp(0.875rem, 2vw, 0.9375rem);
  letter-spacing: 0.01em;
  line-height: 1.65;
}

/* Metadata Panels */
.sr-metadata-panels {
  padding: clamp(0.625rem, 2vw, 0.875rem) clamp(1rem, 3vw, 1.5rem);
  border-top: 1px solid rgba(128, 128, 128, 0.15);
  background-color: var(--background-color-light);
}

.sr-metadata-panel {
  margin-bottom: clamp(0.5rem, 1.5vw, 0.75rem);
}

.sr-metadata-panel:last-child {
  margin-bottom: 0;
}

.metadata-title {
  font-size: clamp(0.8rem, 1.8vw, 0.875rem);
  font-weight: 600;
  color: var(--text-color);
  margin-bottom: clamp(0.3rem, 1vw, 0.4rem);
  display: flex;
  align-items: center;
  gap: 0.35em;
}

.metadata-title i {
  color: #3b82f6;
}

/* Tags */
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: clamp(0.25rem, 0.8vw, 0.35rem);
}

.md-tag {
  color: #8b5cf6;
  font-weight: 500;
  background-color: rgba(139, 92, 246, 0.1);
  padding: clamp(0.2em, 0.6vw, 0.25em) clamp(0.4em, 1vw, 0.5em);
  border-radius: 4px;
  font-size: clamp(0.8em, 1.6vw, 0.85em);
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
  gap: clamp(0.25rem, 0.8vw, 0.35rem);
}

.backlink-item {
  display: flex;
  align-items: center;
  gap: clamp(0.35em, 0.8vw, 0.4em);
  padding: clamp(0.3em, 0.8vw, 0.375em) clamp(0.5em, 1.2vw, 0.625em);
  background-color: var(--background-color);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 5px;
  color: var(--text-color);
  text-decoration: none;
  transition: all 0.2s ease;
  font-size: clamp(0.825em, 1.6vw, 0.875em);
}

.backlink-item:hover {
  background-color: rgba(59, 130, 246, 0.1);
  border-color: #3b82f6;
}

.backlink-item i {
  color: #3b82f6;
  font-size: clamp(0.8em, 1.6vw, 0.85em);
}

/* Statistics */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(clamp(100px, 20vw, 120px), 1fr));
  gap: clamp(0.4rem, 1.2vw, 0.5rem);
}

.stat-item {
  background-color: var(--background-color);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 6px;
  padding: clamp(0.5em, 1.2vw, 0.625em);
  display: flex;
  flex-direction: column;
  gap: clamp(0.25em, 0.8vw, 0.3em);
}

.stat-label {
  font-size: clamp(0.75em, 1.6vw, 0.8em);
  color: var(--text-color-secondary);
  font-weight: 500;
}

.stat-value {
  font-size: clamp(1.1em, 2.2vw, 1.25em);
  font-weight: 700;
  color: #3b82f6;
}
</style>
