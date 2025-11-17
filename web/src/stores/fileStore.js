import { defineStore } from 'pinia';
import fileService from '../services/fileService';

export const useFileStore = defineStore('file', {
  state: () => ({
    vaultId: null,
    currentPath: '',
    treeData: [],
    childrenData: [],
    metadata: null,
    loading: false,
    error: null,
  }),
  actions: {
    setVaultId(id) {
      this.vaultId = id;
    },
    setCurrentPath(path) {
      this.currentPath = path;
    },
    async fetchTree(vaultId, path = '') {
      this.loading = true;
      this.error = null;
      try {
        console.log('Fetching tree for vault:', vaultId, 'path:', path);
        const response = await fileService.getTree(vaultId, path);
        console.log('Tree response:', response);
        this.treeData = response || [];
      } catch (err) {
        console.error('Error fetching tree:', err);
        this.error = err.message || 'Failed to fetch tree';
      } finally {
        this.loading = false;
      }
    },
    async fetchChildren(vaultId, path = '') {
      this.loading = true;
      this.error = null;
      try {
        console.log('Fetching children for vault:', vaultId, 'path:', path);
        const response = await fileService.getChildren(vaultId, path);
        console.log('Children response:', response);
        this.childrenData = response || [];
      } catch (err) {
        console.error('Error fetching children:', err);
        this.error = err.message || 'Failed to fetch children';
      } finally {
        this.loading = false;
      }
    },
    async fetchMetadata(vaultId, path) {
      this.loading = true;
      this.error = null;
      try {
        const response = await fileService.getMetadata(vaultId, path);
        this.metadata = response.data;
      } catch (err) {
        this.error = err.message;
      } finally {
        this.loading = false;
      }
    },
    async refreshTree(vaultId, path = '') {
      this.loading = true;
      this.error = null;
      try {
        await fileService.refreshTree(vaultId, path);
        // After refreshing, re-fetch the tree or children to update the UI
        if (this.currentPath === path) {
          await this.fetchChildren(vaultId, path);
        } else {
          await this.fetchTree(vaultId, path);
        }
      } catch (err) {
        this.error = err.message;
      } finally {
        this.loading = false;
      }
    },
  },
});
