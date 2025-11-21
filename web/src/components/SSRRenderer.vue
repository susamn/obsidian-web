<template>
  <div class="ssr-renderer-wrapper">
    <!-- Loading State -->
    <div v-if="loading" class="ssr-loading">
      <i class="fas fa-spinner fa-spin"></i>
      <p>Rendering on server...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="ssr-error">
      <i class="fas fa-exclamation-circle"></i>
      <p>Error rendering file</p>
      <p class="error-details">{{ error }}</p>
    </div>

    <!-- Rendered Content -->
    <div v-else-if="renderedHTML" class="ssr-content" v-html="renderedHTML"></div>

    <!-- Empty State -->
    <div v-else class="ssr-placeholder">
      <i class="fas fa-server"></i>
      <p>Server-side rendering ready</p>
      <p class="ssr-info">File will be rendered by the server</p>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, computed } from 'vue';

const props = defineProps({
  content: {
    type: String,
    default: '',
  },
  vaultId: {
    type: String,
    default: '',
  },
  fileId: {
    type: String,
    default: '',
  },
});

const emit = defineEmits(['update:markdownResult']);

const loading = ref(false);
const error = ref(null);
const renderedHTML = ref('');

// Markdown result to emit
const markdownResult = ref({
  html: '',
  tags: [],
  frontmatter: {},
  headings: [],
  wikilinks: [],
  stats: { words: 0, chars: 0, readingTime: 0 }
});

// Render file using server-side rendering
const renderFile = async () => {
  // Only render if we have both vaultId and fileId
  if (!props.vaultId || !props.fileId) {
    error.value = null;
    renderedHTML.value = '';
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    const response = await fetch(
      `/api/v1/files/ssr/by-id/${props.vaultId}/${props.fileId}`
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to render file');
    }

    const data = await response.json();

    // Update rendered HTML
    renderedHTML.value = data.data.html;

    // Update markdown result with rendered HTML
    markdownResult.value = {
      html: data.data.html,
      tags: [],
      frontmatter: {},
      headings: [],
      wikilinks: [],
      stats: { words: 0, chars: 0, readingTime: 0 }
    };

    // Emit the markdown result
    emit('update:markdownResult', markdownResult.value);
  } catch (err) {
    console.error('[SSRRenderer] Error rendering file:', err);
    error.value = err.message || 'Failed to render file on server';
    renderedHTML.value = '';
  } finally {
    loading.value = false;
  }
};

// Watch for changes in fileId to trigger re-rendering
watch(() => props.fileId, () => {
  renderFile();
});

// Watch for changes in vaultId to reset
watch(() => props.vaultId, () => {
  renderedHTML.value = '';
  error.value = null;
});
</script>

<style scoped>
.ssr-renderer-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 0;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.ssr-loading,
.ssr-error,
.ssr-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 3rem;
  color: var(--text-color-secondary);
}

.ssr-loading i,
.ssr-error i,
.ssr-placeholder i {
  font-size: 3rem;
  margin-bottom: 1rem;
  opacity: 0.5;
}

.ssr-loading i {
  color: #3b82f6;
}

.ssr-error i {
  color: #ef4444;
}

.ssr-loading p,
.ssr-error p,
.ssr-placeholder p {
  margin: 0.5rem 0;
}

.ssr-info {
  font-size: 0.9rem;
  font-style: italic;
  margin-top: 1rem;
}

.error-details {
  font-size: 0.85rem;
  color: #ef4444;
  margin-top: 0.5rem;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
}

.ssr-content {
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
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
}

/* Markdown content styling for SSR */
.ssr-content h1,
.ssr-content h2,
.ssr-content h3,
.ssr-content h4,
.ssr-content h5,
.ssr-content h6 {
  color: var(--md-heading-color);
  line-height: 1.3;
  margin-top: 1.2em;
  margin-bottom: 0.5em;
}

.ssr-content h1 {
  font-size: 2.15em;
  font-weight: 700;
  margin-top: 1.8em;
  margin-bottom: 0.6em;
}

.ssr-content h2 {
  font-size: 1.75em;
  font-weight: 700;
  margin-top: 1.6em;
  margin-bottom: 0.5em;
}

.ssr-content h3 {
  font-size: 1.4em;
  font-weight: 600;
  margin-top: 1.4em;
  margin-bottom: 0.4em;
}

.ssr-content h4 {
  font-size: 1.2em;
  font-weight: 600;
  margin-top: 1.2em;
  margin-bottom: 0.3em;
}

.ssr-content h5 {
  font-size: 1.05em;
  font-weight: 600;
  margin-top: 1em;
  margin-bottom: 0.3em;
}

.ssr-content h6 {
  font-size: 1em;
  font-weight: 600;
  margin-top: 1em;
  margin-bottom: 0.3em;
}

.ssr-content p {
  margin: 0.8em 0;
  line-height: 1.8;
}

.ssr-content a {
  color: #3b82f6;
  text-decoration: none;
  transition: all 0.2s ease;
  font-weight: 500;
}

.ssr-content a:hover {
  color: #2563eb;
  text-decoration: underline;
}

.ssr-content code {
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

.ssr-content pre {
  background-color: var(--md-pre-bg);
  border: 1px solid var(--md-pre-border);
  border-radius: 6px;
  padding: 1.2em;
  overflow-x: auto;
  margin: 1.2em 0;
}

.ssr-content pre code {
  background-color: transparent;
  color: var(--md-code-text);
  padding: 0;
  font-size: 1em;
  border: none;
  white-space: pre;
}

.ssr-content ul,
.ssr-content ol {
  margin: 0.8em 0;
  padding-left: 2.2em;
}

.ssr-content li {
  margin-bottom: 0.45em;
  line-height: 1.75;
}

.ssr-content table {
  width: 100%;
  border-collapse: collapse;
  margin: 1em 0;
}

.ssr-content th,
.ssr-content td {
  border: 1px solid var(--md-table-border);
  padding: 0.75em 1em;
  text-align: left;
}

.ssr-content th {
  background-color: var(--md-table-header-bg);
  font-weight: bold;
  color: var(--text-color);
}

.ssr-content td {
  background-color: var(--background-color);
}

.ssr-content tbody tr:hover {
  background-color: var(--background-color-light);
}

.ssr-content img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 1em auto;
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

.ssr-content hr {
  border: none;
  border-top: 2px solid var(--md-hr-color);
  margin: 2em 0;
}

.ssr-content blockquote {
  border-left: 4px solid var(--md-blockquote-border);
  padding-left: 1em;
  margin: 0.75em 0;
  color: var(--md-blockquote-text);
}

.ssr-content strong {
  font-weight: 600;
  color: var(--text-color);
}

.ssr-content em {
  font-style: italic;
  color: var(--text-color);
}

.ssr-content > :first-child {
  margin-top: 0;
}

.ssr-content > :last-child {
  margin-bottom: 0;
}
</style>
