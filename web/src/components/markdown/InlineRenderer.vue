<template>
  <template v-for="(token, index) in safeTokens" :key="`token-${index}`">
    <!-- Text -->
    <template v-if="token.type === 'text'">{{ token.content }}</template>

    <!-- Bold -->
    <strong v-else-if="token.type === 'bold'" class="md-bold">
      <template v-if="Array.isArray(token.content)">
        <template v-for="(child, cIndex) in token.content" :key="`bold-${index}-${cIndex}`">
          <template v-if="child.type === 'text'">{{ child.content }}</template>
          <em v-else-if="child.type === 'italic'">{{ flattenContent(child.content) }}</em>
          <template v-else>{{ flattenContent(child.content) }}</template>
        </template>
      </template>
      <template v-else>{{ token.content }}</template>
    </strong>

    <!-- Italic -->
    <em v-else-if="token.type === 'italic'" class="md-italic">
      <template v-if="Array.isArray(token.content)">
        <template v-for="(child, cIndex) in token.content" :key="`italic-${index}-${cIndex}`">
          <template v-if="child.type === 'text'">{{ child.content }}</template>
          <strong v-else-if="child.type === 'bold'">{{ flattenContent(child.content) }}</strong>
          <template v-else>{{ flattenContent(child.content) }}</template>
        </template>
      </template>
      <template v-else>{{ token.content }}</template>
    </em>

    <!-- Strikethrough -->
    <del v-else-if="token.type === 'strikethrough'" class="md-strikethrough">
      {{ flattenContent(token.content) }}
    </del>

    <!-- Highlight -->
    <mark v-else-if="token.type === 'highlight'" class="md-highlight">
      {{ flattenContent(token.content) }}
    </mark>

    <!-- Code -->
    <code v-else-if="token.type === 'code'" class="md-code-inline">{{ token.content }}</code>

    <!-- Link -->
    <a
      v-else-if="token.type === 'link'"
      :href="token.url"
      class="md-link"
      target="_blank"
      rel="noopener noreferrer"
    >
      {{ token.text }}
    </a>

    <!-- Wikilink - check file_type and render accordingly -->
    <template v-else-if="token.type === 'wikilink'">
      <!-- Image wikilink: [[image.png]] -->
      <img
        v-if="token.file_type === 'image' && token.asset_url"
        :src="token.asset_url"
        :alt="token.display || token.target"
        :title="token.display || token.target"
        class="md-image md-wikilink-image"
        loading="lazy"
      />
      <!-- Regular wikilink: [[note]] -->
      <span
        v-else
        :class="['md-wikilink-pill', token.exists === false ? 'md-wikilink-pill-broken' : '']"
        :title="token.exists === false ? `File not found: ${token.display}` : `Open ${token.display}`"
      >
        <a
          href="#"
          class="md-wikilink-pill-link"
          :data-file-id="token.file_id || ''"
          @click.prevent="handleWikilinkClick(token)"
        >
          <span class="md-wikilink-label">B</span>
          <span class="md-wikilink-content">{{ token.display }}</span>
        </a>
      </span>
    </template>

    <!-- Tag -->
    <span v-else-if="token.type === 'tag'" class="md-tag">#{{ token.tag }}</span>

    <!-- Image -->
    <img
      v-else-if="token.type === 'image'"
      :src="token.url"
      :alt="token.alt || ''"
      :title="token.alt || ''"
      :width="token.width || null"
      class="md-image"
      loading="lazy"
    />

    <!-- Embed - check file_type and render accordingly -->
    <template v-else-if="token.type === 'embed'">
      <!-- Image embed: ![[image.png]] -->
      <img
        v-if="token.file_type === 'image' && token.asset_url"
        :src="token.asset_url"
        :alt="token.display || token.target"
        :title="token.display || token.target"
        class="md-image md-embed-image"
        loading="lazy"
      />
      <!-- Not found embed -->
      <div v-else-if="token.exists === false" class="md-embed md-embed-not-found">
        <i class="fas fa-exclamation-triangle"></i>
        <span>Embed not found: {{ token.target }}</span>
      </div>
      <!-- Other embeds (note, pdf, video, audio) -->
      <div v-else class="md-embed md-embed-placeholder">
        <i class="fas fa-file-alt"></i>
        <span>{{ token.target }}</span>
      </div>
    </template>
  </template>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps({
  tokens: {
    type: Array,
    required: true,
    default: () => []
  }
});

const emit = defineEmits(['wikilink-click']);

// Ensure tokens is always an array to prevent infinite loops
const safeTokens = computed(() => {
  if (!props.tokens) return [];
  if (!Array.isArray(props.tokens)) {
    console.warn('InlineRenderer received non-array tokens:', props.tokens);
    return [];
  }
  return props.tokens;
});

// Flatten nested content to plain text (for deeply nested formatting)
function flattenContent(content) {
  if (!content) return '';
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return String(content);

  return content.map(item => {
    if (item.type === 'text') return item.content;
    if (item.content) return flattenContent(item.content);
    return '';
  }).join('');
}

function handleWikilinkClick(token) {
  emit('wikilink-click', {
    fileId: token.file_id,
    path: token.path,
    target: token.target,
    display: token.display,
    exists: token.exists
  });
}
</script>

<style scoped>
/* Bold */
.md-bold {
  font-weight: 700;
}

/* Italic */
.md-italic {
  font-style: italic;
}

/* Strikethrough */
.md-strikethrough {
  text-decoration: line-through;
  opacity: 0.7;
}

/* Highlight */
.md-highlight {
  background-color: rgba(251, 191, 36, 0.3);
  padding: 0.1em 0.2em;
  border-radius: 2px;
}

/* Inline code */
.md-code-inline {
  background-color: rgba(59, 130, 246, 0.15);
  color: #3b82f6;
  border-radius: 3px;
  padding: 0.2em 0.4em;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.9em;
  font-weight: 500;
}

/* Links */
.md-link {
  color: #3b82f6;
  text-decoration: none;
  transition: all 0.2s ease;
  font-weight: 500;
}

.md-link:hover {
  color: #2563eb;
  text-decoration: underline;
}

/* Wikilinks - Pill Style */
.md-wikilink-pill,
.md-wikilink-pill-broken {
  display: inline-flex;
  align-items: center;
  vertical-align: baseline;
  font-size: 1em;
  line-height: 1;
}

.md-wikilink-pill-link {
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

.md-wikilink-pill-broken .md-wikilink-pill-link {
  border-color: rgba(239, 68, 68, 0.3);
}

.md-wikilink-label {
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

.md-wikilink-pill-broken .md-wikilink-label {
  background-color: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border-right-color: rgba(239, 68, 68, 0.3);
}

.md-wikilink-content {
  display: flex;
  align-items: center;
  background-color: rgba(59, 130, 246, 0.05);
  color: #3b82f6;
  padding: 0.25em 0.6em;
  font-weight: 500;
  white-space: nowrap;
  line-height: 1.2;
}

.md-wikilink-pill-broken .md-wikilink-content {
  background-color: rgba(239, 68, 68, 0.05);
  color: #ef4444;
}

.md-wikilink-pill-link:hover {
  border-color: rgba(59, 130, 246, 0.5);
  box-shadow: 0 1px 3px rgba(59, 130, 246, 0.2);
}

.md-wikilink-pill-broken .md-wikilink-pill-link:hover {
  border-color: rgba(239, 68, 68, 0.5);
  box-shadow: 0 1px 3px rgba(239, 68, 68, 0.2);
}

.md-wikilink-pill-link:hover .md-wikilink-label {
  background-color: rgba(59, 130, 246, 0.25);
}

.md-wikilink-pill-broken .md-wikilink-pill-link:hover .md-wikilink-label {
  background-color: rgba(239, 68, 68, 0.25);
}

.md-wikilink-pill-link:hover .md-wikilink-content {
  background-color: rgba(59, 130, 246, 0.1);
}

.md-wikilink-pill-broken .md-wikilink-pill-link:hover .md-wikilink-content {
  background-color: rgba(239, 68, 68, 0.1);
}

/* Tags */
.md-tag {
  color: #8b5cf6;
  font-weight: 500;
  background-color: rgba(139, 92, 246, 0.1);
  padding: 0.2em 0.5em;
  border-radius: 4px;
  font-size: 0.9em;
  cursor: pointer;
  transition: all 0.2s ease;
}

.md-tag:hover {
  background-color: rgba(139, 92, 246, 0.2);
}

/* Images */
.md-image {
  max-width: 100%;
  height: auto;
  display: inline-block;
  margin: 0.5em 0;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  transition: all 0.2s ease;
}

.md-image:hover {
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.15);
  transform: scale(1.01);
}

/* Embeds */
.md-embed {
  display: block;
  margin: 1em 0;
}

.md-embed-not-found {
  background-color: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 6px;
  padding: 1em;
  color: #ef4444;
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.md-embed-placeholder {
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  padding: 1em;
  display: flex;
  align-items: center;
  gap: 0.5em;
  background-color: var(--background-color-light, #f8fafc);
}
</style>
