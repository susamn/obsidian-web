import { defineStore } from 'pinia'

/**
 * PersistentTreeStore manages the full file tree structure across page refreshes
 * - Stores the complete recursive tree with ALL files and folders
 * - Persists to localStorage
 * - Provides navigation methods for deep-linking to files
 * - All nodes are available immediately, no lazy loading needed
 *
 * SECURITY NOTE: Paths stored here are READ-ONLY and used for UI navigation.
 * All API calls MUST use IDs only, never paths.
 */
export const usePersistentTreeStore = defineStore('persistentTree', {
  state: () => ({
    // Map of vault ID -> tree root nodes
    trees: new Map(),

    // Map of vault ID -> Set of expanded node IDs
    expandedNodeIds: new Map(),

    // Map of vault ID -> Map of path -> node (for quick lookup)
    pathIndex: new Map(),

    // Map of vault ID -> Map of ID -> node (for quick lookup)
    idIndex: new Map(),

    // Last update timestamp for each vault
    lastUpdated: new Map(),
  }),

  getters: {
    /**
     * Get the tree for a specific vault
     */
    getTree: (state) => (vaultId) => {
      return state.trees.get(vaultId) || []
    },

    /**
     * Check if a node is expanded
     */
    isExpanded: (state) => (vaultId, nodeId) => {
      const expanded = state.expandedNodeIds.get(vaultId)
      return expanded ? expanded.has(nodeId) : false
    },

    /**
     * Get all expanded node IDs for a vault
     */
    getExpandedNodeIds: (state) => (vaultId) => {
      const expanded = state.expandedNodeIds.get(vaultId)
      return expanded ? Array.from(expanded) : []
    },

    /**
     * Find a node by path (read-only, for UI navigation)
     */
    findNodeByPath: (state) => (vaultId, path) => {
      const index = state.pathIndex.get(vaultId)
      return index ? index.get(path) || null : null
    },

    /**
     * Find a node by ID
     */
    findNodeById: (state) => (vaultId, nodeId) => {
      const index = state.idIndex.get(vaultId)
      return index ? index.get(nodeId) || null : null
    },

    /**
     * Get parent path from a file path
     */
    getParentPath: () => (path) => {
      if (!path || path === '') return ''
      const lastSlash = path.lastIndexOf('/')
      return lastSlash === -1 ? '' : path.substring(0, lastSlash)
    },

    /**
     * Split a path into segments for navigation
     */
    getPathSegments: () => (path) => {
      if (!path || path === '') return []
      return path.split('/').filter((seg) => seg.length > 0)
    },
  },

  actions: {
    /**
     * Initialize tree for a vault with root children
     */
    initializeTree(vaultId, rootChildren) {
      console.log(`[PersistentTree] Initializing tree for vault: ${vaultId}`)
      this.trees.set(vaultId, rootChildren || [])
      this.expandedNodeIds.set(vaultId, new Set())
      this.pathIndex.set(vaultId, new Map())
      this.idIndex.set(vaultId, new Map())
      this.lastUpdated.set(vaultId, Date.now())

      // Build indices
      this._buildIndices(vaultId, rootChildren || [])

      // Persist to storage
      this._persistToStorage()
    },

    /**
     * Set tree nodes for a vault (replaces existing)
     */
    setTree(vaultId, nodes) {
      console.log(`[PersistentTree] Setting tree for vault: ${vaultId}, nodes: ${nodes?.length}`)
      this.trees.set(vaultId, nodes || [])
      this.lastUpdated.set(vaultId, Date.now())

      // Rebuild indices
      this._rebuildIndices(vaultId)

      // Persist to storage
      this._persistToStorage()
    },

    /**
     * Expand a node (children are already present in full tree)
     */
    expandNode(vaultId, nodeId) {
      console.log(`[PersistentTree] Expanding node: ${vaultId}/${nodeId}`)

      // Mark as expanded
      if (!this.expandedNodeIds.has(vaultId)) {
        this.expandedNodeIds.set(vaultId, new Set())
      }
      this.expandedNodeIds.get(vaultId).add(nodeId)

      this.lastUpdated.set(vaultId, Date.now())
      this._persistToStorage()
    },

    /**
     * Collapse a node
     */
    collapseNode(vaultId, nodeId) {
      console.log(`[PersistentTree] Collapsing node: ${vaultId}/${nodeId}`)

      const expanded = this.expandedNodeIds.get(vaultId)
      if (expanded) {
        expanded.delete(nodeId)
      }

      this.lastUpdated.set(vaultId, Date.now())
      this._persistToStorage()
    },

    /**
     * Update children for a specific node (by path or ID)
     */
    updateNodeChildren(vaultId, nodeIdOrPath, children) {
      console.log(`[PersistentTree] Updating children for: ${vaultId}/${nodeIdOrPath}`)

      // Try to find node by ID first, then by path
      let node = this.findNodeById(vaultId, nodeIdOrPath)
      if (!node) {
        node = this.findNodeByPath(vaultId, nodeIdOrPath)
      }

      if (node) {
        node.children = children
        // Rebuild indices to include updated children
        this._rebuildIndices(vaultId)
        this.lastUpdated.set(vaultId, Date.now())
        this._persistToStorage()
      } else {
        console.warn(`[PersistentTree] Node not found: ${nodeIdOrPath}`)
      }
    },

    /**
     * Remove a node from the tree (for file deletion)
     */
    removeNode(vaultId, pathOrId) {
      console.log(`[PersistentTree] Removing node: ${vaultId}/${pathOrId}`)

      const tree = this.trees.get(vaultId)
      if (!tree) return

      // Try to find and remove by path or ID
      const removed = this._removeNodeRecursive(tree, pathOrId)

      if (removed) {
        // Rebuild indices
        this._rebuildIndices(vaultId)
        this.lastUpdated.set(vaultId, Date.now())
        this._persistToStorage()
      }
    },

    /**
     * Navigate to a file by expanding all parent folders
     * Returns a list of node IDs that were expanded
     * Since the full tree is loaded, this just marks folders as expanded
     */
    navigateToPath(vaultId, path) {
      console.log(`[PersistentTree] Navigating to path: ${vaultId}/${path}`)

      const segments = this.getPathSegments(path)
      const expandedNodes = []
      let currentPath = ''

      // Expand each parent directory
      for (let i = 0; i < segments.length - 1; i++) {
        currentPath = currentPath ? `${currentPath}/${segments[i]}` : segments[i]

        // Find node for this path
        const node = this.findNodeByPath(vaultId, currentPath)
        if (!node) {
          console.warn(`[PersistentTree] Node not found for path: ${currentPath}`)
          continue
        }

        // Check if already expanded
        if (this.isExpanded(vaultId, node.metadata.id)) {
          console.log(`[PersistentTree] Already expanded: ${currentPath}`)
          continue
        }

        // Mark as expanded (children are already loaded in full tree)
        this.expandNode(vaultId, node.metadata.id)
        expandedNodes.push(node.metadata.id)
      }

      return expandedNodes
    },

    /**
     * Clear all data for a vault
     */
    clearVault(vaultId) {
      console.log(`[PersistentTree] Clearing vault: ${vaultId}`)
      this.trees.delete(vaultId)
      this.expandedNodeIds.delete(vaultId)
      this.pathIndex.delete(vaultId)
      this.idIndex.delete(vaultId)
      this.lastUpdated.delete(vaultId)
      this._persistToStorage()
    },

    /**
     * Restore state from localStorage
     */
    restoreFromStorage() {
      try {
        const stored = localStorage.getItem('obsidian-web-persistent-trees')
        if (!stored) return

        const data = JSON.parse(stored)

        // Restore each vault's data
        Object.entries(data).forEach(([vaultId, vaultData]) => {
          this.trees.set(vaultId, vaultData.tree || [])
          this.expandedNodeIds.set(vaultId, new Set(vaultData.expandedIds || []))
          this.lastUpdated.set(vaultId, vaultData.lastUpdated || Date.now())

          // Rebuild indices from tree
          this._rebuildIndices(vaultId)
        })

        console.log('[PersistentTree] Restored from localStorage')
      } catch (error) {
        console.error('[PersistentTree] Failed to restore from storage:', error)
      }
    },

    /**
     * Private: Persist state to localStorage
     */
    _persistToStorage() {
      try {
        const data = {}

        this.trees.forEach((tree, vaultId) => {
          data[vaultId] = {
            tree,
            expandedIds: Array.from(this.expandedNodeIds.get(vaultId) || []),
            lastUpdated: this.lastUpdated.get(vaultId) || Date.now(),
          }
        })

        localStorage.setItem('obsidian-web-persistent-trees', JSON.stringify(data))
      } catch (error) {
        console.error('[PersistentTree] Failed to persist to storage:', error)
      }
    },

    /**
     * Private: Build indices for quick lookup
     */
    _buildIndices(vaultId, nodes) {
      if (!this.pathIndex.has(vaultId)) {
        this.pathIndex.set(vaultId, new Map())
      }
      if (!this.idIndex.has(vaultId)) {
        this.idIndex.set(vaultId, new Map())
      }

      const pathIdx = this.pathIndex.get(vaultId)
      const idIdx = this.idIndex.get(vaultId)

      const indexNode = (node) => {
        if (node.metadata) {
          if (node.metadata.path !== undefined) {
            pathIdx.set(node.metadata.path, node)
          }
          if (node.metadata.id) {
            idIdx.set(node.metadata.id, node)
          }

          // Recursively index children
          if (node.children && node.children.length > 0) {
            node.children.forEach(indexNode)
          }
        }
      }

      nodes.forEach(indexNode)
    },

    /**
     * Private: Rebuild all indices for a vault
     */
    _rebuildIndices(vaultId) {
      this.pathIndex.set(vaultId, new Map())
      this.idIndex.set(vaultId, new Map())

      const tree = this.trees.get(vaultId)
      if (tree) {
        this._buildIndices(vaultId, tree)
      }
    },

    /**
     * Private: Recursively remove a node from tree
     */
    _removeNodeRecursive(nodes, pathOrId) {
      for (let i = 0; i < nodes.length; i++) {
        const node = nodes[i]

        // Check if this is the node to remove
        if (
          node.metadata.id === pathOrId ||
          node.metadata.path === pathOrId ||
          node.metadata.name === pathOrId
        ) {
          nodes.splice(i, 1)
          return true
        }

        // Recursively check children
        if (node.children && node.children.length > 0) {
          if (this._removeNodeRecursive(node.children, pathOrId)) {
            return true
          }
        }
      }

      return false
    },
  },
})
