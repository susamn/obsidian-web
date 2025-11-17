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
        />
      </div>
    </aside>
    <main class="main-content">
      <p>Main content will be here.</p>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useFileStore } from '../stores/fileStore';
import { useSSE } from '../composables/useSSE';
import FileTree from '../components/FileTree.vue';

const route = useRoute();
const fileStore = useFileStore();
const vaultName = ref('');
const expandedNodes = ref({});
const sseConnectionStatus = ref('disconnected');

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
        // Assuming the API returns children for the given path,
        // we need to find the node in treeData and update its children.
        // This is a simplified approach; a more robust solution might involve
        // normalizing the tree data in the store.
        updateNodeChildren(fileStore.treeData, node.metadata.path, fileStore.childrenData);
      }
    }
  }
};

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
    sseConnectionStatus.value = 'connected';
  },

  onFileCreated: async (event) => {
    console.log('[VaultView] File created:', event.path);
    await refreshNode(event.path);
  },

  onFileModified: async (event) => {
    console.log('[VaultView] File modified:', event.path);
    await refreshNode(event.path);
  },

  onFileDeleted: async (event) => {
    console.log('[VaultView] File deleted:', event.path);
    await refreshNode(event.path);
  },

  onTreeRefresh: async (event) => {
    console.log('[VaultView] Tree refresh requested:', event.path);
    await refreshNode(event.path);
  },

  onError: (err) => {
    console.error('[VaultView] SSE error:', err);
    sseConnectionStatus.value = 'error';
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
}
</style>