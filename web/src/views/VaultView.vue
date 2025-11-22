<template>
  <div class="vault-view">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="header-top">
          <h2 class="vault-name">{{ vaultName }}</h2>
          <div class="header-actions">
            <button
              class="search-toggle-button"
              :class="{ active: showSearch }"
              @click="toggleSearch"
              :title="showSearch ? 'Show file browser' : 'Search notes'"
            >
              <i :class="showSearch ? 'fas fa-folder' : 'fas fa-search'"></i>
            </button>
          </div>
        </div>
        <div class="connection-status">
          <span
            class="status-indicator"
            :class="{
              'connected': connected && !bulkOperationProgress.active,
              'disconnected': !connected && !error && !bulkOperationProgress.active,
              'error': error,
              'syncing': bulkOperationProgress.active
            }"
            :title="bulkOperationProgress.active ? `Syncing ${bulkOperationProgress.percentage}%` : (error || (connected ? 'Live updates enabled' : 'Connecting...'))"
          >
            <i v-if="bulkOperationProgress.active" class="fas fa-sync fa-spin"></i>
            <i v-else-if="connected" class="fas fa-circle"></i>
            <i v-else-if="error" class="fas fa-exclamation-circle"></i>
            <i v-else class="fas fa-circle-notch fa-spin"></i>
          </span>
          <span class="status-text">
            <template v-if="bulkOperationProgress.active">
              Syncing {{ bulkOperationProgress.percentage }}%
            </template>
            <template v-else>
              {{ connected ? 'Live' : (error ? 'Offline' : 'Connecting') }}
            </template>
          </span>
        </div>
      </div>

      <!-- Search Panel -->
      <div v-if="showSearch" class="search-section">
        <SearchPanel
          :vault-id="fileStore.vaultId"
          @close="closeSearch"
          @search="handleSearchExecuted"
        />
        <SearchResults
          :vault-id="fileStore.vaultId"
          @result-selected="handleSearchResultSelected"
        />
      </div>

      <!-- File Tree -->
      <div v-else class="file-tree">
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
        <!-- Breadcrumb and Header -->
        <div class="file-header-section">
          <div class="breadcrumb">
            <span v-for="(part, index) in breadcrumbParts" :key="index" class="breadcrumb-item">
              <span v-if="index > 0" class="breadcrumb-separator">/</span>
              <span>{{ part }}</span>
            </span>
          </div>
          <h1 class="file-title">{{ currentFileName }}</h1>
        </div>

        <!-- Dynamic Renderer Component -->
        <component
          :is="currentRendererComponent"
          :content="fileStore.selectedFileContent"
          :vault-id="fileStore.vaultId"
          :file-id="currentFileId"
          @update:markdownResult="markdownResult = $event"
        />
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
import { usePersistentTreeStore } from '../stores/persistentTreeStore';
import { useRendererStore } from '../stores/rendererStore';
import { useSearchStore } from '../stores/searchStore';
import { useSSE } from '../composables/useSSE';
import FileTree from '../components/FileTree.vue';
import SearchPanel from '../components/SearchPanel.vue';
import SearchResults from '../components/SearchResults.vue';
import MarkdownRenderer from '../components/MarkdownRenderer.vue';
import SSRRenderer from '../components/SSRRenderer.vue';
import StructuredRenderer from '../components/StructuredRenderer.vue';
import { entryAnimation, exitAnimation } from '../utils/animationUtils';

const route = useRoute();
const fileStore = useFileStore();
const treeWalkerStore = useTreeWalkerStore();
const persistentTreeStore = usePersistentTreeStore();
const rendererStore = useRendererStore();
const searchStore = useSearchStore();

// Load renderer preference on component setup
rendererStore.loadRendererFromLocalStorage();

const vaultName = ref('');
const expandedNodes = ref({});
const connected = ref(false);
const error = ref(null);
const currentFileId = ref(null); // Track the ID of the currently selected file
const showSearch = ref(false); // Toggle between file browser and search

// Bulk operation progress tracking
const bulkOperationProgress = ref({
  active: false,
  processed: 0,
  total: 0,
  percentage: 0
});

// Markdown rendering state
const markdownResult = ref({
  html: '',
  tags: [],
  frontmatter: {},
  headings: [],
  wikilinks: [],
  stats: { words: 0, chars: 0, readingTime: 0 }
});

// Dynamic renderer component
const currentRendererComponent = computed(() => {
  if (rendererStore.isStructuredRenderer) {
    return StructuredRenderer;
  }
  return rendererStore.isBrowserRenderer ? MarkdownRenderer : SSRRenderer;
});

const handleToggleExpand = async (node) => {
  if (node.metadata.is_directory) {
    if (expandedNodes.value[node.metadata.id]) {
      // Collapse
      delete expandedNodes.value[node.metadata.id];
      console.log('[VaultView] Collapsed node:', node.metadata.name);

      // Update persistent tree
      persistentTreeStore.collapseNode(fileStore.vaultId, node.metadata.id);
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

        // Update persistent tree with new children and mark as expanded
        persistentTreeStore.expandNode(fileStore.vaultId, node.metadata.id, fileStore.childrenData);

        // Force update by creating new reference to treeData
        fileStore.treeData = [...fileStore.treeData];
      } else {
        // Children already loaded, just mark as expanded
        persistentTreeStore.expandNode(fileStore.vaultId, node.metadata.id);
      }
    }
  }
};

const handleFileSelected = async (node) => {
  if (!node.metadata.is_directory) {
    // Set current file ID first
    currentFileId.value = node.metadata.id;

    // Use path if available, otherwise use filename
    fileStore.setCurrentPath(node.metadata.path || node.metadata.name);

    // Only fetch file content for browser and SSR renderers
    // Structured renderer fetches its own data
    if (!rendererStore.isStructuredRenderer) {
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, node.metadata.id);

      // Update the path from server response if available (contains relative path)
      // This path is READ-ONLY and used for UI navigation only
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path);
      }
    } else {
      // For structured renderer, just set a placeholder to show the component
      fileStore.selectedFileContent = 'loading';
    }
  }
};

/**
 * Navigate to a file by its path (for deep linking from wikilinks)
 * This will expand all parent folders and select the file (Obsidian-style)
 * @param {string} filePath - The relative path of the file (READ-ONLY, from server)
 */
const navigateToFile = async (filePath) => {
  console.log('[VaultView] Navigating to file:', filePath);

  // Helper function to fetch children by ID
  const fetchChildrenByIdFn = async (vaultId, nodeId) => {
    await fileStore.fetchChildrenByID(vaultId, nodeId);
    return fileStore.childrenData;
  };

  try {
    // Expand all parent folders
    const expandedNodeIds = await persistentTreeStore.navigateToPath(
      fileStore.vaultId,
      filePath,
      fetchChildrenByIdFn
    );

    // Update UI expanded state
    expandedNodeIds.forEach((nodeId) => {
      expandedNodes.value[nodeId] = true;
    });

    // Force update to reflect expanded state
    fileStore.treeData = [...fileStore.treeData];

    // Find and select the file node
    const fileNode = persistentTreeStore.findNodeByPath(fileStore.vaultId, filePath);
    if (fileNode) {
      await handleFileSelected(fileNode);
      console.log('[VaultView] Successfully navigated to file:', filePath);
    } else {
      console.warn('[VaultView] File node not found after navigation:', filePath);
    }
  } catch (error) {
    console.error('[VaultView] Failed to navigate to file:', filePath, error);
  }
};

// Expose navigateToFile for external use (e.g., from wikilink clicks)
// This can be called from the markdown renderer when a wikilink is clicked
defineExpose({
  navigateToFile,
});

/**
 * Toggle search panel
 */
const toggleSearch = () => {
  showSearch.value = !showSearch.value;
  if (!showSearch.value) {
    // Clear search when closing
    searchStore.clearSearch();
  }
};

/**
 * Close search panel and return to file browser
 */
const closeSearch = () => {
  showSearch.value = false;
  searchStore.clearSearch();
};

/**
 * Handle when a search is executed
 */
const handleSearchExecuted = () => {
  console.log('[VaultView] Search executed, results:', searchStore.total);
};

/**
 * Handle when a search result is selected
 */
const handleSearchResultSelected = async (result) => {
  console.log('[VaultView] Search result selected:', result);

  // The result.id should be the file path or file ID
  // We need to navigate to this file
  try {
    // First, try to get the file by ID to ensure it exists
    const fileId = result.id;

    // Set current file ID
    currentFileId.value = fileId;

    // Fetch file content
    if (!rendererStore.isStructuredRenderer) {
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, fileId);

      // Update the path from server response
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path);
      }
    } else {
      // For structured renderer, just set a placeholder
      fileStore.selectedFileContent = 'loading';
      fileStore.setCurrentPath(fileId);
    }

    // Optionally close search panel after selecting a result
    // Uncomment the next line if you want this behavior
    // closeSearch();
  } catch (error) {
    console.error('[VaultView] Failed to load search result:', error);
  }
};

const currentFileName = computed(() => {
  if (!fileStore.currentPath) return 'No file selected';
  const lastSlash = fileStore.currentPath.lastIndexOf('/');
  return lastSlash === -1 ? fileStore.currentPath : fileStore.currentPath.substring(lastSlash + 1);
});

const breadcrumbParts = computed(() => {
  if (!fileStore.currentPath) return [];
  return fileStore.currentPath.split('/');
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

const updateNodeChildrenByPath = (nodes, targetPath, newChildren) => {
  // For root path, update the root nodes directly
  if (targetPath === '') {
    console.log('[VaultView] Updating root children, count:', newChildren.length);
    nodes.splice(0, nodes.length, ...newChildren);
    return true;
  }

  for (let i = 0; i < nodes.length; i++) {
    const node = nodes[i];

    // Use path-based comparison
    if (node.metadata.path === targetPath) {
      console.log('[VaultView] Found matching node by path! Updating children count:', newChildren.length);
      // Update children with new array
      node.children = newChildren;
      return true;
    }
    // Recursively search in child nodes
    if (node.metadata.is_directory && node.children && node.children.length > 0) {
      if (updateNodeChildrenByPath(node.children, targetPath, newChildren)) {
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

    // Get the file name from path
    const lastSlash = event.path.lastIndexOf('/');
    const fileName = event.path.substring(lastSlash + 1);
    const parentPath = lastSlash === -1 ? '' : event.path.substring(0, lastSlash);

    // Store old children for comparison
    const parentNode = findParentNode(fileStore.treeData, parentPath);
    const oldChildrenIds = new Set(
      (parentNode?.children || []).map(child => child.metadata?.id).filter(Boolean)
    );

    // Refresh the parent node to get updated children list
    await fileStore.fetchChildren(fileStore.vaultId, parentPath);
    updateNodeChildrenByPath(fileStore.treeData, parentPath, fileStore.childrenData);

    // Update persistent tree (with error handling)
    try {
      persistentTreeStore.updateNodeChildren(fileStore.vaultId, parentPath, fileStore.childrenData);
    } catch (err) {
      console.warn('[VaultView] Failed to update persistent tree:', err);
    }

    // Register new children
    if (fileStore.childrenData.length > 0) {
      treeWalkerStore.registerNodes(fileStore.vaultId, fileStore.childrenData);
    }

    // Force Vue update first to render the new element
    fileStore.treeData = [...fileStore.treeData];

    // Find the newly added node and animate it with retry logic
    const animateNewElement = (attempt = 0) => {
      const maxAttempts = 10; // Increased for nested folders
      const delayMs = 50 * (attempt + 1); // Progressive delay

      if (attempt >= maxAttempts) {
        console.warn('[VaultView] Failed to animate new file after', maxAttempts, 'attempts:', fileName);
        return;
      }

      // Find the new child that wasn't in the old list
      const newChild = fileStore.childrenData.find(
        child => child.metadata?.id && !oldChildrenIds.has(child.metadata.id)
      );

      if (!newChild || !newChild.metadata?.id) {
        console.warn('[VaultView] New child not found in data');
        return;
      }

      // Find the DOM element
      const element = document.querySelector(`[data-node-id="${newChild.metadata.id}"]`);

      if (element) {
        console.log('[VaultView] Animating new file:', fileName, '(attempt', attempt + 1, ')');

        // Start with opacity 0 and translate
        element.style.opacity = '0';
        element.style.transform = 'translateY(-10px)';

        // Force reflow
        void element.offsetHeight;

        // Animate in
        requestAnimationFrame(() => {
          element.style.transition = 'opacity 0.4s ease-out, transform 0.4s ease-out';
          element.style.opacity = '1';
          element.style.transform = 'translateY(0)';

          // Clean up after animation
          setTimeout(() => {
            element.style.transition = '';
            element.style.transform = '';
          }, 450);
        });
      } else {
        // Element not found yet, try again after a delay
        console.log('[VaultView] Element not found, retrying in', delayMs, 'ms');
        setTimeout(() => animateNewElement(attempt + 1), delayMs);
      }
    };

    // Start animation attempts after Vue renders
    nextTick(() => {
      setTimeout(() => animateNewElement(0), 100);
    });
  },

  onFileModified: async (event) => {
    console.log('[VaultView] File modified event:', event);

    // If the modified file is currently selected, re-fetch its content
    if (fileStore.currentPath === event.path && currentFileId.value) {
      console.log('[VaultView] Refetching content for selected file:', event.path);
      await fileStore.fetchFileContent(fileStore.vaultId, currentFileId.value);
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
      currentFileId.value = null;
    }

    // Find the node to be deleted to get its ID for animation
    const nodeToDelete = findNodeByPath(fileStore.treeData, event.path);
    const nodeId = nodeToDelete?.metadata?.id;

    // Animate the element out before removing it
    let animationCompleted = false;
    if (nodeId) {
      const element = document.querySelector(`[data-node-id="${nodeId}"]`);

      if (element) {
        console.log('[VaultView] Animating deletion of file:', event.path);

        try {
          // Animate out with fade and slide
          element.style.transition = 'opacity 0.4s ease-out, transform 0.4s ease-out';
          element.style.opacity = '0';
          element.style.transform = 'translateX(-10px)';

          // Wait for animation to complete before removing from DOM
          await new Promise(resolve => setTimeout(resolve, 400));
          animationCompleted = true;
        } catch (err) {
          console.warn('[VaultView] Animation error:', err);
        }
      } else {
        console.warn('[VaultView] Could not find element to animate deletion:', event.path);
      }
    }

    // Always remove from tree, even if animation failed
    const removed = removeChildFromParent(fileStore.treeData, event.path);

    if (removed) {
      console.log('[VaultView] Successfully removed node from tree');

      // Update persistent tree (with error handling)
      try {
        persistentTreeStore.removeNode(fileStore.vaultId, event.path);
      } catch (err) {
        console.warn('[VaultView] Failed to remove from persistent tree:', err);
      }

      // Force Vue update
      fileStore.treeData = [...fileStore.treeData];
    } else {
      console.warn('[VaultView] Failed to remove node from tree, path:', event.path);

      // Fallback: refresh the parent folder to sync state
      const lastSlash = event.path.lastIndexOf('/');
      const parentPath = lastSlash === -1 ? '' : event.path.substring(0, lastSlash);

      console.log('[VaultView] Attempting fallback: refreshing parent folder:', parentPath);
      try {
        await fileStore.fetchChildren(fileStore.vaultId, parentPath);
        updateNodeChildrenByPath(fileStore.treeData, parentPath, fileStore.childrenData);
        fileStore.treeData = [...fileStore.treeData];

        // Update persistent tree
        persistentTreeStore.updateNodeChildren(fileStore.vaultId, parentPath, fileStore.childrenData);
      } catch (err) {
        console.error('[VaultView] Fallback refresh failed:', err);
      }
    }
  },

  onTreeRefresh: async (event) => {
    console.log('[VaultView] Tree refresh requested:', event.path);
    // For tree refresh, only update if parent is walked and path exists
    if (event.path && shouldUpdateUI(event.path)) {
      await refreshNode(event.path);
    }
  },

  onBulkUpdate: async (data) => {
    console.log('[VaultView] Bulk update received:', data.summary);

    const total = data.summary.created + data.summary.modified + data.summary.deleted;
    console.log(`[VaultView] Bulk update: ${total} files changed`);

    // Activate progress indicator
    bulkOperationProgress.value = {
      active: true,
      processed: data.changes.length,
      total: total,
      percentage: Math.round((data.changes.length / total) * 100)
    };

    // Refresh the root tree to pick up all changes
    await fileStore.fetchTree(route.params.id);

    // Update persistent storage
    persistentTreeStore.initializeTree(route.params.id, fileStore.treeData);

    // Clear progress after a delay
    setTimeout(() => {
      bulkOperationProgress.value.active = false;
    }, 2000);
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
watch(() => route.params.id, async (newId, oldId) => {
  if (newId) {
    // Disconnect old SSE connection if vault changes
    if (oldId && oldId !== newId) {
      sseDisconnect();
      // Clear tree walker for old vault
      treeWalkerStore.clearVault(oldId);
    }

    fileStore.setVaultId(newId);
    vaultName.value = `Vault ${newId}`;

    // Try to restore tree from persistent storage first
    const savedTree = persistentTreeStore.getTree(newId);
    if (savedTree && savedTree.length > 0) {
      console.log('[VaultView] Restoring tree from persistent storage');
      fileStore.treeData = savedTree;

      // Restore expanded state
      const expandedIds = persistentTreeStore.getExpandedNodeIds(newId);
      expandedIds.forEach((id) => {
        expandedNodes.value[id] = true;
      });
    } else {
      // No saved tree, fetch from server
      console.log('[VaultView] No saved tree, fetching from server');
      await fileStore.fetchTree(newId);

      // Initialize persistent tree with fetched data
      persistentTreeStore.initializeTree(newId, fileStore.treeData);
    }

    // Reset expanded nodes when vault changes if no saved state
    if (!savedTree || savedTree.length === 0) {
      expandedNodes.value = {};
    }

    fileStore.selectedFileContent = null; // Clear selected file content
    currentFileId.value = null; // Clear selected file ID when vault changes

    // Mark root as walked when tree is loaded
    treeWalkerStore.markRootWalked(newId);
    // Register root nodes
    treeWalkerStore.registerNodes(newId, fileStore.treeData);

    // Connect to SSE for the new vault
    sseConnect(newId);
  }
}, { immediate: true }); // Immediate: true to run the watcher on initial component mount

onMounted(() => {
  // Restore persistent tree state from localStorage on mount
  persistentTreeStore.restoreFromStorage();

  // Initial fetch if not already done by watcher (e.g., direct navigation)
  if (!fileStore.vaultId && route.params.id) {
    fileStore.setVaultId(route.params.id);
    vaultName.value = `Vault ${route.params.id}`;

    // Check for saved tree
    const savedTree = persistentTreeStore.getTree(route.params.id);
    if (savedTree && savedTree.length > 0) {
      fileStore.treeData = savedTree;
    } else {
      fileStore.fetchTree(route.params.id);
      persistentTreeStore.initializeTree(route.params.id, fileStore.treeData);
    }

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
  width: 300px;
  min-width: 300px;
  max-width: 300px;
  flex-shrink: 0;
  background-color: var(--background-color-light);
  padding: 1rem;
  border-right: 1px solid var(--border-color);
  overflow-y: auto;
}

.sidebar-header {
  margin-bottom: 1rem;
}

.header-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}

.vault-name {
  font-size: 1.2rem;
  font-weight: bold;
  color: var(--primary-color);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 0.5rem;
}

.search-toggle-button {
  background: none;
  border: none;
  color: var(--text-color);
  cursor: pointer;
  padding: 0.5rem;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s, color 0.2s;
  font-size: 1rem;
}

.search-toggle-button:hover {
  background-color: var(--background-color);
  color: var(--primary-color);
}

.search-toggle-button.active {
  background-color: var(--primary-color);
  color: white;
}

.search-section {
  display: flex;
  flex-direction: column;
  height: calc(100% - 4rem);
  overflow: hidden;
}

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
  color: #98c379;
}

.status-indicator.disconnected {
  color: #e5c07b;
}

.status-indicator.error {
  color: #e06c75;
}

.status-indicator.syncing {
  color: #61afef;
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
  overflow-y: auto;
  background-color: var(--background-color);
  color: var(--text-color);
}

.loading-spinner,
.error-message,
.no-content-message {
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

.loading-spinner i,
.error-message i,
.no-content-message i {
  font-size: 3rem;
  opacity: 0.5;
}

.file-viewer {
  background-color: var(--background-color);
  padding: 0;
  margin: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
}

.file-header-section {
  padding: 1.5rem 2rem 0;
  border-bottom: 1px solid var(--border-color);
}

.breadcrumb {
  font-size: 0.85rem;
  color: var(--text-color-secondary);
  margin-bottom: 0.5rem;
  display: flex;
  align-items: center;
  gap: 0;
}

.breadcrumb-item {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.breadcrumb-separator {
  color: var(--border-color);
}

.file-title {
  font-size: 2em;
  font-weight: 600;
  color: var(--text-color);
  margin: 0.5rem 0 1rem 0;
  padding: 0;
}
</style>