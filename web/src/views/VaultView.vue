<template>
  <div class="vault-view">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2 class="vault-name">{{ vaultName }}</h2>
        <div class="connection-status">
          <span
            class="status-indicator"
            :class="{
              'connected': connected,
              'disconnected': !connected && !error,
              'error': error
            }"
            :title="error || (connected ? 'Live updates enabled' : 'Connecting...')"
          >
            <i v-if="connected" class="fas fa-circle"></i>
            <i v-else-if="error" class="fas fa-exclamation-circle"></i>
            <i v-else class="fas fa-circle-notch fa-spin"></i>
          </span>
          <span class="status-text">
            {{ connected ? 'Live' : (error ? 'Offline' : 'Connecting') }}
          </span>
        </div>
      </div>
      <div class="file-tree">
        <p v-if="fileStore.loading">Loading file tree...</p>
        <p v-else-if="fileStore.error" class="text-red-500">Error: {{ fileStore.error }}</p>
        <FileTree
          v-else
          :nodes="fileStore.treeData"
          :vault-id="fileStore.vaultId"
          :expanded-nodes="expandedNodes"
          @toggle-expand="handleToggleExpand"
          @file-selected="handleFileSelected"
        />
      </div>
    </aside>
    <main class="main-content">
      <div v-if="fileStore.loading" class="loading-spinner">Loading file content...</div>
      <div v-else-if="fileStore.error" class="error-message text-red-500">Error: {{ fileStore.error }}</div>
      <div v-else-if="fileStore.selectedFileContent" class="markdown-content" v-html="renderedMarkdown"></div>
      <div v-else class="no-content-message">Select a file to view its content.</div>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed } from 'vue';
import { useRoute } from 'vue-router';
import { useFileStore } from '../stores/fileStore';
import { useSSE } from '../composables/useSSE';
import FileTree from '../components/FileTree.vue';
import MarkdownIt from 'markdown-it';

const md = new MarkdownIt();

const route = useRoute();
const fileStore = useFileStore();
const vaultName = ref('');
const expandedNodes = ref({});

const handleToggleExpand = async (node) => {
  if (node.metadata.is_directory) {
    if (expandedNodes.value[node.metadata.path]) {
      // Collapse
      delete expandedNodes.value[node.metadata.path];
    } else {
      // Expand
      expandedNodes.value[node.metadata.path] = true;
      // Fetch children if not already fetched
      if (!node.children || node.children.length === 0) {
        await fileStore.fetchChildren(fileStore.vaultId, node.metadata.path);
        updateNodeChildren(fileStore.treeData, node.metadata.path, fileStore.childrenData);
      }
    }
  }
};

const handleFileSelected = async (node) => {
  if (!node.metadata.is_directory) {
    await fileStore.fetchFileContent(fileStore.vaultId, node.metadata.path);
    fileStore.setCurrentPath(node.metadata.path); // Set current path for SSE updates
  }
};

const renderedMarkdown = computed(() => {
  return fileStore.selectedFileContent ? md.render(fileStore.selectedFileContent) : '';
});

const updateNodeChildren = (nodes, targetPath, newChildren) => {
  for (let i = 0; i < nodes.length; i++) {
    if (nodes[i].metadata.path === targetPath) {
      nodes[i].children = newChildren;
      return true;
    }
    if (nodes[i].metadata.is_directory && nodes[i].children) {
      if (updateNodeChildren(nodes[i].children, targetPath, newChildren)) {
        return true;
      }
    }
  }
  return false;
};

/**
 * Refresh a specific node or the entire tree
 */
const refreshNode = async (path = '') => {
  console.log('[VaultView] Refreshing node:', path);

  if (path === '') {
    // Refresh entire tree
    await fileStore.fetchTree(fileStore.vaultId);
  } else {
    // Find the parent directory and refresh it
    const parentPath = path.substring(0, path.lastIndexOf('/'));

    // Check if parent is expanded
    if (expandedNodes.value[parentPath] !== undefined) {
      // Refresh parent's children
      await fileStore.fetchChildren(fileStore.vaultId, parentPath);
      updateNodeChildren(fileStore.treeData, parentPath, fileStore.childrenData);
    } else {
      // If parent is not expanded, just invalidate cache by refreshing the tree
      await fileStore.fetchTree(fileStore.vaultId);
    }
  }
};

// SSE event handlers
const sseCallbacks = {
  onConnected: (data) => {
    console.log('[VaultView] SSE connected:', data);
    connected.value = true;
    error.value = null;
  },

  onFileCreated: async (event) => {
    console.log('[VaultView] File created:', event.path);
    await refreshNode(event.path);
  },

  onFileModified: async (event) => {
    console.log('[VaultView] File modified:', event.path);
    await refreshNode(event.path);
    // If the modified file is currently selected, re-fetch its content
    if (fileStore.currentPath === event.path) {
      await fileStore.fetchFileContent(fileStore.vaultId, event.path);
    }
  },

  onFileDeleted: async (event) => {
    console.log('[VaultView] File deleted:', event.path);
    await refreshNode(event.path);
    // If the deleted file was currently selected, clear its content
    if (fileStore.currentPath === event.path) {
      fileStore.selectedFileContent = null;
    }
  },

  onTreeRefresh: async (event) => {
    console.log('[VaultView] Tree refresh requested:', event.path);
    await refreshNode(event.path);
  },

  onError: (err) => {
    console.error('[VaultView] SSE error:', err);
    error.value = err.message || 'SSE connection error';
    connected.value = false;
  },
};

// Initialize SSE connection (vaultId will be passed when calling connect())
const { connected, error, connect, disconnect, reconnect } = useSSE(sseCallbacks);

// Watch for changes in the route params, specifically the 'id' for the vault
watch(() => route.params.id, (newId, oldId) => {
  if (newId) {
    // Disconnect old SSE connection if vault changes
    if (oldId && oldId !== newId) {
      disconnect();
    }

    fileStore.setVaultId(newId);
    vaultName.value = `Vault ${newId}`;
    fileStore.fetchTree(newId);
    expandedNodes.value = {}; // Reset expanded nodes when vault changes
    fileStore.selectedFileContent = null; // Clear selected file content

    // Connect to SSE for the new vault
    connect(newId);
  }
}, { immediate: true }); // Immediate: true to run the watcher on initial component mount

onMounted(() => {
  // Initial fetch if not already done by watcher (e.g., direct navigation)
  if (!fileStore.vaultId && route.params.id) {
    fileStore.setVaultId(route.params.id);
    vaultName.value = `Vault ${route.params.id}`;
    fileStore.fetchTree(route.params.id);
    connect(route.params.id);
  }
});
</script>

<style scoped>
.vault-view {
  display: flex;
  height: 100vh;
}

.sidebar {
  width: 250px;
  background-color: var(--background-color-light);
  padding: 1rem;
  border-right: 1px solid var(--border-color);
  overflow-y: auto; /* Enable scrolling for the sidebar */
}

.sidebar-header {
  margin-bottom: 1rem;
}

.vault-name {
  font-size: 1.2rem;
  font-weight: bold;
  color: var(--primary-color);
  margin-bottom: 0.5rem;
}

/* Connection status indicator */
.connection-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.85rem;
  margin-top: 0.5rem;
}

.status-indicator {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.7rem;
}

.status-indicator.connected {
  color: #98c379; /* Green */
}

.status-indicator.disconnected {
  color: #e5c07b; /* Yellow/Orange */
}

.status-indicator.error {
  color: #e06c75; /* Red */
}

.status-text {
  color: var(--text-color-secondary, #666);
}

.status-indicator.connected + .status-text {
  color: #98c379;
}

.status-indicator.error + .status-text {
  color: #e06c75;
}

.main-content {
  flex-grow: 1;
  padding: 2rem;
  overflow-y: auto; /* Enable scrolling for the main content */
  background-color: var(--background-color);
  color: var(--text-color);
}

.loading-spinner, .error-message, .no-content-message {
  text-align: center;
  padding: 2rem;
  font-size: 1.1rem;
  color: var(--text-color-secondary);
}

.markdown-content {
  /* Basic styling for rendered markdown */
  line-height: 1.6;
  max-width: 800px; /* Limit width for readability */
  margin: 0 auto; /* Center the content */
}

.markdown-content h1, .markdown-content h2, .markdown-content h3, .markdown-content h4, .markdown-content h5, .markdown-content h6 {
  margin-top: 1.5em;
  margin-bottom: 0.5em;
  font-weight: bold;
  line-height: 1.2;
}

.markdown-content h1 { font-size: 2em; }
.markdown-content h2 { font-size: 1.75em; }
.markdown-content h3 { font-size: 1.5em; }
.markdown-content h4 { font-size: 1.25em; }
.markdown-content h5 { font-size: 1em; }
.markdown-content h6 { font-size: 0.85em; }

.markdown-content p {
  margin-bottom: 1em;
}

.markdown-content ul, .markdown-content ol {
  margin-bottom: 1em;
  padding-left: 2em;
}

.markdown-content code {
  background-color: rgba(135,131,120,0.15);
  border-radius: 3px;
  padding: 0.2em 0.4em;
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 0.85em;
}

.markdown-content pre {
  background-color: #2d2d2d;
  color: #f8f8f2;
  padding: 1em;
  border-radius: 5px;
  overflow-x: auto;
  margin-bottom: 1em;
}

.markdown-content pre code {
  background-color: transparent;
  padding: 0;
  color: inherit;
  font-size: 1em;
}

.markdown-content a {
  color: var(--primary-color);
  text-decoration: none;
}

.markdown-content a:hover {
  text-decoration: underline;
}

.markdown-content blockquote {
  border-left: 4px solid var(--border-color);
  padding-left: 1em;
  margin-left: 0;
  color: var(--text-color-secondary);
}

.markdown-content table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 1em;
}

.markdown-content th, .markdown-content td {
  border: 1px solid var(--border-color);
  padding: 0.5em 0.8em;
  text-align: left;
}

.markdown-content th {
  background-color: var(--background-color-light);
  font-weight: bold;
}

.markdown-content img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 1em 0;
}
</style>