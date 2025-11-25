import axios from 'axios';

const BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api/v1') + '/files';

export default {
  /**
   * Fetches the full recursive directory tree for a given vault.
   * Returns all files and folders with their complete hierarchy.
   * @param {string} vaultId - The ID of the vault.
   * @returns {Promise<Array>} - A promise that resolves to an array of tree nodes.
   */
  getTree(vaultId) {
    return axios.get(`${BASE_URL}/tree/${vaultId}`)
      .then(response => {
        // API returns: { data: { vault_id, nodes, count } }
        // nodes is the full recursive tree structure
        if (response.data && response.data.data && response.data.data.nodes) {
          return response.data.data.nodes;
        }
        return [];
      });
  },

  /**
   * Fetches the direct children of a directory for a given vault and path.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} [path=''] - The directory path (empty for root).
   * @returns {Promise<Array<object>>} - A promise that resolves to an array of child nodes.
   */
  getChildren(vaultId, path = '') {
    return axios.get(`${BASE_URL}/children/${vaultId}`, { params: { path } })
      .then(response => {
        // API returns: { data: { path, children, count } }
        if (response.data && response.data.data && response.data.data.children) {
          return response.data.data.children;
        }
        return [];
      });
  },

  /**
   * Fetches the direct children of a directory by node ID.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} nodeId - The node ID (parent directory ID).
   * @returns {Promise<Array<object>>} - A promise that resolves to an array of child nodes.
   */
  getChildrenByID(vaultId, nodeId) {
    return axios.get(`${BASE_URL}/children-by-id/${vaultId}/${nodeId}`)
      .then(response => {
        // API returns: { data: { id, children, count } }
        if (response.data && response.data.data && response.data.data.children) {
          return response.data.data.children;
        }
        return [];
      });
  },

  /**
   * Fetches metadata for a file or directory.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} path - The file or directory path.
   * @returns {Promise<object>} - A promise that resolves to the metadata.
   */
  getMetadata(vaultId, path) {
    return axios.get(`${BASE_URL}/meta/${vaultId}`, { params: { path } })
      .then(response => {
        // API returns: { data: { path, name, type, ... } }
        if (response.data && response.data.data) {
          return response.data.data;
        }
        return null;
      });
  },

  /**
   * Manually refreshes the cached directory tree for a path.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} [path=''] - The directory path (empty for root).
   * @returns {Promise<object>} - A promise that resolves to a success message.
   */
  refreshTree(vaultId, path = '') {
    return axios.post(`${BASE_URL}/refresh/${vaultId}`, null, { params: { path } });
  },
  /**
   * Fetches the content of a file by its node ID.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} fileId - The file node ID from the database.
   * @returns {Promise<Object>} - A promise that resolves to an object with content, path, id, and name.
   */
  getFileContent(vaultId, fileId) {
    // Use the by-id endpoint: /api/v1/files/by-id/{vault}/{id}
    // SECURITY: This endpoint only accepts ID, never paths. Path is returned read-only for UI navigation.
    const url = `${BASE_URL}/by-id/${vaultId}/${fileId}`;
    return axios.get(url, {
      responseType: 'json'  // Expect JSON response with content and metadata
    })
      .then(response => {
        // API returns { data: { content, path, id, name } }
        // path is READ-ONLY and used for UI navigation only
        if (response.data && response.data.data) {
          return response.data.data;
        }
        return null;
      });
  },
};
