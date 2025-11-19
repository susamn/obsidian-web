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
      <div v-if="fileStore.loading" class="loading-spinner">
        <i class="fas fa-spinner fa-spin"></i>
        <p>Loading file content...</p>
      </div>
      <div v-else-if="fileStore.error" class="error-message">
        <i class="fas fa-exclamation-circle"></i>
        <p>Error: {{ fileStore.error }}</p>
      </div>
      <div v-else-if="fileStore.selectedFileContent" class="file-viewer">
        <div class="file-header">
          <h3 class="file-title">{{ currentFileName }}</h3>
          <div class="file-meta">
            <span class="file-path">{{ fileStore.currentPath }}</span>
          </div>
        </div>
        <div class="markdown-content" v-html="renderedMarkdown"></div>
      </div>
      <div v-else class="no-content-message">
        <i class="fas fa-file"></i>
        <p>Select a file to view its content.</p>
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed, nextTick } from 'vue';
import { useRoute } from 'vue-router';
import { useFileStore } from '../stores/fileStore';
import { useTreeWalkerStore } from '../stores/treeWalkerStore';
import { useSSE } from '../composables/useSSE';
import FileTree from '../components/FileTree.vue';
import { entryAnimation, exitAnimation } from '../utils/animationUtils';
import MarkdownIt from 'markdown-it';

const md = new MarkdownIt();

const route = useRoute();
const fileStore = useFileStore();
const treeWalkerStore = useTreeWalkerStore();
const vaultName = ref('');
const expandedNodes = ref({});
const connected = ref(false);
const error = ref(null);

const handleToggleExpand = async (node) => {
  if (node.metadata.is_directory) {
    if (expandedNodes.value[node.metadata.id]) {
      // Collapse
      delete expandedNodes.value[node.metadata.id];
      console.log('[VaultView] Collapsed node:', node.metadata.name);
    } else {
      // Expand
      expandedNodes.value[node.metadata.id] = true;
      // Fetch children if not already fetched
      // Check if children haven't been loaded yet
      if (!node.children) {
        node.children = [];
      }
      if (node.children.length === 0) {
        console.log('[VaultView] Fetching children for node:', node.metadata.id, node.metadata.name);
        // Use ID-based API call if ID is available, otherwise fallback to path
        if (node.metadata.id) {
          await fileStore.fetchChildrenByID(fileStore.vaultId, node.metadata.id);
        } else {
          await fileStore.fetchChildren(fileStore.vaultId, node.metadata.path);
        }
        console.log('[VaultView] Received children count:', fileStore.childrenData.length);
        updateNodeChildren(fileStore.treeData, node.metadata.id, fileStore.childrenData);

        // Register children in tree walker
        if (fileStore.childrenData.length > 0) {
          treeWalkerStore.registerNodes(fileStore.vaultId, fileStore.childrenData);
        }

        // Mark path as walked
        treeWalkerStore.markPathWalked(fileStore.vaultId, node.metadata.path);
        console.log('[VaultView] Marked path as walked:', node.metadata.path);

        // Force update by creating new reference to treeData
        fileStore.treeData = [...fileStore.treeData];
      }
    }
  }
};

const handleFileSelected = async (node) => {
  if (!node.metadata.is_directory) {
    // Fetch file content using the node ID (more reliable than path)
    await fileStore.fetchFileContent(fileStore.vaultId, node.metadata.id);
    fileStore.setCurrentPath(node.metadata.path); // Set current path for SSE updates
    console.log('[VaultView] Selected file:', node.metadata.name, 'ID:', node.metadata.id, 'Path:', node.metadata.path);
  }
};

const currentFileName = computed(() => {
  if (!fileStore.currentPath) return 'No file selected';
  const lastSlash = fileStore.currentPath.lastIndexOf('/');
  return lastSlash === -1 ? fileStore.currentPath : fileStore.currentPath.substring(lastSlash + 1);
});

const renderedMarkdown = computed(() => {
  return fileStore.selectedFileContent ? md.render(fileStore.selectedFileContent) : '';
});

const updateNodeChildren = (nodes, targetId, newChildren) => {
  for (let i = 0; i < nodes.length; i++) {
    const node = nodes[i];

    // Use ID-based comparison instead of path
    if (node.metadata.id === targetId) {
      console.log('[VaultView] Found matching node! Updating children count:', newChildren.length);
      // Update children with new array
      node.children = newChildren;
      return true;
    }
    // Recursively search in child nodes
    if (node.metadata.is_directory && node.children && node.children.length > 0) {
      if (updateNodeChildren(node.children, targetId, newChildren)) {
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

/**
 * Smart SSE event handler that only updates UI for walked paths
 * This prevents unnecessary updates to unexpanded folders
 */
const shouldUpdateUI = (eventPath) => {
  // Parse the event path to get parent and filename
  const lastSlash = eventPath.lastIndexOf('/');
  const parentPath = lastSlash === -1 ? '' : eventPath.substring(0, lastSlash);

  // Always update if parent is walked (expanded)
  const parentWalked = treeWalkerStore.isPathWalked(fileStore.vaultId, parentPath);
  console.log(`[VaultView] Event path: ${eventPath}, Parent: ${parentPath}, Parent walked: ${parentWalked}`);

  return parentWalked;
};

/**
 * Find a node in the tree by path
 */
const findNodeByPath = (nodes, targetPath) => {
  if (!nodes) return null;

  for (const node of nodes) {
    if (node.metadata.path === targetPath) {
      return node;
    }
    if (node.children && node.children.length > 0) {
      const found = findNodeByPath(node.children, targetPath);
      if (found) return found;
    }
  }
  return null;
};

/**
 * Find parent directory node in the tree
 */
const findParentNode = (nodes, parentPath) => {
  if (parentPath === '') {
    // Root's children are the main nodes
    return { children: nodes };
  }
  return findNodeByPath(nodes, parentPath);
};

/**
 * Find and remove a child node from parent by path
 */
const removeChildFromParent = (nodes, targetPath) => {
  const lastSlash = targetPath.lastIndexOf('/');
  const parentPath = lastSlash === -1 ? '' : targetPath.substring(0, lastSlash);
  const fileName = targetPath.substring(lastSlash + 1);

  const parent = findParentNode(nodes, parentPath);
  if (!parent || !parent.children) return false;

  const index = parent.children.findIndex(
    (child) => child.metadata.name === fileName
  );
  if (index !== -1) {
    parent.children.splice(index, 1);
    return true;
  }
  return false;
};

// SSE event handlers with smart path-based updates
const sseCallbacks = {
  onConnected: (data) => {
    console.log('[VaultView] SSE connected:', data);
    connected.value = true;
    error.value = null;
  },

  onFileCreated: async (event) => {
    console.log('[VaultView] File created event:', event);

    // Only update UI if parent folder is walked
    if (!shouldUpdateUI(event.path)) {
      console.log('[VaultView] Parent not walked, skipping UI update for:', event.path);
      return;
    }

    console.log('[VaultView] Updating UI for created file:', event.path);

    // Refresh the parent node to get updated children list
    const lastSlash = event.path.lastIndexOf('/');
    const parentPath = lastSlash === -1 ? '' : event.path.substring(0, lastSlash);

    await fileStore.fetchChildren(fileStore.vaultId, parentPath);
    updateNodeChildren(fileStore.treeData, parentPath, fileStore.childrenData);

    // Register new children
    if (fileStore.childrenData.length > 0) {
      treeWalkerStore.registerNodes(fileStore.vaultId, fileStore.childrenData);
    }

    // Trigger animation on new items
    nextTick(() => {
      const newItems = document.querySelectorAll(
        `.tree-node-${event.file_data?.name || 'new'}`
      );
      newItems.forEach((item) => {
        entryAnimation(item, 300).catch(() => {
          // Animation might fail, that's ok
        });
      });
    });

    // Force Vue update
    fileStore.treeData = [...fileStore.treeData];
  },

  onFileModified: async (event) => {
    console.log('[VaultView] File modified event:', event);

    // If the modified file is currently selected, re-fetch its content
    if (fileStore.currentPath === event.path) {
      console.log('[VaultView] Refetching content for selected file:', event.path);
      await fileStore.fetchFileContent(fileStore.vaultId, event.path);
    }
  },

  onFileDeleted: async (event) => {
    console.log('[VaultView] File deleted event:', event);

    // Only update UI if parent folder is walked
    if (!shouldUpdateUI(event.path)) {
      console.log('[VaultView] Parent not walked, skipping UI update for deleted:', event.path);
      return;
    }

    console.log('[VaultView] Removing deleted file from UI:', event.path);

    // If the deleted file was currently selected, clear its content
    if (fileStore.currentPath === event.path) {
      fileStore.selectedFileContent = null;
    }

    // Remove from tree
    if (removeChildFromParent(fileStore.treeData, event.path)) {
      console.log('[VaultView] Successfully removed node from tree');

      // Trigger animation on removed items
      nextTick(() => {
        const removedItems = document.querySelectorAll(
          `.tree-node-deleted-${event.file_data?.name || 'removed'}`
        );
        removedItems.forEach((item) => {
          exitAnimation(item, 300).catch(() => {
            // Animation might fail, that's ok
          });
        });
      });

      // Force Vue update
      fileStore.treeData = [...fileStore.treeData];
    }
  },

  onTreeRefresh: async (event) => {
    console.log('[VaultView] Tree refresh requested:', event.path);
    // For tree refresh, only update if parent is walked
    if (shouldUpdateUI(event.path)) {
      await refreshNode(event.path);
    }
  },

  onError: (err) => {
    console.error('[VaultView] SSE error:', err);
    error.value = err.message || 'SSE connection error';
    connected.value = false;
  },
};

// Initialize SSE connection (vaultId will be passed when calling connect())
const sseHooks = useSSE(sseCallbacks);
const sseConnect = sseHooks.connect;
const sseDisconnect = sseHooks.disconnect;
const sseReconnect = sseHooks.reconnect;

// Watch for changes in the route params, specifically the 'id' for the vault
watch(() => route.params.id, (newId, oldId) => {
  if (newId) {
    // Disconnect old SSE connection if vault changes
    if (oldId && oldId !== newId) {
      sseDisconnect();
      // Clear tree walker for old vault
      treeWalkerStore.clearVault(oldId);
    }

    fileStore.setVaultId(newId);
    vaultName.value = `Vault ${newId}`;
    fileStore.fetchTree(newId);
    expandedNodes.value = {}; // Reset expanded nodes when vault changes
    fileStore.selectedFileContent = null; // Clear selected file content

    // Mark root as walked when tree is loaded
    treeWalkerStore.markRootWalked(newId);
    // Register root nodes
    treeWalkerStore.registerNodes(newId, fileStore.treeData);

    // Connect to SSE for the new vault
    sseConnect(newId);
  }
}, { immediate: true }); // Immediate: true to run the watcher on initial component mount

onMounted(() => {
  // Initial fetch if not already done by watcher (e.g., direct navigation)
  if (!fileStore.vaultId && route.params.id) {
    fileStore.setVaultId(route.params.id);
    vaultName.value = `Vault ${route.params.id}`;
    fileStore.fetchTree(route.params.id);

    // Mark root as walked and register nodes
    treeWalkerStore.markRootWalked(route.params.id);
    treeWalkerStore.registerNodes(route.params.id, fileStore.treeData);

    sseConnect(route.params.id);
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
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 2rem;
  font-size: 1.1rem;
  color: var(--text-color-secondary);
  min-height: 300px;
  gap: 1rem;
}

.loading-spinner i, .error-message i, .no-content-message i {
  font-size: 3rem;
  opacity: 0.5;
}

.file-viewer {
  background-color: white;
  border-radius: 8px;
  padding: 2rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.file-header {
  border-bottom: 2px solid var(--border-color);
  margin-bottom: 2rem;
  padding-bottom: 1rem;
}

.file-title {
  font-size: 1.8rem;
  margin: 0 0 0.5rem 0;
  color: var(--primary-color);
  word-break: break-word;
}

.file-meta {
  display: flex;
  gap: 1rem;
  font-size: 0.9rem;
  color: var(--text-color-secondary);
}

.file-path {
  font-family: monospace;
  background-color: rgba(0, 0, 0, 0.05);
  padding: 0.25rem 0.5rem;
  border-radius: 3px;
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