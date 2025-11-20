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

        <!-- Outline Toggle Button -->
        <button class="outline-toggle" @click="showOutline = !showOutline" title="Toggle outline">
          <i class="fas fa-list"></i>
        </button>

        <!-- Outline Panel -->
        <div v-if="showOutline" class="outline-panel">
          <div class="outline-header">Outline</div>
          <div v-if="markdownResult.headings.length === 0" class="outline-empty">No headings</div>
          <nav v-else class="outline-list">
            <a
              v-for="heading in markdownResult.headings"
              :key="heading.id"
              :href="`#${heading.id}`"
              :class="['outline-item', `outline-level-${heading.level}`]"
              @click.prevent="scrollToHeading(heading.id)"
            >
              {{ heading.text }}
            </a>
          </nav>
        </div>

        <!-- Rendered Markdown Content with Collapsible Sections -->
        <div class="markdown-content-wrapper">
          <div class="markdown-content" ref="markdownContentRef" v-html="renderedMarkdown"></div>
        </div>
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
import { renderObsidianMarkdown } from '../utils/obsidianMarkdownRenderer';

const route = useRoute();
const fileStore = useFileStore();
const treeWalkerStore = useTreeWalkerStore();
const vaultName = ref('');
const expandedNodes = ref({});
const connected = ref(false);
const error = ref(null);
const currentFileId = ref(null); // Track the ID of the currently selected file
const showOutline = ref(false);
const markdownContentRef = ref(null);
const collapsibleSections = ref({});

// Markdown rendering state
const markdownResult = ref({
  html: '',
  tags: [],
  frontmatter: {},
  headings: [],
  wikilinks: [],
  stats: { words: 0, chars: 0, readingTime: 0 }
});

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
    // Use path if available, otherwise use filename
    fileStore.setCurrentPath(node.metadata.path || node.metadata.name);
    currentFileId.value = node.metadata.id; // Track the file ID for SSE updates
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

const renderedMarkdown = computed(() => {
  if (!fileStore.selectedFileContent) {
    markdownResult.value = {
      html: '',
      tags: [],
      frontmatter: {},
      headings: [],
      wikilinks: [],
      stats: { words: 0, chars: 0, readingTime: 0 }
    };
    return '';
  }

  // Render markdown with Obsidian features
  markdownResult.value = renderObsidianMarkdown(fileStore.selectedFileContent);

  // Make collapsible sections after rendering
  nextTick(() => {
    makeHeadingsCollapsible();
    renderMathContent();
  });

  return markdownResult.value.html;
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

// Handler for wikilink clicks
const handleWikilinkClick = (event) => {
  const target = event.target;

  if (target.classList.contains('md-wikilink')) {
    event.preventDefault();
    const pageName = target.getAttribute('data-page');

    // TODO: Implement navigation to linked page
    // Example: searchAndNavigateToFile(pageName)
    console.log('Navigate to wikilink:', pageName);
  }
};

// Handler for tag clicks
const handleTagClick = (event) => {
  const target = event.target;

  if (target.classList.contains('md-tag') || target.classList.contains('tag-chip')) {
    const tag = target.getAttribute('data-tag');

    // TODO: Implement tag filtering or search
    // Example: filterFilesByTag(tag)
    console.log('Filter by tag:', tag);
  }
};

// Scroll to heading by ID
const scrollToHeading = (headingId) => {
  nextTick(() => {
    const element = document.getElementById(headingId);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  });
};

// Make headings collapsible
const makeHeadingsCollapsible = () => {
  if (!markdownContentRef.value) return;

  nextTick(() => {
    const headings = markdownContentRef.value.querySelectorAll('h2, h3, h4, h5, h6');

    headings.forEach((heading) => {
      // Add toggle button to heading
      const toggleBtn = document.createElement('button');
      toggleBtn.className = 'heading-toggle';
      toggleBtn.innerHTML = '<i class="fas fa-chevron-down"></i>';
      toggleBtn.setAttribute('aria-expanded', 'true');
      heading.insertBefore(toggleBtn, heading.firstChild);

      // Collect all elements until next heading of same or higher level
      const headingLevel = parseInt(heading.tagName[1]);
      const contentElements = [];
      let nextElement = heading.nextElementSibling;

      while (nextElement) {
        if (nextElement.tagName && /^H[1-6]$/.test(nextElement.tagName)) {
          const nextLevel = parseInt(nextElement.tagName[1]);
          if (nextLevel <= headingLevel) break;
        }
        contentElements.push(nextElement);
        nextElement = nextElement.nextElementSibling;
      }

      // Create wrapper for collapsible content
      const contentWrapper = document.createElement('div');
      contentWrapper.className = 'collapsible-content';
      contentWrapper.style.display = 'block';

      contentElements.forEach((el) => {
        contentWrapper.appendChild(el.cloneNode(true));
      });

      heading.parentNode.insertBefore(contentWrapper, heading.nextSibling);

      // Toggle handler
      toggleBtn.addEventListener('click', (e) => {
        e.preventDefault();
        const isExpanded = contentWrapper.style.display !== 'none';
        contentWrapper.style.display = isExpanded ? 'none' : 'block';
        toggleBtn.setAttribute('aria-expanded', isExpanded ? 'false' : 'true');
        toggleBtn.classList.toggle('collapsed', isExpanded);
      });
    });

    // Remove original content elements to avoid duplicates
    headings.forEach((heading) => {
      const headingLevel = parseInt(heading.tagName[1]);
      let nextElement = heading.nextElementSibling;

      while (nextElement) {
        if (nextElement.tagName && /^H[1-6]$/.test(nextElement.tagName)) {
          const nextLevel = parseInt(nextElement.tagName[1]);
          if (nextLevel <= headingLevel) break;
        }
        const toRemove = nextElement;
        nextElement = nextElement.nextElementSibling;

        // Only remove if not already in a collapsible-content wrapper
        if (!toRemove.classList.contains('collapsible-content') &&
            toRemove.parentNode &&
            !toRemove.parentNode.classList.contains('collapsible-content')) {
          toRemove.remove();
        }
      }
    });
  });
};

// Render math content using KaTeX
const renderMathContent = () => {
  if (!markdownContentRef.value) return;

  nextTick(() => {
    const mathElements = markdownContentRef.value.querySelectorAll('.md-math-inline, .md-math-block');

    if (mathElements.length === 0) return;

    // Load KaTeX dynamically if not already loaded
    if (!window.katex) {
      // Load KaTeX CSS
      const katexCss = document.createElement('link');
      katexCss.rel = 'stylesheet';
      katexCss.href = 'https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.css';
      document.head.appendChild(katexCss);

      // Load KaTeX JS
      const katexJs = document.createElement('script');
      katexJs.src = 'https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.js';
      katexJs.async = true;
      katexJs.onload = () => renderMathElements(mathElements);
      document.head.appendChild(katexJs);
    } else {
      renderMathElements(mathElements);
    }
  });
};

// Helper to render individual math elements
const renderMathElements = (mathElements) => {
  mathElements.forEach((el) => {
    const script = el.querySelector('script');
    if (!script) return;

    const content = script.textContent;
    const isBlock = script.type === 'math/tex; mode=display';

    try {
      window.katex.render(content, el, {
        throwOnError: false,
        displayMode: isBlock
      });
    } catch (error) {
      console.warn('[Math Render Error]', error);
    }
  });
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
    updateNodeChildrenByPath(fileStore.treeData, parentPath, fileStore.childrenData);

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

  // Add event listeners for interactive markdown elements
  const mainContent = document.querySelector('.main-content');

  if (mainContent) {
    mainContent.addEventListener('click', handleWikilinkClick);
    mainContent.addEventListener('click', handleTagClick);
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

.outline-empty {
  padding: 1rem;
  color: var(--text-color-secondary);
  font-size: 0.85rem;
  text-align: center;
}

.outline-list {
  display: flex;
  flex-direction: column;
  padding: 0.5rem;
}

.outline-item {
  padding: 0.5rem 0.75rem;
  color: var(--md-link-color);
  text-decoration: none;
  font-size: 0.85rem;
  border-radius: 3px;
  transition: all 0.2s ease;
  border-left: 2px solid transparent;
  cursor: pointer;
}

.outline-item:hover {
  background-color: var(--background-color);
  color: var(--md-link-hover);
}

.outline-level-2 {
  padding-left: 1rem;
}

.outline-level-3 {
  padding-left: 1.5rem;
}

.outline-level-4 {
  padding-left: 2rem;
}

.outline-level-5 {
  padding-left: 2.5rem;
}

.outline-level-6 {
  padding-left: 3rem;
}

.markdown-content-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 0;
  min-width: 0;
}

.markdown-content {
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
}

/* HEADING STYLES - OBSIDIAN LIKE */

.markdown-content h1 {
  font-size: 2.15em;
  font-weight: 700;
  margin-top: 1.8em;
  margin-bottom: 0.6em;
  color: var(--md-heading-color);
  line-height: 1.3;
}

.markdown-content h2 {
  font-size: 1.75em;
  font-weight: 700;
  margin-top: 1.6em;
  margin-bottom: 0.5em;
  color: var(--md-heading-color);
  line-height: 1.3;
  display: flex;
  align-items: center;
  gap: 0.4em;
}

.markdown-content h3 {
  font-size: 1.4em;
  font-weight: 600;
  margin-top: 1.4em;
  margin-bottom: 0.4em;
  color: var(--md-heading-color);
  line-height: 1.3;
  display: flex;
  align-items: center;
  gap: 0.4em;
}

.markdown-content h4 {
  font-size: 1.2em;
  font-weight: 600;
  margin-top: 1.2em;
  margin-bottom: 0.3em;
  color: var(--md-heading-color);
  line-height: 1.3;
  display: flex;
  align-items: center;
  gap: 0.4em;
}

.markdown-content h5 {
  font-size: 1.05em;
  font-weight: 600;
  margin-top: 1em;
  margin-bottom: 0.3em;
  color: var(--md-heading-color);
  line-height: 1.3;
}

.markdown-content h6 {
  font-size: 1em;
  font-weight: 600;
  margin-top: 1em;
  margin-bottom: 0.3em;
  color: var(--md-heading-color);
  line-height: 1.3;
}

.heading-toggle {
  background: none;
  border: none;
  color: var(--md-heading-color);
  cursor: pointer;
  padding: 0 0.3em;
  margin: 0;
  font-size: 0.75em;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.2s ease;
  flex-shrink: 0;
  opacity: 0.6;
}

.heading-toggle:hover {
  opacity: 1;
}

.heading-toggle.collapsed {
  transform: rotate(-90deg);
}

.collapsible-content {
  transition: max-height 0.3s ease;
}

/* ============================================================================ */
/* CALLOUT / ADMONITION STYLING (Obsidian-like) */
/* ============================================================================ */

.md-callout {
  margin: 1.2em 0;
  border-left: 5px solid;
  border-radius: 6px;
  padding: 1em 1.2em;
  background-color: var(--background-color-light);
  display: flex;
  flex-direction: column;
}

.md-callout-header {
  display: flex;
  align-items: center;
  gap: 0.8em;
  font-weight: 600;
  font-size: 0.95em;
  margin-bottom: 0.6em;
  text-transform: none;
}

.md-callout-icon {
  font-size: 1.3em;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.md-callout-title {
  color: var(--text-color);
  font-weight: 600;
}

.md-callout-content {
  padding-left: 2.4em;
  color: var(--text-color);
  font-size: 0.98em;
  line-height: 1.7;
}

.md-callout-content > :first-child {
  margin-top: 0;
}

.md-callout-content > :last-child {
  margin-bottom: 0;
}

/* Note - Blue */
.md-callout-note,
.md-callout-abstract,
.md-callout-summary,
.md-callout-tldr {
  border-left-color: #0ea5e9;
  background-color: rgba(14, 165, 233, 0.08);
}

.md-callout-note .md-callout-title,
.md-callout-abstract .md-callout-title,
.md-callout-summary .md-callout-title,
.md-callout-tldr .md-callout-title {
  color: #0ea5e9;
}

/* Info - Cyan */
.md-callout-info {
  border-left-color: #06b6d4;
  background-color: rgba(6, 182, 212, 0.08);
}

.md-callout-info .md-callout-title {
  color: #06b6d4;
}

/* Tip/Hint - Green */
.md-callout-tip,
.md-callout-hint {
  border-left-color: #10b981;
  background-color: rgba(16, 185, 129, 0.08);
}

.md-callout-tip .md-callout-title,
.md-callout-hint .md-callout-title {
  color: #10b981;
}

/* Important - Purple */
.md-callout-important {
  border-left-color: #a855f7;
  background-color: rgba(168, 85, 247, 0.08);
}

.md-callout-important .md-callout-title {
  color: #a855f7;
}

/* Warning/Caution - Yellow/Orange */
.md-callout-warning,
.md-callout-caution,
.md-callout-attention {
  border-left-color: #f59e0b;
  background-color: rgba(245, 158, 11, 0.08);
}

.md-callout-warning .md-callout-title,
.md-callout-caution .md-callout-title,
.md-callout-attention .md-callout-title {
  color: #f59e0b;
}

/* Danger/Error/Failure - Red */
.md-callout-danger,
.md-callout-error,
.md-callout-failure,
.md-callout-bug {
  border-left-color: #ef4444;
  background-color: rgba(239, 68, 68, 0.08);
}

.md-callout-danger .md-callout-title,
.md-callout-error .md-callout-title,
.md-callout-failure .md-callout-title,
.md-callout-bug .md-callout-title {
  color: #ef4444;
}

/* Example - Pink */
.md-callout-example {
  border-left-color: #ec4899;
  background-color: rgba(236, 72, 153, 0.08);
}

.md-callout-example .md-callout-title {
  color: #ec4899;
}

/* Quote - Gray */
.md-callout-quote {
  border-left-color: #6b7280;
  background-color: rgba(107, 114, 128, 0.08);
}

.md-callout-quote .md-callout-title {
  color: #6b7280;
}

/* ============================================================================ */
/* BLOCKQUOTE (regular, non-callout) */
/* ============================================================================ */

.md-blockquote {
  border-left: 4px solid var(--md-blockquote-border);
  padding-left: 1em;
  margin: 0.75em 0;
  color: var(--md-blockquote-text);
}

/* ============================================================================ */
/* CODE STYLING - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content .md-inline-code {
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

.markdown-content .md-code-block-wrapper {
  margin: 1.2em 0;
  border: 1px solid var(--md-pre-border);
  border-radius: 6px;
  overflow: hidden;
  background-color: var(--md-pre-bg);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.markdown-content .md-code-lang {
  background-color: rgba(0, 0, 0, 0.3);
  padding: 0.5em 1em;
  font-size: 0.8em;
  color: var(--md-code-text);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  border-bottom: 1px solid var(--md-pre-border);
  font-family: 'Fira Code', monospace;
}

.markdown-content .md-code-block {
  padding: 1.2em;
  overflow-x: auto;
  margin: 0;
  border-radius: 0;
  border: none;
  background-color: transparent;
  color: var(--md-code-text);
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.9em;
  line-height: 1.6;
}

.markdown-content pre code {
  background-color: transparent;
  color: var(--md-code-text);
  padding: 0;
  font-size: 1em;
  border: none;
}

/* ============================================================================ */
/* LIST STYLING - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content ul,
.markdown-content ol {
  margin: 0.8em 0;
  padding-left: 2.2em;
}

.markdown-content ul li,
.markdown-content ol li {
  margin-bottom: 0.45em;
  line-height: 1.75;
}

.markdown-content .md-list {
  margin: 0.8em 0;
  padding-left: 2.2em;
}

.markdown-content .md-list li {
  margin-bottom: 0.45em;
  line-height: 1.75;
}

.markdown-content .md-list ul,
.markdown-content .md-list ol,
.markdown-content .md-list .md-list {
  margin: 0.3em 0;
  padding-left: 1.8em;
}

.markdown-content .md-list-level-2 {
  padding-left: 1.8em;
}

.markdown-content .md-list-level-3 {
  padding-left: 1.8em;
}

/* Ordered list styling */
.markdown-content .md-ordered-list {
  list-style-type: decimal;
}

.markdown-content .md-ordered-list .md-ordered-list {
  list-style-type: lower-alpha;
}

.markdown-content .md-ordered-list .md-ordered-list .md-ordered-list {
  list-style-type: lower-roman;
}

/* Bullet styling for nested lists */
.markdown-content ul {
  list-style-type: disc;
}

.markdown-content ul ul {
  list-style-type: circle;
}

.markdown-content ul ul ul {
  list-style-type: square;
}

/* ============================================================================ */
/* EMPHASIS AND STRONG TEXT - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content strong,
.markdown-content .md-strong {
  font-weight: 600;
  color: var(--text-color);
}

.markdown-content em,
.markdown-content .md-emphasis {
  font-style: italic;
  color: var(--text-color);
}

.markdown-content p {
  margin: 0.8em 0;
  line-height: 1.8;
}

/* ============================================================================ */
/* TABLE STYLING */
/* ============================================================================ */

.markdown-content table {
  width: 100%;
  border-collapse: collapse;
  margin: 1em 0;
}

.markdown-content th, .markdown-content td {
  border: 1px solid var(--md-table-border);
  padding: 0.75em 1em;
  text-align: left;
}

.markdown-content th {
  background-color: var(--md-table-header-bg);
  font-weight: bold;
  color: var(--text-color);
}

.markdown-content td {
  background-color: var(--background-color);
}

.markdown-content tbody tr:hover {
  background-color: var(--background-color-light);
}

/* ============================================================================ */
/* IMAGE STYLING */
/* ============================================================================ */

.markdown-content img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 1em auto;
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

/* ============================================================================ */
/* HORIZONTAL RULE */
/* ============================================================================ */

.markdown-content hr {
  border: none;
  border-top: 2px solid var(--md-hr-color);
  margin: 2em 0;
}

/* ============================================================================ */
/* SPACING AND TYPOGRAPHY - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content {
  font-size: 15px;
  line-height: 1.8;
}

/* Better spacing between different element types */
.markdown-content h1 + p,
.markdown-content h2 + p,
.markdown-content h3 + p,
.markdown-content h4 + p,
.markdown-content h5 + p,
.markdown-content h6 + p {
  margin-top: 0.3em;
}

/* Extra spacing before sections */
.markdown-content h2 {
  margin-top: 1.8em;
}

.markdown-content h3 {
  margin-top: 1.5em;
}

/* ============================================================================ */
/* LINK STYLING - OBSIDIAN-LIKE */
/* ============================================================================ */

.markdown-content a {
  color: #3b82f6;
  text-decoration: none;
  transition: all 0.2s ease;
  font-weight: 500;
}

.markdown-content a:hover {
  color: #2563eb;
  text-decoration: underline;
}

.markdown-content .md-wikilink {
  color: #3b82f6;
  font-weight: 500;
  background-color: rgba(59, 130, 246, 0.1);
  padding: 0.15em 0.35em;
  border-radius: 3px;
  transition: all 0.2s ease;
}

.markdown-content .md-wikilink:hover {
  background-color: rgba(59, 130, 246, 0.2);
  text-decoration: underline;
}

.markdown-content .md-tag {
  color: #8b5cf6;
  font-weight: 500;
  background-color: rgba(139, 92, 246, 0.1);
  padding: 0.15em 0.35em;
  border-radius: 3px;
  font-size: 0.95em;
  transition: all 0.2s ease;
}

.markdown-content .md-tag:hover {
  background-color: rgba(139, 92, 246, 0.2);
}

.markdown-content .md-blockref {
  color: #3b82f6;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.9em;
  background-color: rgba(59, 130, 246, 0.08);
  padding: 0.15em 0.3em;
  border-radius: 3px;
}

/* ============================================================================ */
/* MATH / LATEX STYLING */
/* ============================================================================ */

.markdown-content .md-math-inline {
  display: inline;
  font-family: 'KaTeX_Main', serif;
}

.markdown-content .md-math-inline .katex {
  margin: 0 0.1em;
}

.markdown-content .md-math-block {
  margin: 1.2em 0;
  padding: 1em;
  background-color: rgba(0, 0, 0, 0.03);
  border-left: 3px solid #3b82f6;
  border-radius: 4px;
  overflow-x: auto;
}

.markdown-content .md-math-block .katex-display {
  margin: 0;
}

/* KaTeX styles override for consistency */
.markdown-content .katex {
  color: var(--text-color);
}

/* ============================================================================ */
/* YOUTUBE VIDEO EMBEDDING */
/* ============================================================================ */

.markdown-content .md-youtube-embed {
  margin: 1.5em 0;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  background-color: #000;
}

.markdown-content .md-youtube-embed iframe {
  display: block;
  border: none;
}
</style>