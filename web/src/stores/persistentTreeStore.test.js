import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { usePersistentTreeStore } from './persistentTreeStore';

describe('PersistentTreeStore', () => {
  let store;
  const vaultId = 'test-vault';

  // Mock localStorage
  const localStorageMock = (() => {
    let store = {};
    return {
      getItem: (key) => store[key] || null,
      setItem: (key, value) => {
        store[key] = value.toString();
      },
      clear: () => {
        store = {};
      },
      removeItem: (key) => {
        delete store[key];
      },
    };
  })();

  beforeEach(() => {
    setActivePinia(createPinia());
    store = usePersistentTreeStore();

    // Setup localStorage mock
    global.localStorage = localStorageMock;
    localStorageMock.clear();
  });

  afterEach(() => {
    localStorageMock.clear();
  });

  describe('initializeTree', () => {
    it('should initialize tree with root children', () => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
        {
          metadata: { id: '2', name: 'file1.md', path: 'file1.md', is_directory: false },
        },
      ];

      store.initializeTree(vaultId, rootChildren);

      expect(store.getTree(vaultId)).toEqual(rootChildren);
      expect(store.lastUpdated.get(vaultId)).toBeDefined();
    });

    it('should build indices for quick lookup', () => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
      ];

      store.initializeTree(vaultId, rootChildren);

      const nodeById = store.findNodeById(vaultId, '1');
      const nodeByPath = store.findNodeByPath(vaultId, 'folder1');

      expect(nodeById).toEqual(rootChildren[0]);
      expect(nodeByPath).toEqual(rootChildren[0]);
    });
  });

  describe('expandNode and collapseNode', () => {
    beforeEach(() => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [
            {
              metadata: { id: '1-1', name: 'subfolder', path: 'folder1/subfolder', is_directory: true },
              children: [],
            },
          ],
        },
      ];
      store.initializeTree(vaultId, rootChildren);
    });

    it('should mark a node as expanded', () => {
      store.expandNode(vaultId, '1');
      expect(store.isExpanded(vaultId, '1')).toBe(true);
    });

    it('should preserve existing children when expanding', () => {
      // Children are already loaded in the full tree
      const node = store.findNodeById(vaultId, '1');
      const originalChildren = node.children;

      store.expandNode(vaultId, '1');

      // Children should remain the same (already loaded)
      expect(node.children).toEqual(originalChildren);
      expect(store.isExpanded(vaultId, '1')).toBe(true);
    });

    it('should collapse a node', () => {
      store.expandNode(vaultId, '1');
      expect(store.isExpanded(vaultId, '1')).toBe(true);

      store.collapseNode(vaultId, '1');
      expect(store.isExpanded(vaultId, '1')).toBe(false);
    });
  });

  describe('updateNodeChildren', () => {
    beforeEach(() => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
      ];
      store.initializeTree(vaultId, rootChildren);
    });

    it('should update children by node ID', () => {
      const newChildren = [
        {
          metadata: { id: '1-1', name: 'file.md', path: 'folder1/file.md', is_directory: false },
        },
      ];

      store.updateNodeChildren(vaultId, '1', newChildren);

      const node = store.findNodeById(vaultId, '1');
      expect(node.children).toEqual(newChildren);
    });

    it('should update children by path', () => {
      const newChildren = [
        {
          metadata: { id: '1-1', name: 'file.md', path: 'folder1/file.md', is_directory: false },
        },
      ];

      store.updateNodeChildren(vaultId, 'folder1', newChildren);

      const node = store.findNodeByPath(vaultId, 'folder1');
      expect(node.children).toEqual(newChildren);
    });

    it('should rebuild indices after updating children', () => {
      const newChildren = [
        {
          metadata: { id: '1-1', name: 'file.md', path: 'folder1/file.md', is_directory: false },
        },
      ];

      store.updateNodeChildren(vaultId, '1', newChildren);

      // Should be able to find the new child
      const child = store.findNodeById(vaultId, '1-1');
      expect(child).toBeDefined();
      expect(child.metadata.name).toBe('file.md');
    });
  });

  describe('removeNode', () => {
    beforeEach(() => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [
            {
              metadata: { id: '1-1', name: 'file.md', path: 'folder1/file.md', is_directory: false },
            },
          ],
        },
      ];
      store.initializeTree(vaultId, rootChildren);
    });

    it('should remove a node by ID', () => {
      store.removeNode(vaultId, '1-1');

      const node = store.findNodeById(vaultId, '1-1');
      expect(node).toBeNull();

      const parent = store.findNodeById(vaultId, '1');
      expect(parent.children).toHaveLength(0);
    });

    it('should remove a node by path', () => {
      store.removeNode(vaultId, 'folder1/file.md');

      const node = store.findNodeByPath(vaultId, 'folder1/file.md');
      expect(node).toBeNull();
    });

    it('should rebuild indices after removal', () => {
      store.removeNode(vaultId, '1-1');

      // Indices should be updated
      const pathIdx = store.pathIndex.get(vaultId);
      expect(pathIdx.has('folder1/file.md')).toBe(false);
    });
  });

  describe('navigateToPath', () => {
    it('should expand all parent folders for a nested path with full tree', () => {
      // Initialize with full tree (all nodes have children pre-loaded)
      const tree = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [
            {
              metadata: {
                id: '1-1',
                name: 'subfolder',
                path: 'folder1/subfolder',
                is_directory: true,
              },
              children: [
                {
                  metadata: {
                    id: '1-1-1',
                    name: 'file.md',
                    path: 'folder1/subfolder/file.md',
                    is_directory: false,
                  },
                },
              ],
            },
          ],
        },
      ];
      store.initializeTree(vaultId, tree);

      const expandedNodes = store.navigateToPath(vaultId, 'folder1/subfolder/file.md');

      // Should have expanded folder1 and folder1/subfolder
      expect(expandedNodes).toHaveLength(2);
      expect(store.isExpanded(vaultId, '1')).toBe(true);
      expect(store.isExpanded(vaultId, '1-1')).toBe(true);
    });

    it('should not re-expand already expanded nodes', () => {
      const tree = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [
            {
              metadata: {
                id: '1-1',
                name: 'file.md',
                path: 'folder1/file.md',
                is_directory: false,
              },
            },
          ],
        },
      ];
      store.initializeTree(vaultId, tree);
      store.expandNode(vaultId, '1');

      const expandedNodes = store.navigateToPath(vaultId, 'folder1/file.md');

      // Should not have expanded folder1 again
      expect(expandedNodes).toHaveLength(0);
      expect(store.isExpanded(vaultId, '1')).toBe(true);
    });
  });

  describe('localStorage persistence', () => {
    it('should persist tree to localStorage', () => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
      ];

      store.initializeTree(vaultId, rootChildren);

      const stored = JSON.parse(localStorage.getItem('obsidian-web-persistent-trees'));
      expect(stored[vaultId]).toBeDefined();
      expect(stored[vaultId].tree).toEqual(rootChildren);
    });

    it('should persist expanded state', () => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
      ];

      store.initializeTree(vaultId, rootChildren);
      store.expandNode(vaultId, '1');

      const stored = JSON.parse(localStorage.getItem('obsidian-web-persistent-trees'));
      expect(stored[vaultId].expandedIds).toContain('1');
    });

    it('should restore tree from localStorage', () => {
      const data = {
        [vaultId]: {
          tree: [
            {
              metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
              children: [],
            },
          ],
          expandedIds: ['1'],
          lastUpdated: Date.now(),
        },
      };

      localStorage.setItem('obsidian-web-persistent-trees', JSON.stringify(data));

      const newStore = usePersistentTreeStore();
      newStore.restoreFromStorage();

      expect(newStore.getTree(vaultId)).toEqual(data[vaultId].tree);
      expect(newStore.isExpanded(vaultId, '1')).toBe(true);
    });
  });

  describe('getters', () => {
    beforeEach(() => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
      ];
      store.initializeTree(vaultId, rootChildren);
    });

    it('should get parent path correctly', () => {
      expect(store.getParentPath('folder1/subfolder/file.md')).toBe('folder1/subfolder');
      expect(store.getParentPath('folder1/file.md')).toBe('folder1');
      expect(store.getParentPath('file.md')).toBe('');
      expect(store.getParentPath('')).toBe('');
    });

    it('should get path segments correctly', () => {
      expect(store.getPathSegments('folder1/subfolder/file.md')).toEqual([
        'folder1',
        'subfolder',
        'file.md',
      ]);
      expect(store.getPathSegments('file.md')).toEqual(['file.md']);
      expect(store.getPathSegments('')).toEqual([]);
    });

    it('should get expanded node IDs', () => {
      store.expandNode(vaultId, '1');
      const expandedIds = store.getExpandedNodeIds(vaultId);
      expect(expandedIds).toContain('1');
    });
  });

  describe('clearVault', () => {
    it('should clear all data for a vault', () => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
      ];
      store.initializeTree(vaultId, rootChildren);
      store.expandNode(vaultId, '1');

      store.clearVault(vaultId);

      expect(store.getTree(vaultId)).toEqual([]);
      expect(store.isExpanded(vaultId, '1')).toBe(false);
      expect(store.findNodeById(vaultId, '1')).toBeNull();
    });

    it('should remove vault from localStorage', () => {
      const rootChildren = [
        {
          metadata: { id: '1', name: 'folder1', path: 'folder1', is_directory: true },
          children: [],
        },
      ];
      store.initializeTree(vaultId, rootChildren);

      store.clearVault(vaultId);

      const stored = JSON.parse(localStorage.getItem('obsidian-web-persistent-trees'));
      expect(stored[vaultId]).toBeUndefined();
    });
  });
});
