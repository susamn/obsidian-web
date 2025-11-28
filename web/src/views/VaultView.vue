<template>
  <div class="vault-view">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="header-top">
          <h2 class="vault-name">
            {{ vaultName }}
          </h2>
          <div class="header-actions">
            <button
              class="search-toggle-button"
              :class="{ active: showSearch }"
              :title="showSearch ? 'Show file browser' : 'Search notes'"
              @click="toggleSearch"
            >
              <i :class="showSearch ? 'fas fa-folder' : 'fas fa-search'" />
            </button>
          </div>
        </div>
      </div>

      <!-- Search Panel -->
      <div
        v-if="showSearch"
        class="search-section"
      >
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
      <div
        v-else
        class="file-tree"
      >
        <QuickFind
          :vault-id="fileStore.vaultId"
          @select="handleQuickFindSelect"
        />
        <p v-if="fileStore.loading">
          Loading file tree...
        </p>
        <p
          v-else-if="fileStore.error"
          class="text-red-500"
        >
          Error: {{ fileStore.error }}
        </p>
        <FileTree
          v-else
          :nodes="fileStore.treeData"
          :vault-id="fileStore.vaultId"
          :expanded-nodes="expandedNodes"
          :selected-file-id="selectedFileId"
          @toggle-expand="handleToggleExpand"
          @file-selected="handleFileSelected"
          @create-clicked="handleCreateClick"
        />
      </div>
    </aside>

    <main class="main-content">
      <div
        v-if="fileStore.loading"
        class="loading-spinner"
      >
        <i class="fas fa-spinner fa-spin" />
        <p>Loading file content...</p>
      </div>
      <div
        v-else-if="fileStore.error"
        class="error-message"
      >
        <i class="fas fa-exclamation-circle" />
        <p>Error: {{ fileStore.error }}</p>
      </div>
      <div
        v-else-if="fileStore.selectedFileContent"
        class="file-viewer"
      >
        <!-- Breadcrumb and Header -->
        <div class="file-header-section">
          <div class="navigation-bar">
            <div class="nav-buttons">
              <button
                class="nav-button"
                :disabled="!canGoBack"
                title="Go back"
                @click="goBack"
              >
                <i class="fas fa-arrow-left" />
              </button>
              <button
                class="nav-button"
                :disabled="!canGoForward"
                title="Go forward"
                @click="goForward"
              >
                <i class="fas fa-arrow-right" />
              </button>
            </div>
            <div class="breadcrumb">
              <span
                v-for="(part, index) in breadcrumbParts"
                :key="index"
                class="breadcrumb-item"
              >
                <span
                  v-if="index > 0"
                  class="breadcrumb-separator"
                >/</span>
                <span>{{ part }}</span>
              </span>
            </div>
          </div>
          <div class="title-stats-row">
            <h1 class="file-title">
              {{ currentFileName }}
            </h1>
            <div
              v-if="markdownResult.stats && markdownResult.stats.words > 0"
              class="file-stats"
            >
              <span
                class="stat-chip"
                title="Word count"
              >
                <i class="fas fa-font" />
                {{ markdownResult.stats.words.toLocaleString() }}
              </span>
              <span
                class="stat-chip"
                title="Character count"
              >
                <i class="fas fa-text-width" />
                {{
                  markdownResult.stats.chars
                    ? markdownResult.stats.chars.toLocaleString()
                    : markdownResult.stats.characters?.toLocaleString()
                }}
              </span>
              <span
                class="stat-chip"
                title="Reading time"
              >
                <i class="far fa-clock" />
                {{
                  typeof markdownResult.stats.readingTime === 'number'
                    ? `${markdownResult.stats.readingTime} min`
                    : markdownResult.stats.readingTime
                }}
              </span>
            </div>
          </div>
        </div>

        <!-- Dynamic Renderer Component -->
        <component
          :is="currentRendererComponent"
          :key="`${fileStore.vaultId}-${currentFileId}`"
          :content="fileStore.selectedFileContent"
          :vault-id="fileStore.vaultId"
          :file-id="currentFileId"
          @update:markdown-result="markdownResult = $event"
          @wikilink-click="handleWikilinkNavigation"
        />
      </div>
      <div
        v-else
        class="file-viewer"
      >
        <div class="no-content-message">
          <i class="fas fa-file" />
          <p>Select a file to view its content.</p>
        </div>
      </div>
    </main>

    <!-- Create Note Dialog -->
    <CreateNoteDialog
      :show="showCreateDialog"
      :vault-id="fileStore.vaultId"
      :parent-id="createParentId"
      @close="closeCreateDialog"
      @created="handleFileCreated"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { useFileStore } from '../stores/fileStore'
import { usePersistentTreeStore } from '../stores/persistentTreeStore'
import { useSearchStore } from '../stores/searchStore'
import { useSSE } from '../composables/useSSE'
import FileTree from '../components/FileTree.vue'
import QuickFind from '../components/QuickFind.vue'
import SearchPanel from '../components/SearchPanel.vue'
import SearchResults from '../components/SearchResults.vue'
import StructuredRenderer from '../components/StructuredRenderer.vue'
import CanvasRenderer from '../components/CanvasRenderer.vue'
import ImageRenderer from '../components/ImageRenderer.vue'
import CreateNoteDialog from '../components/CreateNoteDialog.vue'

const route = useRoute()
const fileStore = useFileStore()
const persistentTreeStore = usePersistentTreeStore()
const searchStore = useSearchStore()

const vaultName = ref('')
const expandedNodes = ref({})
const connected = ref(false)
const error = ref(null)
const currentFileId = ref(null) // Track the ID of the currently selected file
const selectedFileId = ref(null) // Track the selected file ID for visual highlighting in tree
const showSearch = ref(false) // Toggle between file browser and search

// Navigation history
const navigationHistory = ref([])
const navigationIndex = ref(-1)
const isNavigatingHistory = ref(false) // Flag to prevent adding to history during back/forward

// Create dialog state
const showCreateDialog = ref(false)
const createParentId = ref(null)

// Bulk operation progress tracking
const bulkOperationProgress = ref({
  active: false,
  processed: 0,
  total: 0,
  percentage: 0,
})

// Markdown rendering state
const markdownResult = ref({
  html: '',
  tags: [],
  frontmatter: {},
  headings: [],
  wikilinks: [],
  stats: { words: 0, chars: 0, readingTime: 0 },
})

// Determine file type from extension
const getFileType = (path) => {
  if (!path) return 'unknown'
  const ext = path.split('.').pop().toLowerCase()

  if (ext === 'canvas') return 'canvas'
  if (['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp'].includes(ext)) return 'image'
  if (ext === 'pdf') return 'pdf'
  if (['md', 'txt'].includes(ext)) return 'markdown'

  return 'markdown' // Default to markdown/structured renderer
}

// Dynamic renderer component
const currentRendererComponent = computed(() => {
  const type = getFileType(fileStore.currentPath)
  switch (type) {
    case 'canvas':
      return CanvasRenderer
    case 'image':
      return ImageRenderer
    default:
      return StructuredRenderer
  }
})

const handleToggleExpand = (node) => {
  if (node.metadata.is_directory) {
    if (expandedNodes.value[node.metadata.id]) {
      // Collapse
      delete expandedNodes.value[node.metadata.id]
      console.log('[VaultView] Collapsed node:', node.metadata.name)

      // Update persistent tree
      persistentTreeStore.collapseNode(fileStore.vaultId, node.metadata.id)
    } else {
      // Expand
      expandedNodes.value[node.metadata.id] = true
      console.log('[VaultView] Expanded node:', node.metadata.name)

      // Children are already loaded in the full tree, just mark as expanded
      persistentTreeStore.expandNode(fileStore.vaultId, node.metadata.id)
    }
  }
}

const handleQuickFindSelect = async (node) => {
  console.log('[VaultView] QuickFind selected:', node.metadata.name)

  const fileId = node.metadata.id
  const relativePath = node.metadata.path || node.metadata.name

  try {
    // Set current file ID
    currentFileId.value = fileId

    // Highlight the selected file in the tree
    selectedFileId.value = fileId

    fileStore.setCurrentPath(relativePath)

    // Add to navigation history
    addToNavigationHistory(fileId, relativePath)

    // Expand folders
    console.log('[VaultView] Expanding tree to reveal file:', relativePath)
    const expandedNodeIds = persistentTreeStore.navigateToPath(fileStore.vaultId, relativePath)

    // Update expandedNodes
    expandedNodeIds.forEach((nodeId) => {
      expandedNodes.value[nodeId] = true
    })

    // Scroll to the file
    nextTick(() => {
      setTimeout(() => {
        const element = document.querySelector(`[data-node-id="${fileId}"]`)
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'center' })
        }
      }, 100)
    })

    // Fetch content
    const type = getFileType(relativePath)

    if (type === 'canvas') {
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, fileId)
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path)
      }
    } else if (type === 'image') {
      fileStore.selectedFileContent = 'asset'
    } else {
      // For StructuredRenderer, set placeholder
      fileStore.selectedFileContent = 'loading'
    }
  } catch (error) {
    console.error('[VaultView] Error selecting file from QuickFind:', error)
  }
}

const handleFileSelected = async (node) => {
  if (!node.metadata.is_directory) {
    // Use path if available, otherwise use filename
    const filePath = node.metadata.path || node.metadata.name
    fileStore.setCurrentPath(filePath)

    // Add to navigation history
    addToNavigationHistory(node.metadata.id, filePath)

    // Set current file ID - this should trigger StructuredRenderer watcher
    currentFileId.value = node.metadata.id

    // Highlight the selected file in the tree
    selectedFileId.value = node.metadata.id

    // Determine file type
    const type = getFileType(filePath)

    if (type === 'canvas') {
      // Canvas files need content fetched
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, node.metadata.id)

      // Update the path from server response if available (contains relative path)
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path)
        // Update history with correct path
        if (navigationHistory.value.length > 0) {
          navigationHistory.value[navigationIndex.value].filePath = fileData.path
        }
      }
    } else if (type === 'image') {
      // For images, we don't need to fetch content via fileStore
      // But we need to set selectedFileContent to truthy to show the renderer
      fileStore.selectedFileContent = 'asset'
    } else {
      // For StructuredRenderer (markdown), the watcher in the component will handle fetching

      // Just set a placeholder to ensure the component is shown
      fileStore.selectedFileContent = 'loading'
    }
  }
}

/**
 * Toggle search panel
 */
const toggleSearch = () => {
  showSearch.value = !showSearch.value
  if (!showSearch.value) {
    // Clear search when closing
    searchStore.clearSearch()
  }
}

/**
 * Close search panel and return to file browser
 */
const closeSearch = () => {
  showSearch.value = false
  searchStore.clearSearch()
}

/**
 * Handle when a search is executed
 */
const handleSearchExecuted = () => {
  console.log('[VaultView] Search executed, results:', searchStore.total)
}

/**
 * Handle when a search result is selected
 */
const handleSearchResultSelected = async (result) => {
  console.log('[VaultView] Search result selected:', result)

  // The result.id is the file ID from database
  // The result.fields.path is the relative path
  try {
    const fileId = result.id
    const relativePath = result.fields?.path || ''

    // Set current file ID
    currentFileId.value = fileId

    // Highlight the selected file in the tree
    selectedFileId.value = fileId

    // Set the current path for display (use relative path from fields)
    if (relativePath) {
      fileStore.setCurrentPath(relativePath)

      // Expand folders to reveal the file in the tree
      console.log('[VaultView] Expanding tree to reveal search result:', relativePath)
      const expandedNodeIds = persistentTreeStore.navigateToPath(fileStore.vaultId, relativePath)

      // Update expandedNodes to trigger UI update
      expandedNodeIds.forEach((nodeId) => {
        expandedNodes.value[nodeId] = true
      })

      // Scroll to the file in the tree after a short delay
      nextTick(() => {
        setTimeout(() => {
          const element = document.querySelector(`[data-node-id="${fileId}"]`)
          if (element) {
            element.scrollIntoView({ behavior: 'smooth', block: 'center' })
          }
        }, 100)
      })
    }

    // Fetch file content using file ID
    const type = getFileType(relativePath)

    if (type === 'canvas') {
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, fileId)

      // Update the path from server response if available
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path)
      }
    } else if (type === 'image') {
      fileStore.selectedFileContent = 'asset'
    } else {
      // For StructuredRenderer, just set a placeholder
      fileStore.selectedFileContent = 'loading'
    }

    // Keep search panel open - don't call closeSearch()
  } catch (error) {
    console.error('[VaultView] Failed to load search result:', error)
  }
}

/**
 * Handle wikilink navigation from markdown renderer
 * Expands tree to file, highlights it, and loads content
 */
const handleWikilinkNavigation = async (event) => {
  try {
    const { fileId, path, exists } = event

    if (!exists || !fileId || !path) {
      return
    }

    // Update breadcrumb/path
    fileStore.setCurrentPath(path)

    // Add to navigation history
    addToNavigationHistory(fileId, path)

    // Update current file ID - this triggers StructuredRenderer watcher to fetch content
    currentFileId.value = fileId

    // Highlight the selected file in the tree
    selectedFileId.value = fileId

    // Expand folders to reveal the file in the tree
    console.log('[VaultView] Expanding tree to reveal file:', path)
    const expandedNodeIds = persistentTreeStore.navigateToPath(fileStore.vaultId, path)

    // Update expandedNodes to trigger UI update
    expandedNodeIds.forEach((nodeId) => {
      expandedNodes.value[nodeId] = true
    })

    // Scroll to the file in the tree after a short delay
    nextTick(() => {
      setTimeout(() => {
        const element = document.querySelector(`[data-node-id="${fileId}"]`)
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'center' })
        }
      }, 100)
    })

    // Check if this is a canvas file
    const type = getFileType(path)

    if (type === 'canvas') {
      // Canvas files need content fetched
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, fileId)
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path)
      }
    } else if (type === 'image') {
      fileStore.selectedFileContent = 'asset'
    } else {
      // For StructuredRenderer, set placeholder to show component
      fileStore.selectedFileContent = 'loading'
    }
  } catch (error) {
    console.error('[VaultView] Failed to navigate to wikilink:', error)
  }
}

/**
 * Handle create button click on folder
 */
const handleCreateClick = (node) => {
  createParentId.value = node.metadata.id
  showCreateDialog.value = true
}

/**
 * Close create dialog
 */
const closeCreateDialog = () => {
  showCreateDialog.value = false
  createParentId.value = null
}

/**
 * Handle file/folder created
 */
const handleFileCreated = async (result) => {
  console.log('[VaultView] File/folder created:', result)

  // Refresh the entire tree to show the new item
  await fileStore.fetchTree(fileStore.vaultId)
  persistentTreeStore.setTree(fileStore.vaultId, fileStore.treeData)
}

const currentFileName = computed(() => {
  if (!fileStore.currentPath) return 'No file selected'
  const lastSlash = fileStore.currentPath.lastIndexOf('/')
  return lastSlash === -1 ? fileStore.currentPath : fileStore.currentPath.substring(lastSlash + 1)
})

const breadcrumbParts = computed(() => {
  if (!fileStore.currentPath) return []
  return fileStore.currentPath.split('/')
})

// Navigation history computed properties
const canGoBack = computed(() => navigationIndex.value > 0)
const canGoForward = computed(() => navigationIndex.value < navigationHistory.value.length - 1)

/**
 * Add file to navigation history
 */
const addToNavigationHistory = (fileId, filePath) => {
  if (!fileId || !filePath || isNavigatingHistory.value) {
    return
  }

  // Don't add if it's the same as the current item
  const currentItem = navigationHistory.value[navigationIndex.value]
  if (currentItem && currentItem.fileId === fileId) {
    return
  }

  // Remove any forward history when navigating to a new file
  navigationHistory.value = navigationHistory.value.slice(0, navigationIndex.value + 1)

  // Add new item
  navigationHistory.value.push({
    fileId,
    filePath,
    timestamp: Date.now(),
  })

  navigationIndex.value = navigationHistory.value.length - 1
}

/**
 * Navigate back in history
 */
const goBack = async () => {
  if (!canGoBack.value) return

  isNavigatingHistory.value = true

  try {
    navigationIndex.value--
    const item = navigationHistory.value[navigationIndex.value]

    // Update breadcrumb
    fileStore.setCurrentPath(item.filePath)

    // Update current file ID - triggers fetch
    currentFileId.value = item.fileId

    // Highlight the file in the tree
    selectedFileId.value = item.fileId

    // Expand folders to reveal the file
    const expandedNodeIds = persistentTreeStore.navigateToPath(fileStore.vaultId, item.filePath)
    expandedNodeIds.forEach((nodeId) => {
      expandedNodes.value[nodeId] = true
    })

    // Scroll to the file
    nextTick(() => {
      setTimeout(() => {
        const element = document.querySelector(`[data-node-id="${item.fileId}"]`)
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'center' })
        }
      }, 100)
    })

    // Check if this is a canvas file
    const type = getFileType(item.filePath)

    if (type === 'canvas') {
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, item.fileId)
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path)
      }
    } else if (type === 'image') {
      fileStore.selectedFileContent = 'asset'
    } else {
      // For StructuredRenderer, set placeholder
      fileStore.selectedFileContent = 'loading'
    }
  } finally {
    isNavigatingHistory.value = false
  }
}

/**
 * Navigate forward in history
 */
const goForward = async () => {
  if (!canGoForward.value) return

  isNavigatingHistory.value = true

  try {
    navigationIndex.value++
    const item = navigationHistory.value[navigationIndex.value]

    // Update breadcrumb
    fileStore.setCurrentPath(item.filePath)

    // Update current file ID - triggers fetch
    currentFileId.value = item.fileId

    // Highlight the file in the tree
    selectedFileId.value = item.fileId

    // Expand folders to reveal the file
    const expandedNodeIds = persistentTreeStore.navigateToPath(fileStore.vaultId, item.filePath)
    expandedNodeIds.forEach((nodeId) => {
      expandedNodes.value[nodeId] = true
    })

    // Scroll to the file
    nextTick(() => {
      setTimeout(() => {
        const element = document.querySelector(`[data-node-id="${item.fileId}"]`)
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'center' })
        }
      }, 100)
    })

    // Check if this is a canvas file
    const type = getFileType(item.filePath)

    if (type === 'canvas') {
      const fileData = await fileStore.fetchFileContent(fileStore.vaultId, item.fileId)
      if (fileData && fileData.path) {
        fileStore.setCurrentPath(fileData.path)
      }
    } else if (type === 'image') {
      fileStore.selectedFileContent = 'asset'
    } else {
      // For StructuredRenderer, set placeholder
      fileStore.selectedFileContent = 'loading'
    }
  } finally {
    isNavigatingHistory.value = false
  }
}

// SSE event handlers
const sseCallbacks = {
  onConnected: (data) => {
    console.log('[VaultView] SSE connected:', data)
    connected.value = true
    error.value = null
  },

  onBulkProcess: async (data) => {
    console.log('[VaultView] Bulk process received:', data)

    const changes = data.changes || []
    console.log(`[VaultView] Processing ${changes.length} file changes`)

    // Refetch the tree from server to get latest state
    // This is simpler and more reliable than trying to modify nested objects
    await fileStore.fetchTree(fileStore.vaultId)

    // Update persistent tree (expanded state is preserved)
    persistentTreeStore.setTree(fileStore.vaultId, fileStore.treeData)

    // If currently selected file was deleted, clear it
    for (const change of changes) {
      if (fileStore.currentPath === change.path && change.action === 'delete') {
        fileStore.selectedFileContent = null
        currentFileId.value = null
        console.log('[VaultView] Cleared selected file (deleted)')
        break
      }
    }

    console.log('[VaultView] Tree updated with latest data')
  },

  onPing: (data) => {
    console.debug('[VaultView] Ping received, pending:', data.pending_count)
    // Ping doesn't require any action, just logs the pending count
  },

  onRefresh: async (data) => {
    console.log('[VaultView] Refresh requested by server')

    // Full tree refresh - collapse all folders and refetch
    expandedNodes.value = {}

    await fileStore.fetchTree(fileStore.vaultId)
    persistentTreeStore.setTree(fileStore.vaultId, fileStore.treeData)

    console.log('[VaultView] Tree fully refreshed and collapsed')
  },

  onError: (err) => {
    console.error('[VaultView] SSE error:', err)
    error.value = err.message || 'SSE connection error'
    connected.value = false
  },
}

// Initialize SSE connection (vaultId will be passed when calling connect())
const sseHooks = useSSE(sseCallbacks)
const sseConnect = sseHooks.connect
const sseDisconnect = sseHooks.disconnect
const sseReconnect = sseHooks.reconnect

// Watch for changes in the route params, specifically the 'id' for the vault
watch(
  () => route.params.id,
  async (newId, oldId) => {
    if (newId) {
      // Disconnect old SSE connection if vault changes
      if (oldId && oldId !== newId) {
        sseDisconnect()
      }

      fileStore.setVaultId(newId)
      vaultName.value = `Vault ${newId}`

      // Try to restore tree from persistent storage first
      const savedTree = persistentTreeStore.getTree(newId)
      if (savedTree && savedTree.length > 0) {
        console.log('[VaultView] Restoring tree from persistent storage')
        fileStore.treeData = savedTree

        // Restore expanded state
        const expandedIds = persistentTreeStore.getExpandedNodeIds(newId)
        expandedIds.forEach((id) => {
          expandedNodes.value[id] = true
        })
      } else {
        // No saved tree, fetch full tree from server
        console.log('[VaultView] No saved tree, fetching full tree from server')
        await fileStore.fetchTree(newId)

        // Initialize persistent tree with fetched data
        persistentTreeStore.initializeTree(newId, fileStore.treeData)
      }

      // Reset expanded nodes when vault changes if no saved state
      if (!savedTree || savedTree.length === 0) {
        expandedNodes.value = {}
      }

      fileStore.selectedFileContent = null // Clear selected file content
      currentFileId.value = null // Clear selected file ID when vault changes

      // Connect to SSE for the new vault
      sseConnect(newId)
    }
  },
  { immediate: true }
) // Immediate: true to run the watcher on initial component mount

onMounted(() => {
  // Restore persistent tree state from localStorage on mount
  persistentTreeStore.restoreFromStorage()

  // Initial fetch if not already done by watcher (e.g., direct navigation)
  if (!fileStore.vaultId && route.params.id) {
    fileStore.setVaultId(route.params.id)
    vaultName.value = `Vault ${route.params.id}`

    // Check for saved tree
    const savedTree = persistentTreeStore.getTree(route.params.id)
    if (savedTree && savedTree.length > 0) {
      fileStore.treeData = savedTree
    } else {
      fileStore.fetchTree(route.params.id)
      persistentTreeStore.initializeTree(route.params.id, fileStore.treeData)
    }

    sseConnect(route.params.id)
  }
})
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
  transition:
    background-color 0.2s,
    color 0.2s;
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

.main-content {
  flex: 1;
  min-height: 0;
  padding: clamp(0.75rem, 2vw, 1.25rem);
  overflow: hidden;
  background-color: var(--background-color);
  color: var(--text-color);
  display: flex;
  flex-direction: column;
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
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
}

.file-header-section {
  flex-shrink: 0;
  padding: clamp(0.5rem, 1.5vw, 0.875rem) clamp(0.75rem, 2vw, 1.25rem) clamp(0.375rem, 1vw, 0.5rem);
  border-bottom: 1px solid var(--border-color);
  background-color: var(--background-color);
}

.navigation-bar {
  display: flex;
  align-items: center;
  gap: clamp(0.5rem, 1.5vw, 0.75rem);
  margin-bottom: clamp(0.25rem, 0.8vw, 0.375rem);
}

.nav-buttons {
  display: flex;
  gap: 0.25rem;
}

.nav-button {
  background: transparent;
  border: 1px solid rgba(128, 128, 128, 0.2);
  color: var(--text-color);
  cursor: pointer;
  padding: clamp(0.25rem, 0.8vw, 0.375rem) clamp(0.375rem, 1vw, 0.5rem);
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  font-size: clamp(0.8rem, 1.8vw, 0.875rem);
}

.nav-button:hover:not(:disabled) {
  background-color: rgba(var(--primary-color-rgb, 59, 130, 246), 0.1);
  border-color: rgba(var(--primary-color-rgb, 59, 130, 246), 0.3);
  color: var(--primary-color);
}

.nav-button:disabled {
  opacity: 0.25;
  cursor: not-allowed;
}

.breadcrumb {
  font-size: clamp(0.7rem, 1.6vw, 0.75rem);
  color: var(--text-color-secondary);
  display: flex;
  align-items: center;
  gap: 0;
  flex: 1;
  opacity: 0.7;
  font-weight: 400;
}

.breadcrumb-item {
  display: flex;
  align-items: center;
  gap: clamp(0.125rem, 0.5vw, 0.1875rem);
}

.breadcrumb-separator {
  color: var(--text-color-secondary);
  opacity: 0.6;
  font-size: 0.9em;
  padding: 0 clamp(0.25em, 0.8vw, 0.35em);
  font-weight: 500;
}

.breadcrumb-separator::before {
  content: '›';
  display: inline-block;
}

.title-stats-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: clamp(1rem, 3vw, 1.5rem);
  flex-wrap: wrap;
}

.file-title {
  font-size: clamp(1.25rem, 3vw, 1.5rem);
  font-weight: 600;
  color: var(--text-color);
  margin: clamp(0.25rem, 1vw, 0.375rem) 0 clamp(0.375rem, 1.2vw, 0.5rem) 0;
  padding: 0;
  line-height: 1.3;
  flex: 0 1 auto;
}

.file-stats {
  display: flex;
  align-items: center;
  gap: clamp(0.375rem, 1vw, 0.5rem);
  flex-shrink: 0;
}

.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: clamp(0.25em, 0.6vw, 0.3em);
  padding: clamp(0.25em, 0.8vw, 0.35em) clamp(0.5em, 1.2vw, 0.625em);
  background-color: rgba(59, 130, 246, 0.08);
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 6px;
  color: var(--text-color);
  font-size: clamp(0.7rem, 1.6vw, 0.75rem);
  font-weight: 500;
  white-space: nowrap;
  transition: all 0.2s ease;
}

.stat-chip:hover {
  background-color: rgba(59, 130, 246, 0.12);
  border-color: rgba(59, 130, 246, 0.3);
}

.stat-chip i {
  color: #3b82f6;
  font-size: 0.9em;
  opacity: 0.8;
}
</style>
