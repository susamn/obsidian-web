<template>
  <ul class="file-tree-list">
    <li
      v-for="node in nodes"
      :key="node.metadata.id"
      :data-node-id="node.metadata.id"
      class="tree-node"
    >
      <div
        class="node-header"
        :class="{
          'selected-file': !node.metadata.is_directory && selectedFileId === node.metadata.id,
        }"
        @click="node.metadata.is_directory ? toggleExpand(node) : selectFile(node)"
      >
        <!-- Expand/Collapse indicator -->
        <span
          v-if="node.metadata.is_directory"
          class="expand-icon"
        >
          <span v-if="expandedNodes[node.metadata.id]">▼</span>
          <span v-else>▶</span>
        </span>
        <span
          v-else
          class="expand-icon-placeholder"
        />

        <!-- File/Folder icon -->
        <span
          v-if="node.metadata.is_directory"
          class="icon folder-icon"
        >
          <i :class="['fas', expandedNodes[node.metadata.id] ? 'fa-folder-open' : 'fa-folder']" />
        </span>
        <span
          v-else
          class="icon file-icon"
        >
          <i :class="getFileIcon(node.metadata)" />
        </span>

        <!-- Node name -->
        <span class="node-name">{{ node.metadata.name }}</span>

        <!-- Create button (only for directories) -->
        <button
          v-if="node.metadata.is_directory"
          class="create-button"
          title="Create new file or folder"
          @click.stop="handleCreateClick(node)"
        >
          <i class="fas fa-plus" />
        </button>
      </div>

      <!-- Children (recursively) -->
      <div
        v-if="node.metadata.is_directory && expandedNodes[node.metadata.id] && node.children"
        class="children"
      >
        <FileTree
          :nodes="node.children"
          :vault-id="vaultId"
          :expanded-nodes="expandedNodes"
          :selected-file-id="selectedFileId"
          @toggle-expand="toggleExpand"
          @file-selected="selectFile"
          @create-clicked="handleCreateClick"
        />
      </div>
    </li>
  </ul>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useFileStore } from '../stores/fileStore'

const props = defineProps({
  nodes: {
    type: Array,
    default: () => [],
  },
  vaultId: {
    type: String,
    required: true,
  },
  expandedNodes: {
    type: Object,
    default: () => ({}),
  },
  selectedFileId: {
    type: String,
    default: null,
  },
})

const emit = defineEmits(['toggle-expand', 'file-selected', 'create-clicked'])

const fileStore = useFileStore()

const toggleExpand = async (node) => {
  emit('toggle-expand', node)
}

const selectFile = (node) => {
  emit('file-selected', node)
}

const handleCreateClick = (node) => {
  emit('create-clicked', node)
}

/**
 * Get appropriate icon class based on file type
 */
const getFileIcon = (metadata) => {
  if (metadata.is_markdown) {
    return 'fas fa-file-alt' // Markdown file icon
  }

  const extension = metadata.name.split('.').pop().toLowerCase()

  // Icon mapping based on file extension
  const iconMap = {
    // Obsidian Canvas
    canvas: 'fas fa-project-diagram',

    // Images
    png: 'fas fa-file-image',
    jpg: 'fas fa-file-image',
    jpeg: 'fas fa-file-image',
    gif: 'fas fa-file-image',
    svg: 'fas fa-file-image',
    webp: 'fas fa-file-image',

    // Documents
    pdf: 'fas fa-file-pdf',
    doc: 'fas fa-file-word',
    docx: 'fas fa-file-word',

    // Code files
    js: 'fas fa-file-code',
    ts: 'fas fa-file-code',
    jsx: 'fas fa-file-code',
    tsx: 'fas fa-file-code',
    vue: 'fas fa-file-code',
    html: 'fas fa-file-code',
    css: 'fas fa-file-code',
    json: 'fas fa-file-code',
    yaml: 'fas fa-file-code',
    yml: 'fas fa-file-code',
    xml: 'fas fa-file-code',

    // Archives
    zip: 'fas fa-file-archive',
    tar: 'fas fa-file-archive',
    gz: 'fas fa-file-archive',
    rar: 'fas fa-file-archive',

    // Text files
    txt: 'fas fa-file-alt',
    log: 'fas fa-file-alt',

    // Video
    mp4: 'fas fa-file-video',
    avi: 'fas fa-file-video',
    mov: 'fas fa-file-video',
    mkv: 'fas fa-file-video',

    // Audio
    mp3: 'fas fa-file-audio',
    wav: 'fas fa-file-audio',
    flac: 'fas fa-file-audio',
    ogg: 'fas fa-file-audio',
  }

  return iconMap[extension] || 'fas fa-file' // Default file icon
}
</script>

<style scoped>
.file-tree-list {
  list-style-type: none;
  padding: 0;
  margin: 0;
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  font-size: 0.9rem;
}

.tree-node {
  margin: 0;
  padding: 0;
}

.node-header {
  display: flex;
  align-items: center;
  cursor: pointer;
  padding: 0.3rem 0.5rem;
  border-radius: 4px;
  transition: background-color 0.15s ease;
  user-select: none;
}

.node-header:hover {
  background-color: var(--hover-color, rgba(0, 0, 0, 0.05));
}

/* Selected file highlighting - theme-aware translucent background */
.node-header.selected-file {
  background-color: color-mix(in srgb, var(--primary-color), transparent 85%);
  border-left: 3px solid var(--primary-color);
  padding-left: calc(0.5rem - 3px); /* Adjust padding to account for border */
}

.node-header.selected-file:hover {
  background-color: color-mix(in srgb, var(--primary-color), transparent 75%);
}

.node-header.selected-file .node-name {
  font-weight: 500;
  color: var(--primary-color);
}

/* Expand/Collapse icon */
.expand-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  margin-right: 2px;
  font-size: 0.65rem;
  color: var(--icon-color, #666);
  flex-shrink: 0;
}

.expand-icon-placeholder {
  display: inline-block;
  width: 14px;
  margin-right: 2px;
  flex-shrink: 0;
}

/* File/Folder icons */
.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  margin-right: 4px;
  flex-shrink: 0;
}

.folder-icon {
  color: var(--primary-color);
  opacity: 0.8;
}

.file-icon {
  color: var(--md-link-color);
  opacity: 0.9;
}

/* Specific icon colors mapped to theme variables */
.fa-file-image,
.fa-file-video,
.fa-file-audio {
  color: var(--md-code-text); /* Often Red/Orange/Pink */
}

.fa-file-code,
.fa-project-diagram {
  color: var(--md-heading-color); /* Often Yellow/Purple */
}

.fa-file-pdf,
.fa-file-word,
.fa-file-archive {
  color: var(--md-code-text);
}

/* Node name */
.node-name {
  color: var(--text-color, #333);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  text-align: left;
  padding-left: 0;
}

/* Create button */
.create-button {
  display: none;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  margin-left: 4px;
  padding: 0;
  background: none;
  border: none;
  border-radius: 3px;
  color: var(--text-color-secondary, #666);
  cursor: pointer;
  font-size: 0.75rem;
  transition: all 0.2s;
  flex-shrink: 0;
}

.node-header:hover .create-button {
  display: inline-flex;
}

.create-button:hover {
  background-color: var(--primary-color, #3b82f6);
  color: white;
}

/* Directory styling */
.tree-node:has(.folder-icon) .node-name {
  font-weight: 500;
}

/* Children container with tree lines */
.children {
  margin-left: 20px;
  padding-left: 8px;
  border-left: 1px solid var(--border-color, #e0e0e0);
}

/* Root level (no left padding) */
.file-tree-list > .tree-node > .node-header {
  padding-left: 0.25rem;
}

/* Dark mode support removed - relying on CSS variables from themes */
</style>
