<template>
  <ul class="file-tree-list">
    <li v-for="node in nodes" :key="node.metadata.id" class="tree-node">
      <div class="node-header" @click="node.metadata.is_directory ? toggleExpand(node) : selectFile(node)">
        <!-- Expand/Collapse indicator -->
        <span v-if="node.metadata.is_directory" class="expand-icon">
          <span v-if="expandedNodes[node.metadata.id]">▼</span>
          <span v-else>▶</span>
        </span>
        <span v-else class="expand-icon-placeholder"></span>

        <!-- File/Folder icon -->
        <span v-if="node.metadata.is_directory" class="icon folder-icon">
          <i :class="['fas', expandedNodes[node.metadata.id] ? 'fa-folder-open' : 'fa-folder']"></i>
        </span>
        <span v-else class="icon file-icon">
          <i :class="getFileIcon(node.metadata)"></i>
        </span>

        <!-- Node name -->
        <span class="node-name">{{ node.metadata.name }}</span>
      </div>

      <!-- Children (recursively) -->
      <div v-if="node.metadata.is_directory && expandedNodes[node.metadata.id] && node.children" class="children">
        <FileTree
          :nodes="node.children"
          :vault-id="vaultId"
          :expanded-nodes="expandedNodes"
          @toggle-expand="toggleExpand"
          @file-selected="selectFile"
        />
      </div>
    </li>
  </ul>
</template>

<script setup>
import { ref, watch } from 'vue';
import { useFileStore } from '../stores/fileStore';

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
});

const emit = defineEmits(['toggle-expand']);

const fileStore = useFileStore();

const toggleExpand = async (node) => {
  emit('toggle-expand', node);
};

const selectFile = (node) => {
  emit('file-selected', node);
};

/**
 * Get appropriate icon class based on file type
 */
const getFileIcon = (metadata) => {
  if (metadata.is_markdown) {
    return 'fas fa-file-alt'; // Markdown file icon
  }

  const extension = metadata.name.split('.').pop().toLowerCase();

  // Icon mapping based on file extension
  const iconMap = {
    // Images
    'png': 'fas fa-file-image',
    'jpg': 'fas fa-file-image',
    'jpeg': 'fas fa-file-image',
    'gif': 'fas fa-file-image',
    'svg': 'fas fa-file-image',
    'webp': 'fas fa-file-image',

    // Documents
    'pdf': 'fas fa-file-pdf',
    'doc': 'fas fa-file-word',
    'docx': 'fas fa-file-word',

    // Code files
    'js': 'fas fa-file-code',
    'ts': 'fas fa-file-code',
    'jsx': 'fas fa-file-code',
    'tsx': 'fas fa-file-code',
    'vue': 'fas fa-file-code',
    'html': 'fas fa-file-code',
    'css': 'fas fa-file-code',
    'json': 'fas fa-file-code',
    'yaml': 'fas fa-file-code',
    'yml': 'fas fa-file-code',
    'xml': 'fas fa-file-code',

    // Archives
    'zip': 'fas fa-file-archive',
    'tar': 'fas fa-file-archive',
    'gz': 'fas fa-file-archive',
    'rar': 'fas fa-file-archive',

    // Text files
    'txt': 'fas fa-file-alt',
    'log': 'fas fa-file-alt',

    // Video
    'mp4': 'fas fa-file-video',
    'avi': 'fas fa-file-video',
    'mov': 'fas fa-file-video',
    'mkv': 'fas fa-file-video',

    // Audio
    'mp3': 'fas fa-file-audio',
    'wav': 'fas fa-file-audio',
    'flac': 'fas fa-file-audio',
    'ogg': 'fas fa-file-audio',
  };

  return iconMap[extension] || 'fas fa-file'; // Default file icon
};
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

/* Expand/Collapse icon */
.expand-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  margin-right: 4px;
  font-size: 0.7rem;
  color: var(--icon-color, #666);
  flex-shrink: 0;
}

.expand-icon-placeholder {
  display: inline-block;
  width: 16px;
  margin-right: 4px;
  flex-shrink: 0;
}

/* File/Folder icons */
.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  margin-right: 6px;
  flex-shrink: 0;
}

.folder-icon {
  color: #f0c674; /* Yellow/gold for folders */
}

.file-icon {
  color: var(--text-color, #333);
}

/* Specific icon colors */
.fa-file-image {
  color: #c678dd; /* Purple for images */
}

.fa-file-pdf {
  color: #e06c75; /* Red for PDFs */
}

.fa-file-word {
  color: #2b5797; /* Blue for Word docs */
}

.fa-file-code {
  color: #61afef; /* Light blue for code */
}

.fa-file-archive {
  color: #e5c07b; /* Orange for archives */
}

.fa-file-video {
  color: #c678dd; /* Purple for videos */
}

.fa-file-audio {
  color: #98c379; /* Green for audio */
}

.fa-file-alt {
  color: #56b6c2; /* Cyan for markdown/text */
}

/* Node name */
.node-name {
  color: var(--text-color, #333);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
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

/* Dark mode support */
@media (prefers-color-scheme: dark) {
  .node-header:hover {
    background-color: rgba(255, 255, 255, 0.1);
  }

  .node-name {
    color: #ddd;
  }

  .expand-icon {
    color: #999;
  }

  .file-icon {
    color: #ddd;
  }

  .children {
    border-left-color: #444;
  }
}
</style>
