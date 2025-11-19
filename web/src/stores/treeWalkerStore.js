import { defineStore } from 'pinia';

/**
 * TreeWalkerStore tracks which folders/paths in the file tree have been "walked" (expanded/loaded)
 * This is critical for smart SSE updates - we only update the UI for changes in folders that
 * have been explicitly expanded by the user
 */
export const useTreeWalkerStore = defineStore('treeWalker', {
  state: () => ({
    // Map of vault ID -> Set of walked paths
    // A path is "walked" when its children have been fetched and displayed
    walkedPaths: new Map(),

    // Map of vault ID -> Map of node ID -> node data
    // This helps us find nodes quickly by ID
    nodeRegistry: new Map(),
  }),

  getters: {
    /**
     * Check if a specific path has been walked in a vault
     */
    isPathWalked: (state) => (vaultId, path) => {
      const walked = state.walkedPaths.get(vaultId);
      if (!walked) return false;
      return walked.has(path);
    },

    /**
     * Check if a parent path has been walked
     * Returns true if the immediate parent has been expanded
     */
    isParentWalked: (state) => (vaultId, path) => {
      if (path === '') return true; // Root is always "walked"

      const lastSlash = path.lastIndexOf('/');
      // If no slash, parent is root
      if (lastSlash === -1) return true;

      const parentPath = path.substring(0, lastSlash);
      const walked = state.walkedPaths.get(vaultId);
      if (!walked) return false;

      // Check if parent is in walked set, or if it's root
      return parentPath === '' || walked.has(parentPath);
    },

    /**
     * Get all walked paths for a vault
     */
    getWalkedPaths: (state) => (vaultId) => {
      const walked = state.walkedPaths.get(vaultId);
      return walked ? Array.from(walked) : [];
    },

    /**
     * Check if a node with given ID exists in registry
     */
    getNodeById: (state) => (vaultId, nodeId) => {
      const registry = state.nodeRegistry.get(vaultId);
      if (!registry) return null;
      return registry.get(nodeId);
    },
  },

  actions: {
    /**
     * Mark a path as walked when it's expanded
     */
    markPathWalked(vaultId, path) {
      if (!this.walkedPaths.has(vaultId)) {
        this.walkedPaths.set(vaultId, new Set());
      }
      this.walkedPaths.get(vaultId).add(path);
      console.log(`[TreeWalker] Marked as walked: ${vaultId}/${path}`);
    },

    /**
     * Mark root path as walked when tree is initially loaded
     */
    markRootWalked(vaultId) {
      if (!this.walkedPaths.has(vaultId)) {
        this.walkedPaths.set(vaultId, new Set());
      }
      this.walkedPaths.get(vaultId).add('');
      console.log(`[TreeWalker] Marked root as walked: ${vaultId}`);
    },

    /**
     * Register a node in the registry for quick lookup
     */
    registerNode(vaultId, node) {
      if (!this.nodeRegistry.has(vaultId)) {
        this.nodeRegistry.set(vaultId, new Map());
      }
      const registry = this.nodeRegistry.get(vaultId);
      registry.set(node.metadata.id, node);
    },

    /**
     * Register multiple nodes (recursively)
     */
    registerNodes(vaultId, nodes) {
      if (!nodes || nodes.length === 0) return;

      nodes.forEach((node) => {
        this.registerNode(vaultId, node);
        // Recursively register children if they exist
        if (node.children && node.children.length > 0) {
          this.registerNodes(vaultId, node.children);
        }
      });
    },

    /**
     * Clear all walked paths for a vault (e.g., when vault changes)
     */
    clearVault(vaultId) {
      this.walkedPaths.delete(vaultId);
      this.nodeRegistry.delete(vaultId);
      console.log(`[TreeWalker] Cleared vault: ${vaultId}`);
    },

    /**
     * Reset all state
     */
    reset() {
      this.walkedPaths.clear();
      this.nodeRegistry.clear();
      console.log('[TreeWalker] State reset');
    },
  },
});
