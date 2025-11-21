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

const emit = defineEmits(['update:markdownResult']);

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
  console.log('Wikilink clicked:', event);
  // You can emit this event or navigate to the file
  // For now, just log it
  if (event.fileId && event.exists) {
    // Navigate to the linked file
    // This would typically trigger a navigation event
  }
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

/* Markdown Content Container */
.sr-markdown-content {
  color: var(--text-color);
  background-color: var(--background-color);
  padding: 2rem;
  margin: 0;
  text-align: left;
  border: 1px solid var(--border-color);
  border-top: none;
  font-size: 15px;
  letter-spacing: 0.3px;
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
