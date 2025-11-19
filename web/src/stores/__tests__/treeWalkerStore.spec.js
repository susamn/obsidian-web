import { describe, it, expect, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useTreeWalkerStore } from '../treeWalkerStore';

describe('TreeWalkerStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('should initialize with empty state', () => {
    const store = useTreeWalkerStore();
    expect(store.walkedPaths.size).toBe(0);
    expect(store.nodeRegistry.size).toBe(0);
  });

  it('should mark a path as walked', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';
    const path = 'folder1/subfolder';

    store.markPathWalked(vaultId, path);

    expect(store.isPathWalked(vaultId, path)).toBe(true);
  });

  it('should mark root path as walked', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';

    store.markRootWalked(vaultId);

    expect(store.isPathWalked(vaultId, '')).toBe(true);
  });

  it('should check if parent path is walked', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';
    const parentPath = 'folder1';
    const childPath = 'folder1/subfolder';

    // Parent not walked initially
    expect(store.isParentWalked(vaultId, childPath)).toBe(false);

    // Mark parent as walked
    store.markPathWalked(vaultId, parentPath);

    // Now parent should be walked
    expect(store.isParentWalked(vaultId, childPath)).toBe(true);
  });

  it('should always return true for root parent check', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';
    const rootChildPath = 'file.md';

    // Root parent should always be considered walked
    expect(store.isParentWalked(vaultId, rootChildPath)).toBe(true);
  });

  it('should register and retrieve nodes by ID', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';
    const node = {
      metadata: {
        id: 'node-1',
        name: 'folder',
        path: 'folder1',
        is_directory: true,
      },
      children: [],
    };

    store.registerNode(vaultId, node);

    const retrieved = store.getNodeById(vaultId, 'node-1');
    expect(retrieved).toEqual(node);
  });

  it('should register multiple nodes recursively', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';
    const nodes = [
      {
        metadata: {
          id: 'node-1',
          name: 'folder1',
          path: 'folder1',
          is_directory: true,
        },
        children: [
          {
            metadata: {
              id: 'node-2',
              name: 'subfolder',
              path: 'folder1/subfolder',
              is_directory: true,
            },
            children: [],
          },
        ],
      },
    ];

    store.registerNodes(vaultId, nodes);

    expect(store.getNodeById(vaultId, 'node-1')).toBeDefined();
    expect(store.getNodeById(vaultId, 'node-2')).toBeDefined();
  });

  it('should get all walked paths for a vault', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';

    store.markPathWalked(vaultId, 'folder1');
    store.markPathWalked(vaultId, 'folder2');
    store.markPathWalked(vaultId, 'folder1/subfolder');

    const walked = store.getWalkedPaths(vaultId);

    expect(walked).toContain('folder1');
    expect(walked).toContain('folder2');
    expect(walked).toContain('folder1/subfolder');
    expect(walked.length).toBe(3);
  });

  it('should return empty array for non-existent vault', () => {
    const store = useTreeWalkerStore();
    const walked = store.getWalkedPaths('non-existent');
    expect(walked).toEqual([]);
  });

  it('should clear all paths for a specific vault', () => {
    const store = useTreeWalkerStore();
    const vault1 = 'vault-1';
    const vault2 = 'vault-2';

    store.markPathWalked(vault1, 'folder1');
    store.markPathWalked(vault2, 'folder1');

    store.clearVault(vault1);

    expect(store.getWalkedPaths(vault1)).toEqual([]);
    expect(store.getWalkedPaths(vault2)).toContain('folder1');
  });

  it('should clear all state on reset', () => {
    const store = useTreeWalkerStore();

    store.markPathWalked('vault-1', 'folder1');
    store.registerNode('vault-1', {
      metadata: { id: 'node-1', name: 'test' },
      children: [],
    });

    store.reset();

    expect(store.walkedPaths.size).toBe(0);
    expect(store.nodeRegistry.size).toBe(0);
  });

  it('should handle multiple vaults independently', () => {
    const store = useTreeWalkerStore();
    const vault1 = 'vault-1';
    const vault2 = 'vault-2';

    store.markPathWalked(vault1, 'folder-v1');
    store.markPathWalked(vault2, 'folder-v2');

    expect(store.isPathWalked(vault1, 'folder-v1')).toBe(true);
    expect(store.isPathWalked(vault1, 'folder-v2')).toBe(false);
    expect(store.isPathWalked(vault2, 'folder-v2')).toBe(true);
    expect(store.isPathWalked(vault2, 'folder-v1')).toBe(false);
  });

  it('should handle deeply nested paths', () => {
    const store = useTreeWalkerStore();
    const vaultId = 'vault-1';
    const deepPath = 'a/b/c/d/e/f/g';

    store.markPathWalked(vaultId, deepPath);

    expect(store.isPathWalked(vaultId, deepPath)).toBe(true);
    expect(store.isParentWalked(vaultId, `${deepPath}/file.md`)).toBe(true);
  });
});
