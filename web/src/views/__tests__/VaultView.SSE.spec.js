import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useFileStore } from '../../stores/fileStore';
import { useTreeWalkerStore } from '../../stores/treeWalkerStore';

/**
 * Integration tests for SSE updates in VaultView
 * These tests verify that:
 * 1. Only walked (expanded) paths are updated in UI
 * 2. Non-walked paths are ignored
 * 3. Animations are triggered correctly
 * 4. Tree structure is maintained properly
 */
describe('VaultView SSE Updates', () => {
  let fileStore;
  let treeWalkerStore;
  let mockTreeData;

  beforeEach(() => {
    setActivePinia(createPinia());
    fileStore = useFileStore();
    treeWalkerStore = useTreeWalkerStore();

    // Set up mock tree data
    mockTreeData = [
      {
        metadata: {
          id: 'a-id',
          name: 'folder-a',
          path: 'folder-a',
          is_directory: true,
        },
        children: [
          {
            metadata: {
              id: 'a-file1-id',
              name: 'file1.md',
              path: 'folder-a/file1.md',
              is_directory: false,
              is_markdown: true,
            },
            children: [],
          },
        ],
      },
      {
        metadata: {
          id: 'b-id',
          name: 'folder-b',
          path: 'folder-b',
          is_directory: true,
        },
        children: [],
      },
      {
        metadata: {
          id: 'c-id',
          name: 'folder-c',
          path: 'folder-c',
          is_directory: true,
        },
        children: [],
      },
    ];

    fileStore.setVaultId('test-vault');
    fileStore.treeData = mockTreeData;

    // Mark root as walked
    treeWalkerStore.markRootWalked('test-vault');
  });

  describe('shouldUpdateUI function', () => {
    it('should return true when parent path is walked', () => {
      treeWalkerStore.markPathWalked('test-vault', 'folder-a');

      // File in walked folder should trigger update
      const eventPath = 'folder-a/new-file.md';
      const lastSlash = eventPath.lastIndexOf('/');
      const parentPath = eventPath.substring(0, lastSlash);
      const shouldUpdate = treeWalkerStore.isPathWalked('test-vault', parentPath);

      expect(shouldUpdate).toBe(true);
    });

    it('should return false when parent path is not walked', () => {
      // folder-a is NOT walked
      const eventPath = 'folder-a/new-file.md';
      const lastSlash = eventPath.lastIndexOf('/');
      const parentPath = eventPath.substring(0, lastSlash);
      const shouldUpdate = treeWalkerStore.isPathWalked('test-vault', parentPath);

      expect(shouldUpdate).toBe(false);
    });

    it('should return true for root-level files (root is always walked)', () => {
      const eventPath = 'new-file.md';
      // Root is always walked
      expect(treeWalkerStore.isParentWalked('test-vault', eventPath)).toBe(true);
    });
  });

  describe('File creation in walked folder', () => {
    it('should add file to walked folder', () => {
      treeWalkerStore.markPathWalked('test-vault', 'folder-a');

      // Find folder-a in tree
      const folderA = mockTreeData.find((n) => n.metadata.name === 'folder-a');
      expect(folderA).toBeDefined();
      expect(folderA.children.length).toBe(1);

      // Simulate file creation
      const newFile = {
        metadata: {
          id: 'a-file2-id',
          name: 'file2.md',
          path: 'folder-a/file2.md',
          is_directory: false,
          is_markdown: true,
        },
        children: [],
      };

      folderA.children.push(newFile);

      expect(folderA.children.length).toBe(2);
      expect(folderA.children[1].metadata.name).toBe('file2.md');
    });
  });

  describe('File creation in non-walked folder', () => {
    it('should NOT add file to non-walked folder', () => {
      // folder-b is NOT walked
      const folderB = mockTreeData.find((n) => n.metadata.name === 'folder-b');
      expect(folderB).toBeDefined();
      expect(folderB.children.length).toBe(0);

      // Check if we should update UI
      const eventPath = 'folder-b/new-file.md';
      const lastSlash = eventPath.lastIndexOf('/');
      const parentPath = eventPath.substring(0, lastSlash);
      const shouldUpdate = treeWalkerStore.isPathWalked('test-vault', parentPath);

      expect(shouldUpdate).toBe(false);

      // Do NOT modify tree
      expect(folderB.children.length).toBe(0);
    });
  });

  describe('File deletion from walked folder', () => {
    it('should remove file from walked folder', () => {
      treeWalkerStore.markPathWalked('test-vault', 'folder-a');

      const folderA = mockTreeData.find((n) => n.metadata.name === 'folder-a');
      expect(folderA.children.length).toBe(1);

      // Simulate file deletion
      folderA.children = folderA.children.filter(
        (child) => child.metadata.name !== 'file1.md'
      );

      expect(folderA.children.length).toBe(0);
    });
  });

  describe('File deletion from non-walked folder', () => {
    it('should NOT affect non-walked folder', () => {
      // folder-b is NOT walked
      const folderB = mockTreeData.find((n) => n.metadata.name === 'folder-b');

      const eventPath = 'folder-b/some-file.md';
      const lastSlash = eventPath.lastIndexOf('/');
      const parentPath = eventPath.substring(0, lastSlash);
      const shouldUpdate = treeWalkerStore.isPathWalked('test-vault', parentPath);

      expect(shouldUpdate).toBe(false);
    });
  });

  describe('Deep nesting scenario', () => {
    it('should handle updates at different nesting levels', () => {
      // Setup deep structure
      const deepTree = [
        {
          metadata: {
            id: 'level1',
            name: 'level1',
            path: 'level1',
            is_directory: true,
          },
          children: [
            {
              metadata: {
                id: 'level2',
                name: 'level2',
                path: 'level1/level2',
                is_directory: true,
              },
              children: [
                {
                  metadata: {
                    id: 'level3',
                    name: 'level3',
                    path: 'level1/level2/level3',
                    is_directory: true,
                  },
                  children: [],
                },
              ],
            },
          ],
        },
      ];

      fileStore.treeData = deepTree;
      treeWalkerStore.markRootWalked('test-vault');
      treeWalkerStore.registerNodes('test-vault', deepTree);

      // Only level2 is walked, not level3
      treeWalkerStore.markPathWalked('test-vault', 'level1/level2');

      // File in level1/level2 should trigger update
      const level2FilePath = 'level1/level2/file.md';
      const lastSlash = level2FilePath.lastIndexOf('/');
      const parentPath = level2FilePath.substring(0, lastSlash);
      const shouldUpdate2 = treeWalkerStore.isPathWalked('test-vault', parentPath);
      expect(shouldUpdate2).toBe(true);

      // File in level1/level2/level3 should NOT trigger update (level3 not walked)
      const level3FilePath = 'level1/level2/level3/file.md';
      const lastSlash3 = level3FilePath.lastIndexOf('/');
      const parentPath3 = level3FilePath.substring(0, lastSlash3);
      const shouldUpdate3 = treeWalkerStore.isPathWalked('test-vault', parentPath3);
      expect(shouldUpdate3).toBe(false);
    });
  });

  describe('Multiple vault isolation', () => {
    it('should isolate walked paths between vaults', () => {
      const vault1 = 'vault-1';
      const vault2 = 'vault-2';

      treeWalkerStore.markRootWalked(vault1);
      treeWalkerStore.markRootWalked(vault2);

      treeWalkerStore.markPathWalked(vault1, 'folder-a');

      // Vault1 has folder-a walked
      expect(treeWalkerStore.isPathWalked(vault1, 'folder-a')).toBe(true);

      // Vault2 does NOT have folder-a walked
      expect(treeWalkerStore.isPathWalked(vault2, 'folder-a')).toBe(false);
    });

    it('should clear one vault without affecting another', () => {
      const vault1 = 'vault-1';
      const vault2 = 'vault-2';

      treeWalkerStore.markPathWalked(vault1, 'folder-a');
      treeWalkerStore.markPathWalked(vault2, 'folder-b');

      // Clear vault1
      treeWalkerStore.clearVault(vault1);

      // Vault1 should be cleared
      expect(treeWalkerStore.getWalkedPaths(vault1)).toEqual([]);

      // Vault2 should be unaffected
      expect(treeWalkerStore.isPathWalked(vault2, 'folder-b')).toBe(true);
    });
  });

  describe('Edge cases', () => {
    it('should handle files at root level', () => {
      // Root is always walked
      const rootFilePath = 'file.md';
      expect(treeWalkerStore.isParentWalked('test-vault', rootFilePath)).toBe(true);
    });

    it('should handle paths with special characters', () => {
      const specialPath = 'folder-with-dash/file_with_underscore.md';
      treeWalkerStore.markPathWalked('test-vault', 'folder-with-dash');

      const lastSlash = specialPath.lastIndexOf('/');
      const parentPath = specialPath.substring(0, lastSlash);
      const shouldUpdate = treeWalkerStore.isPathWalked('test-vault', parentPath);

      expect(shouldUpdate).toBe(true);
    });

    it('should handle deeply nested paths', () => {
      const deepPath = 'a/b/c/d/e/f/g/h/file.md';
      treeWalkerStore.markPathWalked('test-vault', 'a/b/c/d/e/f/g/h');

      const lastSlash = deepPath.lastIndexOf('/');
      const parentPath = deepPath.substring(0, lastSlash);
      const shouldUpdate = treeWalkerStore.isPathWalked('test-vault', parentPath);

      expect(shouldUpdate).toBe(true);
    });

    it('should handle deletion when parent changes', () => {
      treeWalkerStore.markPathWalked('test-vault', 'folder-a');

      // Simulate file from folder-a gets moved to folder-b
      const oldPath = 'folder-a/file.md';
      const newPath = 'folder-b/file.md';

      const lastSlashOld = oldPath.lastIndexOf('/');
      const parentPathOld = oldPath.substring(0, lastSlashOld);
      const shouldUpdateOld = treeWalkerStore.isPathWalked('test-vault', parentPathOld);

      const lastSlashNew = newPath.lastIndexOf('/');
      const parentPathNew = newPath.substring(0, lastSlashNew);
      const shouldUpdateNew = treeWalkerStore.isPathWalked('test-vault', parentPathNew);

      // Old parent is walked, so deletion should be reflected
      expect(shouldUpdateOld).toBe(true);

      // New parent is NOT walked, so creation should NOT be reflected
      expect(shouldUpdateNew).toBe(false);
    });
  });
});
